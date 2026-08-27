# grr.fyi

## TODO

- [x] modernc sqlite
- [x] litestream
- [x] partials / general layout
- [x] S3 / scaleway object storage replication
- [ ] altcha

## URL Shortener Data Model

### Schema Overview

Migrations in `internal/db/migrations/`:

- **`00001_init.sql`** - Creates the `meta` table for key-value storage
- **`00002_url_shortener.sql`** - Core URL shortener tables:
  - **`links`** - The core aggregate: slug, target_url, claim_key_hash, timestamps
  - **`clicks`** - Async click tracking with ip_hash, referrer, country (nullable)
  - **`slug_sequence`** - Singleton table (id=1) tracking the next auto-slug value
- **`00003_clicks_composite_index.sql`** - Composite index on clicks(link_id, ip_hash) for distinct IP queries

### Dynamic Slug Length

Early links get shorter slugs, later ones get longer. We use **sequential base64 encoding**:

- `slug_sequence.next_value` starts at 262144 (ensures minimum 4-character slugs)
- ID 262144 → first 4-char slug, ID 262145 → next, etc.
- Base64 charset: `9hZPFa2KlLX6rTwJkHItzxyYs0Vd4bfAMUQGmvpCjRuoDieO5c7nNqWE38gS1B-_` (64 characters, shuffled)
- Each new link increments the sequence

**Why not random?** Sequential encoding is deterministic, collision-free, and naturally produces short slugs early. The tradeoff is that someone could estimate total link count by decoding slugs, but this is acceptable for our use case.

**Why start at 262144?** This ensures all auto-generated slugs are at least 4 characters long, avoiding very short slugs that might be confusing or easily guessable.

### Custom Slugs

Custom slugs coexist with auto-generated ones:

1. If `CustomSlug` is provided → use it directly
2. If empty → read `slug_sequence.next_value`, encode to base62, increment sequence
3. If custom slug collides → return `ErrSlugTaken`

Custom slugs don't interfere with the sequence. The sequence only advances for auto-generated slugs.

### Claim Key System

No user accounts. Instead, each link has a **claim key** for edit/revoke operations:

- **Generation**: 8 random bytes → base64 encoded (variable length, max ~11 characters)
- **Storage**: SHA-256 hashed (hex-encoded) before storing in `claim_key_hash`
- **Return**: Plaintext shown once in `CreateResponse.ClaimKey`
- **Verification**: `sha256.Sum256(submitted_key)` compared to stored hash

**Why hash?** If the database is compromised, attackers can't forge claim keys. Like passwords: verify without storing the secret.

**UX implication**: Lost claim key = lost link. No recovery possible.

### Async Click Tracking

Clicks are recorded asynchronously to avoid adding latency to redirects:

- `click.Service` has a buffered channel (configurable size)
- `Record()` enqueues click info, drops silently if buffer full
- Worker goroutine drains the channel and inserts into `clicks` table
- `Close()` flushes remaining clicks before shutdown

**Tradeoff**: Clicks in the channel are lost on crash. We prioritize availability over perfect analytics.

### Domain Concepts

| Term | Meaning |
|------|---------|
| **Link** | Aggregate root - a shortened URL record |
| **Slug** | The short identifier (e.g., `abc123` in `grr.fyi/abc123`) |
| **Target URL** | The destination URL being shortened |
| **Claim Key** | Secret token proving ownership (for edit/revoke) |

### Service API

**`internal/link.Service`**:
- `Create(ctx, CreateRequest) (*CreateResponse, error)` - Create link with custom or auto slug
- `Resolve(ctx, slug) (*Link, error)` - Get active link (returns `ErrNotFound`/`ErrRevoked`)
- `Get(ctx, slug) (*Link, error)` - Get any link including revoked (for edit page)
- `GetWithClaimKey(ctx, slug, claimKey) (*Link, error)` - Get link and verify claim key
- `SlugExists(ctx, slug) (bool, error)` - Check if slug is taken (includes revoked links)
- `Update(ctx, slug, claimKey, newTarget) error` - Update target URL
- `Revoke(ctx, slug, claimKey) error` - Mark link as revoked

**`internal/click.Service`**:
- `Record(ctx, Info) error` - Enqueue click (non-blocking, drops if full)
- `Count(ctx, linkID) (int64, error)` - Aggregate count from clicks table
- `CountDistinctIPs(ctx, linkID) (int64, error)` - Count unique IP hashes
- `Stats(ctx, linkID) (*ClickStats, error)` - Get total clicks and distinct IPs in one query
- `Close()` - Graceful shutdown, flush remaining clicks
- `Flush()` - Wait for all pending clicks to be written

## JSON API

The URL shortener exposes a JSON API for creating and resolving shortened links. All POST endpoints return JSON and support CORS.

### Endpoints

#### `GET /:slug` — Redirect to target URL

Resolves a shortened link and redirects to the target URL. Records a click asynchronously.

**Response:**
- `302 Found` — Redirect to target URL (active link)
- `404 Not Found` — Slug does not exist
- `410 Gone` — Link has been revoked

**Example:**
```bash
curl -L https://grr.fyi/abc123
# Redirects to the target URL
```

---

#### `POST /_/api/create_link` — Create a new shortened link

