package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Session represents a user session
type Session struct {
	Token     string
	UserID    uuid.UUID
	CreatedAt time.Time
	ExpiresAt time.Time
	IsActive  bool
}

// CreateSession creates a new session for a user
func (s *Store) CreateSession(ctx context.Context, userID uuid.UUID, token string, expiresAt time.Time) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO sessions (token, user_id, expires_at)
		VALUES ($1, $2, $3)
	`, token, userID, expiresAt)
	if err != nil {
		return err
	}
	return nil
}

// GetSession retrieves a session by token
func (s *Store) GetSession(ctx context.Context, token string) (*Session, error) {
	var session Session
	err := s.pool.QueryRow(ctx, `
		SELECT token, user_id, created_at, expires_at, is_active
		FROM sessions
		WHERE token = $1 AND is_active = true AND expires_at > NOW()
	`, token).Scan(
		&session.Token,
		&session.UserID,
		&session.CreatedAt,
		&session.ExpiresAt,
		&session.IsActive,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &session, nil
}

// DeleteSession invalidates a session
func (s *Store) DeleteSession(ctx context.Context, token string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE sessions
		SET is_active = false
		WHERE token = $1
	`, token)
	return err
}

// CleanupExpiredSessions removes expired sessions
func (s *Store) CleanupExpiredSessions(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `
		DELETE FROM sessions
		WHERE expires_at < NOW()
	`)
	return err
}

// GetAllUsers returns all users
func (s *Store) GetAllUsers(ctx context.Context) ([]User, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, handle, address, pass_hash, display_name, home_relay_id,
		       home_relay_changed_at, status, storage_bytes, letters_today, quota_day, created_at, is_admin, city, state, country
		FROM users
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		err := rows.Scan(
			&u.ID, &u.Handle, &u.Address, &u.PassHash, &u.DisplayName, &u.HomeRelayID,
			&u.HomeRelayChangedAt, &u.Status, &u.StorageBytes, &u.LettersToday, &u.QuotaDay, &u.CreatedAt, &u.IsAdmin, &u.City, &u.State, &u.Country,
		)
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}

// GetAllRelays returns all relays
func (s *Store) GetAllRelays(ctx context.Context) ([]Relay, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, hostname, pubkey, key_fpr, city, region, cc, lat, lon, tz,
		       stamp_svg, stamp_png, stamp_sha256, plate_json, plate_sig,
		       status, official, paused, central, opened_at, retired_at, last_txt_check,
		       letters_passed, complaints_30d
		FROM relays
		ORDER BY letters_passed DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var relays []Relay
	for rows.Next() {
		var r Relay
		err := rows.Scan(
			&r.ID, &r.Hostname, &r.Pubkey, &r.KeyFpr, &r.City, &r.Region, &r.CC, &r.Lat, &r.Lon, &r.TZ,
			&r.StampSVG, &r.StampPNG, &r.StampSHA256, &r.PlateJSON, &r.PlateSig,
			&r.Status, &r.Official, &r.Paused, &r.Central, &r.OpenedAt, &r.RetiredAt, &r.LastTXTCheck,
			&r.LettersPassed, &r.Complaints30d,
		)
		if err != nil {
			return nil, err
		}
		relays = append(relays, r)
	}
	return relays, nil
}

// GetAllLetters returns all letters with sender and recipient information
func (s *Store) GetAllLetters(ctx context.Context) ([]map[string]any, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT e.id, e.subject, e.state, e.queued_at,
		       s.address as sender_address,
		       COALESCE(r.address, e.external_to) as recipient_address
		FROM envelopes e
		LEFT JOIN users s ON e.sender_id = s.id
		LEFT JOIN users r ON e.recipient_id = r.id
		ORDER BY e.queued_at DESC
		LIMIT 100
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var letters []map[string]any
	for rows.Next() {
		var id uuid.UUID
		var subject, state string
		var queuedAt string
		var senderAddress, recipientAddress *string

		if err := rows.Scan(&id, &subject, &state, &queuedAt, &senderAddress, &recipientAddress); err != nil {
			return nil, err
		}

		letter := map[string]any{
			"id":        id,
			"subject":   subject,
			"state":     state,
			"queued_at": queuedAt,
		}

		if senderAddress != nil {
			letter["sender_address"] = *senderAddress
		}
		if recipientAddress != nil {
			letter["recipient_address"] = *recipientAddress
		}

		letters = append(letters, letter)
	}
	return letters, nil
}
