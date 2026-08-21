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
	if err := store.Open(ctx); err != nil {
		return nil, fmt.Errorf("open litestream store: %w", err)
	}

	slog.Info("litestream replication started", "db", cfg.DBPath, "replica", client.Type())
	return store, nil
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
