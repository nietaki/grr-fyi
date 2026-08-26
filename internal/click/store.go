package click

import (
	"context"
	"fmt"

	"github.com/nietaki/grr-fyi/internal/store"
)

type Store struct {
	DB store.DBTX
}

// NewStore creates a new Store. The db parameter can be either *sql.DB
// for regular operations or *sql.Tx for transactional operations.
func NewStore(db store.DBTX) *Store {
	return &Store{DB: db}
}

func (s *Store) Count(ctx context.Context, linkID int64) (int64, error) {
	var count int64
	err := s.DB.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM clicks WHERE link_id = ?", linkID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count clicks: %w", err)
	}
	return count, nil
}

func (s *Store) CountDistinctIPs(ctx context.Context, linkID int64) (int64, error) {
	var count int64
	err := s.DB.QueryRowContext(ctx,
		"SELECT COUNT(DISTINCT ip_hash) FROM clicks WHERE link_id = ?", linkID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count distinct ips: %w", err)
	}
	return count, nil
}

type ClickStats struct {
	Total      int64
	DistinctIP int64
}

func (s *Store) Stats(ctx context.Context, linkID int64) (*ClickStats, error) {
	var stats ClickStats
	err := s.DB.QueryRowContext(ctx,
		"SELECT COUNT(*), COUNT(DISTINCT ip_hash) FROM clicks WHERE link_id = ?", linkID).
		Scan(&stats.Total, &stats.DistinctIP)
	if err != nil {
		return nil, fmt.Errorf("click stats: %w", err)
	}
	return &stats, nil
}

func (s *Store) Insert(ctx context.Context, info Info) error {
	_, err := s.DB.ExecContext(ctx,
		"INSERT INTO clicks (link_id, ip_hash, referrer, country) VALUES (?, ?, ?, ?)",
		info.LinkID, info.IPHash, info.Referrer, info.Country)
	if err != nil {
		return fmt.Errorf("insert click: %w", err)
	}
	return nil
}
