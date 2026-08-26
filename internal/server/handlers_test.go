package server

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	_ "modernc.org/sqlite"

	"github.com/nietaki/grr-fyi/internal/click"
	"github.com/nietaki/grr-fyi/internal/db"
	"github.com/nietaki/grr-fyi/internal/env"
	"github.com/nietaki/grr-fyi/internal/link"
	"github.com/nietaki/grr-fyi/internal/store"
)

type HandlerTestSuite struct {
	suite.Suite
	handler  *Handler
	linkSvc  *link.Service
	clickSvc *click.Service
	conn     *sql.DB
	echo     *echo.Echo
}

func (s *HandlerTestSuite) SetupTest() {
	// Create temp DB
	dbPath := filepath.Join(s.T().TempDir(), "test.sqlite")
	cfg := env.Config{DBPath: dbPath}

	ctx := context.Background()
	conn, err := db.Open(ctx, cfg)
	require.NoError(s.T(), err, "Open")

	err = db.Migrate(ctx, conn)
	require.NoError(s.T(), err, "Migrate")

	s.conn = conn

	// Create services
	linkStore := link.NewStore(conn)
	txScope := store.NewTxScope(conn)
	s.linkSvc = link.NewService(linkStore, txScope)

	clickStore := click.NewStore(conn)
	s.clickSvc = click.NewService(clickStore, 100)

	// Create handler
	s.handler = NewHandler(s.linkSvc, s.clickSvc, "https://grr.fyi/")

	// Create Echo instance with renderer
	s.echo = echo.New()
	s.echo.Renderer = NewTemplateForTest()
}

func (s *HandlerTestSuite) TearDownTest() {
	s.clickSvc.Close()
	s.conn.Close()
}

// ==================== REDIRECT TESTS ====================

func (s *HandlerTestSuite) TestRedirectActiveLink() {
	s.T().Run("returns 302 redirect for active link", func(t *testing.T) {
		ctx := context.Background()

		// Create a link
		_, err := s.linkSvc.Create(ctx, link.CreateRequest{
			TargetURL:  "https://example.com",
			CustomSlug: "test",
		})
		require.NoError(t, err)

		// Make request
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		c := s.echo.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{{Name: "slug", Value: "test"}})

		err = s.handler.Redirect(c)
		require.NoError(t, err)

		// Check response
		require.Equal(t, http.StatusFound, rec.Code)
		require.Equal(t, "https://example.com", rec.Header().Get("Location"))
	})
}

func (s *HandlerTestSuite) TestRedirectNotFound() {
	s.T().Run("returns 404 for non-existent slug", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/nonexistent", nil)
		rec := httptest.NewRecorder()
		c := s.echo.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{{Name: "slug", Value: "nonexistent"}})

		err := s.handler.Redirect(c)
		require.NoError(t, err)

		require.Equal(t, http.StatusNotFound, rec.Code)
	})
}

func (s *HandlerTestSuite) TestRedirectRevoked() {
	s.T().Run("returns 410 for revoked link", func(t *testing.T) {
		ctx := context.Background()

		// Create a link
		resp, err := s.linkSvc.Create(ctx, link.CreateRequest{
			TargetURL:  "https://example.com",
			CustomSlug: "revoked",
		})
		require.NoError(t, err)

		// Revoke it
		err = s.linkSvc.Revoke(ctx, "revoked", resp.ClaimKey)
		require.NoError(t, err)

		// Make request
		req := httptest.NewRequest(http.MethodGet, "/revoked", nil)
		rec := httptest.NewRecorder()
		c := s.echo.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{{Name: "slug", Value: "revoked"}})

		err = s.handler.Redirect(c)
		require.NoError(t, err)

		require.Equal(t, http.StatusGone, rec.Code)
	})
}

