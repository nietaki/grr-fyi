# AGENTS.md

## Commands

```bash
make check        # goimports + coverage + vet + staticcheck (run before committing)
make test         # go test ./...
make coverage     # tests with coverage report
make run          # build and run (server on :30666)
make run-pprof    # build and run with pprof endpoints enabled
make build        # go build
```

### Profiling (requires `make run-pprof` running in another terminal)

```bash
make profile-cpu        # Collect 30s CPU profile, open flamegraph on :30669
make profile-heap       # Collect heap profile, open on :30669
make profile-goroutine  # Show goroutine count and first 20 stacks
make pprof-shell        # Interactive pprof shell (CPU, 30s)
```

## Architecture

- **Entry point**: `main.go` → conditionally starts replication (if `REPLICATION_ENABLED=true`), opens DB, runs migrations, starts Echo server
- **internal/db**: SQLite connection + goose migrations (embedded from `internal/db/migrations/`)
- **internal/replication**: Litestream library-mode WAL replication
- **internal/server**: Echo v5 web server, templates in `templates/`, views in `views/`
- **internal/env**: Config via env vars (`SERVER_PORT`, `DB_PATH`, `LITESTREAM_REPLICA_URL`, `REPLICATION_ENABLED`, `PPROF_ENABLED`, `LOG_LEVEL`)
- **internal/site**: Site config from `priv/site.yml`

## Critical Constraints

- **SQLite driver**: Must use `modernc.org/sqlite` (pure Go). Do NOT add `mattn/go-sqlite3` (cgo) — it conflicts with Litestream's POSIX locks.
- **Migrations**: Use goose. Add `.sql` files to `internal/db/migrations/` with `-- +goose Up` / `-- +goose Down` markers. They're embedded via `//go:embed`.
- **Shutdown order**: LIFO defers — app `conn.Close()` runs before `store.Close()` (Litestream final sync).
- **Bash scripts**: When running ephemeral debugging commands or one-off scripts during a session, use relative paths. Absolute paths may be blocked by opencode's sandbox, which restricts writes outside the workspace.

## Testing

- **Framework**: Use `github.com/stretchr/testify` (with `testify/suite` and `testify/require`) for all tests. Write tests for any new functionality that's not very difficult to test.
- **Suite pattern**: Use `type XxxTestSuite struct { suite.Suite }` with `SetupTest()` for setup, `TearDownTest()` for teardown, and `TestXxx()` methods with `s.T().Run("description", func(t *testing.T) { ... })` subtests. Add a `func TestXxxSuite(t *testing.T) { suite.Run(t, new(XxxTestSuite)) }` runner.
- **Assertions**: Use `require.NoError`, `require.Equal`, `require.True`, etc. (from `testify/require`) instead of manual `if err != nil { t.Fatalf(...) }` patterns. Use `require` (not `assert`) so tests stop on failure.

## Feature Implementation Approach

Follow a **TDD (Test-Driven Development) workflow** for new features:

### RED → GREEN → REFACTOR

**1. RED Stage - Define Interface First**
- Start by defining the **function signatures and types** (the interface/contract) before writing tests
- This includes:
  - Function/method signatures with parameters and return types
  - Domain types (structs, interfaces)
  - Sentinel errors (e.g., `ErrNotFound`, `ErrInvalidInput`)
- Then write failing tests that exercise the interface
- Tests should fail to compile or run because the implementation doesn't exist yet

**2. GREEN Stage - Minimal Implementation**
- Implement just enough code to make the tests pass
- Don't over-engineer; focus on the specific behavior being tested
- Resist adding features not covered by tests

**3. REFACTOR Stage - Clean Up**
- Improve code structure while keeping tests green
- Extract common patterns, improve naming, reduce duplication
- Run `make check` to ensure code quality

### Example Flow

For a new `CreateLink` function:
1. Define `CreateRequest`, `CreateResponse`, `ErrSlugTaken` in `link/types.go`
2. Define `func (s *Service) Create(ctx context.Context, req CreateRequest) (*CreateResponse, error)` signature
3. Write tests in `create_test.go` that fail (implementation returns nil or placeholder)
4. Implement `Create` to make tests pass
5. Refactor if needed

## Environment

| Variable | Default | Notes |
|----------|---------|-------|
| `SERVER_PORT` | `30666` | |
| `DB_PATH` | `db/filedb.sqlite` | |
| `LITESTREAM_REPLICA_URL` | empty | Empty = local `db/litestream/` dir; set `s3://...` for object store |
| `LITESTREAM_META_PATH` | `./litestream-cache` | Litestream local cache directory (separate from DB) |
| `REPLICATION_ENABLED` | `false` | Set `true` to enable Litestream WAL replication |
| `PPROF_ENABLED` | `false` | Set `true` to enable `/debug/pprof/*` endpoints |
| `LOG_LEVEL` | | Set `debug` for verbose logging |
| `ALTCHA_SECRET` | empty | HMAC secret for ALTCHA captcha. Empty = captcha disabled |
| `ALTCHA_COST` | `5000` | PBKDF2 iterations for ALTCHA challenge |
| `ALTCHA_EXPIRY_MINUTES` | `10` | Challenge expiry time in minutes |

### Environment Configuration Files

The project uses direnv with a layered configuration approach:

- **`.envrc`** — Main environment configuration (committed to git). Sets non-sensitive defaults like `REPLICATION_ENABLED`, `SITE_URL`, and `LITESTREAM_REPLICA_URL`. Sources `.envrc-priv` if it exists.
- **`.envrc-priv-sample`** — Template showing what sensitive credentials should be placed in `.envrc-priv` (committed to git). Use this as a reference for required secrets.
- **`.envrc-priv`** — Actual sensitive credentials like API keys and secrets (gitignored, NOT in repository).

**IMPORTANT**: Agents should NEVER read or write `.envrc-priv`. Only reference `.envrc-priv-sample` when documenting required environment variables or helping users configure their setup.

## Versions

- App version: `APP_VERSION.txt`
- Helm chart version: `CHART_VERSION.txt`
- Go version: `.tool-versions` (mise)

## Static Assets

- `static/` — served at `/`
- `templates/` — base layout partials (`base.html`, `head.html`, etc.)
- `views/` — page templates (rendered inside `base.html`)
