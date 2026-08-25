# Tasks

## Create basic JSON API for the url shortener

We'll need two endpoints:

- GET `/<slug>` doing the correct 3xx redirect
- POST `/_/create_link` for creating a new link
- POST `/_/slug_availability` for checking if a slug is available to be populated

Let's not worry about rate-limiting right now

The POST endpoints should be easy to called using https://alpine-ajax.js.org/

### Acceptance Criteria

- The endpoints return JSON
- The endpoints perform the correct db mutations
- The endpoints return human-readable JSON errors and error HTTP codes on failure
- the POST endpoints handle CORS correctly
- the POST endpoint can be conveniently called with alpine-ajax

### Dependencies

- alpine.js and alpine ajax scripts placed in `static/js/`

## Notes

### Implementation Approach

- **TDD workflow**: Tests written first using testify/suite pattern, then implementation to make them pass
- **Files created**:
  - `internal/server/handlers.go` — HTTP handlers, request/response types, validation functions
  - `internal/server/handlers_test.go` — comprehensive test suite (15+ test cases)
  - `internal/link/slugexists_test.go` — tests for the new SlugExists service method
- **Files modified**:
  - `internal/server/server.go` — added dependency injection, API routes, CORS middleware
  - `main.go` — service wiring (link.Service, click.Service passed to server.Start)
  - `internal/link/service.go` — added SlugExists method to service layer

### Key Design Decisions

- **Dependency injection**: Services are created in `main.go` and passed to `server.Start()` rather than created inside the server package. This makes testing easier and keeps concerns separated.
- **Validation rules**:
  - URLs must use `http` or `https` scheme (rejects `javascript:`, `ftp:`, etc.)
  - Slugs: alphanumeric + hyphens + underscores, max 50 chars, cannot start with underscore (reserved for system routes like `/_/`)
- **CORS**: Configured with `AllowOrigins: ["*"]` for public API access. POST endpoints only.
- **Error handling**: JSON responses with human-readable messages. Domain errors mapped to HTTP codes:
  - `ErrNotFound` → 404
  - `ErrRevoked` → 410
  - `ErrSlugTaken` → 409
  - Validation errors → 422
  - Malformed JSON → 400
- **Click tracking**: Async via channel-based service. `Record()` is non-blocking and drops clicks silently if buffer is full.

### Gotchas

- **`SlugExists` treats revoked links as "taken"**: The query `SELECT EXISTS(SELECT 1 FROM links WHERE slug = ?)` does not filter on `revoked_at`, so revoked slugs cannot be re-used. This may need clarification — should revoked slugs be reclaimable?
- **No rate limiting yet**: The API is currently open. Rate limiting should be added before production use.
- **Click tracking is lossy**: Clicks in the channel buffer are lost on crash. This is an intentional tradeoff for availability over perfect analytics.
- **CORS uses wildcard origin**: `AllowOrigins: ["*"]` is permissive. If the API needs to be restricted to specific domains in the future, this will need to be updated.

### Follow-up Tasks

- **Rate limiting**: Add rate limiting to prevent abuse (e.g., per-IP or per-slug creation limits)
- **Analytics dashboard**: Build UI to view click counts, referrers, and geographic data
- **Link management UI**: Create pages for editing/revoking links using claim keys
- **Slug reclamation**: Decide if revoked slugs should be reclaimable and update `SlugExists` logic accordingly
- **API documentation**: Consider adding OpenAPI/Swagger spec for the JSON API
- **Input sanitization**: Consider additional URL validation (e.g., rejecting private IPs, localhost)

### Testing

- **Coverage**: 76.2% overall, 64% for server package
- **Test pattern**: Uses `testify/suite` with `SetupTest()`/`TearDownTest()` and temp SQLite databases
- **Test cases**: 15+ scenarios covering all endpoints, validation rules, error conditions, and edge cases
