package replication

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/benbjohnson/litestream"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	_ "modernc.org/sqlite"

	"github.com/nietaki/grr-fyi/internal/env"
)

type ReplicationTestSuite struct {
	suite.Suite
	cfg env.Config
}

func (s *ReplicationTestSuite) SetupTest() {
	s.cfg = env.Config{DBPath: filepath.Join(s.T().TempDir(), "app.sqlite")}
}

func openAppDB(path string) (*sql.DB, error) {
	conn, err := sql.Open("sqlite",
		"file:"+path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(wal)")
	if err != nil {
		return nil, err
	}
	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}

func (s *ReplicationTestSuite) TestStartFileReplica() {
	s.T().Run("creates replica directory after sync", func(t *testing.T) {
		ctx := context.Background()

		store, err := Start(ctx, s.cfg)
		require.NoError(t, err, "Start")

		appDB, err := openAppDB(s.cfg.DBPath)
		require.NoError(t, err, "open app db")

		_, err = appDB.Exec(`CREATE TABLE kv (k TEXT PRIMARY KEY, v TEXT)`)
		require.NoError(t, err, "create table")

		_, err = appDB.Exec(`INSERT INTO kv (k, v) VALUES ('a', 'b')`)
		require.NoError(t, err, "insert")

		err = appDB.Close()
		require.NoError(t, err, "close app db")

		_, err = store.SyncDB(ctx, s.cfg.DBPath, true)
		require.NoError(t, err, "sync")

		Close(store)

		replicaDir := filepath.Join(filepath.Dir(s.cfg.DBPath), "litestream")
		fi, err := os.Stat(replicaDir)
		require.NoError(t, err, "file replica dir missing")
		require.True(t, fi.IsDir(), "expected %s to be a directory", replicaDir)
	})
}

func (s *ReplicationTestSuite) TestStartRestoresDeletedDatabase() {
	s.T().Run("restores database from replica after local files are deleted", func(t *testing.T) {
		ctx := context.Background()

		store, err := Start(ctx, s.cfg)
		require.NoError(t, err, "Start (1)")

		appDB, err := openAppDB(s.cfg.DBPath)
		require.NoError(t, err, "open app db (1)")

		_, err = appDB.Exec(`CREATE TABLE kv (k TEXT PRIMARY KEY, v TEXT)`)
		require.NoError(t, err, "create table")

		_, err = appDB.Exec(`INSERT INTO kv (k, v) VALUES ('answer', '42')`)
		require.NoError(t, err, "insert")

		err = appDB.Close()
		require.NoError(t, err, "close app db (1)")

		_, err = store.SyncDB(ctx, s.cfg.DBPath, true)
		require.NoError(t, err, "sync (1)")

		db := store.DBs()[0]
		_, err = store.CompactDB(ctx, db, store.SnapshotLevel())
		if err != nil && !errors.Is(err, litestream.ErrNoCompaction) &&
			!errors.Is(err, litestream.ErrCompactionTooEarly) {
			require.NoError(t, err, "force snapshot")
		}

		_, err = store.SyncDB(ctx, s.cfg.DBPath, true)
		require.NoError(t, err, "sync (2)")

		Close(store)

		for _, suffix := range []string{"", "-wal", "-shm"} {
			err = os.Remove(s.cfg.DBPath + suffix)
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				require.NoError(t, err, "remove %s%s", s.cfg.DBPath, suffix)
			}
		}

		store2, err := Start(ctx, s.cfg)
		require.NoError(t, err, "Start (2)")
		defer Close(store2)

		appDB2, err := openAppDB(s.cfg.DBPath)
		require.NoError(t, err, "open app db (2)")
		defer appDB2.Close()

		var v string
		err = appDB2.QueryRow(`SELECT v FROM kv WHERE k = 'answer'`).Scan(&v)
		require.NoError(t, err, "query restored row")
		require.Equal(t, "42", v)
	})
}

func (s *ReplicationTestSuite) TestClose() {
	s.T().Run("closes store without panic", func(t *testing.T) {
		ctx := context.Background()

		store, err := Start(ctx, s.cfg)
		require.NoError(t, err, "Start")

		require.NotPanics(t, func() {
			Close(store)
		})
	})
}

func (s *ReplicationTestSuite) TestMetaPath() {
	s.T().Run("uses custom meta path when configured", func(t *testing.T) {
		ctx := context.Background()
		tmpDir := s.T().TempDir()
		customMetaPath := filepath.Join(tmpDir, "custom-meta")

		cfg := env.Config{
			DBPath:             filepath.Join(tmpDir, "app.sqlite"),
			LitestreamMetaPath: customMetaPath,
		}

		store, err := Start(ctx, cfg)
		require.NoError(t, err, "Start")

		appDB, err := openAppDB(cfg.DBPath)
		require.NoError(t, err, "open app db")

		_, err = appDB.Exec(`CREATE TABLE kv (k TEXT PRIMARY KEY, v TEXT)`)
		require.NoError(t, err, "create table")

		_, err = appDB.Exec(`INSERT INTO kv (k, v) VALUES ('test', 'value')`)
		require.NoError(t, err, "insert")

		err = appDB.Close()
		require.NoError(t, err, "close app db")

		_, err = store.SyncDB(ctx, cfg.DBPath, true)
		require.NoError(t, err, "sync")

		Close(store)

		fi, err := os.Stat(customMetaPath)
		require.NoError(t, err, "custom meta path missing")
		require.True(t, fi.IsDir(), "expected %s to be a directory", customMetaPath)

		ltxDir := filepath.Join(customMetaPath, "ltx")
		fi, err = os.Stat(ltxDir)
		require.NoError(t, err, "ltx directory missing in custom meta path")
		require.True(t, fi.IsDir(), "expected %s to be a directory", ltxDir)
	})
}

func (s *ReplicationTestSuite) TestWaitForInitialSync() {
	s.T().Run("returns immediately when no replica data exists", func(t *testing.T) {
		ctx := context.Background()

		store, err := Start(ctx, s.cfg)
		require.NoError(t, err, "Start")
		defer Close(store)

		db := store.DBs()[0]
		status, err := db.SyncStatus(ctx)
		require.NoError(t, err, "SyncStatus")
		require.Equal(t, uint64(0), uint64(status.LocalTXID), "local TXID should be 0")
		require.Equal(t, uint64(0), uint64(status.RemoteTXID), "remote TXID should be 0")
	})
}

func (s *ReplicationTestSuite) TestSnapshotSettings() {
	s.T().Run("sets snapshot interval to 6h and retention to 168h", func(t *testing.T) {
		ctx := context.Background()

		store, err := Start(ctx, s.cfg)
		require.NoError(t, err, "Start")
		defer Close(store)

		require.Equal(t, 6*time.Hour, store.SnapshotInterval, "snapshot interval should be 6h")
		require.Equal(t, 7*24*time.Hour, store.SnapshotRetention, "snapshot retention should be 168h")
	})
}

func TestReplicationTestSuite(t *testing.T) {
	suite.Run(t, new(ReplicationTestSuite))
}