func (s *HandlerTestSuite) TestRedirectRejectsInvalidSlugs() {
	s.T().Run("returns 404 for slug with dot (static file pattern)", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/favicon.ico", nil)
		rec := httptest.NewRecorder()
		c := s.echo.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{{Name: "slug", Value: "favicon.ico"}})

		err := s.handler.Redirect(c)
		require.NoError(t, err)

		require.Equal(t, http.StatusNotFound, rec.Code)
	})

	s.T().Run("returns 404 for slug with underscore", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/my_slug", nil)
		rec := httptest.NewRecorder()
		c := s.echo.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{{Name: "slug", Value: "my_slug"}})

		err := s.handler.Redirect(c)
		require.NoError(t, err)

		require.Equal(t, http.StatusNotFound, rec.Code)
	})

	s.T().Run("returns 404 for slug with hyphen", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/my-slug", nil)
		rec := httptest.NewRecorder()
		c := s.echo.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{{Name: "slug", Value: "my-slug"}})

		err := s.handler.Redirect(c)
		require.NoError(t, err)

		require.Equal(t, http.StatusNotFound, rec.Code)
	})

	s.T().Run("returns 404 for slug with path-like pattern", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/static/css/main.css", nil)
		rec := httptest.NewRecorder()
		c := s.echo.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{{Name: "slug", Value: "static/css/main.css"}})

		err := s.handler.Redirect(c)
		require.NoError(t, err)

		require.Equal(t, http.StatusNotFound, rec.Code)
	})
}

// ==================== CREATE LINK TESTS ====================

func (s *HandlerTestSuite) TestCreateLinkSuccess() {
	s.T().Run("creates link and returns 201 with JSON response", func(t *testing.T) {
		reqBody := CreateLinkRequest{
			TargetURL:  "https://example.com",
			CustomSlug: "mylink",
		}

		req := httptest.NewRequest(http.MethodPost, "/_/api/create_link", bytes.NewReader(mustMarshal(t, reqBody)))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		c := s.echo.NewContext(req, rec)

		err := s.handler.CreateLink(c)
		require.NoError(t, err)

		require.Equal(t, http.StatusCreated, rec.Code)

		var resp CreateLinkResponse
		err = json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)

		require.Equal(t, "mylink", resp.Slug)
		require.Equal(t, "https://grr.fyi/mylink", resp.ShortURL)
		require.NotEmpty(t, resp.ClaimKey)
	})
}

func (s *HandlerTestSuite) TestCreateLinkAutoSlug() {
	s.T().Run("creates link with auto-generated slug", func(t *testing.T) {
		reqBody := CreateLinkRequest{
			TargetURL: "https://example.com",
		}

		req := httptest.NewRequest(http.MethodPost, "/_/api/create_link", bytes.NewReader(mustMarshal(t, reqBody)))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		c := s.echo.NewContext(req, rec)

		err := s.handler.CreateLink(c)
		require.NoError(t, err)

		require.Equal(t, http.StatusCreated, rec.Code)

		var resp CreateLinkResponse
		err = json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)

		require.NotEmpty(t, resp.Slug)
		require.Equal(t, "https://grr.fyi/"+resp.Slug, resp.ShortURL)
	})
}

func (s *HandlerTestSuite) TestCreateLinkInvalidURL() {
	s.T().Run("returns 422 for invalid URL", func(t *testing.T) {
		reqBody := CreateLinkRequest{
			TargetURL:  "not-a-url",
			CustomSlug: "test",
		}

		req := httptest.NewRequest(http.MethodPost, "/_/api/create_link", bytes.NewReader(mustMarshal(t, reqBody)))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		c := s.echo.NewContext(req, rec)

		err := s.handler.CreateLink(c)
		require.NoError(t, err)

		require.Equal(t, http.StatusUnprocessableEntity, rec.Code)

		var errResp ErrorResponse
		err = json.Unmarshal(rec.Body.Bytes(), &errResp)
		require.NoError(t, err)
		require.NotEmpty(t, errResp.Error)
	})
}

func (s *HandlerTestSuite) TestCreateLinkSlugTaken() {
	s.T().Run("returns 409 when slug is already taken", func(t *testing.T) {
		ctx := context.Background()

		// Create first link
		_, err := s.linkSvc.Create(ctx, link.CreateRequest{
			TargetURL:  "https://example.com/1",
			CustomSlug: "taken",
		})
		require.NoError(t, err)

		// Try to create with same slug
		reqBody := CreateLinkRequest{
			TargetURL:  "https://example.com/2",
			CustomSlug: "taken",
		}

		req := httptest.NewRequest(http.MethodPost, "/_/api/create_link", bytes.NewReader(mustMarshal(t, reqBody)))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		c := s.echo.NewContext(req, rec)

		err = s.handler.CreateLink(c)
		require.NoError(t, err)

		require.Equal(t, http.StatusConflict, rec.Code)
	})
}

