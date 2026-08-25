package link

import (
	"errors"
	"time"

	"github.com/mvrahden/go-test/pkg/gotest"
)

type TypesTestSuite struct{}

func (s *TypesTestSuite) BeforeEach(t *gotest.T) {}

func (s *TypesTestSuite) TestCreateRequest(t *gotest.T) {
	t.It("holds target URL and optional custom slug", func(it *gotest.T) {
		req := CreateRequest{
			TargetURL:  "https://example.com",
			CustomSlug: "mylink",
		}
		gotest.Equal(it, "https://example.com", req.TargetURL)
		gotest.Equal(it, "mylink", req.CustomSlug)
	})
}

func (s *TypesTestSuite) TestCreateResponse(t *gotest.T) {
	t.It("holds link and plaintext claim key", func(it *gotest.T) {
		link := &Link{
			Slug:      "abc123",
			TargetURL: "https://example.com",
			CreatedAt: time.Now(),
		}
		resp := CreateResponse{
			Link:     link,
			ClaimKey: "secretkey123",
		}
		gotest.Equal(it, "abc123", resp.Link.Slug)
		gotest.Equal(it, "secretkey123", resp.ClaimKey)
	})
}

func (s *TypesTestSuite) TestLink(t *gotest.T) {
	t.It("holds slug, target URL, timestamps, and click count", func(it *gotest.T) {
		now := time.Now()
		link := Link{
			Slug:       "abc123",
			TargetURL:  "https://example.com",
			CreatedAt:  now,
			RevokedAt:  nil,
			ClickCount: 42,
		}
		gotest.Equal(it, "abc123", link.Slug)
		gotest.Equal(it, "https://example.com", link.TargetURL)
		gotest.Equal(it, now, link.CreatedAt)
		gotest.Equal(it, (*time.Time)(nil), link.RevokedAt)
		gotest.Equal(it, int64(42), link.ClickCount)
	})

	t.It("can be revoked", func(it *gotest.T) {
		now := time.Now()
		link := Link{
			Slug:      "abc123",
			TargetURL: "https://example.com",
			CreatedAt: now,
			RevokedAt: &now,
		}
		gotest.NotNil(it, link.RevokedAt)
	})
}

type ErrorsTestSuite struct{}

func (s *ErrorsTestSuite) BeforeEach(t *gotest.T) {}

func (s *ErrorsTestSuite) TestErrNotFound(t *gotest.T) {
	t.It("is a sentinel error", func(it *gotest.T) {
		gotest.NotNil(it, ErrNotFound)
		gotest.True(it, errors.Is(ErrNotFound, ErrNotFound))
	})
}

func (s *ErrorsTestSuite) TestErrRevoked(t *gotest.T) {
	t.It("is a sentinel error", func(it *gotest.T) {
		gotest.NotNil(it, ErrRevoked)
		gotest.True(it, errors.Is(ErrRevoked, ErrRevoked))
	})
}

func (s *ErrorsTestSuite) TestErrSlugTaken(t *gotest.T) {
	t.It("is a sentinel error", func(it *gotest.T) {
		gotest.NotNil(it, ErrSlugTaken)
		gotest.True(it, errors.Is(ErrSlugTaken, ErrSlugTaken))
	})
}

func (s *ErrorsTestSuite) TestErrInvalidClaim(t *gotest.T) {
	t.It("is a sentinel error", func(it *gotest.T) {
		gotest.NotNil(it, ErrInvalidClaim)
		gotest.True(it, errors.Is(ErrInvalidClaim, ErrInvalidClaim))
	})
}
