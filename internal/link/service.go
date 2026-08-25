package link

import (
	"context"
	"crypto/rand"
	"database/sql"
	"time"

	"github.com/nietaki/grr-fyi/internal/store"
	"golang.org/x/crypto/bcrypt"
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
	link, err := s.store.CreateLink(ctx, slug, req.TargetURL, string(hash), now)
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

			slug = EncodeBase62(nextValue)

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

	err = verifyClaimKey(link, claimKey)
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

	err = verifyClaimKey(link, claimKey)
	if err != nil {
		return err
	}

	return s.store.RevokeLink(ctx, slug, time.Now().UTC())
}

func verifyClaimKey(link *Link, claimKey string) error {
	err := bcrypt.CompareHashAndPassword([]byte(link.ClaimKeyHash), []byte(claimKey))
	if err != nil {
		return ErrInvalidClaim
	}
	return nil
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

	return EncodeBase62(int64(n % (1 << 62)))
}
