package sqltest


import (
	"testing"
	"fmt"
	"math/rand"
	"database/sql"
	"strconv"
	"strings"
	"github.com/emillamm/goext/env"
	"github.com/emillamm/pgmigrate"
)

type SqlSession struct {
	Host string
	Port int
	Driver string
	Db *sql.DB
}

func NewSqlSession(driver string) (session *SqlSession, err error) {
	connParams, err := DefaultConnectionParams(driver)
	if err != nil {
		return
	}
	db, err := openConnection(connParams.ConnectionString(), driver)
	if err == nil {
		session = &SqlSession{
			Host: connParams.Host,
			Port: connParams.Port,
			Driver: driver,
			Db: db,
		}
	}
	return 
}

func (parentSession *SqlSession) Close() {
	parentSession.Db.Close()
}

func (parentSession *SqlSession) EphemeralSession(
	t testing.TB,
	block func(db *sql.DB),
) {
	t.Helper()

	user := randomUser()
	password := "test"

	createRoleQ := fmt.Sprintf("create role %s with login password '%s';", user, password)
	if _, err := parentSession.Db.Exec(createRoleQ); err != nil {
		t.Errorf("failed to create role %s: %s", user, err)
		return
	}

	// Create user and database from the same name
	createDbQ := fmt.Sprintf("create database %s owner %s;", user, user)
	if _, err := parentSession.Db.Exec(createDbQ); err != nil {
		t.Errorf("failed to create database %s: %s", user, err)
		return
	}

	// Set up connection
	connStr := fmt.Sprintf("user=%s password=%s host=%s port=%d sslmode=disable", user, password, parentSession.Host, parentSession.Port)
	db, err := openConnection(connStr, parentSession.Driver)
	if err != nil {
		t.Errorf("failed to open connection %s", err)
		return
	}

	// Perform migrations
	// Convention: migrations folder always exists one level up from db tests.
	provider := pgmigrate.FileMigrationProvider{Directory: "../migrations"}
	migrations := provider.GetMigrations()
	_, err = pgmigrate.RunMigrations(db, migrations, 0)
	if err != nil {
		t.Errorf("unable to complete some or all migrations: %v", err)
		return
	}

	defer func() {
		db.Close()
		dropDbQ := fmt.Sprintf("drop database %s;", user)
		if _, err = parentSession.Db.Exec(dropDbQ); err != nil {
			t.Errorf("failed to drop database %s: %s", user, err)
			return
		}
		dropRoleQ := fmt.Sprintf("drop role %s;", user)
		if _, err = parentSession.Db.Exec(dropRoleQ); err != nil {
			t.Errorf("failed to drop role %s: %s", user, err)
			return
		}
	}()

	block(db)
}

type ConnectionParams struct {
	Host string
	Port int
	User string
	Pass string
}

func (c ConnectionParams) ConnectionString() string {
	return fmt.Sprintf("user=%s password=%s host=%s port=%d sslmode=disable", c.User, c.Pass, c.Host, c.Port)
}

func DefaultConnectionParams(driver string) (params ConnectionParams, err error) {
	var host, portStr, user, pass string
	switch {
	case strings.EqualFold(driver, "postgres"):
		host = env.GetenvWithDefault("POSTGRES_HOST", "localhost")
		portStr = env.GetenvWithDefault("POSTGRES_PORT", "5432")
		user = env.GetenvWithDefault("POSTGRES_USER", "postgres")
		pass = env.GetenvWithDefault("POSTGRES_PASSWORD", "postgres")
	default:
		// As long as we rely on pgmigrate to do migrations, other drivers aren't supported.
		// One possible solution is to turn pgmigrate into sqlmigrate which requires some refactoring.
		err = fmt.Errorf("invalid driver %s: postgres is the only supported driver at this time", driver)
		return
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		err = fmt.Errorf("invalid PORT %s", portStr)
		return
	}
	params = ConnectionParams{
		Host: host,
		Port: port,
		User: user,
		Pass: pass,
	}
	return
}

func openConnection(connStr string, driver string) (db *sql.DB, err error) {
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