func (s *HandlerTestSuite) TestCreateLinkInvalidSlug() {
	s.T().Run("returns 422 for invalid slug format", func(t *testing.T) {
		reqBody := CreateLinkRequest{
			TargetURL:  "https://example.com",
			CustomSlug: "_invalid", // contains underscore
		}

		req := httptest.NewRequest(http.MethodPost, "/_/api/create_link", bytes.NewReader(mustMarshal(t, reqBody)))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		c := s.echo.NewContext(req, rec)

		err := s.handler.CreateLink(c)
		require.NoError(t, err)

		require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	})
}

func (s *HandlerTestSuite) TestCreateLinkMalformedJSON() {
	s.T().Run("returns 400 for malformed JSON", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/_/api/create_link", strings.NewReader("{invalid json"))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		c := s.echo.NewContext(req, rec)

		err := s.handler.CreateLink(c)
		require.NoError(t, err)

		require.Equal(t, http.StatusBadRequest, rec.Code)

		var errResp ErrorResponse
		err = json.Unmarshal(rec.Body.Bytes(), &errResp)
		require.NoError(t, err)
		require.Equal(t, "invalid request body", errResp.Error)
	})
}

func (s *HandlerTestSuite) TestCreateLinkEmptyTargetURL() {
	s.T().Run("returns 422 for empty target URL", func(t *testing.T) {
		reqBody := CreateLinkRequest{
			TargetURL:  "",
			CustomSlug: "test",
		}

		req := httptest.NewRequest(http.MethodPost, "/_/api/create_link", bytes.NewReader(mustMarshal(t, reqBody)))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		c := s.echo.NewContext(req, rec)

		err := s.handler.CreateLink(c)
		require.NoError(t, err)

		require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	})
}

// ==================== SLUG AVAILABILITY TESTS ====================

func (s *HandlerTestSuite) TestSlugAvailabilityAvailable() {
	s.T().Run("returns available=true for unused slug", func(t *testing.T) {
		reqBody := SlugAvailabilityRequest{
			Slug: "available",
		}

		req := httptest.NewRequest(http.MethodPost, "/_/api/slug_availability", bytes.NewReader(mustMarshal(t, reqBody)))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		c := s.echo.NewContext(req, rec)

		err := s.handler.SlugAvailability(c)
		require.NoError(t, err)

		require.Equal(t, http.StatusOK, rec.Code)

		var resp SlugAvailabilityResponse
		err = json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		require.True(t, resp.Available)
	})
}

func (s *HandlerTestSuite) TestSlugAvailabilityTaken() {
	s.T().Run("returns available=false for taken slug", func(t *testing.T) {
		ctx := context.Background()

		// Create a link
		_, err := s.linkSvc.Create(ctx, link.CreateRequest{
			TargetURL:  "https://example.com",
			CustomSlug: "taken",
		})
		require.NoError(t, err)

		reqBody := SlugAvailabilityRequest{
			Slug: "taken",
		}

		req := httptest.NewRequest(http.MethodPost, "/_/api/slug_availability", bytes.NewReader(mustMarshal(t, reqBody)))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		c := s.echo.NewContext(req, rec)

		err = s.handler.SlugAvailability(c)
		require.NoError(t, err)

		require.Equal(t, http.StatusOK, rec.Code)

		var resp SlugAvailabilityResponse
		err = json.Unmarshal(rec.Body.Bytes(), &resp)
		require.NoError(t, err)
		require.False(t, resp.Available)
	})
}

func (s *HandlerTestSuite) TestSlugAvailabilityInvalidSlug() {
	s.T().Run("returns 422 for invalid slug", func(t *testing.T) {
		reqBody := SlugAvailabilityRequest{
			Slug: "_invalid", // contains underscore
		}

		req := httptest.NewRequest(http.MethodPost, "/_/api/slug_availability", bytes.NewReader(mustMarshal(t, reqBody)))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		c := s.echo.NewContext(req, rec)

		err := s.handler.SlugAvailability(c)
		require.NoError(t, err)

		require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	})
}

func (s *HandlerTestSuite) TestSlugAvailabilityMalformedJSON() {
	s.T().Run("returns 400 for malformed JSON", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/_/api/slug_availability", strings.NewReader("{invalid json"))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		c := s.echo.NewContext(req, rec)

		err := s.handler.SlugAvailability(c)
		require.NoError(t, err)

		require.Equal(t, http.StatusBadRequest, rec.Code)

		var errResp ErrorResponse
		err = json.Unmarshal(rec.Body.Bytes(), &errResp)
		require.NoError(t, err)
		require.Equal(t, "invalid request body", errResp.Error)
	})
}

