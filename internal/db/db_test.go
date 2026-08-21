package db

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nietaki/grr-fyi/internal/env"
)

func newTestConfig(t *testing.T) env.Config {
	t.Helper()
	return env.Config{DBPath: filepath.Join(t.TempDir(), "test.sqlite")}
}

func TestOpenCreatesDatabaseFile(t *testing.T) {
	ctx := context.Background()
	cfg := newTestConfig(t)

	conn, err := Open(ctx, cfg)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, "CREATE TABLE t (id INTEGER)"); err != nil {
		t.Fatalf("create table: %v", err)
	}

	if _, err := os.Stat(cfg.DBPath); err != nil {
		t.Fatalf("database file not created: %v", err)
	}

	var mode string
	if err := conn.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatalf("query journal_mode: %v", err)
	}
	if !strings.EqualFold(mode, "wal") {
		t.Fatalf("journal_mode = %q, want wal", mode)
	}
}

func TestMigrateSeedsSchemaVersion(t *testing.T) {
	ctx := context.Background()
	cfg := newTestConfig(t)

	conn, err := Open(ctx, cfg)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer conn.Close()

	if err := Migrate(ctx, conn); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	var version string
	if err := conn.QueryRowContext(ctx,
		`SELECT value FROM meta WHERE key = 'schema_version'`).Scan(&version); err != nil {
		t.Fatalf("query schema_version: %v", err)
	}
	if version != "1" {
		t.Fatalf("schema_version = %q, want 1", version)
	}
}

func TestMigrateIsIdempotent(t *testing.T) {
	ctx := context.Background()
	cfg := newTestConfig(t)

	conn, err := Open(ctx, cfg)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer conn.Close()

	for i := 0; i < 3; i++ {
		if err := Migrate(ctx, conn); err != nil {
			t.Fatalf("Migrate run %d: %v", i+1, err)
		}
	}

	var count int
	if err := conn.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM meta WHERE key = 'schema_version'`).Scan(&count); err != nil {
		t.Fatalf("count schema_version: %v", err)
	}
	if count != 1 {
		t.Fatalf("schema_version rows = %d, want 1", count)
	}
}
