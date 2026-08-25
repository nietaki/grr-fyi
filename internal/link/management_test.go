package link

import (
	"context"
	"path/filepath"
	"time"

	"github.com/mvrahden/go-test/pkg/gotest"
	_ "modernc.org/sqlite"

	"github.com/nietaki/grr-fyi/internal/db"
	"github.com/nietaki/grr-fyi/internal/env"
)

type ManagementTestSuite struct {
	service *Service
	dbPath  string
}

func (s *ManagementTestSuite) BeforeEach(t *gotest.T) {
	s.dbPath = filepath.Join(t.T().TempDir(), "test.sqlite")
	cfg := env.Config{DBPath: s.dbPath}

	ctx := context.Background()
	conn, err := db.Open(ctx, cfg)
	gotest.NoError(t, err, "Open")

	err = db.Migrate(ctx, conn)
	gotest.NoError(t, err, "Migrate")

	s.service = NewService(conn)
}

func (s *ManagementTestSuite) TestGetActiveLink(t *gotest.T) {
	t.It("returns an active link", func(it *gotest.T) {
		ctx := context.Background()

		_, err := s.service.Create(ctx, CreateRequest{TargetURL: "https://example.com", CustomSlug: "test"})
		gotest.NoError(it, err, "Create")

		link, err := s.service.Get(ctx, "test")
		gotest.NoError(it, err, "Get")
		gotest.NotNil(it, link)
		gotest.Equal(it, "test", link.Slug)
		gotest.Equal(it, (*time.Time)(nil), link.RevokedAt)
	})
}

func (s *ManagementTestSuite) TestGetRevokedLink(t *gotest.T) {
	t.It("returns a revoked link", func(it *gotest.T) {
		ctx := context.Background()

		resp, err := s.service.Create(ctx, CreateRequest{TargetURL: "https://example.com", CustomSlug: "revoked"})
		gotest.NoError(it, err, "Create")

		err = s.service.Revoke(ctx, "revoked", resp.ClaimKey)
		gotest.NoError(it, err, "Revoke")

		link, err := s.service.Get(ctx, "revoked")
		gotest.NoError(it, err, "Get")
		gotest.NotNil(it, link)
		gotest.NotNil(it, link.RevokedAt)
	})
}

func (s *ManagementTestSuite) TestGetNotFound(t *gotest.T) {
	t.It("returns ErrNotFound when slug does not exist", func(it *gotest.T) {
		ctx := context.Background()

		_, err := s.service.Get(ctx, "nonexistent")
		gotest.ErrorIs(it, err, ErrNotFound)
	})
}

func (s *ManagementTestSuite) TestUpdateWithValidClaim(t *gotest.T) {
	t.It("updates the target URL with valid claim key", func(it *gotest.T) {
		ctx := context.Background()

		resp, err := s.service.Create(ctx, CreateRequest{TargetURL: "https://example.com", CustomSlug: "test"})
		gotest.NoError(it, err, "Create")

		err = s.service.Update(ctx, "test", resp.ClaimKey, "https://newexample.com")
		gotest.NoError(it, err, "Update")

		link, err := s.service.Get(ctx, "test")
		gotest.NoError(it, err, "Get")
		gotest.Equal(it, "https://newexample.com", link.TargetURL)
	})
}

func (s *ManagementTestSuite) TestUpdateWithInvalidClaim(t *gotest.T) {
	t.It("returns ErrInvalidClaim with wrong claim key", func(it *gotest.T) {
		ctx := context.Background()

		_, err := s.service.Create(ctx, CreateRequest{TargetURL: "https://example.com", CustomSlug: "test"})
		gotest.NoError(it, err, "Create")

		err = s.service.Update(ctx, "test", "wrongkey", "https://newexample.com")
		gotest.ErrorIs(it, err, ErrInvalidClaim)
	})
}

func (s *ManagementTestSuite) TestRevokeWithValidClaim(t *gotest.T) {
	t.It("revokes the link with valid claim key", func(it *gotest.T) {
		ctx := context.Background()

		resp, err := s.service.Create(ctx, CreateRequest{TargetURL: "https://example.com", CustomSlug: "test"})
		gotest.NoError(it, err, "Create")

		err = s.service.Revoke(ctx, "test", resp.ClaimKey)
		gotest.NoError(it, err, "Revoke")

		link, err := s.service.Get(ctx, "test")
		gotest.NoError(it, err, "Get")
		gotest.NotNil(it, link.RevokedAt)
	})
}

func (s *ManagementTestSuite) TestRevokeWithInvalidClaim(t *gotest.T) {
	t.It("returns ErrInvalidClaim with wrong claim key", func(it *gotest.T) {
		ctx := context.Background()

		_, err := s.service.Create(ctx, CreateRequest{TargetURL: "https://example.com", CustomSlug: "test"})
		gotest.NoError(it, err, "Create")

		err = s.service.Revoke(ctx, "test", "wrongkey")
		gotest.ErrorIs(it, err, ErrInvalidClaim)
	})
}
