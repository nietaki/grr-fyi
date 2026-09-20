package server

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/labstack/echo/v5"
	"github.com/nietaki/grr-fyi/internal/captcha"
	"github.com/nietaki/grr-fyi/internal/click"
	"github.com/nietaki/grr-fyi/internal/link"
	"github.com/nietaki/grr-fyi/internal/stats"
)

type Handler struct {
	linkSvc  *link.Service
	clickSvc *click.Service
	statsSvc *stats.Service
	siteURL  string
	captcha  captcha.Verifier
}

func NewHandler(linkSvc *link.Service, clickSvc *click.Service, statsSvc *stats.Service, siteURL string, captcha captcha.Verifier) *Handler {
	if !strings.HasSuffix(siteURL, "/") {
		siteURL += "/"
	}
	return &Handler{
		linkSvc:  linkSvc,
		clickSvc: clickSvc,
		statsSvc: statsSvc,
		siteURL:  siteURL,
		captcha:  captcha,
	}
}

func (h *Handler) joinURL(elems ...string) string {
	result, _ := url.JoinPath(h.siteURL, elems...)
	return result
}

// Request/Response types for JSON API

type CreateLinkRequest struct {
	TargetURL  string `json:"target_url"`
	CustomSlug string `json:"custom_slug,omitempty"`
	Altcha     string `json:"altcha,omitempty"`
}

// CreateLinkResponse is the JSON response for successful link creation
type CreateLinkResponse struct {
	Slug     string `json:"slug"`      // the short identifier (e.g., "abc123")
	ShortURL string `json:"short_url"` // full short URL (e.g., "https://grr.fyi/abc123")
	ClaimKey string `json:"claim_key"` // secret key for editing/revoking the link (shown once)
}

// SlugAvailabilityRequest is the JSON request body for POST /_/api/slug_availability
type SlugAvailabilityRequest struct {
	Slug string `json:"slug"` // the custom slug to check
}

// SlugAvailabilityResponse is the JSON response for slug availability check
type SlugAvailabilityResponse struct {
	Available bool `json:"available"` // true if the slug is available, false if taken
}

// ErrorResponse is the standard JSON error response format
type ErrorResponse struct {
	Error string `json:"error"`           // human-readable error message
	Field string `json:"field,omitempty"` // form field at fault, "" if not attributable to one
}

// Form field identifiers reported in ErrorResponse.Field. They match the JSON
// property names of CreateLinkRequest so API clients and the browser client
// agree on which input to highlight.
const (
	fieldTargetURL  = "target_url"
	fieldCustomSlug = "custom_slug"
)

// Handler methods

// jsonError sends a JSON error response with the given status code and message.
//
// It is used for errors that are not attributable to a single form field; see
// jsonFieldError for validation errors tied to a specific input.
func jsonError(c *echo.Context, status int, msg string) error {
	return c.JSON(status, ErrorResponse{Error: msg})
}

// jsonFieldError sends a JSON error response tagged with the form field at
// fault (e.g. fieldCustomSlug) so clients can highlight the offending input.
func jsonFieldError(c *echo.Context, status int, field, msg string) error {
	return c.JSON(status, ErrorResponse{Error: msg, Field: field})
}

// renderError renders the error view with the given status code and optional description.
func renderError(c *echo.Context, status int, description string) error {
	return c.Render(status, "error.html", map[string]any{
		"StatusCode":  status,
		"StatusText":  http.StatusText(status),
		"Description": description,
	})
}

// Stats renders the public /_/stats page with aggregate site, runtime,
// and replication metrics.
func (h *Handler) Stats(c *echo.Context) error {
	if h.statsSvc == nil {
		return renderError(c, http.StatusServiceUnavailable, "Stats are unavailable.")
	}
	data, err := h.statsSvc.Page(c.Request().Context())
	if err != nil {
		return renderError(c, http.StatusInternalServerError, "Could not compute stats.")
	}
	// Render as a map (like the error page) so the shared base template's
	// optional fields (e.g. captchaEnabled) resolve cleanly.
	return c.Render(http.StatusOK, "stats.html", map[string]any{
		"Site":        data.Site,
		"Runtime":     data.Runtime,
		"Replication": data.Replication,
		"GeneratedAt": data.GeneratedAt,
	})
}

