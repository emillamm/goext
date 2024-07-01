package env

import (
	"os"
	"log"
)

func Getenv(name string) string {
	key := os.Getenv(name + "_KEY")
	if key == "" {
		key = name
	}
	return os.Getenv(key)
}

func GetenvOrFatal(name string) string {
	v := Getenv(name)
	if v == "" {
		log.Fatalf("%s cannot be empty", name)
	}
	return v
}

func GetenvWithDefault(name string, defaultValue string) string {
	value := Getenv(name)
	if value == "" {
		value = defaultValue
	}
	return value
}

