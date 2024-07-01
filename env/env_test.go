package env

import (
	"testing"
	"os"
)

func TestEnv(t *testing.T) {
	t.Run("Getenv should return an env var if it exists", func(t *testing.T) {
		varName := "TEST_VAR1"
		if Getenv(varName) != "" {
			t.Errorf("%s should not exist", varName)
		}
		os.Setenv(varName, "abc")
		if Getenv(varName) != "abc" {
			t.Errorf("%s should be abc", varName)
		}
		os.Unsetenv(varName)
	})
	t.Run("Getenv should use <VAR>_KEY as a pointer to the name of the actual variable", func(t *testing.T) {
		varName := "TEST_VAR2"
		os.Setenv("TEST_VAR2_KEY", "TEST_VAR3")
		if Getenv(varName) != "" {
			t.Errorf("%s should not exist", varName)
		}
		os.Setenv("TEST_VAR3", "abc")
		if Getenv(varName) != "abc" {
			t.Errorf("%s should be abc", varName)
		}
		os.Unsetenv(varName)
		os.Unsetenv("TEST_VAR3")
	})
}

