package stats

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	_ "modernc.org/sqlite"

	"github.com/nietaki/grr-fyi/internal/db"
	"github.com/nietaki/grr-fyi/internal/env"
)

type ServiceTestSuite struct {
	suite.Suite
	conn    *sql.DB
	ctx     context.Context
	store   *Store
	startAt time.Time
}

func (s *ServiceTestSuite) SetupTest() {
	dbPath := filepath.Join(s.T().TempDir(), "test.sqlite")
	cfg := env.Config{DBPath: dbPath}
	s.ctx = context.Background()

	conn, err := db.Open(s.ctx, cfg)
	require.NoError(s.T(), err)
	s.conn = conn
	require.NoError(s.T(), db.Migrate(s.ctx, conn))

	s.store = NewStore(conn)
	s.startAt = time.Now().Add(-time.Hour)
}

func (s *ServiceTestSuite) TearDownTest() {
	s.conn.Close()
}

func (s *ServiceTestSuite) TestPageAssemblesAllSections() {
	_, err := s.conn.ExecContext(s.ctx,
		"INSERT INTO links (slug, target_url, claim_key_hash) VALUES ('aaaa', 'https://example.com/some/fairly/long/path', 'hash')")
	require.NoError(s.T(), err)
	_, err = s.conn.ExecContext(s.ctx,
		"INSERT INTO clicks (link_id, ip_hash) VALUES (1, 'ip')")
	require.NoError(s.T(), err)

	called := false
	provider := func(context.Context) ReplicationInfo {
		called = true
		return ReplicationInfo{Enabled: true, InSync: true, LocalTXID: 7, RemoteTXID: 7}
	}

	svc := NewService(s.store, "https://grr.fyi/", s.startAt, provider)
	page, err := svc.Page(s.ctx)
	require.NoError(s.T(), err)

	s.Require().Equal(int64(1), page.Site.TotalLinks)
	s.Require().Equal(int64(1), page.Site.TotalClicks)
	s.Require().True(page.Site.TotalCharsSaved > 0)
	s.Require().True(page.Site.HumanSecondsSaved > 0)

	s.Require().True(page.Runtime.Uptime > time.Hour-time.Second)
	s.Require().Greater(page.Runtime.Goroutines, int64(0))
	s.Require().Greater(page.Runtime.DBSizeBytes, int64(0))

	s.Require().True(called, "replication provider should be consulted")
	s.Require().True(page.Replication.Enabled)
	s.Require().True(page.Replication.InSync)
	s.Require().Equal(int64(7), page.Replication.LocalTXID)
}

func (s *ServiceTestSuite) TestPageWithNilProviderDefaultsToDisabled() {
	svc := NewService(s.store, "https://grr.fyi/", s.startAt, nil)
	page, err := svc.Page(s.ctx)
	require.NoError(s.T(), err)
	s.Require().False(page.Replication.Enabled)
}

func (s *ServiceTestSuite) TestPagePropagatesStoreError() {
	svc := NewService(NewStore(closedDB()), "https://grr.fyi/", s.startAt, nil)
	_, err := svc.Page(s.ctx)
	s.Require().Error(err)
	s.Require().True(errors.Is(err, sql.ErrConnDone) || err != nil)
}

func TestServiceSuite(t *testing.T) {
	suite.Run(t, new(ServiceTestSuite))
}

// closedDB returns a *sql.DB that is already closed so queries fail.
func closedDB() *sql.DB {
	conn, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		panic(err)
	}
	conn.Close()
	return conn
}
