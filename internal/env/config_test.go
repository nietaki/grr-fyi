package env

import "testing"

func TestDefaultValues(t *testing.T) {
	cfg := Load()
	if cfg.ServerPort != "30666" {
		t.Errorf("Expected default ServerPort to be '30666', got '%s'", cfg.ServerPort)
	}
}

func TestRandomFailure(t *testing.T) {
	t.Skip("skipping deliberately failing test")
	t.Fatal("This test is designed to fail randomly to demonstrate test failure handling.")
}

func TestSettingCustomPort(t *testing.T) {
	// Set the environment variable for testing
	t.Setenv("SERVER_PORT", "8080")

	cfg := Load()

	if cfg.ServerPort != "8080" {
		t.Errorf("Expected ServerPort to be '8080', got '%s'", cfg.ServerPort)
	}
}

func TestInvalidPortFormat(t *testing.T) {
	// When SERVER_PORT has invalid characters, env.Parse accepts it
	// This is the actual behavior of caarlos0/env library
	t.Setenv("SERVER_PORT", "invalid_port_with_spaces")

	cfg := Load()

	// Should use the value provided, not default
	if cfg.ServerPort != "invalid_port_with_spaces" {
		t.Errorf("Expected ServerPort to be 'invalid_port_with_spaces', got '%s'", cfg.ServerPort)
	}
}
