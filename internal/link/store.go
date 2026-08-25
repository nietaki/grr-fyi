package link

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

type dbtx interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

const timeFormat = "2006-01-02 15:04:05.999999999 -0700 MST"

type Store struct {
	db   dbtx
	conn *sql.DB
}

func NewStore(conn *sql.DB) *Store {
	return &Store{db: conn, conn: conn}
}

func (s *Store) WithTx(ctx context.Context, fn func(*Store) error) error {
	if s.conn == nil {
		return errors.New("store is not connected to a database")
	}
	tx, err := s.conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	txStore := &Store{db: tx}
	if err := fn(txStore); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) CreateLink(ctx context.Context, slug, targetURL, claimKeyHash string, createdAt time.Time) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO links (slug, target_url, claim_key_hash, created_at) VALUES (?, ?, ?, ?)",
		slug, targetURL, claimKeyHash, createdAt)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return err
		}
		if isUniqueConstraintError(err) {
			return ErrSlugTaken
		}
		return fmt.Errorf("create link: %w", err)
	}
	return nil
}

func (s *Store) GetBySlug(ctx context.Context, slug string) (*Link, error) {
	var link Link
	var revokedAt sql.NullString
	var createdAt string

	err := s.db.QueryRowContext(ctx,
		"SELECT id, slug, target_url, created_at, revoked_at, claim_key_hash FROM links WHERE slug = ?",
		slug).Scan(&link.ID, &link.Slug, &link.TargetURL, &createdAt, &revokedAt, &link.ClaimKeyHash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get link by slug: %w", err)
	}

	link.CreatedAt, err = time.Parse(timeFormat, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse created_at: %w", err)
	}

	if revokedAt.Valid {
		t, err := time.Parse(timeFormat, revokedAt.String)
		if err != nil {
			return nil, fmt.Errorf("parse revoked_at: %w", err)
		}
		link.RevokedAt = &t
	}

	return &link, nil
}

func (s *Store) UpdateTargetURL(ctx context.Context, slug, newTarget string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE links SET target_url = ? WHERE slug = ?", newTarget, slug)
	if err != nil {
		return fmt.Errorf("update target url: %w", err)
	}
	return nil
}

func (s *Store) RevokeLink(ctx context.Context, slug string, revokedAt time.Time) error {
	_, err := s.db.ExecContext(ctx, "UPDATE links SET revoked_at = ? WHERE slug = ?", revokedAt, slug)
	if err != nil {
		return fmt.Errorf("revoke link: %w", err)
	}
	return nil
}

func (s *Store) NextSlugSequence(ctx context.Context) (int64, error) {
	var nextValue int64
	err := s.db.QueryRowContext(ctx, "SELECT next_value FROM slug_sequence WHERE id = 1").Scan(&nextValue)
	if err != nil {
		return 0, fmt.Errorf("get next slug sequence: %w", err)
	}
	return nextValue, nil
}

func (s *Store) SetNextSlugValue(ctx context.Context, value int64) error {
	_, err := s.db.ExecContext(ctx, "UPDATE slug_sequence SET next_value = ? WHERE id = 1", value)
	if err != nil {
		return fmt.Errorf("set next slug value: %w", err)
	}
	return nil
}

func (s *Store) SlugExists(ctx context.Context, slug string) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM links WHERE slug = ?)", slug).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check slug exists: %w", err)
	}
	return exists, nil
}

func isUniqueConstraintError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}
