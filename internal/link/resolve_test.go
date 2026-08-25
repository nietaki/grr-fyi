package link

import (
	"context"
	"path/filepath"

	"github.com/mvrahden/go-test/pkg/gotest"
	_ "modernc.org/sqlite"

	"github.com/nietaki/grr-fyi/internal/db"
	"github.com/nietaki/grr-fyi/internal/env"
)

type ResolveTestSuite struct {
	service *Service
	dbPath  string
}

func (s *ResolveTestSuite) BeforeEach(t *gotest.T) {
	s.dbPath = filepath.Join(t.T().TempDir(), "test.sqlite")
	cfg := env.Config{DBPath: s.dbPath}

	ctx := context.Background()
	conn, err := db.Open(ctx, cfg)
	gotest.NoError(t, err, "Open")

	err = db.Migrate(ctx, conn)
	gotest.NoError(t, err, "Migrate")

	s.service = NewService(conn)
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
