package pgtest

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"sync"
	"testing"

	"github.com/emillamm/envx"
	"github.com/emillamm/goext/pg"
	"github.com/emillamm/pgmigrate"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type SessionManager struct {
	Host         string
	Port         int
	MigrationDir string
	db           *sql.DB
	sessions     map[*EphemeralSession]sync.Once
}

type EphemeralSession struct {
	Params    pg.ConnectionParams
	conn      *pgxpool.Pool
	closeOnce sync.Once
}

func (e *EphemeralSession) Connect() (*pgxpool.Pool, error) {
	if e.conn == nil {
		// Create pgx connection for using in the test
		conn, err := pgxpool.New(context.Background(), e.Params.ConnectionString())
		if err != nil {
			return nil, fmt.Errorf("failed to open pgx connection: %w", err)
		}
		e.conn = conn
	}
	return e.conn, nil
}

func (e *EphemeralSession) Close() {
	if e.conn != nil {
		e.closeOnce.Do(func() {
			e.conn.Close()
		})
	}
}

func (sm *SessionManager) Cleanup(session *EphemeralSession) error {
	session.Close()
	cleanupOnce, ok := sm.sessions[session]
	if !ok {
		// session doesn't exist and has likely already been cleaned up
		return nil
	}

	var cleanupErr error
	cleanupOnce.Do(func() {
		dropDbQ := fmt.Sprintf("drop database %s;", session.Params.Database)
		if _, err := sm.db.Exec(dropDbQ); err != nil {
			cleanupErr = fmt.Errorf("failed to drop database %s: %w", session.Params.Database, err)
		}
		dropRoleQ := fmt.Sprintf("drop role %s;", session.Params.User)
		if _, err := sm.db.Exec(dropRoleQ); err != nil {
			cleanupErr = fmt.Errorf("failed to drop role %s: %w", session.Params.User, err)
		}
		delete(sm.sessions, session)
	})
	return cleanupErr
}

func NewSessionManager(env envx.EnvX) (*SessionManager, error) {
	connParams, err := LoadTestConnectionParams(env)
	if err != nil {
		return nil, fmt.Errorf("unable to create connection params: %w", err)
	}
	db, err := openLegacyConnection(connParams.ConnectionString(), "pgx")
	if err != nil {
		return nil, fmt.Errorf("unable to open legacy connection: %w", err)
	}
	// Convention: migrations folder generally exists two levels up from db tests.
	migrationDir, err := env.String("MIGRATION_DIR").Value()
	if err != nil {
		return nil, fmt.Errorf("could not read variable MIGRATION_DIR: %v", err)
	}
	sm := &SessionManager{
		Host:         connParams.Host,
		Port:         connParams.Port,
		MigrationDir: migrationDir,
		db:           db,
		sessions:     make(map[*EphemeralSession]sync.Once),
	}
	return sm, nil
}

func (sm *SessionManager) Close() {
	// close and cleanup all ephemeral sessions
	if sm.sessions != nil {
		for session := range sm.sessions {
			session.Close()
			sm.Cleanup(session)
		}
	}
	if sm.db != nil {
		sm.db.Close()
	}
}

func (sm *SessionManager) NewEphemeralSession() (*EphemeralSession, error) {
	entry := randomEntry()
	password := "test"

	params := pg.ConnectionParams{
		Host:     sm.Host,
		Port:     sm.Port,
		Database: entry,
		User:     entry,
		Pass:     password,
	}

	session := &EphemeralSession{
		Params: params,
	}

	createRoleQ := fmt.Sprintf("create role %s with login password '%s';", params.User, params.Pass)
	if _, err := sm.db.Exec(createRoleQ); err != nil {
		return nil, fmt.Errorf("failed to create role %s: %w", params.User, err)
	}

	// Create user and database from the same name
	createDbQ := fmt.Sprintf("create database %s owner %s;", params.Database, params.User)
	if _, err := sm.db.Exec(createDbQ); err != nil {
		return nil, fmt.Errorf("failed to create database %s: %w", params.Database, err)
	}

	// register newly created session on this SessionManager with a close handle
	var closeOnce sync.Once
	sm.sessions[session] = closeOnce

	// Set up legacy connection for running migrations
	migrationDb, err := openLegacyConnection(params.ConnectionString(), "pgx")
	if err != nil {
		sm.Cleanup(session)
		return nil, fmt.Errorf("failed to open connection for migrations: %w", err)
	}
	defer migrationDb.Close()

	// Perform migrations
	provider := pgmigrate.FileMigrationProvider{Directory: sm.MigrationDir}
	migrations := provider.GetMigrations()
	_, err = pgmigrate.RunMigrations(migrationDb, migrations, 0)
	if err != nil {
		migrationDb.Close()
		sm.Cleanup(session)
		return nil, fmt.Errorf("unable to complete some or all migrations: %w", err)
	}

	return session, nil
}

func LoadTestConnectionParams(env envx.EnvX) (pg.ConnectionParams, error) {
	checks := envx.NewChecks()

	// These defaults work with the offical postgres Docker image when running:
	// docker run --rm --name postgres -p 5432:5432 -e POSTGRES_PASSWORD=postgres postgres
	host := envx.Check(env.String("POSTGRES_HOST").Default("localhost"))(checks)
	port := envx.Check(env.Int("POSTGRES_PORT").Default(5432))(checks)
	database := envx.Check(env.String("POSTGRES_DATABASE").Default("postgres"))(checks)
	user := envx.Check(env.String("POSTGRES_USER").Default("postgres"))(checks)
	pass := envx.Check(env.String("POSTGRES_PASS").Default("postgres"))(checks)

	params := pg.ConnectionParams{
		Host:     host,
		Port:     port,
		Database: database,
		User:     user,
		Pass:     pass,
	}

	return params, checks.Err()
}

func (sm *SessionManager) Run(t *testing.T, msg string, testFn func(*testing.T, *pgxpool.Pool)) {
	t.Run(msg, func(t *testing.T) {
		// Create session
		session, err := sm.NewEphemeralSession()
		if err != nil {
			t.Errorf("failed to create EphemeralSession: %v", err)
			return
		}
		defer sm.Cleanup(session)

		// Connect to session
		conn, err := session.Connect()
		if err != nil {
			t.Errorf("failed to connect to EphemeralSession: %v", err)
		}
		defer session.Close()

		// Run test
		testFn(t, conn)
	})
}

func openLegacyConnection(connStr string, driver string) (db *sql.DB, err error) {
	db, err = sql.Open(driver, connStr)
	if db != nil {
		err = db.Ping()
	}
	return
}

// Generates user/DB name in the form of "test_[a-z]7" e.g. test_hqbrluz
func randomEntry() string {
	chars := "abcdefghijklmnopqrstuvwxyz"
	length := 7
	b := make([]byte, length)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return fmt.Sprintf("test_%s", string(b))
}
