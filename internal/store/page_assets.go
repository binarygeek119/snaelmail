package store

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type PageAsset struct {
	ID         uuid.UUID `json:"id"`
	RelayID    uuid.UUID `json:"relay_id"`
	Kind       string    `json:"kind"`
	Filename   string    `json:"filename"`
	SHA256     []byte    `json:"sha256"`
	Ord        int       `json:"ord"`
	Status     string    `json:"status"`
	ReviewNote *string   `json:"review_note,omitempty"`
}

func (s *Store) UpsertPageAssetWithStatus(ctx context.Context, relayID uuid.UUID, kind, filename string, data []byte, sha [32]byte, ord int, status string) error {
	if status == "" {
		status = "pending"
	}
	_, err := s.pool.Exec(ctx, `DELETE FROM page_assets WHERE relay_id=$1 AND filename=$2`, relayID, filename)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO page_assets (id, relay_id, kind, filename, bytes, sha256, ord, status)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
	`, uuid.New(), relayID, kind, filename, data, sha[:], ord, status)
	return err
}

func (s *Store) ListPageAssets(ctx context.Context, relayID uuid.UUID) ([]PageAsset, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, relay_id, kind, filename, sha256, ord, COALESCE(status,'approved'), review_note
		FROM page_assets WHERE relay_id=$1 ORDER BY ord, filename
	`, relayID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PageAsset
	for rows.Next() {
		var a PageAsset
		if err := rows.Scan(&a.ID, &a.RelayID, &a.Kind, &a.Filename, &a.SHA256, &a.Ord, &a.Status, &a.ReviewNote); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) ListPendingPageAssets(ctx context.Context) ([]PageAsset, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, relay_id, kind, filename, sha256, ord, COALESCE(status,'pending'), review_note
		FROM page_assets WHERE status='pending' ORDER BY relay_id, ord
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PageAsset
	for rows.Next() {
		var a PageAsset
		if err := rows.Scan(&a.ID, &a.RelayID, &a.Kind, &a.Filename, &a.SHA256, &a.Ord, &a.Status, &a.ReviewNote); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) SetPageAssetStatus(ctx context.Context, assetID uuid.UUID, status string, note *string) error {
	_, err := s.pool.Exec(ctx, `UPDATE page_assets SET status=$1, review_note=$2 WHERE id=$3`, status, note, assetID)
	return err
}

func (s *Store) GetPageAssetByID(ctx context.Context, assetID uuid.UUID) (*PageAsset, []byte, error) {
	var a PageAsset
	var data []byte
	err := s.pool.QueryRow(ctx, `
		SELECT id, relay_id, kind, filename, bytes, sha256, ord, COALESCE(status,'approved'), review_note
		FROM page_assets WHERE id=$1
	`, assetID).Scan(&a.ID, &a.RelayID, &a.Kind, &a.Filename, &data, &a.SHA256, &a.Ord, &a.Status, &a.ReviewNote)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	return &a, data, nil
}
