package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// User represents a registered user.
type User struct {
	ID                uuid.UUID
	Handle            string
	Address           string
	Email             *string
	PassHash          string
	DisplayName       *string
	HomeRelayID       *uuid.UUID
	HomeRelayChangedAt *time.Time
	Status            string
	StorageBytes      int64
	LettersToday      int
	QuotaDay          time.Time
	CreatedAt         time.Time
}

// CreateUser inserts a new user. Returns the created user with generated fields.
func (s *Store) CreateUser(ctx context.Context, handle, address, passHash string, email *string, displayName *string, homeRelayID *uuid.UUID) (*User, error) {
	id := uuid.New()
	var u User
	err := s.pool.QueryRow(ctx, `
		INSERT INTO users (id, handle, address, email, pass_hash, display_name, home_relay_id, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 'unverified')
		RETURNING id, handle, address, email, pass_hash, display_name, home_relay_id,
		          home_relay_changed_at, status, storage_bytes, letters_today, quota_day, created_at
	`, id, handle, address, email, passHash, displayName, homeRelayID).Scan(
		&u.ID, &u.Handle, &u.Address, &u.Email, &u.PassHash, &u.DisplayName, &u.HomeRelayID,
		&u.HomeRelayChangedAt, &u.Status, &u.StorageBytes, &u.LettersToday, &u.QuotaDay, &u.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	return &u, nil
}

// GetUserByID fetches a user by ID.
func (s *Store) GetUserByID(ctx context.Context, id uuid.UUID) (*User, error) {
	var u User
	err := s.pool.QueryRow(ctx, `
		SELECT id, handle, address, email, pass_hash, display_name, home_relay_id,
		       home_relay_changed_at, status, storage_bytes, letters_today, quota_day, created_at
		FROM users WHERE id = $1
	`, id).Scan(
		&u.ID, &u.Handle, &u.Address, &u.Email, &u.PassHash, &u.DisplayName, &u.HomeRelayID,
		&u.HomeRelayChangedAt, &u.Status, &u.StorageBytes, &u.LettersToday, &u.QuotaDay, &u.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return &u, nil
}

// GetUserByHandle fetches a user by handle (case-insensitive via citext).
func (s *Store) GetUserByHandle(ctx context.Context, handle string) (*User, error) {
	var u User
	err := s.pool.QueryRow(ctx, `
		SELECT id, handle, address, email, pass_hash, display_name, home_relay_id,
		       home_relay_changed_at, status, storage_bytes, letters_today, quota_day, created_at
		FROM users WHERE handle = $1
	`, handle).Scan(
		&u.ID, &u.Handle, &u.Address, &u.Email, &u.PassHash, &u.DisplayName, &u.HomeRelayID,
		&u.HomeRelayChangedAt, &u.Status, &u.StorageBytes, &u.LettersToday, &u.QuotaDay, &u.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user by handle: %w", err)
	}
	return &u, nil
}

// GetUserByAddress fetches a user by email address (case-insensitive via citext).
func (s *Store) GetUserByAddress(ctx context.Context, address string) (*User, error) {
	var u User
	err := s.pool.QueryRow(ctx, `
		SELECT id, handle, address, email, pass_hash, display_name, home_relay_id,
		       home_relay_changed_at, status, storage_bytes, letters_today, quota_day, created_at
		FROM users WHERE address = $1
	`, address).Scan(
		&u.ID, &u.Handle, &u.Address, &u.Email, &u.PassHash, &u.DisplayName, &u.HomeRelayID,
		&u.HomeRelayChangedAt, &u.Status, &u.StorageBytes, &u.LettersToday, &u.QuotaDay, &u.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user by address: %w", err)
	}
	return &u, nil
}

// GetUserByEmail fetches a user by email address (case-insensitive via citext).
func (s *Store) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	var u User
	err := s.pool.QueryRow(ctx, `
		SELECT id, handle, address, email, pass_hash, display_name, home_relay_id,
		       home_relay_changed_at, status, storage_bytes, letters_today, quota_day, created_at
		FROM users WHERE email = $1
	`, email).Scan(
		&u.ID, &u.Handle, &u.Address, &u.Email, &u.PassHash, &u.DisplayName, &u.HomeRelayID,
		&u.HomeRelayChangedAt, &u.Status, &u.StorageBytes, &u.LettersToday, &u.QuotaDay, &u.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	return &u, nil
}

// VerifyUser sets user status to 'verified'.
func (s *Store) VerifyUser(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `UPDATE users SET status = 'verified' WHERE id = $1`, id)
	return err
}

// Verification represents a pending verification code.
type Verification struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	CodeHash   []byte
	Kind       string
	ExpiresAt  time.Time
	Attempts   int
	ConsumedAt *time.Time
}

