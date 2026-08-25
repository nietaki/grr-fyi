package link

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"

	"github.com/nietaki/grr-fyi/internal/db"
	"github.com/nietaki/grr-fyi/internal/env"
	"github.com/nietaki/grr-fyi/internal/store"
)

func setupTestService(t *testing.T) (*Service, *sql.DB) {
	dbPath := filepath.Join(t.TempDir(), "test.sqlite")
	cfg := env.Config{DBPath: dbPath}

	ctx := context.Background()
	conn, err := db.Open(ctx, cfg)
	require.NoError(t, err, "Open")

	err = db.Migrate(ctx, conn)
	require.NoError(t, err, "Migrate")

	linkStore := NewStore(conn)
	txScope := store.NewTxScope(conn)
	return NewService(linkStore, txScope), conn
}
