package env

import (
	"github.com/mvrahden/go-test/pkg/gotest"
)

type ConfigTestSuite struct{}

func (s *ConfigTestSuite) TestDefaultValues(t *gotest.T) {
	t.It("loads default ServerPort", func(it *gotest.T) {
		cfg := Load()
		gotest.Equal(it, "30666", cfg.ServerPort)
	})
}

func (s *ConfigTestSuite) TestRandomFailure(t *gotest.T) {
	t.Skipf("skipping deliberately failing test")
}

func (s *ConfigTestSuite) TestSettingCustomPort(t *gotest.T) {
	t.It("loads custom port from environment", func(it *gotest.T) {
		it.Setenv("SERVER_PORT", "8080")
		cfg := Load()
		gotest.Equal(it, "8080", cfg.ServerPort)
	})
}

func (s *ConfigTestSuite) TestInvalidPortFormat(t *gotest.T) {
	t.It("accepts invalid port format from environment", func(it *gotest.T) {
		it.Setenv("SERVER_PORT", "invalid_port_with_spaces")
		cfg := Load()
		gotest.Equal(it, "invalid_port_with_spaces", cfg.ServerPort)
	})
}
