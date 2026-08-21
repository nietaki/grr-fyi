package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"

	"github.com/nietaki/grr-fyi/internal/env"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Open opens the application's SQLite connection to cfg.DBPath. The database
// is managed by litestream (see internal/replication); the WAL pragma ensures
// writes land in the WAL file that litestream's monitor replicates.
func Open(ctx context.Context, cfg env.Config) (*sql.DB, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(wal)", cfg.DBPath)
	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if err := conn.PingContext(ctx); err != nil {
		conn.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	conn.SetMaxOpenConns(1)
	return conn, nil
}

// Migrate applies all pending migrations. It is idempotent and safe to run on every start.
func Migrate(ctx context.Context, conn *sql.DB) error {
	goose.SetBaseFS(migrationsFS)
	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}
	if err := goose.Up(conn, "migrations"); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}
	return nil
}
