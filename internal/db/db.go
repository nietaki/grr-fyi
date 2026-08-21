package db

import (
	"context"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"

	"github.com/nietaki/grr-fyi/internal/env"
)

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
	return conn, nil
}

// Migrate applies the schema. It is idempotent and safe to run on every start.
func Migrate(ctx context.Context, conn *sql.DB) error {
	if _, err := conn.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS meta (
			key   TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)`); err != nil {
		return fmt.Errorf("create meta table: %w", err)
	}
	if _, err := conn.ExecContext(ctx,
		`INSERT OR IGNORE INTO meta (key, value) VALUES ('schema_version', '1')`); err != nil {
		return fmt.Errorf("seed meta: %w", err)
	}
	return nil
}
