package link

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"time"

	"github.com/nietaki/grr-fyi/internal/store"
)

type Service struct {
	store   *Store
	txScope *store.TxScope
}

func NewService(store *Store, txScope *store.TxScope) *Service {
	return &Service{store: store, txScope: txScope}
}

func (s *Service) Create(ctx context.Context, req CreateRequest) (*CreateResponse, error) {
	claimKey := generateClaimKey()

	hash := hashClaimKey(claimKey)

	var slug string
	var err error
	if req.CustomSlug != "" {
		if err := ValidateSlug(req.CustomSlug); err != nil {
			return nil, err
		}
		slug = req.CustomSlug
	} else {
		slug, err = s.generateAutoSlug(ctx)
		if err != nil {
			return nil, err
		}
	}

	now := time.Now().UTC()
	link, err := s.store.CreateLink(ctx, slug, req.TargetURL, hash, now)
	if err != nil {
		return nil, err
	}

	return &CreateResponse{
		Link:     link,
		ClaimKey: claimKey,
	}, nil
}

func (s *Service) generateAutoSlug(ctx context.Context) (string, error) {
	for {
		var slug string
		var advanced bool

		err := s.txScope.RunInTx(ctx, func(tx *sql.Tx) error {
			txStore := NewStore(tx)

			nextValue, err := txStore.NextSlugSequence(ctx)
			if err != nil {
				return err
			}

			slug = EncodeSlug(nextValue)

			exists, err := txStore.SlugExists(ctx, slug)
			if err != nil {
				return err
			}

			if err := txStore.SetNextSlugValue(ctx, nextValue+1); err != nil {
				return err
			}

			if !exists {
				advanced = true
			}
			return nil
		})
		if err != nil {
			return "", err
		}
		if advanced {
			return slug, nil
		}
	}
}

func (s *Service) Resolve(ctx context.Context, slug string) (*Link, error) {
	link, err := s.store.GetBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}

	if link.RevokedAt != nil {
		return nil, ErrRevoked
	}

	return link, nil
}

func (s *Service) Get(ctx context.Context, slug string) (*Link, error) {
	return s.store.GetBySlug(ctx, slug)
}

func (s *Service) GetWithClaimKey(ctx context.Context, slug, claimKey string) (*Link, error) {
	link, err := s.store.GetBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}

	if link.RevokedAt != nil {
		return nil, ErrRevoked
	}

	err = link.VerifyClaimKey(claimKey)
	if err != nil {
		return nil, err
	}

	return link, nil
}

// SlugExists checks if a slug is already in use in the database.
// Note: This returns true even for revoked links — revoked slugs cannot be re-used.
func (s *Service) SlugExists(ctx context.Context, slug string) (bool, error) {
	return s.store.SlugExists(ctx, slug)
}

func (s *Service) Update(ctx context.Context, slug, claimKey, newTarget string) error {
	link, err := s.Get(ctx, slug)
	if err != nil {
		return err
	}

	err = link.VerifyClaimKey(claimKey)
	if err != nil {
		return err
	}

	return s.store.UpdateTargetURL(ctx, slug, newTarget)
}

func (s *Service) Revoke(ctx context.Context, slug, claimKey string) error {
	link, err := s.Get(ctx, slug)
	if err != nil {
		return err
	}

	err = link.VerifyClaimKey(claimKey)
	if err != nil {
		return err
	}

	return s.store.RevokeLink(ctx, slug, time.Now().UTC())
}

func (l *Link) VerifyClaimKey(claimKey string) error {
	if l.ClaimKeyHash != hashClaimKey(claimKey) {
		return ErrInvalidClaim
	}
	return nil
}

func hashClaimKey(claimKey string) string {
	sum := sha256.Sum256([]byte(claimKey))
	return hex.EncodeToString(sum[:])
}

func generateClaimKey() string {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		panic(err)
	}

	var n uint64
	for _, b := range bytes {
		n = n*256 + uint64(b)
	}

	return EncodeSlug(int64(n % (1 << 62)))
}
