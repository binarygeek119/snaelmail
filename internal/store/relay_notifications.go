package store

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type RelayNotification struct {
	ID        uuid.UUID  `json:"id"`
	RelayID   uuid.UUID  `json:"relay_id"`
	Type      string     `json:"type"`
	Message   string     `json:"message"`
	Severity  string     `json:"severity"`
	ReadAt    *time.Time `json:"read_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

func (s *Store) CreateRelayNotification(ctx context.Context, n *RelayNotification) error {
	if n.ID == uuid.Nil {
		n.ID = uuid.New()
	}
	if n.CreatedAt.IsZero() {
		n.CreatedAt = time.Now()
	}
	if n.Severity == "" {
		n.Severity = "info"
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO relay_notifications (id, relay_id, type, message, severity, created_at)
		VALUES ($1,$2,$3,$4,$5,$6)
	`, n.ID, n.RelayID, n.Type, n.Message, n.Severity, n.CreatedAt)
	return err
}

func (s *Store) ListRelayNotifications(ctx context.Context, relayID uuid.UUID, limit int) ([]RelayNotification, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, relay_id, type, message, severity, read_at, created_at
		FROM relay_notifications WHERE relay_id=$1
		ORDER BY created_at DESC LIMIT $2
	`, relayID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RelayNotification
	for rows.Next() {
		var n RelayNotification
		if err := rows.Scan(&n.ID, &n.RelayID, &n.Type, &n.Message, &n.Severity, &n.ReadAt, &n.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (s *Store) MarkRelayNotificationsRead(ctx context.Context, relayID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `UPDATE relay_notifications SET read_at=now() WHERE relay_id=$1 AND read_at IS NULL`, relayID)
	return err
}
