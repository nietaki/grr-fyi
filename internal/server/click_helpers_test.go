package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/require"
)

func TestExtractIP(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*http.Request)
		expected string
	}{
		{
			name: "X-Forwarded-For header",
			setup: func(r *http.Request) {
				r.Header.Set("X-Forwarded-For", "203.0.113.195")
			},
			expected: "203.0.113.195",
		},
		{
			name: "X-Forwarded-For with multiple IPs",
			setup: func(r *http.Request) {
				r.Header.Set("X-Forwarded-For", "203.0.113.195, 70.41.3.18")
			},
			expected: "203.0.113.195",
		},
		{
			name: "X-Real-IP header",
			setup: func(r *http.Request) {
				r.Header.Set("X-Real-IP", "198.51.100.22")
			},
			expected: "198.51.100.22",
		},
		{
			name: "X-Forwarded-For takes precedence over X-Real-IP",
			setup: func(r *http.Request) {
				r.Header.Set("X-Forwarded-For", "203.0.113.195")
				r.Header.Set("X-Real-IP", "198.51.100.22")
			},
			expected: "203.0.113.195",
		},
		{
			name:     "fallback to RemoteAddr",
			setup:    func(r *http.Request) {},
			expected: "192.0.2.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = "192.0.2.1:12345"
			tt.setup(req)

			e := echo.New()
			c := e.NewContext(req, nil)

			result := extractIP(c)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestHashIP(t *testing.T) {
	t.Run("produces consistent hash", func(t *testing.T) {
		hash1 := hashIP("192.168.1.1")
		hash2 := hashIP("192.168.1.1")
		require.Equal(t, hash1, hash2)
	})

	t.Run("different IPs produce different hashes", func(t *testing.T) {
		hash1 := hashIP("192.168.1.1")
		hash2 := hashIP("192.168.1.2")
		require.NotEqual(t, hash1, hash2)
	})

	t.Run("hash is 16 characters", func(t *testing.T) {
		hash := hashIP("192.168.1.1")
		require.Len(t, hash, 16)
	})

	t.Run("empty IP returns empty hash", func(t *testing.T) {
		hash := hashIP("")
		require.Equal(t, "", hash)
	})
}

func TestExtractReferrer(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*http.Request)
		expected string
	}{
		{
			name: "Referer header present",
			setup: func(r *http.Request) {
				r.Header.Set("Referer", "https://google.com/search?q=test")
			},
			expected: "https://google.com/search?q=test",
		},
		{
			name: "Origin header fallback",
			setup: func(r *http.Request) {
				r.Header.Set("Origin", "https://example.com")
			},
			expected: "https://example.com",
		},
		{
			name: "Referer takes precedence over Origin",
			setup: func(r *http.Request) {
				r.Header.Set("Referer", "https://google.com")
				r.Header.Set("Origin", "https://example.com")
			},
			expected: "https://google.com",
		},
		{
			name:     "no headers returns empty string",
			setup:    func(r *http.Request) {},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			tt.setup(req)

			e := echo.New()
			c := e.NewContext(req, nil)

			result := extractReferrer(c)
			require.Equal(t, tt.expected, result)
		})
	}
}
