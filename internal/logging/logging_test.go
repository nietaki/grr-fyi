package logging

import (
	"log/slog"

	"github.com/mvrahden/go-test/pkg/gotest"
)

type LoggingTestSuite struct{}

func (s *LoggingTestSuite) TestGetHandlerOptions(t *gotest.T) {
	t.It("returns Info level when not verbose", func(it *gotest.T) {
		opts := getHandlerOptions(false)
		gotest.True(it, opts.Level.Level() == slog.LevelInfo)
	})

	t.It("returns Debug level when verbose", func(it *gotest.T) {
		opts := getHandlerOptions(true)
		gotest.True(it, opts.Level.Level() == slog.LevelDebug)
	})
}

func (s *LoggingTestSuite) TestSetAndGetLogger(t *gotest.T) {
	t.It("returns the logger that was set", func(it *gotest.T) {
		logger := slog.Default()
		SetLogger(logger)
		gotest.Equal(it, logger, GetLogger())
	})
}
