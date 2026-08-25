package link

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const timeFormat = "2006-01-02 15:04:05.999999999 -0700 MST"

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

func (s *Service) Create(ctx context.Context, req CreateRequest) (*CreateResponse, error) {
	claimKey, err := generateClaimKey()
	if err != nil {
		return nil, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(claimKey), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	var slug string
	if req.CustomSlug != "" {
		slug = req.CustomSlug
	} else {
		slug, err = s.generateAutoSlug(ctx)
		if err != nil {
			return nil, err
		}
	}

	now := time.Now().UTC()
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO links (slug, target_url, claim_key_hash, created_at) VALUES (?, ?, ?, ?)`,
		slug, req.TargetURL, string(hash), now)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return nil, err
		}
		if isUniqueConstraintError(err) {
			return nil, ErrSlugTaken
		}
		return nil, err
	}

	link := &Link{
		Slug:       slug,
		TargetURL:  req.TargetURL,
		CreatedAt:  now,
		ClickCount: 0,
	}

	return &CreateResponse{
		Link:     link,
		ClaimKey: claimKey,
	}, nil
}

func (s *Service) generateAutoSlug(ctx context.Context) (string, error) {
	for {
		var nextValue int64
		err := s.db.QueryRowContext(ctx, "SELECT next_value FROM slug_sequence WHERE id = 1").Scan(&nextValue)
		if err != nil {
			return "", err
		}

		slug := encodeBase62(nextValue)

		var exists bool
		err = s.db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM links WHERE slug = ?)", slug).Scan(&exists)
		if err != nil {
			return "", err
		}

		_, err = s.db.ExecContext(ctx, "UPDATE slug_sequence SET next_value = ? WHERE id = 1", nextValue+1)
		if err != nil {
			return "", err
		}

		if !exists {
			return slug, nil
		}
	}
}

func (s *Service) Resolve(ctx context.Context, slug string) (*Link, error) {
	link, err := s.scanLinkBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}

	if link.RevokedAt != nil {
		return nil, ErrRevoked
	}

	return link, nil
}

func (s *Service) Get(ctx context.Context, slug string) (*Link, error) {
	return s.scanLinkBySlug(ctx, slug)
}

func (s *Service) scanLinkBySlug(ctx context.Context, slug string) (*Link, error) {
	var link Link
	var revokedAt sql.NullString
	var createdAt string

	err := s.db.QueryRowContext(ctx,
		`SELECT id, slug, target_url, created_at, revoked_at, claim_key_hash FROM links WHERE slug = ?`,
		slug).Scan(&link.ID, &link.Slug, &link.TargetURL, &createdAt, &revokedAt, &link.ClaimKeyHash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	link.CreatedAt, err = time.Parse(timeFormat, createdAt)
	if err != nil {
		return nil, err
	}

	if revokedAt.Valid {
		t, err := time.Parse(timeFormat, revokedAt.String)
		if err != nil {
			return nil, err
		}
		link.RevokedAt = &t
	}

	return &link, nil
}

func (s *Service) Update(ctx context.Context, slug, claimKey, newTarget string) error {
	link, err := s.Get(ctx, slug)
	if err != nil {
		return err
	}

	err = verifyClaimKey(link, claimKey)
	if err != nil {
		return err
	}

	_, err = s.db.ExecContext(ctx,
		`UPDATE links SET target_url = ? WHERE slug = ?`,
		newTarget, slug)
	return err
}

func (s *Service) Revoke(ctx context.Context, slug, claimKey string) error {
	link, err := s.Get(ctx, slug)
	if err != nil {
		return err
	}

	err = verifyClaimKey(link, claimKey)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	_, err = s.db.ExecContext(ctx,
		`UPDATE links SET revoked_at = ? WHERE slug = ?`,
		now, slug)
	return err
}

func verifyClaimKey(link *Link, claimKey string) error {
	err := bcrypt.CompareHashAndPassword([]byte(link.ClaimKeyHash), []byte(claimKey))
	if err != nil {
		return ErrInvalidClaim
	}
	return nil
}

func generateClaimKey() (string, error) {
	bytes := make([]byte, 8)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}

	var n uint64
	for _, b := range bytes {
		n = n*256 + uint64(b)
	}

	return encodeBase62(int64(n % (1 << 62))), nil
}

func isUniqueConstraintError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}
