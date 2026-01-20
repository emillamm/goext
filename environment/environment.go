package environment

import (
	"strings"

	"github.com/emillamm/envx"
)

type Environment int

const (
	Prod Environment = iota
	Local
)

func (e Environment) String() string {
	switch e {
	case Prod:
		return "prod"
	case Local:
		return "local"
	default:
		return "unknown"
	}
}

func LoadEnvironment(env envx.EnvX) Environment {
	value, err := env.String("ENVIRONMENT").Value()
	if err != nil {
		panic("ENVIRONMENT variable is required")
	}

	switch strings.ToLower(value) {
	case "prod":
		return Prod
	case "local":
		return Local
	default:
		panic("ENVIRONMENT must be 'prod' or 'local', got: " + value)
	}
}
