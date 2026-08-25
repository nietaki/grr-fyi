package link

import (
	"context"

	"github.com/mvrahden/go-test/pkg/gotest"
)

type ResolveTestSuite struct {
	service *Service
}

func (s *ResolveTestSuite) BeforeEach(t *gotest.T) {
	s.service, _ = setupTestService(t)
}

func (s *ResolveTestSuite) TestResolveActiveLink(t *gotest.T) {
	t.It("returns the link when it exists and is active", func(it *gotest.T) {
		ctx := context.Background()

		_, err := s.service.Create(ctx, CreateRequest{TargetURL: "https://example.com", CustomSlug: "test"})
		gotest.NoError(it, err, "Create")

		link, err := s.service.Resolve(ctx, "test")
		gotest.NoError(it, err, "Resolve")
		gotest.NotNil(it, link)
		gotest.Equal(it, "test", link.Slug)
		gotest.Equal(it, "https://example.com", link.TargetURL)
	})
}

func (s *ResolveTestSuite) TestResolveNotFound(t *gotest.T) {
	t.It("returns ErrNotFound when slug does not exist", func(it *gotest.T) {
		ctx := context.Background()

		_, err := s.service.Resolve(ctx, "nonexistent")
		gotest.ErrorIs(it, err, ErrNotFound)
	})
}
