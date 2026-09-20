package stats

import (
	"context"
	"fmt"

	"github.com/nietaki/grr-fyi/internal/store"
)

// WPM and chars-per-word assumptions for the "human time saved" stat.
const (
	typingWPM     = 80
	charsPerWord  = 5
	humanCharRate = typingWPM * charsPerWord // characters typed per minute
)

type Store struct {
	DB store.DBTX
}

func NewStore(db store.DBTX) *Store {
	return &Store{DB: db}
}

// SiteStats computes aggregate link/click counters. baseURL is the normalized
// site URL including trailing slash (e.g. "https://grr.fyi/"); the shortened
// length of a link is len(baseURL) + len(slug).
func (s *Store) SiteStats(ctx context.Context, baseURL string) (*SiteStats, error) {
	stats := &SiteStats{}

	err := s.DB.QueryRowContext(ctx,
		`SELECT COUNT(*),
		        COALESCE(SUM(revoked_at IS NULL), 0),
		        COALESCE(SUM(revoked_at IS NOT NULL), 0)
		 FROM links`).
		Scan(&stats.TotalLinks, &stats.ActiveLinks, &stats.RevokedLinks)
	if err != nil {
		return nil, fmt.Errorf("stats: count links: %w", err)
	}

	err = s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM clicks`).
		Scan(&stats.TotalClicks)
	if err != nil {
		return nil, fmt.Errorf("stats: count clicks: %w", err)
	}

	err = s.DB.QueryRowContext(ctx,
		`SELECT COALESCE(SUM((LENGTH(l.target_url) - (? + LENGTH(l.slug))) * c.cnt), 0)
		 FROM links l
		 JOIN (SELECT link_id, COUNT(*) AS cnt FROM clicks GROUP BY link_id) c
		   ON c.link_id = l.id`,
		len(baseURL)).
		Scan(&stats.TotalCharsSaved)
	if err != nil {
		return nil, fmt.Errorf("stats: chars saved: %w", err)
	}

	return stats, nil
}

// DBSizeBytes returns the SQLite database file size in bytes.
func (s *Store) DBSizeBytes(ctx context.Context) (int64, error) {
	var size int64
	err := s.DB.QueryRowContext(ctx,
		`SELECT page_count * page_size
		 FROM pragma_page_count(), pragma_page_size()`).
		Scan(&size)
	if err != nil {
		return 0, fmt.Errorf("stats: db size: %w", err)
	}
	return size, nil
}
