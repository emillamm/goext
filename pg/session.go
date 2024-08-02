package pg

import (
	"fmt"
	"github.com/emillamm/envx"
	"strconv"
)

type ConnectionParams struct {
	Host string
	Port int
	Database string
	User string
	Pass string
}

func (c ConnectionParams) ConnectionString() string {
	return fmt.Sprintf("user=%s password=%s host=%s port=%d database=%s sslmode=disable", c.User, c.Pass, c.Host, c.Port, c.Database)
}

func LoadConnectionParams(env envx.EnvX) (ConnectionParams, error) {
	var err envx.Errors

	host := env.Getenv("POSTGRES_HOST", envx.Default("localhost"))
	port := env.AsInt().Getenv("POSTGRES_PORT", envx.Default[int](5432), envx.Observe[int](&err))
	database := env.Getenv("POSTGRES_DATABASE", envx.Observe[string](&err))
	user := env.Getenv("POSTGRES_USER", envx.Observe[string](&err))
	pass := env.Getenv("POSTGRES_PASS", envx.Observe[string](&err))

	params := ConnectionParams{
		Host: host,
		Port: port,
		Database: database,
		User: user,
		Pass: pass,
	}

	return params, err.Error()
}

func (c ConnectionParams) EnvOverwrite(env envx.EnvX) envx.EnvX {
	return func (name string) string {
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

