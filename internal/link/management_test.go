package link

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type ManagementTestSuite struct {
	suite.Suite
	service *Service
}

func (s *ManagementTestSuite) SetupTest() {
	s.service, _ = setupTestService(s.T())
}

func (s *ManagementTestSuite) TestGetActiveLink() {
	s.T().Run("returns an active link", func(t *testing.T) {
		ctx := context.Background()

		_, err := s.service.Create(ctx, CreateRequest{TargetURL: "https://example.com", CustomSlug: "test"})
		require.NoError(t, err, "Create")

		link, err := s.service.Get(ctx, "test")
		require.NoError(t, err, "Get")
		require.NotNil(t, link)
		require.Equal(t, "test", link.Slug)
		require.Equal(t, (*time.Time)(nil), link.RevokedAt)
	})
}

func (s *ManagementTestSuite) TestGetRevokedLink() {
	s.T().Run("returns a revoked link", func(t *testing.T) {
		ctx := context.Background()

		resp, err := s.service.Create(ctx, CreateRequest{TargetURL: "https://example.com", CustomSlug: "revoked"})
		require.NoError(t, err, "Create")

		err = s.service.Revoke(ctx, "revoked", resp.ClaimKey)
		require.NoError(t, err, "Revoke")

		link, err := s.service.Get(ctx, "revoked")
		require.NoError(t, err, "Get")
		require.NotNil(t, link)
		require.NotNil(t, link.RevokedAt)
	})
}

func (s *ManagementTestSuite) TestGetNotFound() {
	s.T().Run("returns ErrNotFound when slug does not exist", func(t *testing.T) {
		ctx := context.Background()

		_, err := s.service.Get(ctx, "nonexistent")
		require.ErrorIs(t, err, ErrNotFound)
	})
}

func (s *ManagementTestSuite) TestUpdateWithValidClaim() {
	s.T().Run("updates the target URL with valid claim key", func(t *testing.T) {
		ctx := context.Background()

		resp, err := s.service.Create(ctx, CreateRequest{TargetURL: "https://example.com", CustomSlug: "test"})
		require.NoError(t, err, "Create")

		err = s.service.Update(ctx, "test", resp.ClaimKey, "https://newexample.com")
		require.NoError(t, err, "Update")

		link, err := s.service.Get(ctx, "test")
		require.NoError(t, err, "Get")
		require.Equal(t, "https://newexample.com", link.TargetURL)
	})
}

func (s *ManagementTestSuite) TestUpdateWithInvalidClaim() {
	s.T().Run("returns ErrInvalidClaim with wrong claim key", func(t *testing.T) {
		ctx := context.Background()

		_, err := s.service.Create(ctx, CreateRequest{TargetURL: "https://example.com", CustomSlug: "test"})
		require.NoError(t, err, "Create")

		err = s.service.Update(ctx, "test", "wrongkey", "https://newexample.com")
		require.ErrorIs(t, err, ErrInvalidClaim)
	})
}

func (s *ManagementTestSuite) TestRevokeWithValidClaim() {
	s.T().Run("revokes the link with valid claim key", func(t *testing.T) {
		ctx := context.Background()

		resp, err := s.service.Create(ctx, CreateRequest{TargetURL: "https://example.com", CustomSlug: "test"})
		require.NoError(t, err, "Create")

		err = s.service.Revoke(ctx, "test", resp.ClaimKey)
		require.NoError(t, err, "Revoke")

		link, err := s.service.Get(ctx, "test")
		require.NoError(t, err, "Get")
		require.NotNil(t, link.RevokedAt)
	})
}

func (s *ManagementTestSuite) TestRevokeWithInvalidClaim() {
	s.T().Run("returns ErrInvalidClaim with wrong claim key", func(t *testing.T) {
		ctx := context.Background()

		_, err := s.service.Create(ctx, CreateRequest{TargetURL: "https://example.com", CustomSlug: "test"})
		require.NoError(t, err, "Create")

		err = s.service.Revoke(ctx, "test", "wrongkey")
		require.ErrorIs(t, err, ErrInvalidClaim)
	})
}

func TestManagementTestSuite(t *testing.T) {
	suite.Run(t, new(ManagementTestSuite))
}
