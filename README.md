# grr.fyi

## TODO

- [ ] partials / general layout
- [ ] altcha
- [x] modernc sqlite
- [x] litestream

## SQLite persistence & Litestream replication

The app stores data in a single SQLite file (`DB_PATH`, default `db/filedb.sqlite`)
and replicates it in the background with [litestream](https://github.com/benbjohnson/litestream)
in **library mode**. When `LITESTREAM_REPLICA` is set (e.g. `s3://bucket/path`) the
replica target is that object store; when it is empty a local `litestream/` directory
next to the database is used instead, so `make run` works with no object store.

Wiring lives in `internal/replication/replication.go` (store lifecycle),
`internal/db/db.go` (app connection + schema), and `main.go` (startup/shutdown order).

### Why library mode

- The application opens the database with an ordinary `database/sql` connection in
  **WAL** mode (`?_pragma=journal_mode(wal)`). Litestream opens its *own* connection to
  the same file and watches the WAL; it does not wrap the app's connection.
- We use the pure-Go driver `modernc.org/sqlite` because litestream uses it internally.
  Mixing it with a cgo driver (e.g. `mattn/go-sqlite3`) causes POSIX lock conflicts.

### Key types

| Type | Role |
| --- | --- |
| `litestream.Store` | Top-level manager. Owns the `[]*DB`, the compaction levels, and the background monitors (compaction, snapshots, L0 retention, heartbeats). Entry points: `NewStore(dbs, levels)`, `Open(ctx)`, `Close(ctx)`. |
| `litestream.DB` | One managed SQLite file. Holds the WAL-monitor loop and checkpoint policy. `NewDB(path)`, `Open()`, `Close(ctx)`, `EnsureExists(ctx)`, `Sync(ctx)`. |
| `litestream.Replica` | The replication unit for a DB; binds a `DB` to a `ReplicaClient`. `NewReplicaWithClient(db, client)`. |
| `litestream.ReplicaClient` | Storage backend interface (`file`, `s3`, …). Schemes are registered via blank imports; `NewReplicaClientFromURL` selects by scheme. |
| `litestream.CompactionLevel` / `CompactionLevels` | LSM-style compaction tiers. We use `CompactionLevels{{Level: 0}, {Level: 1, Interval: 10 * time.Second}}`. |
| `litestream.SnapshotLevel` | `const = 9`. A pseudo-level for full snapshots — *not* a real compaction tier. |

### Key variables and defaults

The knobs we rely on (defaults in parentheses), from `store.go` / `db.go` / `replica.go`:

| Variable | Default | Meaning |
| --- | --- | --- |
| `Store.SnapshotInterval` | 24h | How often a full DB snapshot is written to the replica |
| `Store.SnapshotRetention` | 24h | How long snapshots are kept |
| `Store.L0Retention` | 5m | How long raw L0 LTX files are kept after being compacted |
| `Store.L0RetentionCheckInterval` | 15s | How often L0 retention is enforced |
| `DB.MonitorInterval` | 1s | How often the per-DB sync loop wakes |
| `DB.MinCheckpointPageN` | 1000 (~4MB) | Size threshold for a PASSIVE checkpoint |
| `DB.CheckpointInterval` | 1m | Time threshold for a PASSIVE checkpoint |
| `DB.TruncatePageN` | 121359 (~500MB) | Emergency TRUNCATE (blocking) checkpoint threshold |
| `DB.MaxSyncWALBytes` | 64MB | Max WAL bytes drained per bounded sync chunk |
| `Replica.SyncInterval` | 1s | Replica monitor tick |
| `Replica.MaxSyncLTXFiles` | 256 | Max L0 files uploaded per sync batch |

### Startup order

`main.go` → `replication.Start` (`internal/replication/replication.go`):

1. `litestream.NewDB(DB_PATH)` — build the DB struct (file not opened yet).
2. Build the replica client: `NewReplicaClientFromURL(LITESTREAM_REPLICA)` when set,
   else `file.NewReplicaClient(<db dir>/litestream)`.
3. `NewReplicaWithClient(db, client)` and wire it onto `db.Replica`.
4. `client.Init(ctx)` — idempotent backend init.
5. `db.EnsureExists(ctx)` — **if the local file is missing**, create its parent dir and
   restore from the replica (no-op on a fresh start with no backup yet). This is what lets
   a fresh pod rebuild the database from S3.
6. `NewStore([]*DB{db}, levels)` then `store.Open(ctx)`: validates the compaction levels,
   opens each DB (starts the WAL-monitor goroutine), and starts one compaction-monitor
   goroutine per non-L0 level **plus** one for the snapshot level, and the L0-retention monitor.

Note that `db.Open()` only starts the monitor loop; the actual SQLite connection is opened
lazily on the first sync (`db.init`). That init sets `wal_autocheckpoint(0)` on litestream's
own connection and holds a long-running read lock so that **litestream alone controls
checkpointing** — the app's connections never checkpoint on their own.

### The replication loop

A background loop (the `DB` monitor, every `DB.MonitorInterval`, default 1s) repeatedly
calls `DB.Sync`. Each sync:

1. reads new WAL frames since the last replicated position,
2. writes them to the replica as LTX files (`ReplicaClient.WriteLTXFile`),
3. runs the checkpoint policy below.

On repeated errors the loop backs off exponentially (1s → 2s → … up to 5m) and
rate-limits error logging.

### Checkpointing (when & how)

After each sync, `checkpointIfNeeded` (`db.go`) applies a 3-tier policy, in priority order:

1. **`TruncatePageN` (~500MB)** — TRUNCATE mode, *blocking*. Emergency brake against
   runaway WAL growth (e.g. a long-lived reader). Tries a PASSIVE checkpoint first and only
   blocks if that cannot restart the WAL.
2. **`MinCheckpointPageN` (~4MB)** — PASSIVE mode, *non-blocking*, by size.
3. **`CheckpointInterval` (1m)** — PASSIVE mode, *non-blocking*, by time; only fires if data
   was actually synced since the last checkpoint (avoids producing empty LTX files).

The old RESTART mode was removed upstream (it could block writes indefinitely). Each
checkpoint seals the WAL, which is what creates the LTX file boundaries that get uploaded.

### Compaction (when & how)

LTX files live in LSM-style levels: level 0 holds raw LTX files and higher levels merge the
previous level into larger time granularities. Our config uses two tiers:

```go
CompactionLevels{{Level: 0}, {Level: 1, Interval: 10 * time.Second}}
```

so L0→L1 compaction is attempted every 10s. `store.Open` runs a `monitorCompactionLevel`
goroutine per tier; each wakes aligned to the tier's `NextCompactionAt` and calls
`Store.CompactDB`, which:

- skips a DB that is not ready yet (`PageSize() == 0`),
- skips re-compacting within the same interval window (`ErrCompactionTooEarly`),
- for the **snapshot tier**: writes a full snapshot if the last one is older than the current
  position (`ErrNoCompaction` otherwise), and
- otherwise merges the previous level into this one (`db.Compact`).

Compaction also drives retention: compacting into L1 deletes L0 files older than
`L0Retention` (5m), and the snapshot tier enforces `SnapshotRetention` (24h) while always
keeping at least one snapshot.

### Snapshots & restore

- A **snapshot** is a full copy of the database at the current position, written to the
  replica's snapshot tier every `SnapshotInterval` (24h). Snapshots are the recovery base.
- **Restore** happens at startup via `db.EnsureExists` (step 5 above): if the local file is
  gone, litestream replays the replica's LTX files on top of the newest snapshot to rebuild
  it. If there is no backup yet it simply creates a fresh database.

### Shutdown order

`main.go` uses LIFO `defer`s so the app connection closes **before** the store:

1. `conn.Close()` — stops the app from writing new WAL frames.
2. `store.Close(shutdownCtx)` — a final sync flushes any remaining WAL to the replica, then
   the compaction/snapshot/retention monitors are torn down. A fresh 30s context is used so
   the shutdown is not cut short by the already-cancelled request context.

Keeping a single writer (`replicaCount: 1`) matters: two replicas writing the same file would
fight over the WAL.

