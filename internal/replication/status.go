package replication

import (
	"context"
	"fmt"

	"github.com/benbjohnson/litestream"

	"github.com/nietaki/grr-fyi/internal/stats"
)

// StatusProvider returns a stats.ReplicationProvider backed by the live
// Litestream store. It reports the sync status of the first database in the
// store (this app has exactly one).
func StatusProvider(store *litestream.Store) stats.ReplicationProvider {
	return func(ctx context.Context) stats.ReplicationInfo {
		dbs := store.DBs()
		if len(dbs) == 0 {
			return stats.ReplicationInfo{Enabled: true, Err: "no databases registered"}
		}
		db := dbs[0]
		if db.Replica == nil {
			return stats.ReplicationInfo{Enabled: true, Err: "no replica configured"}
		}
		status, err := db.SyncStatus(ctx)
		if err != nil {
			return stats.ReplicationInfo{Enabled: true, Err: fmt.Sprintf("sync status: %v", err)}
		}
		return stats.ReplicationInfo{
			Enabled:    true,
			InSync:     status.InSync,
			LocalTXID:  int64(status.LocalTXID),
			RemoteTXID: int64(status.RemoteTXID),
		}
	}
}
