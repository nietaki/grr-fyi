package replication

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/benbjohnson/litestream"
	"github.com/benbjohnson/litestream/file"
	_ "github.com/benbjohnson/litestream/s3" // registers the s3:// scheme for NewReplicaClientFromURL

	"github.com/nietaki/grr-fyi/internal/env"
)

// Start opens the database at cfg.DBPath under litestream and begins
// background replication and compaction. The replica target comes from
// cfg.LitestreamReplica (for example "s3://bucket/path"); when it is empty a
// local directory next to the database file is used instead, which keeps
// `make run` working without any object store.
func Start(ctx context.Context, cfg env.Config) (*litestream.Store, error) {
	db := litestream.NewDB(cfg.DBPath)
	if cfg.LitestreamMetaPath != "" {
		db.SetMetaPath(cfg.LitestreamMetaPath)
	}

	client, err := newReplicaClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("create replica client: %w", err)
	}

	replica := litestream.NewReplicaWithClient(db, client)
	db.Replica = replica
	setClientReplica(client, replica)

	// Initialize the client before restore (store.Open also inits it; both
	// backends make Init idempotent).
	if err := client.Init(ctx); err != nil {
		return nil, fmt.Errorf("init replica client: %w", err)
	}

	// Restore from the replica if the local database file is missing (fresh
	// pod / wiped volume). No-op when the file already exists or no backup is
	// available yet.
	if err := db.EnsureExists(ctx); err != nil {
		return nil, fmt.Errorf("ensure database exists: %w", err)
	}

	levels := litestream.CompactionLevels{
		{Level: 0},
		{Level: 1, Interval: 10 * time.Second},
	}

	store := litestream.NewStore([]*litestream.DB{db}, levels)
	store.SnapshotInterval = 6 * time.Hour
	store.SnapshotRetention = 7 * 24 * time.Hour

	if err := store.Open(ctx); err != nil {
		return nil, fmt.Errorf("open litestream store: %w", err)
	}

	if err := waitForInitialSync(ctx, store); err != nil {
		return nil, fmt.Errorf("wait for initial sync: %w", err)
	}

	slog.Info("litestream replication started", "db", cfg.DBPath, "meta_path", db.MetaPath(), "replica", client.Type())
	return store, nil
}

func Close(store *litestream.Store) {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := store.Close(shutdownCtx); err != nil {
		slog.Error("failed to close litestream store", "error", err)
	}
}

// waitForInitialSync blocks until all databases in the store have caught up
// with their remote replica. This ensures the database is fully restored
// before the application starts serving requests.
//
// After EnsureExists() restores from a snapshot, the database may be behind
// the replica if newer L0 LTX files exist. The monitor goroutine fetches
// these asynchronously via checkDatabaseBehindReplica(). This function polls
// SyncStatus() until the local TXID matches the remote TXID.
//
// Returns immediately if:
//   - No replica is configured for a database
//   - Both local and remote TXID are 0 (fresh install, no backup exists)
//
// Times out after 30 seconds if sync doesn't complete.
func waitForInitialSync(ctx context.Context, store *litestream.Store) error {
	slog.Info("waiting for initial replica sync")
	const (
		pollInterval = 100 * time.Millisecond
		timeout      = 30 * time.Second
	)

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		allSynced := true

		for _, db := range store.DBs() {
			if db.Replica == nil {
				continue
			}

			status, err := db.SyncStatus(ctx)
			if err != nil {
				slog.Warn("sync status check failed", "db", db.Path(), "error", err)
				allSynced = false
				continue
			}

			if status.LocalTXID == 0 && status.RemoteTXID == 0 {
				continue
			}

			if !status.InSync {
				allSynced = false
				slog.Debug("waiting for replica sync",
					"db", db.Path(),
					"local_txid", status.LocalTXID,
					"remote_txid", status.RemoteTXID)
			}
		}

		if allSynced {
			return nil
		}

		select {
		case <-ctx.Done():
			if ctx.Err() == context.DeadlineExceeded {
				return fmt.Errorf("timeout after %v waiting for replica sync", timeout)
			}
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func newReplicaClient(cfg env.Config) (litestream.ReplicaClient, error) {
	if cfg.ReplicaUrl != "" {
		return litestream.NewReplicaClientFromURL(cfg.ReplicaUrl)
	}
	return file.NewReplicaClient(filepath.Join(filepath.Dir(cfg.DBPath), "litestream")), nil
}

// setClientReplica wires the file client's back-reference to the replica so it
// can mirror the source database's ownership. Only the file backend exposes
// this field; other backends ignore it.
func setClientReplica(client litestream.ReplicaClient, replica *litestream.Replica) {
	if c, ok := client.(*file.ReplicaClient); ok {
		c.Replica = replica
	}
}