// Redirect handles GET /<slug> requests.
// It resolves the slug to a target URL and returns a 302 redirect.
// Click tracking is performed asynchronously via the click service.
//
// Returns:
//   - 302 Found with Location header (active link)
//   - 404 Not Found (slug does not exist)
//   - 410 Gone (link has been revoked)
func (h *Handler) Redirect(c *echo.Context) error {
	slug := c.Param("slug")

	// Validate slug format - reject anything that doesn't match the slug pattern
	// This allows static files (which have extensions like .ico, .css, .js) to be served
	if err := link.ValidateSlug(slug); err != nil {
		return renderError(c, http.StatusNotFound, "")
	}

	ctx := c.Request().Context()
	foundLink, err := h.linkSvc.Resolve(ctx, slug)
	if err != nil {
		if errors.Is(err, link.ErrNotFound) {
			return renderError(c, http.StatusNotFound, "The requested link could not be found.")
		}
		if errors.Is(err, link.ErrRevoked) {
			return renderError(c, http.StatusGone, "This link has been revoked.")
		}
		return err
	}

	// Record click asynchronously
	_ = h.clickSvc.Record(ctx, click.Info{
		LinkID:   foundLink.ID,
		IPHash:   hashIP(extractIP(c)),
		Referrer: extractReferrer(c),
	})

	return c.Redirect(http.StatusFound, foundLink.TargetURL)
}

// CreateLink handles POST /_/api/create_link requests.
// It creates a new shortened URL with an optional custom slug.
//
// Request body: {"target_url": "https://...", "custom_slug": "optional"}
// Response (201): {"slug": "...", "short_url": "...", "claim_key": "..."}
//
// Returns:
//   - 201 Created with link details and claim key
//   - 400 Bad Request (malformed JSON)
//   - 409 Conflict (custom slug already taken)
//   - 422 Unprocessable Entity (validation error)
func (h *Handler) CreateLink(c *echo.Context) error {
	var req CreateLinkRequest
	if err := c.Bind(&req); err != nil {
		return jsonError(c, http.StatusBadRequest, "invalid request body")
	}

	if h.captcha.Enabled() {
		if err := h.captcha.Verify(req.Altcha); err != nil {
			if errors.Is(err, captcha.ErrExpired) {
				return jsonError(c, http.StatusUnprocessableEntity, "captcha expired")
			}
			return jsonError(c, http.StatusUnprocessableEntity, "captcha verification failed")
		}
	}

	if err := validateURL(req.TargetURL); err != nil {
		return jsonFieldError(c, http.StatusUnprocessableEntity, fieldTargetURL, err.Error())
	}

	if req.CustomSlug != "" {
		if err := link.ValidateSlug(req.CustomSlug); err != nil {
			return jsonFieldError(c, http.StatusUnprocessableEntity, fieldCustomSlug, err.Error())
		}
	}

	ctx := c.Request().Context()
	resp, err := h.linkSvc.Create(ctx, link.CreateRequest{
		TargetURL:  req.TargetURL,
		CustomSlug: req.CustomSlug,
	})
	if err != nil {
		if errors.Is(err, link.ErrSlugTaken) {
			return jsonFieldError(c, http.StatusConflict, fieldCustomSlug, "slug already taken")
		}
		return err
	}

	return c.JSON(http.StatusCreated, CreateLinkResponse{
		Slug:     resp.Link.Slug,
		ShortURL: h.joinURL(resp.Link.Slug),
		ClaimKey: resp.ClaimKey,
	})
}

func (h *Handler) AltchaChallenge(c *echo.Context) error {
	if !h.captcha.Enabled() {
		return jsonError(c, http.StatusNotFound, "captcha not enabled")
	}
	challengeJSON, err := h.captcha.NewChallenge()
	if err != nil {
		return jsonError(c, http.StatusInternalServerError, "failed to create challenge")
	}
	c.Response().Header().Set("Content-Type", "application/json")
	return c.JSONBlob(http.StatusOK, challengeJSON)
}

