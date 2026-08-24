package replication

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/benbjohnson/litestream"
	"github.com/mvrahden/go-test/pkg/gotest"
	_ "modernc.org/sqlite"

	"github.com/nietaki/grr-fyi/internal/env"
)

type ReplicationTestSuite struct {
	cfg env.Config
}

func (s *ReplicationTestSuite) BeforeEach(t *gotest.T) {
	s.cfg = env.Config{DBPath: filepath.Join(t.T().TempDir(), "app.sqlite")}
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

func closeStore(ctx context.Context, store *litestream.Store) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return store.Close(timeoutCtx)
}

func (s *ReplicationTestSuite) TestStartFileReplica(t *gotest.T) {
	t.It("creates replica directory after sync", func(it *gotest.T) {
		ctx := context.Background()

		store, err := Start(ctx, s.cfg)
		gotest.NoError(it, err, "Start")

		appDB, err := openAppDB(s.cfg.DBPath)
		gotest.NoError(it, err, "open app db")

		_, err = appDB.Exec(`CREATE TABLE kv (k TEXT PRIMARY KEY, v TEXT)`)
		gotest.NoError(it, err, "create table")

		_, err = appDB.Exec(`INSERT INTO kv (k, v) VALUES ('a', 'b')`)
		gotest.NoError(it, err, "insert")

		err = appDB.Close()
		gotest.NoError(it, err, "close app db")

		_, err = store.SyncDB(ctx, s.cfg.DBPath, true)
		gotest.NoError(it, err, "sync")

		err = closeStore(ctx, store)
		gotest.NoError(it, err, "store.Close")

		replicaDir := filepath.Join(filepath.Dir(s.cfg.DBPath), "litestream")
		fi, err := os.Stat(replicaDir)
		gotest.NoError(it, err, "file replica dir missing")
		gotest.True(it, fi.IsDir(), "expected %s to be a directory", replicaDir)
	})
}

func (s *ReplicationTestSuite) TestStartRestoresDeletedDatabase(t *gotest.T) {
	t.It("restores database from replica after local files are deleted", func(it *gotest.T) {
		ctx := context.Background()

		store, err := Start(ctx, s.cfg)
		gotest.NoError(it, err, "Start (1)")

		appDB, err := openAppDB(s.cfg.DBPath)
		gotest.NoError(it, err, "open app db (1)")

		_, err = appDB.Exec(`CREATE TABLE kv (k TEXT PRIMARY KEY, v TEXT)`)
		gotest.NoError(it, err, "create table")

		_, err = appDB.Exec(`INSERT INTO kv (k, v) VALUES ('answer', '42')`)
		gotest.NoError(it, err, "insert")

		err = appDB.Close()
		gotest.NoError(it, err, "close app db (1)")

		_, err = store.SyncDB(ctx, s.cfg.DBPath, true)
		gotest.NoError(it, err, "sync (1)")

		db := store.DBs()[0]
		_, err = store.CompactDB(ctx, db, store.SnapshotLevel())
		if err != nil && !errors.Is(err, litestream.ErrNoCompaction) &&
			!errors.Is(err, litestream.ErrCompactionTooEarly) {
			gotest.NoError(it, err, "force snapshot")
		}

		_, err = store.SyncDB(ctx, s.cfg.DBPath, true)
		gotest.NoError(it, err, "sync (2)")

		err = closeStore(ctx, store)
		gotest.NoError(it, err, "store.Close (1)")

		for _, suffix := range []string{"", "-wal", "-shm"} {
			err = os.Remove(s.cfg.DBPath + suffix)
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				gotest.NoError(it, err, "remove %s%s", s.cfg.DBPath, suffix)
			}
		}

		store2, err := Start(ctx, s.cfg)
		gotest.NoError(it, err, "Start (2)")
		defer func() {
			_ = closeStore(ctx, store2)
		}()

		appDB2, err := openAppDB(s.cfg.DBPath)
		gotest.NoError(it, err, "open app db (2)")
		defer appDB2.Close()

		var v string
		err = appDB2.QueryRow(`SELECT v FROM kv WHERE k = 'answer'`).Scan(&v)
		gotest.NoError(it, err, "query restored row")
		gotest.Equal(it, "42", v)
	})
}
