package pg

import (
	"fmt"
	"strconv"

	"github.com/emillamm/envx"
)

type ConnectionParams struct {
	Host     string
	Port     int
	Database string
	User     string
	Pass     string
}

func (c ConnectionParams) ConnectionString() string {
	return fmt.Sprintf("user=%s password=%s host=%s port=%d database=%s sslmode=disable", c.User, c.Pass, c.Host, c.Port, c.Database)
}

func LoadConnectionParams(env envx.EnvX) (ConnectionParams, error) {
	checks := envx.NewChecks()

	host := envx.Check(env.String("POSTGRES_HOST").Default("localhost"))(checks)
	port := envx.Check(env.Int("POSTGRES_PORT").Default(5432))(checks)
	database := envx.Check(env.String("POSTGRES_DATABASE").Value())(checks)
	user := envx.Check(env.String("POSTGRES_USER").Default(""))(checks)
	pass := envx.Check(env.String("POSTGRES_PASS").Default(""))(checks)

	params := ConnectionParams{
		Host:     host,
		Port:     port,
		Database: database,
		User:     user,
		Pass:     pass,
	}

	return params, checks.Err()
}

func (c ConnectionParams) EnvOverwrite(env envx.EnvX) envx.EnvX {
	return func(name string) string {
		switch name {
		case "POSTGRES_HOST":
			return c.Host
		case "POSTGRES_PORT":
			return strconv.Itoa(c.Port)
		case "POSTGRES_DATABASE":
			return c.Database
		case "POSTGRES_USER":
			return c.User
		case "POSTGRES_PASS":
			return c.Pass
		default:
			return env(name)
		}
	}
}