// CreateVerification inserts a new verification record.
func (s *Store) CreateVerification(ctx context.Context, userID uuid.UUID, code, kind string, expiresAt time.Time) (*Verification, error) {
	id := uuid.New()
	hash := sha256.Sum256([]byte(code))
	var v Verification
	err := s.pool.QueryRow(ctx, `
		INSERT INTO verifications (id, user_id, code_hash, kind, expires_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, user_id, code_hash, kind, expires_at, attempts, consumed_at
	`, id, userID, hash[:], kind, expiresAt).Scan(
		&v.ID, &v.UserID, &v.CodeHash, &v.Kind, &v.ExpiresAt, &v.Attempts, &v.ConsumedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create verification: %w", err)
	}
	return &v, nil
}

// ConsumeVerification checks and consumes a verification code.
// Returns error if code is invalid, expired, or max attempts exceeded.
func (s *Store) ConsumeVerification(ctx context.Context, userID uuid.UUID, code, kind string) error {
	hash := sha256.Sum256([]byte(code))
	hashHex := hex.EncodeToString(hash[:])

	// Fetch unexpired, unconsumed verification for this user and kind
	var v Verification
	err := s.pool.QueryRow(ctx, `
		SELECT id, user_id, code_hash, kind, expires_at, attempts, consumed_at
		FROM verifications
		WHERE user_id = $1 AND kind = $2 AND consumed_at IS NULL AND expires_at > now()
		ORDER BY expires_at DESC
		LIMIT 1
	`, userID, kind).Scan(
		&v.ID, &v.UserID, &v.CodeHash, &v.Kind, &v.ExpiresAt, &v.Attempts, &v.ConsumedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return errors.New("no pending verification")
	}
	if err != nil {
		return fmt.Errorf("fetch verification: %w", err)
	}

	// Check attempts
	if v.Attempts >= 3 {
		return errors.New("max attempts exceeded")
	}

	// Check code hash
	storedHex := hex.EncodeToString(v.CodeHash)
	if storedHex != hashHex {
		// Increment attempts
		_, err = s.pool.Exec(ctx, `UPDATE verifications SET attempts = attempts + 1 WHERE id = $1`, v.ID)
		if err != nil {
			return fmt.Errorf("increment attempts: %w", err)
		}
		return errors.New("invalid code")
	}

	// Mark consumed
	_, err = s.pool.Exec(ctx, `UPDATE verifications SET consumed_at = now() WHERE id = $1`, v.ID)
	if err != nil {
		return fmt.Errorf("consume verification: %w", err)
	}
	return nil
}

// CheckQuota checks if a user can send a letter today.
// Returns (allowed, lettersRemaining, maxPerDay).
func (s *Store) CheckQuota(ctx context.Context, userID uuid.UUID) (bool, int, int, error) {
	var u User
	err := s.pool.QueryRow(ctx, `
		SELECT id, status, letters_today, quota_day, created_at
		FROM users WHERE id = $1
	`, userID).Scan(&u.ID, &u.Status, &u.LettersToday, &u.QuotaDay, &u.CreatedAt)
	if err != nil {
		return false, 0, 0, fmt.Errorf("check quota: %w", err)
	}

	// Reset quota if day has changed
	today := time.Now().Truncate(24 * time.Hour)
	if u.QuotaDay.Before(today) {
		_, err = s.pool.Exec(ctx, `
			UPDATE users SET letters_today = 0, quota_day = $1 WHERE id = $2
		`, today, userID)
		if err != nil {
			return false, 0, 0, fmt.Errorf("reset quota: %w", err)
		}
		u.LettersToday = 0
	}

	// Determine max per day based on status and age
	maxPerDay := 0
	if u.Status == "verified" {
		maxPerDay = 20
		age := time.Since(u.CreatedAt)
		if age > 30*24*time.Hour {
			maxPerDay = 40
		}
	}

	remaining := maxPerDay - u.LettersToday
	allowed := remaining > 0
	return allowed, remaining, maxPerDay, nil
}

// IncrementLettersToday increments the user's letter counter.
func (s *Store) IncrementLettersToday(ctx context.Context, userID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE users SET letters_today = letters_today + 1 WHERE id = $1
	`, userID)
	return err
}

// UpdateStorageBytes adjusts the user's storage usage.
func (s *Store) UpdateStorageBytes(ctx context.Context, userID uuid.UUID, delta int64) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE users SET storage_bytes = storage_bytes + $1 WHERE id = $2
	`, delta, userID)
	return err
}
type Envelope struct {
	ID           uuid.UUID
	SenderID     *uuid.UUID
	RecipientID  *uuid.UUID
	Direction    string
	Class        string
	Subject      string
	Ciphertext   []byte
	SizeBytes    int
	Route        []byte // JSON
	TotalKm      float64
	QueuedAt     time.Time
	DeliverAt    time.Time
	State        string
	ExternalTo   *string
	BounceToken  []byte
	LogLeaf      []byte
	TrackingID   *string
	TrackingShared bool
}

