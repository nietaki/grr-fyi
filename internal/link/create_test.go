package link

import (
	"context"

	"github.com/mvrahden/go-test/pkg/gotest"
)

type CreateTestSuite struct {
	service *Service
}

func (s *CreateTestSuite) BeforeEach(t *gotest.T) {
	s.service = setupTestService(t)
}

func (s *CreateTestSuite) TestCreateWithCustomSlug(t *gotest.T) {
	t.It("creates a link with the specified custom slug", func(it *gotest.T) {
		ctx := context.Background()

		req := CreateRequest{
			TargetURL:  "https://example.com",
			CustomSlug: "mylink",
		}

		resp, err := s.service.Create(ctx, req)
		gotest.NoError(it, err, "Create")
		gotest.NotNil(it, resp)
		gotest.Equal(it, "mylink", resp.Link.Slug)
		gotest.Equal(it, "https://example.com", resp.Link.TargetURL)
		gotest.NotEqual(it, "", resp.ClaimKey)
	})
}

func (s *CreateTestSuite) TestCreateWithAutoSlug(t *gotest.T) {
	t.It("creates a link with an auto-generated slug", func(it *gotest.T) {
		ctx := context.Background()

		req := CreateRequest{
			TargetURL: "https://example.com",
		}

		resp, err := s.service.Create(ctx, req)
		gotest.NoError(it, err, "Create")
		gotest.NotNil(it, resp)
		gotest.NotEqual(it, "", resp.Link.Slug)
		gotest.Equal(it, "https://example.com", resp.Link.TargetURL)
	})
}

func (s *CreateTestSuite) TestCreateAutoSlugIsSequential(t *gotest.T) {
	t.It("generates sequential slugs for auto-generated links", func(it *gotest.T) {
		ctx := context.Background()

		resp1, err := s.service.Create(ctx, CreateRequest{TargetURL: "https://example.com/1"})
		gotest.NoError(it, err, "Create 1")

		resp2, err := s.service.Create(ctx, CreateRequest{TargetURL: "https://example.com/2"})
		gotest.NoError(it, err, "Create 2")

		gotest.Equal(it, "a", resp1.Link.Slug)
		gotest.Equal(it, "b", resp2.Link.Slug)
	})
}

func (s *CreateTestSuite) TestCreateCustomSlugCollision(t *gotest.T) {
	t.It("returns ErrSlugTaken when custom slug is already used", func(it *gotest.T) {
		ctx := context.Background()

		_, err := s.service.Create(ctx, CreateRequest{TargetURL: "https://example.com/1", CustomSlug: "taken"})
		gotest.NoError(it, err, "Create 1")

		_, err = s.service.Create(ctx, CreateRequest{TargetURL: "https://example.com/2", CustomSlug: "taken"})
		gotest.ErrorIs(it, err, ErrSlugTaken)
	})
}

func (s *CreateTestSuite) TestCreateClaimKeyIsHashed(t *gotest.T) {
	t.It("stores hashed claim key, returns plaintext", func(it *gotest.T) {
		ctx := context.Background()

		resp, err := s.service.Create(ctx, CreateRequest{TargetURL: "https://example.com"})
		gotest.NoError(it, err, "Create")

		var storedHash string
		err = s.service.db.QueryRowContext(ctx,
			"SELECT claim_key_hash FROM links WHERE slug = ?", resp.Link.Slug).Scan(&storedHash)
		gotest.NoError(it, err, "query hash")

		gotest.NotEqual(it, resp.ClaimKey, storedHash)
		gotest.NotEqual(it, "", storedHash)
	})
}

func (s *CreateTestSuite) TestCreateReturnsCreatedAt(t *gotest.T) {
	t.It("returns the created timestamp", func(it *gotest.T) {
		ctx := context.Background()

		resp, err := s.service.Create(ctx, CreateRequest{TargetURL: "https://example.com"})
		gotest.NoError(it, err, "Create")

		gotest.False(it, resp.Link.CreatedAt.IsZero())
	})
}

func (s *CreateTestSuite) TestAutoSlugSkipsCustomSlugCollision(t *gotest.T) {
	t.It("skips to next slug when auto-slug collides with custom slug", func(it *gotest.T) {
		ctx := context.Background()

		_, err := s.service.Create(ctx, CreateRequest{
			TargetURL:  "https://example.com/custom",
			CustomSlug: "a",
		})
		gotest.NoError(it, err, "Create custom")

		resp, err := s.service.Create(ctx, CreateRequest{
			TargetURL: "https://example.com/auto",
		})
		gotest.NoError(it, err, "Create auto")
		gotest.Equal(it, "b", resp.Link.Slug)
	})
}
