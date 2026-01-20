package environment

import (
	"testing"

	"github.com/emillamm/envx"
)

func mockEnv(vars map[string]string) envx.EnvX {
	return func(name string) string {
		return vars[name]
	}
}

func TestLoadEnvironment(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected Environment
	}{
		{"prod lowercase", "prod", Prod},
		{"prod uppercase", "PROD", Prod},
		{"prod mixed case", "Prod", Prod},
		{"local lowercase", "local", Local},
		{"local uppercase", "LOCAL", Local},
		{"local mixed case", "Local", Local},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := mockEnv(map[string]string{"ENVIRONMENT": tt.value})
			result := LoadEnvironment(env)
			if result != tt.expected {
				t.Errorf("LoadEnvironment() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestLoadEnvironmentPanicsOnMissing(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("LoadEnvironment() did not panic on missing value")
		}
	}()

	env := mockEnv(map[string]string{})
	LoadEnvironment(env)
}

func TestLoadEnvironmentPanicsOnInvalid(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("LoadEnvironment() did not panic on invalid value")
		}
	}()

	env := mockEnv(map[string]string{"ENVIRONMENT": "invalid"})
	LoadEnvironment(env)
}
