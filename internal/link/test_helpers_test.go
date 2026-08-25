package link

import (
	"context"
	"database/sql"
	"path/filepath"

	"github.com/mvrahden/go-test/pkg/gotest"
	_ "modernc.org/sqlite"

	"github.com/nietaki/grr-fyi/internal/db"
	"github.com/nietaki/grr-fyi/internal/env"
	"github.com/nietaki/grr-fyi/internal/store"
)

func setupTestService(t *gotest.T) (*Service, *sql.DB) {
	dbPath := filepath.Join(t.T().TempDir(), "test.sqlite")
	cfg := env.Config{DBPath: dbPath}

	ctx := context.Background()
	conn, err := db.Open(ctx, cfg)
	gotest.NoError(t, err, "Open")

	err = db.Migrate(ctx, conn)
	gotest.NoError(t, err, "Migrate")

	linkStore := NewStore(conn)
	txScope := store.NewTxScope(conn)
	return NewService(linkStore, txScope), conn
}