Creates a new shortened URL. You can optionally specify a custom slug.

**Request:**
```json
{
  "target_url": "https://example.com/very/long/url",
  "custom_slug": "mylink"  // optional, omit for auto-generated slug
}
```

**Response (201 Created):**
```json
{
  "slug": "mylink",
  "short_url": "https://grr.fyi/mylink",
  "claim_key": "aB3xY9kL2mN"
}
```

**Error Responses:**
- `400 Bad Request` — Malformed JSON
- `409 Conflict` — Custom slug already taken
- `422 Unprocessable Entity` — Validation error (invalid URL or slug format)

**Validation Rules:**
- `target_url` must be a valid HTTP or HTTPS URL (rejects `javascript:`, `ftp:`, etc.)
- `custom_slug` (if provided) must be 1-50 characters, alphanumeric + hyphens + underscores, cannot start with `_`

**Example:**
```bash
curl -X POST https://grr.fyi/_/api/create_link \
  -H "Content-Type: application/json" \
  -d '{"target_url": "https://example.com", "custom_slug": "demo"}'
```

**Important:** The `claim_key` is shown only once. Store it securely — it's required to edit or revoke the link. Lost claim key = lost link.

---

#### `POST /_/api/slug_availability` — Check if a custom slug is available

Checks whether a custom slug is available before attempting to create a link.

**Request:**
```json
{
  "slug": "mylink"
}
```

**Response (200 OK):**
```json
{
  "available": true
}
```

**Error Responses:**
- `400 Bad Request` — Malformed JSON
- `422 Unprocessable Entity` — Invalid slug format

**Example:**
```bash
curl -X POST https://grr.fyi/_/api/slug_availability \
  -H "Content-Type: application/json" \
  -d '{"slug": "demo"}'
```

**Note:** A slug is considered "taken" if it exists in the database, even if the link has been revoked. Revoked slugs cannot be re-used.

---

#### `GET /_/edit_link/:slug?claim_key=...` — Edit link page

Displays the edit page for a link after verifying the claim key. Shows the target URL, short URL, edit page URL, and click statistics (total clicks and distinct IPs).

**Query Parameters:**
- `claim_key` — The claim key for the link (required)

**Response:**
- `200 OK` — Edit page rendered (HTML)
- `400 Bad Request` — Missing claim key
- `401 Unauthorized` — Invalid claim key
- `404 Not Found` — Slug does not exist
- `410 Gone` — Link has been revoked

**Example:**
```bash
curl "https://grr.fyi/_/edit_link/demo?claim_key=aB3xY9kL2mN"
# Returns HTML edit page with link details and stats
```

---

### CORS

All POST endpoints (`/_/api/create_link`, `/_/api/slug_availability`) support CORS with `Access-Control-Allow-Origin: *`. This allows the API to be called from any domain.

**Preflight request:**
```bash
curl -X OPTIONS https://grr.fyi/_/api/create_link \
  -H "Origin: https://example.com" \
  -H "Access-Control-Request-Method: POST" \
  -H "Access-Control-Request-Headers: Content-Type"
```

### Using with Alpine.js

The API is designed to work seamlessly with [Alpine Ajax](https://alpine-ajax.js.org/):

```html
<form ax-post="/_/api/create_link" ax-target="#result">
  <input type="url" name="target_url" required>
  <input type="text" name="custom_slug">
  <button type="submit">Shorten</button>
</form>

<div id="result"></div>
```

### Error Format

All errors follow a consistent JSON format:

```json
{
  "error": "human-readable error message"
}
```

## SQLite persistence & Litestream replication

The app stores data in a single SQLite file (`DB_PATH`, default `db/filedb.sqlite`)
and replicates it in the background with [litestream](https://github.com/benbjohnson/litestream)
in **library mode**. When `LITESTREAM_REPLICA_URL` is set (e.g. `s3://bucket/path`) the
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

### Performance PRAGMAs

The app connection uses several PRAGMAs to optimize SQLite performance (set in `internal/db/db.go`):

| PRAGMA | Value | Purpose |
| --- | --- | --- |
| `journal_mode` | `WAL` | Write-ahead logging for concurrent reads/writes |
| `synchronous` | `NORMAL` | Reduced fsync calls (safe with WAL mode) |
| `cache_size` | `-64000` | 64MB page cache (negative = KB) |
| `mmap_size` | `268435456` | 256MB memory-mapped I/O |
| `temp_store` | `MEMORY` | Store temp tables in memory |
| `foreign_keys` | `ON` | Enforce foreign key constraints |
| `wal_autocheckpoint` | `0` | Disabled (litestream controls checkpointing) |
| `busy_timeout` | `5000` | Wait 5s for locks instead of failing immediately |

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
2. Build the replica client: `NewReplicaClientFromURL(LITESTREAM_REPLICA_URL)` when set,
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

## Site Configuration

Site-specific settings (name, URL, etc.) are loaded from `priv/site.yml` (configurable via `SITE_FILE_PATH`). Environment variables prefixed with `SITE_` override YAML values — for example, `SITE_URL=https://example.com/` overrides the `url` field. The site config is accessible in templates via the `site` function (e.g., `{{ site "url" }}`).

