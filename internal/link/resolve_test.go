package link

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type ResolveTestSuite struct {
	suite.Suite
	service *Service
}

func (s *ResolveTestSuite) SetupTest() {
	s.service, _ = setupTestService(s.T())
}

func (s *ResolveTestSuite) TestResolveActiveLink() {
	s.T().Run("returns the link when it exists and is active", func(t *testing.T) {
		ctx := context.Background()

		_, err := s.service.Create(ctx, CreateRequest{TargetURL: "https://example.com", CustomSlug: "test"})
		require.NoError(t, err, "Create")

		link, err := s.service.Resolve(ctx, "test")
		require.NoError(t, err, "Resolve")
		require.NotNil(t, link)
		require.Equal(t, "test", link.Slug)
		require.Equal(t, "https://example.com", link.TargetURL)
	})
}

func (s *ResolveTestSuite) TestResolveNotFound() {
	s.T().Run("returns ErrNotFound when slug does not exist", func(t *testing.T) {
		ctx := context.Background()

		_, err := s.service.Resolve(ctx, "nonexistent")
		require.ErrorIs(t, err, ErrNotFound)
	})
}

func TestResolveTestSuite(t *testing.T) {
	suite.Run(t, new(ResolveTestSuite))
}
