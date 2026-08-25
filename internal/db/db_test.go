package db

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"

	"github.com/mvrahden/go-test/pkg/gotest"
	"github.com/pressly/goose/v3"

	"github.com/nietaki/grr-fyi/internal/env"
)

type DBTestSuite struct {
	cfg env.Config
}

func (s *DBTestSuite) BeforeEach(t *gotest.T) {
	s.cfg = env.Config{DBPath: filepath.Join(t.T().TempDir(), "test.sqlite")}
}

func (s *DBTestSuite) TestOpenCreatesDatabaseFile(t *gotest.T) {
	t.It("creates database file and enables WAL mode", func(it *gotest.T) {
		ctx := context.Background()

		conn, err := Open(ctx, s.cfg)
		gotest.NoError(it, err, "Open")
		defer conn.Close()

		_, err = conn.ExecContext(ctx, "CREATE TABLE t (id INTEGER)")
		gotest.NoError(it, err, "create table")

		_, err = os.Stat(s.cfg.DBPath)
		gotest.NoError(it, err, "database file not created")

		var mode string
		err = conn.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&mode)
		gotest.NoError(it, err, "query journal_mode")
		gotest.True(it, strings.EqualFold(mode, "wal"), "journal_mode = %q, want wal", mode)
	})
}

func (s *DBTestSuite) TestMigrateCreatesMetaTable(t *gotest.T) {
	t.It("creates meta table after migration", func(it *gotest.T) {
		ctx := context.Background()

		conn, err := Open(ctx, s.cfg)
		gotest.NoError(it, err, "Open")
		defer conn.Close()

		err = Migrate(ctx, conn)
		gotest.NoError(it, err, "Migrate")

		var name string
		err = conn.QueryRowContext(ctx,
			`SELECT name FROM sqlite_master WHERE type='table' AND name='meta'`).Scan(&name)
		gotest.NoError(it, err, "meta table not found")
		gotest.Equal(it, "meta", name)
	})
}

func (s *DBTestSuite) TestMigrateIsIdempotent(t *gotest.T) {
	t.It("can run migrations multiple times safely", func(it *gotest.T) {
		ctx := context.Background()

		conn, err := Open(ctx, s.cfg)
		gotest.NoError(it, err, "Open")
		defer conn.Close()

		for i := 0; i < 3; i++ {
			err = Migrate(ctx, conn)
			gotest.NoError(it, err, "Migrate run %d", i+1)
		}

		var maxVersion int
		err = conn.QueryRowContext(ctx,
			`SELECT MAX(version_id) FROM goose_db_version WHERE is_applied = 1`).Scan(&maxVersion)
		gotest.NoError(it, err, "query max version")
		gotest.Equal(it, 2, maxVersion)
	})
}

func (s *DBTestSuite) TestMigrateDown(t *gotest.T) {
	t.It("drops all tables when migrating down", func(it *gotest.T) {
		ctx := context.Background()

		conn, err := Open(ctx, s.cfg)
		gotest.NoError(it, err, "Open")
		defer conn.Close()

		err = Migrate(ctx, conn)
		gotest.NoError(it, err, "Migrate")

		goose.SetBaseFS(migrationsFS)
		err = goose.SetDialect("sqlite3")
		gotest.NoError(it, err, "set goose dialect")

		err = goose.Reset(conn, "migrations")
		gotest.NoError(it, err, "goose.Reset")

		var name string
		err = conn.QueryRowContext(ctx,
			`SELECT name FROM sqlite_master WHERE type='table' AND name='meta'`).Scan(&name)
		gotest.ErrorIs(it, err, sql.ErrNoRows, "expected meta table to be dropped")

		err = conn.QueryRowContext(ctx,
			`SELECT name FROM sqlite_master WHERE type='table' AND name='links'`).Scan(&name)
		gotest.ErrorIs(it, err, sql.ErrNoRows, "expected links table to be dropped")
	})
}

func (s *DBTestSuite) TestMigrateCreatesUrlShortenerTables(t *gotest.T) {
	t.It("creates links, clicks, and slug_sequence tables", func(it *gotest.T) {
		ctx := context.Background()

		conn, err := Open(ctx, s.cfg)
		gotest.NoError(it, err, "Open")
		defer conn.Close()

		err = Migrate(ctx, conn)
		gotest.NoError(it, err, "Migrate")

		expectedTables := []string{"links", "clicks", "slug_sequence"}
		for _, table := range expectedTables {
			var name string
			err = conn.QueryRowContext(ctx,
				`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&name)
			gotest.NoError(it, err, "table %q not found", table)
			gotest.Equal(it, table, name)
		}
	})
}

func (s *DBTestSuite) TestSlugSequenceInitialized(t *gotest.T) {
	t.It("initializes slug_sequence with next_value = 0", func(it *gotest.T) {
		ctx := context.Background()

		conn, err := Open(ctx, s.cfg)
		gotest.NoError(it, err, "Open")
		defer conn.Close()

		err = Migrate(ctx, conn)
		gotest.NoError(it, err, "Migrate")

		var nextValue int
		err = conn.QueryRowContext(ctx, `SELECT next_value FROM slug_sequence WHERE id = 1`).Scan(&nextValue)
		gotest.NoError(it, err, "slug_sequence not initialized")
		gotest.Equal(it, 0, nextValue)
	})
}