// CreateEnvelope inserts a new envelope.
func (s *Store) CreateEnvelope(ctx context.Context, senderID, recipientID *uuid.UUID, direction, class, subject string, ciphertext []byte, sizeBytes int, route []byte, totalKm float64, queuedAt, deliverAt time.Time, externalTo *string, bounceToken []byte) (*Envelope, error) {
	id := uuid.New()
	var e Envelope
	err := s.pool.QueryRow(ctx, `
		INSERT INTO envelopes (id, sender_id, recipient_id, direction, class, subject, ciphertext,
		                       size_bytes, route, total_km, queued_at, deliver_at, state, external_to, bounce_token)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, 'queued', $13, $14)
		RETURNING id, sender_id, recipient_id, direction, class, subject, ciphertext, size_bytes,
		          route, total_km, queued_at, deliver_at, state, external_to, bounce_token, log_leaf,
		          tracking_id, tracking_shared
	`, id, senderID, recipientID, direction, class, subject, ciphertext, sizeBytes, route, totalKm,
		queuedAt, deliverAt, externalTo, bounceToken).Scan(
		&e.ID, &e.SenderID, &e.RecipientID, &e.Direction, &e.Class, &e.Subject, &e.Ciphertext,
		&e.SizeBytes, &e.Route, &e.TotalKm, &e.QueuedAt, &e.DeliverAt, &e.State, &e.ExternalTo,
		&e.BounceToken, &e.LogLeaf, &e.TrackingID, &e.TrackingShared,
	)
	if err != nil {
		return nil, fmt.Errorf("create envelope: %w", err)
	}
	return &e, nil
}

// GetEnvelopeByID fetches an envelope by ID.
func (s *Store) GetEnvelopeByID(ctx context.Context, id uuid.UUID) (*Envelope, error) {
	var e Envelope
	err := s.pool.QueryRow(ctx, `
		SELECT id, sender_id, recipient_id, direction, class, subject, ciphertext, size_bytes,
		       route, total_km, queued_at, deliver_at, state, external_to, bounce_token, log_leaf,
		       tracking_id, tracking_shared
		FROM envelopes WHERE id = $1
	`, id).Scan(
		&e.ID, &e.SenderID, &e.RecipientID, &e.Direction, &e.Class, &e.Subject, &e.Ciphertext,
		&e.SizeBytes, &e.Route, &e.TotalKm, &e.QueuedAt, &e.DeliverAt, &e.State, &e.ExternalTo,
		&e.BounceToken, &e.LogLeaf, &e.TrackingID, &e.TrackingShared,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get envelope by id: %w", err)
	}
	return &e, nil
}

// ListInbox returns envelopes delivered to a user.
func (s *Store) ListInbox(ctx context.Context, userID uuid.UUID, limit int) ([]*Envelope, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, sender_id, recipient_id, direction, class, subject, ciphertext, size_bytes,
		       route, total_km, queued_at, deliver_at, state, external_to, bounce_token, log_leaf,
		       tracking_id, tracking_shared
		FROM envelopes
		WHERE recipient_id = $1 AND state = 'delivered'
		ORDER BY deliver_at DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("list inbox: %w", err)
	}
	defer rows.Close()

	var envs []*Envelope
	for rows.Next() {
		var e Envelope
		if err := rows.Scan(
			&e.ID, &e.SenderID, &e.RecipientID, &e.Direction, &e.Class, &e.Subject, &e.Ciphertext,
			&e.SizeBytes, &e.Route, &e.TotalKm, &e.QueuedAt, &e.DeliverAt, &e.State, &e.ExternalTo,
			&e.BounceToken, &e.LogLeaf, &e.TrackingID, &e.TrackingShared,
		); err != nil {
			return nil, fmt.Errorf("scan inbox: %w", err)
		}
		envs = append(envs, &e)
	}
	return envs, nil
}

// ListOutbox returns envelopes sent by a user.
func (s *Store) ListOutbox(ctx context.Context, userID uuid.UUID, limit int) ([]*Envelope, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, sender_id, recipient_id, direction, class, subject, ciphertext, size_bytes,
		       route, total_km, queued_at, deliver_at, state, external_to, bounce_token, log_leaf,
		       tracking_id, tracking_shared
		FROM envelopes
		WHERE sender_id = $1
		ORDER BY queued_at DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("list outbox: %w", err)
	}
	defer rows.Close()

	var envs []*Envelope
	for rows.Next() {
		var e Envelope
		if err := rows.Scan(
			&e.ID, &e.SenderID, &e.RecipientID, &e.Direction, &e.Class, &e.Subject, &e.Ciphertext,
			&e.SizeBytes, &e.Route, &e.TotalKm, &e.QueuedAt, &e.DeliverAt, &e.State, &e.ExternalTo,
			&e.BounceToken, &e.LogLeaf, &e.TrackingID, &e.TrackingShared,
		); err != nil {
			return nil, fmt.Errorf("scan outbox: %w", err)
		}
		envs = append(envs, &e)
	}
	return envs, nil
}
