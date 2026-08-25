package click

import (
	"context"
	"database/sql"
	"fmt"
)

type dbtx interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type Store struct {
	db dbtx
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Count(ctx context.Context, linkID int64) (int64, error) {
	var count int64
	err := s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM clicks WHERE link_id = ?", linkID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count clicks: %w", err)
	}
	return count, nil
}

func (s *Store) Insert(ctx context.Context, info Info) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO clicks (link_id, ip_hash, referrer, country) VALUES (?, ?, ?, ?)",
		info.LinkID, info.IPHash, info.Referrer, info.Country)
	if err != nil {
		return fmt.Errorf("insert click: %w", err)
	}
	return nil
}
