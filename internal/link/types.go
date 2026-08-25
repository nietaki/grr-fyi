package link

import (
	"errors"
	"time"
)

type CreateRequest struct {
	TargetURL  string
	CustomSlug string
}

type CreateResponse struct {
	Link     *Link
	ClaimKey string
}

type Link struct {
	ID           int64
	Slug         string
	TargetURL    string
	CreatedAt    time.Time
	RevokedAt    *time.Time
	ClickCount   int64
	ClaimKeyHash string
}

var (
	ErrNotFound     = errors.New("link not found")
	ErrRevoked      = errors.New("link revoked")
	ErrSlugTaken    = errors.New("slug already taken")
	ErrInvalidClaim = errors.New("invalid claim key")
)
