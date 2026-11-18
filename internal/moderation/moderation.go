// Package moderation handles user and relay moderation actions.
package moderation

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Action represents a moderation action type.
type Action string

const (
	ActionSuspendUser   Action = "suspend_user"
	ActionUnsuspendUser Action = "unsuspend_user"
	ActionSuspendRelay  Action = "suspend_relay"
	ActionUnsuspendRelay Action = "unsuspend_relay"
	ActionCloseAccount  Action = "close_account"
	ActionReopenAccount Action = "reopen_account"
)

// Record represents a moderation action record.
type Record struct {
	ID        uuid.UUID  `json:"id"`
	Action    Action     `json:"action"`
	TargetID  uuid.UUID  `json:"target_id"`
	TargetType string    `json:"target_type"` // "user" or "relay"
	AdminID   uuid.UUID  `json:"admin_id"`
	Reason    string     `json:"reason"`
	CreatedAt time.Time  `json:"created_at"`
}

var (
	ErrInvalidAction = errors.New("invalid moderation action")
	ErrNotFound      = errors.New("target not found")
)

// LogAction records a moderation action in the database.
func LogAction(ctx context.Context, pool *pgxpool.Pool, action Action, targetID uuid.UUID, targetType string, adminID uuid.UUID, reason string) error {
	// Validate action
	validActions := map[Action]bool{
		ActionSuspendUser:   true,
		ActionUnsuspendUser: true,
		ActionSuspendRelay:  true,
		ActionUnsuspendRelay: true,
		ActionCloseAccount:  true,
		ActionReopenAccount: true,
	}
	if !validActions[action] {
		return ErrInvalidAction
	}

	// Insert record
	_, err := pool.Exec(ctx, `
		INSERT INTO moderation_log (id, action, target_id, target_type, admin_id, reason, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW())
	`, uuid.New(), action, targetID, targetType, adminID, reason)

	return err
}

// GetHistory retrieves moderation history for a target.
func GetHistory(ctx context.Context, pool *pgxpool.Pool, targetID uuid.UUID) ([]Record, error) {
	rows, err := pool.Query(ctx, `
		SELECT id, action, target_id, target_type, admin_id, reason, created_at
		FROM moderation_log
		WHERE target_id = $1
		ORDER BY created_at DESC
	`, targetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []Record
	for rows.Next() {
		var r Record
		if err := rows.Scan(&r.ID, &r.Action, &r.TargetID, &r.TargetType, &r.AdminID, &r.Reason, &r.CreatedAt); err != nil {
			return nil, err
		}
		records = append(records, r)
	}

	return records, rows.Err()
}

// SuspendUser suspends a user account.
func SuspendUser(ctx context.Context, pool *pgxpool.Pool, userID, adminID uuid.UUID, reason string) error {
	// Check user exists
	var exists bool
	err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)`, userID).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return ErrNotFound
	}

	// Update user status
	_, err = pool.Exec(ctx, `UPDATE users SET status = 'suspended' WHERE id = $1`, userID)
	if err != nil {
		return err
	}

	// Log action
	return LogAction(ctx, pool, ActionSuspendUser, userID, "user", adminID, reason)
}

// UnsuspendUser reactivates a suspended user account.
func UnsuspendUser(ctx context.Context, pool *pgxpool.Pool, userID, adminID uuid.UUID, reason string) error {
	// Check user exists and is suspended
	var status string
	err := pool.QueryRow(ctx, `SELECT status FROM users WHERE id = $1`, userID).Scan(&status)
	if err == pgx.ErrNoRows {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if status != "suspended" {
		return errors.New("user is not suspended")
	}

	// Update user status
	_, err = pool.Exec(ctx, `UPDATE users SET status = 'active' WHERE id = $1`, userID)
	if err != nil {
		return err
	}

	// Log action
	return LogAction(ctx, pool, ActionUnsuspendUser, userID, "user", adminID, reason)
}

// SuspendRelay suspends a relay.
func SuspendRelay(ctx context.Context, pool *pgxpool.Pool, relayID, adminID uuid.UUID, reason string) error {
	// Check relay exists
	var exists bool
	err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM relays WHERE id = $1)`, relayID).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return ErrNotFound
	}

	// Update relay status
	_, err = pool.Exec(ctx, `UPDATE relays SET status = 'suspended' WHERE id = $1`, relayID)
	if err != nil {
		return err
	}

	// Log action
	return LogAction(ctx, pool, ActionSuspendRelay, relayID, "relay", adminID, reason)
}

// UnsuspendRelay reactivates a suspended relay.
func UnsuspendRelay(ctx context.Context, pool *pgxpool.Pool, relayID, adminID uuid.UUID, reason string) error {
	// Check relay exists and is suspended
	var status string
	err := pool.QueryRow(ctx, `SELECT status FROM relays WHERE id = $1`, relayID).Scan(&status)
	if err == pgx.ErrNoRows {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if status != "suspended" {
		return errors.New("relay is not suspended")
	}

	// Update relay status to active
	_, err = pool.Exec(ctx, `UPDATE relays SET status = 'active' WHERE id = $1`, relayID)
	if err != nil {
		return err
	}

	// Log action
	return LogAction(ctx, pool, ActionUnsuspendRelay, relayID, "relay", adminID, reason)
}

// CloseAccount permanently closes a user account.
func CloseAccount(ctx context.Context, pool *pgxpool.Pool, userID, adminID uuid.UUID, reason string) error {
	// Check user exists
	var exists bool
	err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)`, userID).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return ErrNotFound
	}

	// Update user status
	_, err = pool.Exec(ctx, `UPDATE users SET status = 'closed' WHERE id = $1`, userID)
	if err != nil {
		return err
	}

	// Log action
	return LogAction(ctx, pool, ActionCloseAccount, userID, "user", adminID, reason)
}

// ReopenAccount reopens a closed account.
func ReopenAccount(ctx context.Context, pool *pgxpool.Pool, userID, adminID uuid.UUID, reason string) error {
	// Check user exists and is closed
	var status string
	err := pool.QueryRow(ctx, `SELECT status FROM users WHERE id = $1`, userID).Scan(&status)
	if err == pgx.ErrNoRows {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if status != "closed" {
		return errors.New("account is not closed")
	}

	// Update user status to active
	_, err = pool.Exec(ctx, `UPDATE users SET status = 'active' WHERE id = $1`, userID)
	if err != nil {
		return err
	}

	// Log action
	return LogAction(ctx, pool, ActionReopenAccount, userID, "user", adminID, reason)
}
