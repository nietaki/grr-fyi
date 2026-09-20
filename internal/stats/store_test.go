package stats

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	_ "modernc.org/sqlite"

	"github.com/nietaki/grr-fyi/internal/db"
	"github.com/nietaki/grr-fyi/internal/env"
)

type StoreTestSuite struct {
	suite.Suite
	conn *sql.DB
	ctx  context.Context
}

func (s *StoreTestSuite) SetupTest() {
	dbPath := filepath.Join(s.T().TempDir(), "test.sqlite")
	cfg := env.Config{DBPath: dbPath}
	s.ctx = context.Background()

	conn, err := db.Open(s.ctx, cfg)
	require.NoError(s.T(), err)
	s.conn = conn

	require.NoError(s.T(), db.Migrate(s.ctx, conn))
}

func (s *StoreTestSuite) TearDownTest() {
	s.conn.Close()
}

// newLink inserts a link and returns its id.
func (s *StoreTestSuite) newLink(slug, target string, revoked bool) int64 {
	var revokedAt any
	if revoked {
		revokedAt = "2025-01-01T00:00:00Z"
	}
	res, err := s.conn.ExecContext(s.ctx,
		"INSERT INTO links (slug, target_url, claim_key_hash, revoked_at) VALUES (?, ?, 'hash', ?)",
		slug, target, revokedAt)
	require.NoError(s.T(), err)
	id, err := res.LastInsertId()
	require.NoError(s.T(), err)
	return id
}

func (s *StoreTestSuite) click(linkID int64, n int) {
	for i := 0; i < n; i++ {
		_, err := s.conn.ExecContext(s.ctx,
			"INSERT INTO clicks (link_id, ip_hash) VALUES (?, 'ip')", linkID)
		require.NoError(s.T(), err)
	}
}

const testBaseURL = "https://grr.fyi/"

func (s *StoreTestSuite) siteStore() *Store {
	return NewStore(s.conn)
}

func (s *StoreTestSuite) TestEmptyDB() {
	stats, err := s.siteStore().SiteStats(s.ctx, testBaseURL)
	require.NoError(s.T(), err)

	s.Require().Equal(int64(0), stats.TotalLinks)
	s.Require().Equal(int64(0), stats.ActiveLinks)
	s.Require().Equal(int64(0), stats.RevokedLinks)
	s.Require().Equal(int64(0), stats.TotalClicks)
	s.Require().Equal(int64(0), stats.TotalCharsSaved)

	FillDerived(stats)
	s.Require().Equal(0.0, stats.AvgCharsSavedPerLink)
	s.Require().Equal(0.0, stats.HumanSecondsSaved)
}

func (s *StoreTestSuite) TestLinkCountsActiveRevokedSplit() {
	s.newLink("aaaa", "https://example.com/one", false)
	s.newLink("bbbb", "https://example.com/two", true)
	s.newLink("cccc", "https://example.com/three", false)

	stats, err := s.siteStore().SiteStats(s.ctx, testBaseURL)
	require.NoError(s.T(), err)

	s.Require().Equal(int64(3), stats.TotalLinks)
	s.Require().Equal(int64(2), stats.ActiveLinks)
	s.Require().Equal(int64(1), stats.RevokedLinks)
}

func (s *StoreTestSuite) TestCharsSavedCountsClicksPerLink() {
	// shortened = "https://grr.fyi/aaaa" (20 chars), target = "https://example.com/very-long" (29 chars)
	// saving per click = 9, 3 clicks -> 27
	id := s.newLink("aaaa", "https://example.com/very-long", false)
	s.click(id, 3)

	// link with no clicks contributes nothing
	s.newLink("bbbb", "https://example.com/also-very-long-indeed", false)

	stats, err := s.siteStore().SiteStats(s.ctx, testBaseURL)
	require.NoError(s.T(), err)

	s.Require().Equal(int64(3), stats.TotalClicks)
	s.Require().Equal(int64(9*3), stats.TotalCharsSaved)
}

func (s *StoreTestSuite) TestCharsSavedNegativeWhenTargetShorterThanShortURL() {
	// shortened = 20 chars, target = "http://x.io" (11 chars) -> -9 per click
	id := s.newLink("aaaa", "http://x.io", false)
	s.click(id, 2)

	stats, err := s.siteStore().SiteStats(s.ctx, testBaseURL)
	require.NoError(s.T(), err)
	s.Require().Equal(int64(-18), stats.TotalCharsSaved)
}

func (s *StoreTestSuite) TestAvgCharsSavedPerLink() {
	id := s.newLink("aaaa", "https://example.com/very-long", false) // +9 per click
	s.click(id, 2)
	s.newLink("bbbb", "https://other.example.com/much-longer-target-url", false) // 0 clicks

	stats, err := s.siteStore().SiteStats(s.ctx, testBaseURL)
	require.NoError(s.T(), err)
	FillDerived(stats)

	s.Require().Equal(float64(18)/float64(2), stats.AvgCharsSavedPerLink)
}

func (s *StoreTestSuite) TestHumanSecondsSaved() {
	// 80 wpm * 5 chars = 400 chars/min = 6.667 chars/sec -> 400 chars = 60s
	s.Require().Equal(60.0, HumanSecondsSaved(400))
	s.Require().Equal(30.0, HumanSecondsSaved(200))
	s.Require().Equal(-60.0, HumanSecondsSaved(-400))
}

func (s *StoreTestSuite) TestDBSizeBytes() {
	size, err := s.siteStore().DBSizeBytes(s.ctx)
	require.NoError(s.T(), err)
	s.Require().Greater(size, int64(0))
}

func TestStoreSuite(t *testing.T) {
	suite.Run(t, new(StoreTestSuite))
}
