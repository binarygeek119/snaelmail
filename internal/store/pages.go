package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"snailmail/internal/page"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Page represents a relay's public page.
type Page struct {
	ID          int64
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
	err = s.pool.QueryRow(ctx, `
		INSERT INTO pages (relay_id, revision, manifest, signature, status, published_at)
		VALUES ($1, (SELECT COALESCE(MAX(revision), 0) + 1 FROM pages WHERE relay_id = $1),
		        $2, $3, $4, $5)
		RETURNING id, relay_id, revision, manifest, signature, status, published_at, review_note, created_at
	`, relayID, manifestJSON, signature, status, publishedAt).Scan(
		&p.ID, &p.RelayID, &p.Revision, &p.Manifest, &p.Signature, &p.Status,
		&p.PublishedAt, &p.ReviewNote, &p.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// GetPageByRelayID fetches the latest page for a relay.
func (s *Store) GetPageByRelayID(ctx context.Context, relayID uuid.UUID) (*Page, error) {
	var p Page
	var manifestJSON []byte
	err := s.pool.QueryRow(ctx, `
		SELECT id, relay_id, revision, manifest, signature, status, published_at, review_note, created_at
		FROM pages WHERE relay_id = $1
		ORDER BY revision DESC
		LIMIT 1
	`, relayID).Scan(
		&p.ID, &p.RelayID, &p.Revision, &manifestJSON, &p.Signature, &p.Status,
		&p.PublishedAt, &p.ReviewNote, &p.CreatedAt,
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

// GetPageByID fetches a page by its ID.
func (s *Store) GetPageByID(ctx context.Context, id int64) (*Page, error) {
	var p Page
	var manifestJSON []byte
	err := s.pool.QueryRow(ctx, `
		SELECT id, relay_id, revision, manifest, signature, status, published_at, review_note, created_at
		FROM pages WHERE id = $1
	`, id).Scan(
		&p.ID, &p.RelayID, &p.Revision, &manifestJSON, &p.Signature, &p.Status,
		&p.PublishedAt, &p.ReviewNote, &p.CreatedAt,
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
