package pgtest

import (
	"fmt"
	"testing"
	"github.com/jackc/pgx/v5"
	"context"
	"os"
)

func TestSession(t *testing.T) {

	ctx := context.Background()
	sm, err := NewSessionManager(os.Getenv)
	if err != nil {
		t.Fatal(err)
	}
	defer sm.Close()

	t.Run("Ping SessionManager conn", func(t *testing.T) {
		if err := sm.db.Ping(); err != nil {
			t.Fatalf("unable to ping SessionManager conn: %s", err)
		}
	})

	t.Run("Create new ephemeral database and user", func(t *testing.T) {
		es, err := sm.NewEphemeralSession()
		if err != nil {
			t.Fatal(err)
		}

		conn, err := es.Connect()
		if err != nil {
			t.Fatal(err)
		}

		if err := verifyTableExistence(ctx, es); err != nil {
			t.Fatal(err)
		}

		es.Close()
		if !conn.IsClosed() {
			t.Errorf("connection not closed")
		}

		sm.Cleanup(es)
		if err := verifyTableExistence(ctx, es); err == nil {
			t.Fatalf("db and table should not exist anymore: %#v", es.Params)
		}
	})

	sm.Run(t, "Run tests with an ephemeral postgres database", func(t *testing.T, conn *pgx.Conn) {

		if err := conn.Ping(ctx); err != nil {
			t.Errorf("unable to ping session: %s", err)
			return
		}

		if err := verifyTableExistenceWithConn(ctx, conn); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("Close SessionManager and cleanup underlying dbs/users", func(t *testing.T) {
		es, err := sm.NewEphemeralSession()
		if err != nil {
			t.Fatal(err)
		}

		_, err = es.Connect()
		if err != nil {
			t.Fatal(err)
		}

		if err := verifyTableExistence(ctx, es); err != nil {
			t.Fatal(err)
		}

		if len(sm.sessions) == 0 {
			t.Fatalf("number of sessions should be greater than 0")
		}

		sm.Close()

		if err := verifyTableExistence(ctx, es); err == nil {
			t.Fatalf("db and table should not exist anymore: %#v", es.Params)
		}

		if len(sm.sessions) != 0 {
			t.Fatalf("number of sessions should be 0, was %d", len(sm.sessions))
		}
	})
}

func verifyTableExistence(ctx context.Context, session *EphemeralSession) error {
	conn, err := pgx.Connect(context.Background(), session.Params.ConnectionString())
	if err != nil {
		return fmt.Errorf("failed to open pgx connection: %w", err)
	}
	defer conn.Close(context.Background())
	return verifyTableExistenceWithConn(ctx, conn)
}

func verifyTableExistenceWithConn(ctx context.Context, conn *pgx.Conn) error {
	var brand string
	if err := conn.QueryRow(ctx, "select brand from cars;").Scan(&brand); err != nil {
		return fmt.Errorf("unable to read from ephemeral table: %w", err)
	}
	if brand != "volvo" {
		return fmt.Errorf("invalid value. Got '%s', wanted 'volvo'", brand)
	}
	return nil
}

