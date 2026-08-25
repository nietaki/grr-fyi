package link

import (
	"context"
	"crypto/rand"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	store *Store
}

func NewService(store *Store) *Service {
	return &Service{store: store}
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
	err = s.store.CreateLink(ctx, slug, req.TargetURL, string(hash), now)
	if err != nil {
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
		var slug string
		var advanced bool

		err := s.store.WithTx(ctx, func(tx *Store) error {
			nextValue, err := tx.NextSlugSequence(ctx)
			if err != nil {
				return err
			}

			slug = encodeBase62(nextValue)

			exists, err := tx.SlugExists(ctx, slug)
			if err != nil {
				return err
			}

			if err := tx.SetNextSlugValue(ctx, nextValue+1); err != nil {
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
