package link

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type SlugExistsTestSuite struct {
	suite.Suite
	service *Service
}

func (s *SlugExistsTestSuite) SetupTest() {
	s.service, _ = setupTestService(s.T())
}

func (s *SlugExistsTestSuite) TestSlugExistsReturnsTrueForExistingSlug() {
	s.T().Run("returns true when slug exists", func(t *testing.T) {
		ctx := context.Background()

		_, err := s.service.Create(ctx, CreateRequest{
			TargetURL:  "https://example.com",
			CustomSlug: "existing",
		})
		require.NoError(t, err)

		exists, err := s.service.SlugExists(ctx, "existing")
		require.NoError(t, err)
		require.True(t, exists)
	})
}

func (s *SlugExistsTestSuite) TestSlugExistsReturnsFalseForNonExistingSlug() {
	s.T().Run("returns false when slug does not exist", func(t *testing.T) {
		ctx := context.Background()

		exists, err := s.service.SlugExists(ctx, "nonexistent")
		require.NoError(t, err)
		require.False(t, exists)
	})
}

func TestSlugExistsTestSuite(t *testing.T) {
	suite.Run(t, new(SlugExistsTestSuite))
}
