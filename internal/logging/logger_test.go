package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"strings"
	"testing"
	"testing/slogtest"
)

func TestInitDefaultLogger(t *testing.T) {
	ctx := context.Background()
	Init(ctx, false)

	if defaultLogger == nil {
		t.Error("Expected defaultLogger to be initialized")
	}

	if GetLogger() == nil {
		t.Error("Expected GetLogger to return non-nil logger")
	}
}

func TestInitVerboseLogger(t *testing.T) {
	ctx := context.Background()
	Init(ctx, true)

	if defaultLogger == nil {
		t.Error("Expected defaultLogger to be initialized with verbose mode")
	}
}

func TestInitJSONFormat(t *testing.T) {
	t.Setenv("LOG_FORMAT", "json")
	ctx := context.Background()
	Init(ctx, false)

	_, ok := defaultLogger.Handler().(*slog.JSONHandler)
	if !ok {
		t.Error("Expected JSONHandler when LOG_FORMAT=json")
	}
}

func TestJSONHandlerOutput(t *testing.T) {
	var buf bytes.Buffer
	h := slog.NewJSONHandler(&buf, nil)

	results := func() []map[string]any {
		var ms []map[string]any
		for line := range bytes.SplitSeq(buf.Bytes(), []byte{'\n'}) {
			if len(line) == 0 {
				continue
			}
			var m map[string]any
			if err := json.Unmarshal(line, &m); err != nil {
				t.Fatalf("Failed to unmarshal JSON log line: %v", err)
			}
			ms = append(ms, m)
		}
		return ms
	}

	err := slogtest.TestHandler(h, results)
	if err != nil {
		t.Errorf("JSONHandler test failed: %v", err)
	}
}

func TestLogMethodsProduceOutput(t *testing.T) {
	var buf bytes.Buffer
	h := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(h)
	SetLogger(logger)

	Debug("debug message", "key", "value")
	Info("info message", "key", "value")
	Warn("warn message", "key", "value")
	Error("error message", "key", "value")

	output := buf.String()

	if !strings.Contains(output, "debug message") {
		t.Error("Expected debug message in output")
	}
	if !strings.Contains(output, "info message") {
		t.Error("Expected info message in output")
	}
	if !strings.Contains(output, "warn message") {
		t.Error("Expected warn message in output")
	}
	if !strings.Contains(output, "error message") {
		t.Error("Expected error message in output")
	}
	if !strings.Contains(output, "key=value") {
		t.Error("Expected key=value in output")
	}
}

func TestLogLevelsAreCorrect(t *testing.T) {
	var buf bytes.Buffer
	h := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(h)
	SetLogger(logger)

	Debug("test", "level", "debug")
	output := buf.String()

	if !strings.Contains(output, "level=DEBUG") {
		t.Error("Expected DEBUG level in output")
	}

	buf.Reset()
	Info("test", "level", "info")
	output = buf.String()

	if !strings.Contains(output, "level=INFO") {
		t.Error("Expected INFO level in output")
	}

	buf.Reset()
	Warn("test", "level", "warn")
	output = buf.String()

	if !strings.Contains(output, "level=WARN") {
		t.Error("Expected WARN level in output")
	}

	buf.Reset()
	Error("test", "level", "error")
	output = buf.String()

	if !strings.Contains(output, "level=ERROR") {
		t.Error("Expected ERROR level in output")
	}
}

func TestWithLogger(t *testing.T) {
	ctx := context.Background()
	Init(ctx, false)

	logger := With("service", "test")
	if logger == nil {
		t.Error("Expected With to return non-nil logger")
	}
}

func TestSetLogger(t *testing.T) {
	ctx := context.Background()
	Init(ctx, false)

	newLogger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	SetLogger(newLogger)

	if GetLogger() != newLogger {
		t.Error("Expected GetLogger to return the set logger")
	}
}
