package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"snailmail/internal/page"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Page represents a relay's public page.
type Page struct {
	RelayID     uuid.UUID
	Revision    int
	Manifest    page.Manifest
	Signature   []byte
	Status      string // draft, review, live, rejected
	PublishedAt *time.Time
	ReviewNote  *string
	CreatedAt   time.Time
}

// CreatePage creates a new page or updates an existing one for a relay.
func (s *Store) CreatePage(ctx context.Context, relayID uuid.UUID, manifest page.Manifest, signature []byte, status string) (*Page, error) {
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		return nil, err
	}

	var publishedAt *time.Time
	if status == "live" {
		now := time.Now()
		publishedAt = &now
	}

	var p Page
	var mJSON []byte
	err = s.pool.QueryRow(ctx, `
		INSERT INTO pages (relay_id, revision, manifest, sig, status, published_at)
		VALUES ($1, (SELECT COALESCE(MAX(revision), 0) + 1 FROM pages WHERE relay_id = $1),
		        $2, $3, $4, $5)
		RETURNING relay_id, revision, manifest, sig, status, published_at, review_note
	`, relayID, manifestJSON, signature, status, publishedAt).Scan(
		&p.RelayID, &p.Revision, &mJSON, &p.Signature, &p.Status,
		&p.PublishedAt, &p.ReviewNote,
	)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(mJSON, &p.Manifest); err != nil {
		return nil, err
	}
	return &p, nil
}

// GetPageByRelayID fetches the latest page for a relay.
func (s *Store) GetPageByRelayID(ctx context.Context, relayID uuid.UUID) (*Page, error) {
	var p Page
	var manifestJSON []byte
	err := s.pool.QueryRow(ctx, `
		SELECT relay_id, revision, manifest, sig, status, published_at, review_note
		FROM pages WHERE relay_id = $1
		ORDER BY revision DESC
		LIMIT 1
	`, relayID).Scan(
		&p.RelayID, &p.Revision, &manifestJSON, &p.Signature, &p.Status,
		&p.PublishedAt, &p.ReviewNote,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(manifestJSON, &p.Manifest); err != nil {
		return nil, err
	}
	return &p, nil
}

// GetPageByID fetches a page by its relay_id (since there's no separate id column).
func (s *Store) GetPageByID(ctx context.Context, id int64) (*Page, error) {
	// Since pages use relay_id as PK, this function is deprecated
	// Use GetPageByRelayID instead
	return nil, fmt.Errorf("GetPageByID not supported, use GetPageByRelayID")
}

// UpdatePageStatus updates the status of a page.
func (s *Store) UpdatePageStatus(ctx context.Context, id int64, status string, reviewNote *string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE pages SET status = $1, review_note = $2 WHERE id = $3
	`, status, reviewNote, id)
	return err
}

// PublishPage sets a page to live status with the current timestamp.
func (s *Store) PublishPage(ctx context.Context, id int64) error {
	now := time.Now()
	_, err := s.pool.Exec(ctx, `
		UPDATE pages SET status = 'live', published_at = $1 WHERE id = $2
	`, now, id)
	return err
}

// IncrementReviewClean increments the review_clean counter for a relay.
func (s *Store) IncrementReviewClean(ctx context.Context, relayID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE relays SET review_clean = review_clean + 1 WHERE id = $1
	`, relayID)
	return err
}

// GetRelayReviewClean returns the review_clean counter for a relay.
func (s *Store) GetRelayReviewClean(ctx context.Context, relayID uuid.UUID) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx, `SELECT review_clean FROM relays WHERE id = $1`, relayID).Scan(&count)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	return count, err
}

// UpsertPageAsset stores a page asset (image/logo) for a relay.
func (s *Store) UpsertPageAsset(ctx context.Context, relayID uuid.UUID, kind, filename string, data []byte, sha [32]byte, ord int) error {
	_, err := s.pool.Exec(ctx, `
		DELETE FROM page_assets WHERE relay_id = $1 AND filename = $2
	`, relayID, filename)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO page_assets (id, relay_id, kind, filename, bytes, sha256, ord)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, uuid.New(), relayID, kind, filename, data, sha[:], ord)
	return err
}

// GetPageAsset fetches a page asset by relay and filename.
func (s *Store) GetPageAsset(ctx context.Context, relayID uuid.UUID, filename string) ([]byte, error) {
	var data []byte
	err := s.pool.QueryRow(ctx, `
		SELECT bytes FROM page_assets WHERE relay_id = $1 AND filename = $2
		ORDER BY id DESC LIMIT 1
	`, relayID, filename).Scan(&data)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return data, err
}
