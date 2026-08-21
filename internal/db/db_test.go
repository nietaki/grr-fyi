package db

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pressly/goose/v3"

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

func TestMigrateCreatesMetaTable(t *testing.T) {
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

	var name string
	if err := conn.QueryRowContext(ctx,
		`SELECT name FROM sqlite_master WHERE type='table' AND name='meta'`).Scan(&name); err != nil {
		t.Fatalf("meta table not found: %v", err)
	}
	if name != "meta" {
		t.Fatalf("table name = %q, want meta", name)
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

	var maxVersion int
	if err := conn.QueryRowContext(ctx,
		`SELECT MAX(version_id) FROM goose_db_version WHERE is_applied = 1`).Scan(&maxVersion); err != nil {
		t.Fatalf("query max version: %v", err)
	}
	if maxVersion != 1 {
		t.Fatalf("max applied version = %d, want 1", maxVersion)
	}
}

func TestMigrateDown(t *testing.T) {
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

	goose.SetBaseFS(migrationsFS)
	if err := goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("set goose dialect: %v", err)
	}
	if err := goose.Down(conn, "migrations"); err != nil {
		t.Fatalf("goose.Down: %v", err)
	}

	var name string
	err = conn.QueryRowContext(ctx,
		`SELECT name FROM sqlite_master WHERE type='table' AND name='meta'`).Scan(&name)
	if err != sql.ErrNoRows {
		t.Fatalf("expected meta table to be dropped, got err=%v name=%q", err, name)
	}
}
