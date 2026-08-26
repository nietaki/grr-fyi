package db

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/nietaki/grr-fyi/internal/env"
)

type DBTestSuite struct {
	suite.Suite
	cfg env.Config
}

func (s *DBTestSuite) SetupTest() {
	s.cfg = env.Config{DBPath: filepath.Join(s.T().TempDir(), "test.sqlite")}
}

func (s *DBTestSuite) TestOpenCreatesDatabaseFile() {
	s.T().Run("creates database file and enables WAL mode", func(t *testing.T) {
		ctx := context.Background()

		conn, err := Open(ctx, s.cfg)
		require.NoError(t, err, "Open")
		defer conn.Close()

		_, err = conn.ExecContext(ctx, "CREATE TABLE t (id INTEGER)")
		require.NoError(t, err, "create table")

		_, err = os.Stat(s.cfg.DBPath)
		require.NoError(t, err, "database file not created")

		var mode string
		err = conn.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&mode)
		require.NoError(t, err, "query journal_mode")
		require.True(t, strings.EqualFold(mode, "wal"), "journal_mode = %q, want wal", mode)
	})
}

func (s *DBTestSuite) TestMigrateCreatesMetaTable() {
	s.T().Run("creates meta table after migration", func(t *testing.T) {
		ctx := context.Background()

		conn, err := Open(ctx, s.cfg)
		require.NoError(t, err, "Open")
		defer conn.Close()

		err = Migrate(ctx, conn)
		require.NoError(t, err, "Migrate")

		var name string
		err = conn.QueryRowContext(ctx,
			`SELECT name FROM sqlite_master WHERE type='table' AND name='meta'`).Scan(&name)
		require.NoError(t, err, "meta table not found")
		require.Equal(t, "meta", name)
	})
}

func (s *DBTestSuite) TestMigrateIsIdempotent() {
	s.T().Run("can run migrations multiple times safely", func(t *testing.T) {
		ctx := context.Background()

		conn, err := Open(ctx, s.cfg)
		require.NoError(t, err, "Open")
		defer conn.Close()

		for i := 0; i < 3; i++ {
			err = Migrate(ctx, conn)
			require.NoError(t, err, "Migrate run %d", i+1)
		}

		var maxVersion int
		err = conn.QueryRowContext(ctx,
			`SELECT MAX(version_id) FROM goose_db_version WHERE is_applied = 1`).Scan(&maxVersion)
		require.NoError(t, err, "query max version")
		require.Equal(t, 3, maxVersion)
	})
}

func (s *DBTestSuite) TestMigrateDown() {
	s.T().Run("drops all tables when migrating down", func(t *testing.T) {
		ctx := context.Background()

		conn, err := Open(ctx, s.cfg)
		require.NoError(t, err, "Open")
		defer conn.Close()

		err = Migrate(ctx, conn)
		require.NoError(t, err, "Migrate")

		goose.SetBaseFS(migrationsFS)
		err = goose.SetDialect("sqlite3")
		require.NoError(t, err, "set goose dialect")

		err = goose.Reset(conn, "migrations")
		require.NoError(t, err, "goose.Reset")

		var name string
		err = conn.QueryRowContext(ctx,
			`SELECT name FROM sqlite_master WHERE type='table' AND name='meta'`).Scan(&name)
		require.ErrorIs(t, err, sql.ErrNoRows, "expected meta table to be dropped")

		err = conn.QueryRowContext(ctx,
			`SELECT name FROM sqlite_master WHERE type='table' AND name='links'`).Scan(&name)
		require.ErrorIs(t, err, sql.ErrNoRows, "expected links table to be dropped")
	})
}

func (s *DBTestSuite) TestMigrateCreatesUrlShortenerTables() {
	s.T().Run("creates links, clicks, and slug_sequence tables", func(t *testing.T) {
		ctx := context.Background()

		conn, err := Open(ctx, s.cfg)
		require.NoError(t, err, "Open")
		defer conn.Close()

		err = Migrate(ctx, conn)
		require.NoError(t, err, "Migrate")

		expectedTables := []string{"links", "clicks", "slug_sequence"}
		for _, table := range expectedTables {
			var name string
			err = conn.QueryRowContext(ctx,
				`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&name)
			require.NoError(t, err, "table %q not found", table)
			require.Equal(t, table, name)
		}
	})
}

func (s *DBTestSuite) TestSlugSequenceInitialized() {
	s.T().Run("initializes slug_sequence with next_value = 262144", func(t *testing.T) {
		ctx := context.Background()

		conn, err := Open(ctx, s.cfg)
		require.NoError(t, err, "Open")
		defer conn.Close()

		err = Migrate(ctx, conn)
		require.NoError(t, err, "Migrate")

		var nextValue int
		err = conn.QueryRowContext(ctx, `SELECT next_value FROM slug_sequence WHERE id = 1`).Scan(&nextValue)
		require.NoError(t, err, "slug_sequence not initialized")
		require.Equal(t, 262144, nextValue)
	})
}

func (s *DBTestSuite) TestPerformancePragmas() {
	s.T().Run("sets all performance PRAGMAs correctly", func(t *testing.T) {
		ctx := context.Background()

		conn, err := Open(ctx, s.cfg)
		require.NoError(t, err, "Open")
		defer conn.Close()

		var synchronous string
		err = conn.QueryRowContext(ctx, "PRAGMA synchronous").Scan(&synchronous)
		require.NoError(t, err, "query synchronous")
		require.Equal(t, "1", synchronous, "synchronous should be NORMAL (1)")

		var cacheSize int
		err = conn.QueryRowContext(ctx, "PRAGMA cache_size").Scan(&cacheSize)
		require.NoError(t, err, "query cache_size")
		require.Equal(t, -64000, cacheSize, "cache_size should be -64000 (64MB)")

		var mmapSize int64
		err = conn.QueryRowContext(ctx, "PRAGMA mmap_size").Scan(&mmapSize)
		require.NoError(t, err, "query mmap_size")
		require.Equal(t, int64(268435456), mmapSize, "mmap_size should be 256MB")

		var tempStore string
		err = conn.QueryRowContext(ctx, "PRAGMA temp_store").Scan(&tempStore)
		require.NoError(t, err, "query temp_store")
		require.Equal(t, "2", tempStore, "temp_store should be MEMORY (2)")

		var foreignKeys int
		err = conn.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&foreignKeys)
		require.NoError(t, err, "query foreign_keys")
		require.Equal(t, 1, foreignKeys, "foreign_keys should be ON (1)")

		var autocheckpoint int
		err = conn.QueryRowContext(ctx, "PRAGMA wal_autocheckpoint").Scan(&autocheckpoint)
		require.NoError(t, err, "query wal_autocheckpoint")
		require.Equal(t, 0, autocheckpoint, "wal_autocheckpoint should be 0 (disabled)")
	})
}

func TestDBTestSuite(t *testing.T) {
	suite.Run(t, new(DBTestSuite))
}
