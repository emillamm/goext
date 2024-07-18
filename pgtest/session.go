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
)

type SqlSession struct {
	Host string
	Port int
	t *testing.T
	db *sql.DB
}

func (s *SqlSession) Ready() bool {
	if s.db != nil {
		return true
	}
	return false
}

func (s *SqlSession) Run(msg string, testFn func(*testing.T, *pgx.Conn))  {
	if !s.Ready() {
		return
	}
	s.t.Run(msg, func(t *testing.T) {
		s.EphemeralSession(t, func(conn *pgx.Conn) {
			testFn(t, conn)
		})

	})
}

func NewSession(t *testing.T, env envx.EnvX) *SqlSession {
	t.Helper()
	connParams, err := LoadTestConnectionParams(env)
	if err != nil {
		t.Errorf("unable to create connection params: %#v", err)
		return &SqlSession{}
	}
	println(connParams.ConnectionString())
	db, err := openLegacyConnection(connParams.ConnectionString(), "pgx")
	if err != nil {
		t.Errorf("unable to open legacy connection: %v", err)
		return &SqlSession{}
	}
	return &SqlSession{
		Host: connParams.Host,
		Port: connParams.Port,
		t: t,
		db: db,
	}
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


func (parentSession *SqlSession) Close() {
	if parentSession.db != nil {
		parentSession.db.Close()
	}
}

func (parentSession *SqlSession) EphemeralSession(
	t testing.TB,
	block func(conn *pgx.Conn),
) {
	t.Helper()

	user := randomUser()
	password := "test"

	createRoleQ := fmt.Sprintf("create role %s with login password '%s';", user, password)
	if _, err := parentSession.db.Exec(createRoleQ); err != nil {
		t.Errorf("failed to create role %s: %s", user, err)
		return
	}

	// Create user and database from the same name
	createDbQ := fmt.Sprintf("create database %s owner %s;", user, user)
	if _, err := parentSession.db.Exec(createDbQ); err != nil {
		t.Errorf("failed to create database %s: %s", user, err)
		return
	}

	defer func() {
		dropDbQ := fmt.Sprintf("drop database %s;", user)
		if _, err := parentSession.db.Exec(dropDbQ); err != nil {
			t.Errorf("failed to drop database %s: %s", user, err)
			return
		}
		dropRoleQ := fmt.Sprintf("drop role %s;", user)
		if _, err := parentSession.db.Exec(dropRoleQ); err != nil {
			t.Errorf("failed to drop role %s: %s", user, err)
			return
		}
	}()

	connStr := fmt.Sprintf("user=%s password=%s host=%s port=%d database=%s sslmode=disable",
		user, password, parentSession.Host, parentSession.Port, user)

	// Set up legacy connection for running migrations
	migrationDb, err := openLegacyConnection(connStr, "pgx")
	if err != nil {
		t.Errorf("failed to open connection for migrations %s", err)
		return
	}
	defer migrationDb.Close()

	// Perform migrations
	// Convention: migrations folder always exists one level up from db tests.
	provider := pgmigrate.FileMigrationProvider{Directory: "../migrations"}
	migrations := provider.GetMigrations()
	_, err = pgmigrate.RunMigrations(migrationDb, migrations, 0)
	if err != nil {
		t.Errorf("unable to complete some or all migrations: %v", err)
		return
	}

	// Create pgx connection for using in the test
	conn, err := pgx.Connect(context.Background(), connStr)
	if err != nil {
		t.Errorf("failed to open pgx connection %s", err)
		return
	}
	defer conn.Close(context.Background())

	block(conn)
}

func openLegacyConnection(connStr string, driver string) (db *sql.DB, err error) {
	db, err = sql.Open(driver, connStr)
	if db != nil {
		err = db.Ping()
	}
	return
}

// Generates user/DB name in the form of "test_[a-z]7" e.g. test_hqbrluz
func randomUser() string {
	chars := "abcdefghijklmnopqrstuvwxyz"
	length := 7
	b := make([]byte, length)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return fmt.Sprintf("test_%s", string(b))
}

