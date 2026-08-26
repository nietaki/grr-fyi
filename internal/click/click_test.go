package click

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	_ "modernc.org/sqlite"

	"github.com/nietaki/grr-fyi/internal/db"
	"github.com/nietaki/grr-fyi/internal/env"
	"github.com/nietaki/grr-fyi/internal/link"
	"github.com/nietaki/grr-fyi/internal/store"
)

type ClickTestSuite struct {
	suite.Suite
	linkService  *link.Service
	clickService *Service
	dbPath       string
}

func (s *ClickTestSuite) SetupTest() {
	s.dbPath = filepath.Join(s.T().TempDir(), "test.sqlite")
	cfg := env.Config{DBPath: s.dbPath}

	ctx := context.Background()
	conn, err := db.Open(ctx, cfg)
	require.NoError(s.T(), err, "Open")

	err = db.Migrate(ctx, conn)
	require.NoError(s.T(), err, "Migrate")

	s.linkService = link.NewService(link.NewStore(conn), store.NewTxScope(conn))
	s.clickService = NewService(NewStore(conn), 100)
}

func (s *ClickTestSuite) TearDownTest() {
	s.clickService.Close()
}

func (s *ClickTestSuite) TestRecordClick() {
	s.T().Run("records a click asynchronously", func(t *testing.T) {
		ctx := context.Background()

		_, err := s.linkService.Create(ctx, link.CreateRequest{TargetURL: "https://example.com", CustomSlug: "test"})
		require.NoError(t, err, "Create")

		linkObj, err := s.linkService.Get(ctx, "test")
		require.NoError(t, err, "Get")

		info := Info{
			LinkID:   linkObj.ID,
			IPHash:   "abc123",
			Referrer: "https://google.com",
			Country:  "",
		}

		err = s.clickService.Record(ctx, info)
		require.NoError(t, err, "Record")

		s.clickService.Flush()

		count, err := s.clickService.Count(ctx, linkObj.ID)
		require.NoError(t, err, "Count")
		require.Equal(t, int64(1), count)
	})
}

func (s *ClickTestSuite) TestRecordMultipleClicks() {
	s.T().Run("records multiple clicks", func(t *testing.T) {
		ctx := context.Background()

		_, err := s.linkService.Create(ctx, link.CreateRequest{TargetURL: "https://example.com", CustomSlug: "test"})
		require.NoError(t, err, "Create")

		linkObj, err := s.linkService.Get(ctx, "test")
		require.NoError(t, err, "Get")

		for i := 0; i < 5; i++ {
			info := Info{
				LinkID: linkObj.ID,
				IPHash: "hash" + string(rune('0'+i)),
			}
			err = s.clickService.Record(ctx, info)
			require.NoError(t, err, "Record %d", i)
		}

		s.clickService.Flush()

		count, err := s.clickService.Count(ctx, linkObj.ID)
		require.NoError(t, err, "Count")
		require.Equal(t, int64(5), count)
	})
}

func (s *ClickTestSuite) TestCountZeroForNewLink() {
	s.T().Run("returns 0 for a link with no clicks", func(t *testing.T) {
		ctx := context.Background()

		_, err := s.linkService.Create(ctx, link.CreateRequest{TargetURL: "https://example.com", CustomSlug: "test"})
		require.NoError(t, err, "Create")

		linkObj, err := s.linkService.Get(ctx, "test")
		require.NoError(t, err, "Get")

		count, err := s.clickService.Count(ctx, linkObj.ID)
		require.NoError(t, err, "Count")
		require.Equal(t, int64(0), count)
	})
}

