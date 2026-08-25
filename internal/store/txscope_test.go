package store_test

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	_ "modernc.org/sqlite"

	"github.com/nietaki/grr-fyi/internal/click"
	"github.com/nietaki/grr-fyi/internal/db"
	"github.com/nietaki/grr-fyi/internal/env"
	"github.com/nietaki/grr-fyi/internal/link"
	"github.com/nietaki/grr-fyi/internal/store"
)

type TxScopeTestSuite struct {
	suite.Suite
	conn    *sql.DB
	txScope *store.TxScope
}

func (s *TxScopeTestSuite) SetupTest() {
	dbPath := filepath.Join(s.T().TempDir(), "test.sqlite")
	cfg := env.Config{DBPath: dbPath}

	ctx := context.Background()
	conn, err := db.Open(ctx, cfg)
	require.NoError(s.T(), err, "Open")

	err = db.Migrate(ctx, conn)
	require.NoError(s.T(), err, "Migrate")

	s.conn = conn
	s.txScope = store.NewTxScope(conn)
}

func (s *TxScopeTestSuite) TearDownTest() {
	s.conn.Close()
}

func (s *TxScopeTestSuite) TestCrossStoreTransactionCommit() {
	s.T().Run("commits all operations when transaction succeeds", func(t *testing.T) {
		ctx := context.Background()
		linkStore := link.NewStore(s.conn)
		clickStore := click.NewStore(s.conn)

		createdAt := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		_, err := linkStore.CreateLink(ctx, "test", "https://example.com", "hash", createdAt)
		require.NoError(t, err, "Create link")

		err = s.txScope.RunInTx(ctx, func(tx *sql.Tx) error {
			txLink := link.NewStore(tx)
			txClick := click.NewStore(tx)

			err := txLink.UpdateTargetURL(ctx, "test", "https://updated.com")
			if err != nil {
				return err
			}

			return txClick.Insert(ctx, click.Info{
				LinkID: 1,
				IPHash: "test-hash",
			})
		})
		require.NoError(t, err, "Cross-store transaction should succeed")

		updatedLink, err := linkStore.GetBySlug(ctx, "test")
		require.NoError(t, err, "Get updated link")
		require.Equal(t, "https://updated.com", updatedLink.TargetURL)

		count, err := clickStore.Count(ctx, 1)
		require.NoError(t, err, "Count clicks")
		require.Equal(t, int64(1), count)
	})
}

func (s *TxScopeTestSuite) TestCrossStoreTransactionRollback() {
	s.T().Run("rolls back all operations when transaction fails", func(t *testing.T) {
		ctx := context.Background()
		linkStore := link.NewStore(s.conn)
		clickStore := click.NewStore(s.conn)

		createdAt := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		_, err := linkStore.CreateLink(ctx, "rollback-test", "https://example.com", "hash", createdAt)
		require.NoError(t, err, "Create link")

		originalLink, err := linkStore.GetBySlug(ctx, "rollback-test")
		require.NoError(t, err, "Get original link")
		originalURL := originalLink.TargetURL

		err = s.txScope.RunInTx(ctx, func(tx *sql.Tx) error {
			txLink := link.NewStore(tx)
			txClick := click.NewStore(tx)

			err := txLink.UpdateTargetURL(ctx, "rollback-test", "https://should-rollback.com")
			if err != nil {
				return err
			}

			err = txClick.Insert(ctx, click.Info{
				LinkID: 1,
				IPHash: "test-hash",
			})
			if err != nil {
				return err
			}

			return fmt.Errorf("simulated failure")
		})
		require.Error(t, err, "Transaction should fail")

		rolledBackLink, err := linkStore.GetBySlug(ctx, "rollback-test")
		require.NoError(t, err, "Get link after rollback")
		require.Equal(t, originalURL, rolledBackLink.TargetURL, "URL should be unchanged")

		count, err := clickStore.Count(ctx, 1)
		require.NoError(t, err, "Count clicks after rollback")
		require.Equal(t, int64(0), count, "Click should not exist")
	})
}

func TestTxScopeTestSuite(t *testing.T) {
	suite.Run(t, new(TxScopeTestSuite))
}
