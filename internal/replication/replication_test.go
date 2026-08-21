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
	_ "modernc.org/sqlite"

	"github.com/nietaki/grr-fyi/internal/env"
)

// openAppDB opens the same SQLite file litestream is monitoring, using a WAL
// connection exactly like the application does.
func openAppDB(t *testing.T, path string) *sql.DB {
	t.Helper()
	conn, err := sql.Open("sqlite",
		"file:"+path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(wal)")
	if err != nil {
		t.Fatalf("open app db: %v", err)
	}
	if err := conn.Ping(); err != nil {
		conn.Close()
		t.Fatalf("ping app db: %v", err)
	}
	return conn
}

func closeStore(t *testing.T, store *litestream.Store) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := store.Close(ctx); err != nil {
		t.Fatalf("store.Close: %v", err)
	}
}

func TestStartFileReplica(t *testing.T) {
	ctx := context.Background()
	cfg := env.Config{DBPath: filepath.Join(t.TempDir(), "app.sqlite")}

	store, err := Start(ctx, cfg)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Write a transaction and flush it so the replica directory is created.
	appDB := openAppDB(t, cfg.DBPath)
	if _, err := appDB.Exec(`CREATE TABLE kv (k TEXT PRIMARY KEY, v TEXT)`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if _, err := appDB.Exec(`INSERT INTO kv (k, v) VALUES ('a', 'b')`); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := appDB.Close(); err != nil {
		t.Fatalf("close app db: %v", err)
	}
	if _, err := store.SyncDB(ctx, cfg.DBPath, true); err != nil {
		t.Fatalf("sync: %v", err)
	}
	closeStore(t, store)

	replicaDir := filepath.Join(filepath.Dir(cfg.DBPath), "litestream")
	fi, err := os.Stat(replicaDir)
	if err != nil {
		t.Fatalf("file replica dir missing: %v", err)
	}
	if !fi.IsDir() {
		t.Fatalf("expected %s to be a directory", replicaDir)
	}
}

func TestStartRestoresDeletedDatabase(t *testing.T) {
	ctx := context.Background()
	cfg := env.Config{DBPath: filepath.Join(t.TempDir(), "app.sqlite")}

	// Phase 1: create data, force a snapshot into the replica, then shut down.
	store, err := Start(ctx, cfg)
	if err != nil {
		t.Fatalf("Start (1): %v", err)
	}

	appDB := openAppDB(t, cfg.DBPath)
	if _, err := appDB.Exec(`CREATE TABLE kv (k TEXT PRIMARY KEY, v TEXT)`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if _, err := appDB.Exec(`INSERT INTO kv (k, v) VALUES ('answer', '42')`); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := appDB.Close(); err != nil {
		t.Fatalf("close app db: %v", err)
	}

	// Flush frames, then force a level-9 snapshot (snapshots otherwise only run
	// on the store's 24h interval).
	if _, err := store.SyncDB(ctx, cfg.DBPath, true); err != nil {
		t.Fatalf("sync (1): %v", err)
	}
	db := store.DBs()[0]
	if _, err := store.CompactDB(ctx, db, store.SnapshotLevel()); err != nil &&
		!errors.Is(err, litestream.ErrNoCompaction) &&
		!errors.Is(err, litestream.ErrCompactionTooEarly) {
		t.Fatalf("force snapshot: %v", err)
	}
	if _, err := store.SyncDB(ctx, cfg.DBPath, true); err != nil {
		t.Fatalf("sync (2): %v", err)
	}
	closeStore(t, store)

	// Simulate a lost local database (fresh pod / wiped volume).
	for _, suffix := range []string{"", "-wal", "-shm"} {
		if err := os.Remove(cfg.DBPath + suffix); err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("remove %s%s: %v", cfg.DBPath, suffix, err)
		}
	}

	// Phase 2: starting again must restore the database from the replica.
	store2, err := Start(ctx, cfg)
	if err != nil {
		t.Fatalf("Start (2): %v", err)
	}
	defer closeStore(t, store2)

	appDB2 := openAppDB(t, cfg.DBPath)
	defer appDB2.Close()

	var v string
	if err := appDB2.QueryRow(`SELECT v FROM kv WHERE k = 'answer'`).Scan(&v); err != nil {
		t.Fatalf("query restored row: %v", err)
	}
	if v != "42" {
		t.Fatalf("restored value = %q, want 42", v)
	}
}
