package click

import (
	"context"
	"path/filepath"
	"time"

	"github.com/mvrahden/go-test/pkg/gotest"
	_ "modernc.org/sqlite"

	"github.com/nietaki/grr-fyi/internal/db"
	"github.com/nietaki/grr-fyi/internal/env"
	"github.com/nietaki/grr-fyi/internal/link"
)

type ClickTestSuite struct {
	linkService  *link.Service
	clickService *Service
	dbPath       string
}

func (s *ClickTestSuite) BeforeEach(t *gotest.T) {
	s.dbPath = filepath.Join(t.T().TempDir(), "test.sqlite")
	cfg := env.Config{DBPath: s.dbPath}

	ctx := context.Background()
	conn, err := db.Open(ctx, cfg)
	gotest.NoError(t, err, "Open")

	err = db.Migrate(ctx, conn)
	gotest.NoError(t, err, "Migrate")

	s.linkService = link.NewService(link.NewStore(conn))
	s.clickService = NewService(NewStore(conn), 100)
}

func (s *ClickTestSuite) AfterEach(t *gotest.T) {
	s.clickService.Close()
}

func (s *ClickTestSuite) TestRecordClick(t *gotest.T) {
	t.It("records a click asynchronously", func(it *gotest.T) {
		ctx := context.Background()

		_, err := s.linkService.Create(ctx, link.CreateRequest{TargetURL: "https://example.com", CustomSlug: "test"})
		gotest.NoError(it, err, "Create")

		linkObj, err := s.linkService.Get(ctx, "test")
		gotest.NoError(it, err, "Get")

		info := Info{
			LinkID:   linkObj.ID,
			IPHash:   "abc123",
			Referrer: "https://google.com",
			Country:  "",
		}

		err = s.clickService.Record(ctx, info)
		gotest.NoError(it, err, "Record")

		time.Sleep(100 * time.Millisecond)

		count, err := s.clickService.Count(ctx, linkObj.ID)
		gotest.NoError(it, err, "Count")
		gotest.Equal(it, int64(1), count)
	})
}

func (s *ClickTestSuite) TestRecordMultipleClicks(t *gotest.T) {
	t.It("records multiple clicks", func(it *gotest.T) {
		ctx := context.Background()

		_, err := s.linkService.Create(ctx, link.CreateRequest{TargetURL: "https://example.com", CustomSlug: "test"})
		gotest.NoError(it, err, "Create")

		linkObj, err := s.linkService.Get(ctx, "test")
		gotest.NoError(it, err, "Get")

		for i := 0; i < 5; i++ {
			info := Info{
				LinkID: linkObj.ID,
				IPHash: "hash" + string(rune('0'+i)),
			}
			err = s.clickService.Record(ctx, info)
			gotest.NoError(it, err, "Record %d", i)
		}

		time.Sleep(200 * time.Millisecond)

		count, err := s.clickService.Count(ctx, linkObj.ID)
		gotest.NoError(it, err, "Count")
		gotest.Equal(it, int64(5), count)
	})
}

func (s *ClickTestSuite) TestCountZeroForNewLink(t *gotest.T) {
	t.It("returns 0 for a link with no clicks", func(it *gotest.T) {
		ctx := context.Background()

		_, err := s.linkService.Create(ctx, link.CreateRequest{TargetURL: "https://example.com", CustomSlug: "test"})
		gotest.NoError(it, err, "Create")

		linkObj, err := s.linkService.Get(ctx, "test")
		gotest.NoError(it, err, "Get")

		count, err := s.clickService.Count(ctx, linkObj.ID)
		gotest.NoError(it, err, "Count")
		gotest.Equal(it, int64(0), count)
	})
}
