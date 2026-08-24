# AGENTS.md

## Commands

```bash
make check        # goimports + coverage + vet + staticcheck (run before committing)
make test         # gotest ./...
make coverage     # tests with coverage report
make run          # build and run (server on :30666)
make build        # go build
```

## Architecture

- **Entry point**: `main.go` → starts replication, opens DB, runs migrations, starts Echo server
- **internal/db**: SQLite connection + goose migrations (embedded from `internal/db/migrations/`)
- **internal/replication**: Litestream library-mode WAL replication
- **internal/server**: Echo v5 web server, templates in `templates/`, views in `views/`
- **internal/env**: Config via env vars (`SERVER_PORT`, `DB_PATH`, `LITESTREAM_REPLICA_URL`, `LOG_LEVEL`)
- **internal/site**: Site config from `priv/site.yml`

## Critical Constraints

- **SQLite driver**: Must use `modernc.org/sqlite` (pure Go). Do NOT add `mattn/go-sqlite3` (cgo) — it conflicts with Litestream's POSIX locks.
- **Migrations**: Use goose. Add `.sql` files to `internal/db/migrations/` with `-- +goose Up` / `-- +goose Down` markers. They're embedded via `//go:embed`.
- **Shutdown order**: LIFO defers — app `conn.Close()` runs before `store.Close()` (Litestream final sync).
- **Bash scripts**: When running ephemeral debugging commands or one-off scripts during a session, use relative paths. Absolute paths may be blocked by opencode's sandbox, which restricts writes outside the workspace.

## Testing

- **Framework**: Use `github.com/mvrahden/go-test/pkg/gotest` for all tests. Write tests for any new functionality that's not very difficult to test.
- **Suite pattern**: Use `type XxxTestSuite struct{}` with `BeforeEach(t *gotest.T)` for setup and `TestXxx(t *gotest.T)` methods with `t.It("description", func(it *gotest.T) { ... })` subtests.
- **Assertions**: Use `gotest.Equal`, `gotest.NoError`, `gotest.True`, etc. instead of manual `if err != nil { t.Fatalf(...) }` patterns.

## Environment

| Variable | Default | Notes |
|----------|---------|-------|
| `SERVER_PORT` | `30666` | |
| `DB_PATH` | `db/filedb.sqlite` | |
| `LITESTREAM_REPLICA_URL` | empty | Empty = local `db/litestream/` dir; set `s3://...` for object store |
| `LOG_LEVEL` | | Set `debug` for verbose logging |

## Versions

- App version: `APP_VERSION.txt`
- Helm chart version: `CHART_VERSION.txt`
- Go version: `.tool-versions` (mise)

## Static Assets

- `static/` — served at `/`
- `templates/` — base layout partials (`base.html`, `head.html`, etc.)
- `views/` — page templates (rendered inside `base.html`)
