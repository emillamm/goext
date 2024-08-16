package pgtest


import (
	"testing"
	"fmt"
	"math/rand"
	"database/sql"
	"github.com/emillamm/envx"
	"github.com/emillamm/goext/pg"
	"github.com/emillamm/pgmigrate"
	"github.com/jackc/pgx/v5"
	_ "github.com/jackc/pgx/v5/stdlib"
	"context"
	"sync"
)

type SessionManager struct {
	Host string
	Port int
	db *sql.DB
	sessions map[*EphemeralSession]sync.Once
}

type EphemeralSession struct {
	Params pg.ConnectionParams
	conn *pgx.Conn
	closeOnce sync.Once
}

func (e *EphemeralSession) Connect() (*pgx.Conn, error) {
	if e.conn == nil {
		// Create pgx connection for using in the test
		conn, err := pgx.Connect(context.Background(), e.Params.ConnectionString())
		if err != nil {
			return nil, fmt.Errorf("failed to open pgx connection: %w", err)
		}
		e.conn = conn
	}
	return e.conn, nil
}

func (e *EphemeralSession) Close() {
	if e.conn != nil {
		e.closeOnce.Do(func () {
			e.conn.Close(context.Background())
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
	sm :=  &SessionManager{
		Host: connParams.Host,
		Port: connParams.Port,
		db: db,
		sessions: make(map[*EphemeralSession]sync.Once),
	}
	return sm, nil
}

func (sm *SessionManager) Close() {
	// close and cleanup all ephemeral sessions
	if sm.sessions != nil {
		for session, _ := range sm.sessions {
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
		Host: sm.Host,
		Port: sm.Port,
		Database: entry,
		User: entry,
		Pass: password,
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
	// Convention: migrations folder always exists one level up from db tests.
	provider := pgmigrate.FileMigrationProvider{Directory: "../migrations"}
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
	var err envx.Errors

	// These defaults work with the offical postgres Docker image when running:
	// docker run --rm --name postgres -p 5432:5432 -e POSTGRES_PASSWORD=postgres postgres
	host := env.Getenv("POSTGRES_HOST", envx.Default("localhost"))
	port := env.AsInt().Getenv("POSTGRES_PORT", envx.Default(5432), envx.Observe[int](&err))
	database := env.Getenv("POSTGRES_DATABASE", envx.Default("postgres"))
	user := env.Getenv("POSTGRES_USER", envx.Default("postgres"))
	pass := env.Getenv("POSTGRES_PASS", envx.Default("postgres"))

	params := pg.ConnectionParams{
		Host: host,
		Port: port,
		Database: database,
		User: user,
		Pass: pass,
	}

	return params, err.Error()
}

func (sm *SessionManager) Run(t *testing.T, msg string, testFn func(*testing.T, *pgx.Conn))  {
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

