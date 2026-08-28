package env

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type ConfigTestSuite struct {
	suite.Suite
}

func (s *ConfigTestSuite) TestDefaultValues() {
	s.T().Run("loads default ServerPort", func(t *testing.T) {
		cfg := Load()
		require.Equal(t, "30666", cfg.ServerPort)
	})
}

func (s *ConfigTestSuite) TestRandomFailure() {
	s.T().Skip("skipping deliberately failing test")
}

func (s *ConfigTestSuite) TestSettingCustomPort() {
	s.T().Run("loads custom port from environment", func(t *testing.T) {
		t.Setenv("SERVER_PORT", "8080")
		cfg := Load()
		require.Equal(t, "8080", cfg.ServerPort)
	})
}

func (s *ConfigTestSuite) TestInvalidPortFormat() {
	s.T().Run("accepts invalid port format from environment", func(t *testing.T) {
		t.Setenv("SERVER_PORT", "invalid_port_with_spaces")
		cfg := Load()
		require.Equal(t, "invalid_port_with_spaces", cfg.ServerPort)
	})
}

func (s *ConfigTestSuite) TestGet() {
	s.T().Run("returns the loaded config", func(t *testing.T) {
		t.Setenv("SERVER_PORT", "9999")
		Load()
		cfg := Get()
		require.Equal(t, "9999", cfg.ServerPort)
	})
}

func (s *ConfigTestSuite) TestReplicationEnabled() {
	s.T().Run("defaults to false", func(t *testing.T) {
		t.Setenv("REPLICATION_ENABLED", "")
		cfg := Load()
		require.False(t, cfg.ReplicationEnabled)
	})

	s.T().Run("can be set to true", func(t *testing.T) {
		t.Setenv("REPLICATION_ENABLED", "true")
		cfg := Load()
		require.True(t, cfg.ReplicationEnabled)
	})

	s.T().Run("can be set to false explicitly", func(t *testing.T) {
		t.Setenv("REPLICATION_ENABLED", "false")
		cfg := Load()
		require.False(t, cfg.ReplicationEnabled)
	})
}

func (s *ConfigTestSuite) TestAltchaDefaults() {
	s.T().Run("AltchaSecret defaults to empty", func(t *testing.T) {
		cfg := Load()
		require.Empty(t, cfg.AltchaSecret)
	})

	s.T().Run("AltchaCost defaults to 5000", func(t *testing.T) {
		cfg := Load()
		require.Equal(t, 5000, cfg.AltchaCost)
	})

	s.T().Run("AltchaExpiryMin defaults to 10", func(t *testing.T) {
		cfg := Load()
		require.Equal(t, 10, cfg.AltchaExpiryMin)
	})
}

func (s *ConfigTestSuite) TestAltchaCustomValues() {
	s.T().Run("can set AltchaSecret", func(t *testing.T) {
		t.Setenv("ALTCHA_SECRET", "my-secret")
		cfg := Load()
		require.Equal(t, "my-secret", cfg.AltchaSecret)
	})

	s.T().Run("can set AltchaCost", func(t *testing.T) {
		t.Setenv("ALTCHA_COST", "10000")
		cfg := Load()
		require.Equal(t, 10000, cfg.AltchaCost)
	})

	s.T().Run("can set AltchaExpiryMin", func(t *testing.T) {
		t.Setenv("ALTCHA_EXPIRY_MINUTES", "30")
		cfg := Load()
		require.Equal(t, 30, cfg.AltchaExpiryMin)
	})
}

func (s *ConfigTestSuite) TestLitestreamMetaPath() {
	s.T().Run("defaults to ./litestream-cache", func(t *testing.T) {
		cfg := Load()
		require.Equal(t, "./litestream-cache", cfg.LitestreamMetaPath)
	})

	s.T().Run("can be set to custom path", func(t *testing.T) {
		t.Setenv("LITESTREAM_META_PATH", "/custom/path")
		cfg := Load()
		require.Equal(t, "/custom/path", cfg.LitestreamMetaPath)
	})
}

func TestConfigTestSuite(t *testing.T) {
	suite.Run(t, new(ConfigTestSuite))
}
