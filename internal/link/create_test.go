package link

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type CreateTestSuite struct {
	suite.Suite
	service *Service
	conn    *sql.DB
}

func (s *CreateTestSuite) SetupTest() {
	s.service, s.conn = setupTestService(s.T())
}

func (s *CreateTestSuite) TestCreateWithCustomSlug() {
	s.T().Run("creates a link with the specified custom slug", func(t *testing.T) {
		ctx := context.Background()

		req := CreateRequest{
			TargetURL:  "https://example.com",
			CustomSlug: "mylink",
		}

		resp, err := s.service.Create(ctx, req)
		require.NoError(t, err, "Create")
		require.NotNil(t, resp)
		require.Equal(t, "mylink", resp.Link.Slug)
		require.Equal(t, "https://example.com", resp.Link.TargetURL)
		require.NotEqual(t, "", resp.ClaimKey)
	})
}

func (s *CreateTestSuite) TestCreateWithAutoSlug() {
	s.T().Run("creates a link with an auto-generated slug", func(t *testing.T) {
		ctx := context.Background()

		req := CreateRequest{
			TargetURL: "https://example.com",
		}

		resp, err := s.service.Create(ctx, req)
		require.NoError(t, err, "Create")
		require.NotNil(t, resp)
		require.NotEqual(t, "", resp.Link.Slug)
		require.Equal(t, "https://example.com", resp.Link.TargetURL)
	})
}

func (s *CreateTestSuite) TestCreateAutoSlugIsSequential() {
	s.T().Run("generates sequential slugs for auto-generated links", func(t *testing.T) {
		ctx := context.Background()

		resp1, err := s.service.Create(ctx, CreateRequest{TargetURL: "https://example.com/1"})
		require.NoError(t, err, "Create 1")

		resp2, err := s.service.Create(ctx, CreateRequest{TargetURL: "https://example.com/2"})
		require.NoError(t, err, "Create 2")

		require.Equal(t, EncodeSlug(0), resp1.Link.Slug)
		require.Equal(t, EncodeSlug(1), resp2.Link.Slug)
	})
}

func (s *CreateTestSuite) TestCreateCustomSlugCollision() {
	s.T().Run("returns ErrSlugTaken when custom slug is already used", func(t *testing.T) {
		ctx := context.Background()

		_, err := s.service.Create(ctx, CreateRequest{TargetURL: "https://example.com/1", CustomSlug: "taken"})
		require.NoError(t, err, "Create 1")

		_, err = s.service.Create(ctx, CreateRequest{TargetURL: "https://example.com/2", CustomSlug: "taken"})
		require.ErrorIs(t, err, ErrSlugTaken)
	})
}

func (s *CreateTestSuite) TestCreateClaimKeyIsHashed() {
	s.T().Run("stores hashed claim key, returns plaintext", func(t *testing.T) {
		ctx := context.Background()

		resp, err := s.service.Create(ctx, CreateRequest{TargetURL: "https://example.com"})
		require.NoError(t, err, "Create")

		var storedHash string
		err = s.conn.QueryRowContext(ctx,
			"SELECT claim_key_hash FROM links WHERE slug = ?", resp.Link.Slug).Scan(&storedHash)
		require.NoError(t, err, "query hash")

		require.NotEqual(t, resp.ClaimKey, storedHash)
		require.NotEqual(t, "", storedHash)
	})
}

func (s *CreateTestSuite) TestCreateReturnsCreatedAt() {
	s.T().Run("returns the created timestamp", func(t *testing.T) {
		ctx := context.Background()

		resp, err := s.service.Create(ctx, CreateRequest{TargetURL: "https://example.com"})
		require.NoError(t, err, "Create")

		require.False(t, resp.Link.CreatedAt.IsZero())
	})
}

func (s *CreateTestSuite) TestAutoSlugSkipsCustomSlugCollision() {
	s.T().Run("skips to next slug when auto-slug collides with custom slug", func(t *testing.T) {
		ctx := context.Background()

		_, err := s.service.Create(ctx, CreateRequest{
			TargetURL:  "https://example.com/custom",
			CustomSlug: EncodeSlug(0),
		})
		require.NoError(t, err, "Create custom")

		resp, err := s.service.Create(ctx, CreateRequest{
			TargetURL: "https://example.com/auto",
		})
		require.NoError(t, err, "Create auto")
		require.Equal(t, EncodeSlug(1), resp.Link.Slug)
	})
}

func TestCreateTestSuite(t *testing.T) {
	suite.Run(t, new(CreateTestSuite))
}
