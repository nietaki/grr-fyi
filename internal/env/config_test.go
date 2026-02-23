package env

import "testing"

func TestDefaultValues(t *testing.T) {
	cfg := Load()

	if cfg.ServerPort != "50666" {
		t.Errorf("Expected default ServerPort to be '50666', got '%s'", cfg.ServerPort)
	}
}

func TestSettingCustomPort(t *testing.T) {
	// Set the environment variable for testing
	t.Setenv("SERVER_PORT", "8080")

	cfg := Load()

	if cfg.ServerPort != "8080" {
		t.Errorf("Expected ServerPort to be '8080', got '%s'", cfg.ServerPort)
	}
}
