# AGENTS.md

This file provides guidance for agentic coding agents working in this repository.

## Development Setup

### Prerequisites

- Go 1.25.6 or later
- make
- git

The project uses Go Modules (`go.mod`) for dependency management.

### Installing Dependencies

Run the following to install required tools:

```bash
make install
```

This installs:
- `goimports` for import formatting
- `revive` for linting
- `staticcheck` for static analysis
- `govulncheck` (currently commented out due to compatibility issues)

## Build, Lint, and Test Commands

### Running Tests

**Run all tests:**
```bash
make test
```

**Run a specific test:**
```bash
# Run all test functions in the env package
go test ./internal/env/...

# Run a specific test function
# Note: Go does not have built-in support for running single test functions
# Use -run flag to filter by name
go test ./internal/env/... -run TestDefaultValues
go test ./internal/env/... -run TestSettingCustomPort
```

### Linting and Code Quality

**Run all checks:**
```bash
make check
```

This executes:
- `goimports` - formats imports and reports unfixed files
- `go vet` - examines Go code and reports suspicious constructs
- `staticcheck` - advanced static analysis
- `revive` - configurable linter (with minimal config in `revive_config.toml`)

### Building

**Build for current platform:**
```bash
make build
```

**Build for all platforms:**
```bash
make build-all
```

Builds binaries in the `build/` directory for:
- Linux amd64/arm64
- Darwin (macOS) amd64/arm64

### Running

**Run the application:**
```bash
make run
```

This starts the web server with the configuration from environment variables.

### Docker

**Build and push Docker images:**
```bash
make push-docker
```

## Code Style Guidelines

### Imports

- Group imports by standard library, external packages, and internal packages
- No blank lines between import groups
- No commented-out imports
- Run `goimports -l -w .` to fix imports

Example:
```go
import (
    "context"
    "fmt"
    "io"

    "github.com/labstack/echo/v5"
    "github.com/nietaki/epstein-file-review/internal/env"
    "github.com/nietaki/epstein-file-review/internal/filedb"
)
```

### Formatting

- Use `go fmt` for basic formatting
- Use `goimports` for import organization
- No trailing whitespace
- Lines should not exceed 80 characters (soft limit)
- Use spaces for indentation (tabs are not used in this codebase)

### Naming Conventions

**Packages:**
- Use lowercase, single word names (e.g., `env`, `server`, `filedb`)
- No underscores
- Group related functionality in packages under `internal/`

**Variables and Functions:**
- Use camalCase (not snake_case or PascalCase)
- Be descriptive but concise
- Example: `filedb.GetRandomFilenameByQuery`, not `getRandomFilenameByQuery` or `GetRandomFilenameByQuery`

**Constants:**
- Use ALL_CAPS with underscores
- Example: `SIGNATURE_VALIDITY_SECONDS`

**Types:**
- Use PascalCase for custom types
- Example: `Template` struct

### Error Handling

- Check errors explicitly, don't ignore them
- Return errors to callers when appropriate
- Log meaningful error messages for debugging

Example:
```go
filetypes, err := echo.FormValues[string](c, "filetypes[]")
if err != nil {
    filetypes = []string{}
    // Optionally log: fmt.Printf("Error getting filetypes: %v\n", err)
}
```

### Testing

- Place test files in the same package as the code being tested
- Use `_test.go` suffix for test files
- Use standard `testing` package
- Write clear assertions and meaningful test names

Example structure:
```
internal/
  env/
    config.go
    config_test.go
  server/
    server.go
    server_test.go
```

Test patterns:
- `Test[X]` for unit tests (where X describes what's being tested)
- `Test[Description]` for tests describing behavior
- Use `t.Errorf`, `t.Fatal`, etc. for assertions

### Documentation Comments

- Use Go's standard comment style
- Place comments directly above the function/type they describe
- No need for excessive commentary, the code should be self-documenting

### Project Structure

```
.github/         - GitHub-specific configurations
static/          - Static files (fonts, images, CSS, JS)
public/views/    - HTML templates
internal/        - Internal packages (not importable by external code)
  env/           - Environment configuration
  filedb/        - File database operations
  server/        - HTTP server implementation
  signing/       - Signing and verification logic
  stats/         - Statistics collection
  util/          - Utility functions
main.go          - Application entry point
```

### Coding Standards

- Avoid panics, handle errors gracefully with proper error messages
- Use context for cancellation and timeouts (30-second timeout used in middleware)
- Keep functions focused and single-purpose
- Follow the principle of least surprise
- Use constants for magic numbers and strings (e.g., `SIGNATURE_VALIDITY_SECONDS`)

## Tooling Configuration

### revive (Linting)

Configured via `revive_config.toml` with minimal rules:
```toml
[rule.redefines-builtin-id]
```

This prevents redeclaring built-in identifiers (like `string`, `int`, etc.)

### Static Analysis

- Uses `staticcheck` for advanced linting
- Currently no custom configuration
- Reports potential bugs, performance issues, and suspicious constructs

### Imports Organization

- Uses `goimports` with `-l` (list) flag to report unfixed imports
- Uses `-w` (write) flag to fix imports in-place
- Runs `goimports -l -w .` before commits

## Architecture Notes

### Web Framework

- Uses Echo v5 framework (`github.com/labstack/echo/v5`)
- Template rendering with `html/template`
- Middleware for recovery, context timeouts
- No CORS or rate-limiting middleware (currently)

### Database

- SQLite for file metadata storage
- FileType determines how files are served (inline vs attachment)
- Supported file types: pdf, video, audio, image

### Security

- Signature validation for file access (JWT-like mechanism)
- Time-limited signatures (300 seconds validity)
- HMAC-SHA256 for signing

## Testing Conventions

- Tests should be deterministic and not rely on external systems
- Use `t.Setenv()` for setting environment variables in tests
- Clean up resources after tests when needed
- Include multiple test cases for edge cases

Example test structure:
```go
func TestDefaultValues(t *testing.T) {
    cfg := Load()

    if cfg.ServerPort != "30666" {
        t.Errorf("Expected default ServerPort to be '30666', got '%s'", cfg.ServerPort)
    }
}

func TestSettingCustomPort(t *testing.T) {
    t.Setenv("SERVER_PORT", "8080")

    cfg := Load()

    if cfg.ServerPort != "8080" {
        t.Errorf("Expected ServerPort to be '8080', got '%s'", cfg.ServerPort)
    }
}
```

## Build Process

1. Dependencies are managed via Go Modules
2. Run `make check` to validate code quality before building
3. Use `make build` for local development builds
4. Use `make build-all` to create binaries for all supported platforms
5. Use `make push-docker` for production Docker builds

## Environment Variables

- Application uses environment variable based configuration (via `caarlos0/env/v11`)
- Default server port: 30666
- Set `SERVER_PORT` to customize the port
- Set `SIGNING_SECRET` to customize the signing secret (defaults to `bnh!gng7waf3BKD-zgd`)

## Key Development Workflow

1. Make changes to the code
2. Run `make build` to test the build process
3. Run `make test` to run all tests
4. Run `make check` to ensure code quality
5. Address the uncovered issues - default to fixing the tested code, not disabling any of the checks

NOTE: don't run ` | head -n <SOME_VALUE>` on the output of any of the `make <TARGET>` commands - all of the lines may be relevant.
