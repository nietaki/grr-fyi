package logging

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type LoggingTestSuite struct {
	suite.Suite
}

func (s *LoggingTestSuite) TestGetHandlerOptions() {
	s.T().Run("returns Info level when not verbose", func(t *testing.T) {
		opts := getHandlerOptions(false)
		require.True(t, opts.Level.Level() == slog.LevelInfo)
	})

	s.T().Run("returns Debug level when verbose", func(t *testing.T) {
		opts := getHandlerOptions(true)
		require.True(t, opts.Level.Level() == slog.LevelDebug)
	})
}

func (s *LoggingTestSuite) TestSetAndGetLogger() {
	s.T().Run("returns the logger that was set", func(t *testing.T) {
		logger := slog.Default()
		SetLogger(logger)
		require.Equal(t, logger, GetLogger())
	})
}

func TestLoggingTestSuite(t *testing.T) {
	suite.Run(t, new(LoggingTestSuite))
}