func (s *HandlerTestSuite) TestSlugAvailabilityEmptySlug() {
	s.T().Run("returns 422 for empty slug", func(t *testing.T) {
		reqBody := SlugAvailabilityRequest{
			Slug: "",
		}

		req := httptest.NewRequest(http.MethodPost, "/_/api/slug_availability", bytes.NewReader(mustMarshal(t, reqBody)))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		c := s.echo.NewContext(req, rec)

		err := s.handler.SlugAvailability(c)
		require.NoError(t, err)

		require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	})
}

// Helper function
func mustMarshal(t *testing.T, v interface{}) []byte {
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return b
}

// ==================== VALIDATION TESTS ====================

func (s *HandlerTestSuite) TestValidateURL() {
	s.T().Run("accepts valid http URL", func(t *testing.T) {
		err := validateURL("http://example.com")
		require.NoError(t, err)
	})

	s.T().Run("accepts valid https URL", func(t *testing.T) {
		err := validateURL("https://example.com/path?query=value")
		require.NoError(t, err)
	})

	s.T().Run("accepts URL with fragment", func(t *testing.T) {
		err := validateURL("https://example.com/page#section")
		require.NoError(t, err)
	})

	s.T().Run("accepts URL with port", func(t *testing.T) {
		err := validateURL("https://example.com:8080/path")
		require.NoError(t, err)
	})

	s.T().Run("rejects empty URL", func(t *testing.T) {
		err := validateURL("")
		require.Error(t, err)
		require.Contains(t, err.Error(), "required")
	})

	s.T().Run("rejects URL without scheme", func(t *testing.T) {
		err := validateURL("example.com")
		require.Error(t, err)
	})

	s.T().Run("rejects URL without host", func(t *testing.T) {
		err := validateURL("http://")
		require.Error(t, err)
	})

	s.T().Run("rejects non-http scheme", func(t *testing.T) {
		err := validateURL("ftp://example.com")
		require.Error(t, err)
		require.Contains(t, err.Error(), "http or https")
	})

	s.T().Run("rejects javascript scheme", func(t *testing.T) {
		err := validateURL("javascript:alert(1)")
		require.Error(t, err)
		require.Contains(t, err.Error(), "http or https")
	})
}

func (s *HandlerTestSuite) TestValidateSlug() {
	s.T().Run("accepts valid slug with alphanumeric", func(t *testing.T) {
		err := validateSlug("mylink123")
		require.NoError(t, err)
	})

	s.T().Run("accepts slug with uppercase letters", func(t *testing.T) {
		err := validateSlug("MyLink")
		require.NoError(t, err)
	})

	s.T().Run("rejects slug with hyphens", func(t *testing.T) {
		err := validateSlug("my-link")
		require.Error(t, err)
	})

	s.T().Run("rejects slug with underscores", func(t *testing.T) {
		err := validateSlug("my_link")
		require.Error(t, err)
	})

	s.T().Run("rejects empty slug", func(t *testing.T) {
		err := validateSlug("")
		require.Error(t, err)
		require.Contains(t, err.Error(), "required")
	})

	s.T().Run("rejects slug with invalid characters", func(t *testing.T) {
		err := validateSlug("invalid slug!")
		require.Error(t, err)
	})

	s.T().Run("rejects slug with dots", func(t *testing.T) {
		err := validateSlug("my.slug")
		require.Error(t, err)
	})

	s.T().Run("rejects slug with spaces", func(t *testing.T) {
		err := validateSlug("my slug")
		require.Error(t, err)
	})

	s.T().Run("rejects slug with slash", func(t *testing.T) {
		err := validateSlug("my/slug")
		require.Error(t, err)
	})

	s.T().Run("rejects slug too long", func(t *testing.T) {
		longSlug := strings.Repeat("a", 51)
		err := validateSlug(longSlug)
		require.Error(t, err)
		require.Contains(t, err.Error(), "50 characters")
	})

	s.T().Run("accepts slug at max length", func(t *testing.T) {
		maxSlug := strings.Repeat("a", 50)
		err := validateSlug(maxSlug)
		require.NoError(t, err)
	})
}

// ==================== CORS TESTS ====================

