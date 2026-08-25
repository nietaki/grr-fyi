package server

import (
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/labstack/echo/v5"
	"github.com/nietaki/grr-fyi/internal/click"
	"github.com/nietaki/grr-fyi/internal/link"
)

// Handler holds dependencies for HTTP handlers.
// It provides JSON API endpoints for the URL shortener.
type Handler struct {
	linkSvc  *link.Service
	clickSvc *click.Service
	siteURL  string // base URL of the site (e.g., "https://grr.fyi/"), always ends with "/"
}

// NewHandler creates a new Handler with the given dependencies.
// siteURL is the base URL used to construct short URLs in responses.
func NewHandler(linkSvc *link.Service, clickSvc *click.Service, siteURL string) *Handler {
	// Ensure siteURL ends with a slash
	if !strings.HasSuffix(siteURL, "/") {
		siteURL += "/"
	}
	return &Handler{
		linkSvc:  linkSvc,
		clickSvc: clickSvc,
		siteURL:  siteURL,
	}
}

// Request/Response types for JSON API

// CreateLinkRequest is the JSON request body for POST /_/create_link
type CreateLinkRequest struct {
	TargetURL  string `json:"target_url"`            // required, must be http or https
	CustomSlug string `json:"custom_slug,omitempty"` // optional, auto-generated if empty
}

// CreateLinkResponse is the JSON response for successful link creation
type CreateLinkResponse struct {
	Slug     string `json:"slug"`      // the short identifier (e.g., "abc123")
	ShortURL string `json:"short_url"` // full short URL (e.g., "https://grr.fyi/abc123")
	ClaimKey string `json:"claim_key"` // secret key for editing/revoking the link (shown once)
}

// SlugAvailabilityRequest is the JSON request body for POST /_/slug_availability
type SlugAvailabilityRequest struct {
	Slug string `json:"slug"` // the custom slug to check
}

// SlugAvailabilityResponse is the JSON response for slug availability check
type SlugAvailabilityResponse struct {
	Available bool `json:"available"` // true if the slug is available, false if taken
}

// ErrorResponse is the standard JSON error response format
type ErrorResponse struct {
	Error string `json:"error"` // human-readable error message
}

// Handler methods

// jsonError sends a JSON error response with the given status code and message.
func jsonError(c *echo.Context, status int, msg string) error {
	return c.JSON(status, ErrorResponse{Error: msg})
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

	ctx := c.Request().Context()
	foundLink, err := h.linkSvc.Resolve(ctx, slug)
	if err != nil {
		if errors.Is(err, link.ErrNotFound) {
			return c.NoContent(http.StatusNotFound)
		}
		if errors.Is(err, link.ErrRevoked) {
			return c.NoContent(http.StatusGone)
		}
		return err
	}

	// Record click asynchronously
	_ = h.clickSvc.Record(ctx, click.Info{
		LinkID: foundLink.ID,
	})

	return c.Redirect(http.StatusFound, foundLink.TargetURL)
}

// CreateLink handles POST /_/create_link requests.
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

	// Validate URL
	if err := validateURL(req.TargetURL); err != nil {
		return jsonError(c, http.StatusUnprocessableEntity, err.Error())
	}

	// Validate custom slug if provided
	if req.CustomSlug != "" {
		if err := validateSlug(req.CustomSlug); err != nil {
			return jsonError(c, http.StatusUnprocessableEntity, err.Error())
		}
	}

	ctx := c.Request().Context()
	resp, err := h.linkSvc.Create(ctx, link.CreateRequest{
		TargetURL:  req.TargetURL,
		CustomSlug: req.CustomSlug,
	})
	if err != nil {
		if errors.Is(err, link.ErrSlugTaken) {
			return jsonError(c, http.StatusConflict, "slug already taken")
		}
		return err
	}

	return c.JSON(http.StatusCreated, CreateLinkResponse{
		Slug:     resp.Link.Slug,
		ShortURL: h.siteURL + resp.Link.Slug,
		ClaimKey: resp.ClaimKey,
	})
}

// SlugAvailability handles POST /_/slug_availability requests.
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
	if err := validateSlug(req.Slug); err != nil {
		return jsonError(c, http.StatusUnprocessableEntity, err.Error())
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

// Validation functions

var slugRegex = regexp.MustCompile(`^[a-zA-Z0-9]+$`)

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

// validateSlug checks that the slug is valid for use as a URL path segment.
//
// Rules:
//   - Must not be empty
//   - Must be 50 characters or less
//   - Can only contain letters and numbers
//
// Returns an error with a human-readable message if validation fails.
func validateSlug(slug string) error {
	if slug == "" {
		return errors.New("slug is required")
	}

	if len(slug) > 50 {
		return errors.New("slug must be 50 characters or less")
	}

	if !slugRegex.MatchString(slug) {
		return errors.New("slug can only contain letters and numbers")
	}

	return nil
}
