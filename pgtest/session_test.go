package pgtest

import (
	"testing"
	"github.com/jackc/pgx/v5"
	"context"
	"os"
)

func TestSession(t *testing.T) {

	ctx := context.Background()
	session := NewSession(t, os.Getenv)
	defer session.Close()

	session.Run("Run tests with an ephemeral postgres database", func(t *testing.T, conn *pgx.Conn) {

		if err := session.db.Ping(); err != nil {
			t.Errorf("unable to ping parent session: %s", err)
			return
		}

		if err := conn.Ping(ctx); err != nil {
			t.Errorf("unable to ping session: %s", err)
			return
		}

		var brand string
		if err := conn.QueryRow(ctx, "select brand from cars;").Scan(&brand); err != nil {
			t.Errorf("unable to read from ephemeral table: %v", err)
		}
		if brand != "volvo" {
			t.Errorf("invalid value. Got '%s', wanted 'volvo'", brand)
		}
	})
}