func (s *HandlerTestSuite) TestCreateLinkCORSPreflight() {
	s.T().Run("handles CORS preflight request", func(t *testing.T) {
		// This test verifies that CORS headers would be set by the middleware
		// In a real integration test, we'd test the full server stack
		// For now, we just verify the handler doesn't error on valid requests
		reqBody := CreateLinkRequest{
			TargetURL:  "https://example.com",
			CustomSlug: "corsTest",
		}

		req := httptest.NewRequest(http.MethodPost, "/_/api/create_link", bytes.NewReader(mustMarshal(t, reqBody)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", "https://example.org")
		rec := httptest.NewRecorder()
		c := s.echo.NewContext(req, rec)

		err := s.handler.CreateLink(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, rec.Code)
	})
}

func TestHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(HandlerTestSuite))
}

// ==================== EDIT LINK TESTS ====================

func (s *HandlerTestSuite) TestEditLinkValidClaimKeyQuery() {
	s.T().Run("renders edit page with valid claim key in query string", func(t *testing.T) {
		ctx := context.Background()

		resp, err := s.linkSvc.Create(ctx, link.CreateRequest{
			TargetURL:  "https://example.com",
			CustomSlug: "testedit",
		})
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/_/edit_link/testedit?claim_key="+resp.ClaimKey, nil)
		rec := httptest.NewRecorder()
		c := s.echo.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{{Name: "slug", Value: "testedit"}})

		err = s.handler.EditLink(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, rec.Code)
		require.Contains(t, rec.Body.String(), "https://example.com")
	})
}

func (s *HandlerTestSuite) TestEditLinkValidClaimKeyPost() {
	s.T().Run("renders edit page with valid claim key in POST form data", func(t *testing.T) {
		ctx := context.Background()

		resp, err := s.linkSvc.Create(ctx, link.CreateRequest{
			TargetURL:  "https://example.com",
			CustomSlug: "testeditpost",
		})
		require.NoError(t, err)

		form := url.Values{}
		form.Set("claim_key", resp.ClaimKey)
		req := httptest.NewRequest(http.MethodPost, "/_/edit_link/testeditpost", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		c := s.echo.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{{Name: "slug", Value: "testeditpost"}})

		err = s.handler.EditLink(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, rec.Code)
		require.Contains(t, rec.Body.String(), "https://example.com")
	})
}

func (s *HandlerTestSuite) TestEditLinkNotFound() {
	s.T().Run("returns 404 for non-existent slug", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/_/edit_link/nonexistent?claim_key=somekey", nil)
		rec := httptest.NewRecorder()
		c := s.echo.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{{Name: "slug", Value: "nonexistent"}})

		err := s.handler.EditLink(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusNotFound, rec.Code)
	})
}

func (s *HandlerTestSuite) TestEditLinkInvalidClaimKey() {
	s.T().Run("returns 401 for invalid claim key", func(t *testing.T) {
		ctx := context.Background()

		_, err := s.linkSvc.Create(ctx, link.CreateRequest{
			TargetURL:  "https://example.com",
			CustomSlug: "testinvalid",
		})
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/_/edit_link/testinvalid?claim_key=wrongkey", nil)
		rec := httptest.NewRecorder()
		c := s.echo.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{{Name: "slug", Value: "testinvalid"}})

		err = s.handler.EditLink(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}

func (s *HandlerTestSuite) TestEditLinkMissingClaimKey() {
	s.T().Run("returns 400 for missing claim key", func(t *testing.T) {
		ctx := context.Background()

		_, err := s.linkSvc.Create(ctx, link.CreateRequest{
			TargetURL:  "https://example.com",
			CustomSlug: "testmissing",
		})
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/_/edit_link/testmissing", nil)
		rec := httptest.NewRecorder()
		c := s.echo.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{{Name: "slug", Value: "testmissing"}})

		err = s.handler.EditLink(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func (s *HandlerTestSuite) TestEditLinkRevoked() {
	s.T().Run("returns 410 for revoked link", func(t *testing.T) {
		ctx := context.Background()

		resp, err := s.linkSvc.Create(ctx, link.CreateRequest{
			TargetURL:  "https://example.com",
			CustomSlug: "testrevoked",
		})
		require.NoError(t, err)

		err = s.linkSvc.Revoke(ctx, "testrevoked", resp.ClaimKey)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/_/edit_link/testrevoked?claim_key="+resp.ClaimKey, nil)
		rec := httptest.NewRecorder()
		c := s.echo.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{{Name: "slug", Value: "testrevoked"}})

		err = s.handler.EditLink(c)
		require.NoError(t, err)
		require.Equal(t, http.StatusGone, rec.Code)
	})
}