// SlugAvailability handles POST /_/api/slug_availability requests.
// It checks whether a custom slug is available for use.
//
// Request body: {"slug": "..."}
// Response (200): {"available": true/false}
//
// Note: A slug is considered "taken" if it exists in the database,
// even if the link has been revoked. Revoked slugs cannot be re-used.
//
// Returns:
//   - 200 OK with availability status
//   - 400 Bad Request (malformed JSON)
//   - 422 Unprocessable Entity (invalid slug format)
func (h *Handler) SlugAvailability(c *echo.Context) error {
	var req SlugAvailabilityRequest
	if err := c.Bind(&req); err != nil {
		return jsonError(c, http.StatusBadRequest, "invalid request body")
	}

	// Validate slug
	if err := link.ValidateSlug(req.Slug); err != nil {
		return jsonFieldError(c, http.StatusUnprocessableEntity, fieldCustomSlug, err.Error())
	}

	ctx := c.Request().Context()
	exists, err := h.linkSvc.SlugExists(ctx, req.Slug)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, SlugAvailabilityResponse{
		Available: !exists,
	})
}

// EditLink handles GET/POST /_/edit_link/:slug requests.
// It displays the edit page for a link after verifying the claim key.
//
// The claim key can be provided as:
//   - Query string parameter: ?claim_key=...
//   - POST form data: claim_key=...
//
// Returns:
//   - 200 OK with edit page rendered
//   - 400 Bad Request (missing claim key)
//   - 401 Unauthorized (invalid claim key)
//   - 404 Not Found (slug does not exist)
//   - 410 Gone (link has been revoked)
func (h *Handler) EditLink(c *echo.Context) error {
	slug := c.Param("slug")

	claimKey := c.QueryParam("claim_key")
	if claimKey == "" {
		claimKey = c.FormValue("claim_key")
	}
	if claimKey == "" {
		return renderError(c, http.StatusBadRequest, "claim_key is required.")
	}

	ctx := c.Request().Context()
	foundLink, err := h.linkSvc.GetWithClaimKey(ctx, slug, claimKey)
	if err != nil {
		if errors.Is(err, link.ErrNotFound) {
			return renderError(c, http.StatusNotFound, "")
		}
		if errors.Is(err, link.ErrRevoked) {
			return renderError(c, http.StatusGone, "This link has been revoked.")
		}
		if errors.Is(err, link.ErrInvalidClaim) {
			return renderError(c, http.StatusUnauthorized, "Invalid claim key.")
		}
		return err
	}

	stats, err := h.clickSvc.Stats(ctx, foundLink.ID)
	if err != nil {
		return err
	}

	editURL, _ := url.Parse(h.joinURL("_/edit_link", foundLink.Slug))
	q := editURL.Query()
	q.Set("claim_key", claimKey)
	editURL.RawQuery = q.Encode()

	data := map[string]any{
		"Slug":        foundLink.Slug,
		"TargetURL":   foundLink.TargetURL,
		"ShortURL":    h.joinURL(foundLink.Slug),
		"EditURL":     editURL.String(),
		"TotalClicks": stats.Total,
		"DistinctIPs": stats.DistinctIP,
	}

	return c.Render(http.StatusOK, "edit_link.html", data)
}

// Validation functions

// validateURL checks that the URL is valid and uses http or https scheme.
// Rejects empty URLs, URLs without a host, and non-http(s) schemes like
// javascript:, ftp:, file:, etc.
//
// Returns an error with a human-readable message if validation fails.
func validateURL(rawURL string) error {
	if rawURL == "" {
		return errors.New("URL is required")
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return errors.New("invalid URL format")
	}

	// Check the scheme before the host so non-http(s) schemes like
	// `javascript:` are rejected with a scheme-specific message.
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.New("URL must use http or https scheme")
	}

	if parsed.Host == "" {
		return errors.New("URL must include a host")
	}

	return nil
}
