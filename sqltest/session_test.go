package sqltest

import (
	"testing"
	"database/sql"
)

func TestSession(t *testing.T) {

	t.Run("start new postgres session and create ephemeral session", func(t *testing.T) {
		session, err := NewSqlSession("postgres")
		defer session.Close()

		if err != nil {
			t.Errorf("unable to create parent session: %s", err)
			return
		}

		if err = session.Db.Ping(); err != nil {
			t.Errorf("unable to ping parent session: %s", err)
			return
		}

		session.EphemeralSession(t, func(db *sql.DB) {
			if err = db.Ping(); err != nil {
				t.Errorf("unable to ping ephemeral session: %s", err)
				return
			}

			var brand string
			if err := db.QueryRow("select brand from cars;").Scan(&brand); err != nil {
				t.Errorf("unable to read from ephemeral table: %v", err)
			}
			if brand != "volvo" {
				t.Errorf("invalid value. Got '%s', wanted 'volvo'", brand)
			}
		})
	})
}