func (s *ClickTestSuite) TestCountDistinctIPs() {
	s.T().Run("counts distinct IP hashes", func(t *testing.T) {
		ctx := context.Background()

		_, err := s.linkService.Create(ctx, link.CreateRequest{TargetURL: "https://example.com", CustomSlug: "test"})
		require.NoError(t, err, "Create")

		linkObj, err := s.linkService.Get(ctx, "test")
		require.NoError(t, err, "Get")

		// 3 clicks from same IP, 2 from different IPs
		for i := 0; i < 3; i++ {
			err = s.clickService.Record(ctx, Info{LinkID: linkObj.ID, IPHash: "same-ip"})
			require.NoError(t, err)
		}
		err = s.clickService.Record(ctx, Info{LinkID: linkObj.ID, IPHash: "ip-2"})
		require.NoError(t, err)
		err = s.clickService.Record(ctx, Info{LinkID: linkObj.ID, IPHash: "ip-3"})
		require.NoError(t, err)

		s.clickService.Flush()

		count, err := s.clickService.CountDistinctIPs(ctx, linkObj.ID)
		require.NoError(t, err, "CountDistinctIPs")
		require.Equal(t, int64(3), count)
	})

	s.T().Run("returns 0 for a link with no clicks", func(t *testing.T) {
		ctx := context.Background()

		_, err := s.linkService.Create(ctx, link.CreateRequest{TargetURL: "https://example.com", CustomSlug: "test2"})
		require.NoError(t, err, "Create")

		linkObj, err := s.linkService.Get(ctx, "test2")
		require.NoError(t, err, "Get")

		count, err := s.clickService.CountDistinctIPs(ctx, linkObj.ID)
		require.NoError(t, err, "CountDistinctIPs")
		require.Equal(t, int64(0), count)
	})

	s.T().Run("counts empty IP hash as distinct", func(t *testing.T) {
		ctx := context.Background()

		_, err := s.linkService.Create(ctx, link.CreateRequest{TargetURL: "https://example.com", CustomSlug: "test3"})
		require.NoError(t, err, "Create")

		linkObj, err := s.linkService.Get(ctx, "test3")
		require.NoError(t, err, "Get")

		err = s.clickService.Record(ctx, Info{LinkID: linkObj.ID, IPHash: ""})
		require.NoError(t, err)
		err = s.clickService.Record(ctx, Info{LinkID: linkObj.ID, IPHash: "real-ip"})
		require.NoError(t, err)

		s.clickService.Flush()

		count, err := s.clickService.CountDistinctIPs(ctx, linkObj.ID)
		require.NoError(t, err, "CountDistinctIPs")
		require.Equal(t, int64(2), count)
	})
}

func (s *ClickTestSuite) TestStats() {
	s.T().Run("returns total clicks and distinct IPs in one query", func(t *testing.T) {
		ctx := context.Background()

		_, err := s.linkService.Create(ctx, link.CreateRequest{TargetURL: "https://example.com", CustomSlug: "stats-test"})
		require.NoError(t, err, "Create")

		linkObj, err := s.linkService.Get(ctx, "stats-test")
		require.NoError(t, err, "Get")

		for i := 0; i < 3; i++ {
			err = s.clickService.Record(ctx, Info{LinkID: linkObj.ID, IPHash: "same-ip"})
			require.NoError(t, err)
		}
		err = s.clickService.Record(ctx, Info{LinkID: linkObj.ID, IPHash: "ip-2"})
		require.NoError(t, err)
		err = s.clickService.Record(ctx, Info{LinkID: linkObj.ID, IPHash: "ip-3"})
		require.NoError(t, err)

		s.clickService.Flush()

		stats, err := s.clickService.Stats(ctx, linkObj.ID)
		require.NoError(t, err, "Stats")
		require.Equal(t, int64(5), stats.Total)
		require.Equal(t, int64(3), stats.DistinctIP)
	})

	s.T().Run("returns zeros for a link with no clicks", func(t *testing.T) {
		ctx := context.Background()

		_, err := s.linkService.Create(ctx, link.CreateRequest{TargetURL: "https://example.com", CustomSlug: "stats-empty"})
		require.NoError(t, err, "Create")

		linkObj, err := s.linkService.Get(ctx, "stats-empty")
		require.NoError(t, err, "Get")

		stats, err := s.clickService.Stats(ctx, linkObj.ID)
		require.NoError(t, err, "Stats")
		require.Equal(t, int64(0), stats.Total)
		require.Equal(t, int64(0), stats.DistinctIP)
	})
}

func TestClickTestSuite(t *testing.T) {
	suite.Run(t, new(ClickTestSuite))
}
