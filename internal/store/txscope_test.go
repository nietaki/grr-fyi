package store_test

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"time"

	"github.com/mvrahden/go-test/pkg/gotest"
	_ "modernc.org/sqlite"

	"github.com/nietaki/grr-fyi/internal/click"
	"github.com/nietaki/grr-fyi/internal/db"
	"github.com/nietaki/grr-fyi/internal/env"
	"github.com/nietaki/grr-fyi/internal/link"
	"github.com/nietaki/grr-fyi/internal/store"
)

type TxScopeTestSuite struct {
	conn    *sql.DB
	txScope *store.TxScope
}

func (s *TxScopeTestSuite) BeforeEach(t *gotest.T) {
	dbPath := filepath.Join(t.T().TempDir(), "test.sqlite")
	cfg := env.Config{DBPath: dbPath}

	ctx := context.Background()
	conn, err := db.Open(ctx, cfg)
	gotest.NoError(t, err, "Open")

	err = db.Migrate(ctx, conn)
	gotest.NoError(t, err, "Migrate")

	s.conn = conn
	s.txScope = store.NewTxScope(conn)
}

func (s *TxScopeTestSuite) AfterEach(t *gotest.T) {
	s.conn.Close()
}

func (s *TxScopeTestSuite) TestCrossStoreTransactionCommit(t *gotest.T) {
	t.It("commits all operations when transaction succeeds", func(it *gotest.T) {
		ctx := context.Background()
		linkStore := link.NewStore(s.conn)
		clickStore := click.NewStore(s.conn)

		// Create a link to work with
		createdAt := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		err := linkStore.CreateLink(ctx, "test", "https://example.com", "hash", createdAt)
		gotest.NoError(it, err, "Create link")

		// Perform cross-store transaction
		err = s.txScope.RunInTx(ctx, func(tx *sql.Tx) error {
			txLink := link.NewStore(tx)
			txClick := click.NewStore(tx)

			// Update the link
			err := txLink.UpdateTargetURL(ctx, "test", "https://updated.com")
			if err != nil {
				return err
			}

			// Insert a click
			return txClick.Insert(ctx, click.Info{
				LinkID: 1,
				IPHash: "test-hash",
			})
		})
		gotest.NoError(it, err, "Cross-store transaction should succeed")

		// Verify both operations persisted
		updatedLink, err := linkStore.GetBySlug(ctx, "test")
		gotest.NoError(it, err, "Get updated link")
		gotest.Equal(it, "https://updated.com", updatedLink.TargetURL)

		count, err := clickStore.Count(ctx, 1)
		gotest.NoError(it, err, "Count clicks")
		gotest.Equal(it, int64(1), count)
	})
}

func (s *TxScopeTestSuite) TestCrossStoreTransactionRollback(t *gotest.T) {
	t.It("rolls back all operations when transaction fails", func(it *gotest.T) {
		ctx := context.Background()
		linkStore := link.NewStore(s.conn)
		clickStore := click.NewStore(s.conn)

		// Create a link to work with
		createdAt := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		err := linkStore.CreateLink(ctx, "rollback-test", "https://example.com", "hash", createdAt)
		gotest.NoError(it, err, "Create link")

		// Get original state
		originalLink, err := linkStore.GetBySlug(ctx, "rollback-test")
		gotest.NoError(it, err, "Get original link")
		originalURL := originalLink.TargetURL

		// Perform transaction that fails partway through
		err = s.txScope.RunInTx(ctx, func(tx *sql.Tx) error {
			txLink := link.NewStore(tx)
			txClick := click.NewStore(tx)

			// Update the link
			err := txLink.UpdateTargetURL(ctx, "rollback-test", "https://should-rollback.com")
			if err != nil {
				return err
			}

			// Insert a click
			err = txClick.Insert(ctx, click.Info{
				LinkID: 1,
				IPHash: "test-hash",
			})
			if err != nil {
				return err
			}

			// Simulate a failure
			return fmt.Errorf("simulated failure")
		})
		gotest.Error(it, err, "Transaction should fail")

		// Verify both operations were rolled back
		rolledBackLink, err := linkStore.GetBySlug(ctx, "rollback-test")
		gotest.NoError(it, err, "Get link after rollback")
		gotest.Equal(it, originalURL, rolledBackLink.TargetURL, "URL should be unchanged")

		count, err := clickStore.Count(ctx, 1)
		gotest.NoError(it, err, "Count clicks after rollback")
		gotest.Equal(it, int64(0), count, "Click should not exist")
	})
}
