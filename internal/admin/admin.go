package admin

import (
	"context"
	"fmt"
	"time"

	"snailmail/internal/store"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AdminService provides admin operations
type AdminService struct {
	store *store.Store
	pool  *pgxpool.Pool
}

// NewAdminService creates a new admin service
func NewAdminService(store *store.Store, pool *pgxpool.Pool) *AdminService {
	return &AdminService{
		store: store,
		pool:  pool,
	}
}

// SuspendUser suspends a user account for a specified duration
func (s *AdminService) SuspendUserWithDuration(ctx context.Context, userID uuid.UUID, adminID uuid.UUID, reason string, duration time.Duration) error {
	// Log the action
	err := s.logAdminAction(ctx, adminID, "suspend_user", userID, reason)
	if err != nil {
		return fmt.Errorf("failed to log admin action: %w", err)
	}

	// Calculate suspension end time
	var suspendedUntil *time.Time
	if duration > 0 {
		endTime := time.Now().Add(duration)
		suspendedUntil = &endTime
	}

	// Suspend user with duration
	_, err = s.pool.Exec(ctx, `
		UPDATE users 
		SET status = 'suspended', suspended_until = $1
		WHERE id = $2
	`, suspendedUntil, userID)
	if err != nil {
		return fmt.Errorf("failed to suspend user: %w", err)
	}

	return nil
}

// UnsuspendUser reactivates a suspended user account
func (s *AdminService) UnsuspendUser(ctx context.Context, userID uuid.UUID, adminID uuid.UUID, reason string) error {
	// Log the action
	err := s.logAdminAction(ctx, adminID, "unsuspend_user", userID, reason)
	if err != nil {
		return fmt.Errorf("failed to log admin action: %w", err)
	}

	// Unsuspend user
	_, err = s.pool.Exec(ctx, `
		UPDATE users 
		SET status = 'verified'
		WHERE id = $1
	`, userID)
	if err != nil {
		return fmt.Errorf("failed to unsuspend user: %w", err)
	}

	return nil
}

// SuspendRelay suspends a relay
func (s *AdminService) SuspendRelay(ctx context.Context, relayID uuid.UUID, adminID uuid.UUID, reason string) error {
	// Log the action
	err := s.logAdminAction(ctx, adminID, "suspend_relay", relayID, reason)
	if err != nil {
		return fmt.Errorf("failed to log admin action: %w", err)
	}

	// Suspend relay
	_, err = s.pool.Exec(ctx, `
		UPDATE relays 
		SET status = 'suspended'
		WHERE id = $1
	`, relayID)
	if err != nil {
		return fmt.Errorf("failed to suspend relay: %w", err)
	}

	return nil
}

// UnsuspendRelay reactivates a suspended relay
func (s *AdminService) UnsuspendRelay(ctx context.Context, relayID uuid.UUID, adminID uuid.UUID, reason string) error {
	// Log the action
	err := s.logAdminAction(ctx, adminID, "unsuspend_relay", relayID, reason)
	if err != nil {
		return fmt.Errorf("failed to log admin action: %w", err)
	}

	// Unsuspend relay
	_, err = s.pool.Exec(ctx, `
		UPDATE relays 
		SET status = 'active'
		WHERE id = $1
	`, relayID)
	if err != nil {
		return fmt.Errorf("failed to unsuspend relay: %w", err)
	}

	return nil
}

// DeleteUser permanently deletes a user account
func (s *AdminService) DeleteUser(ctx context.Context, userID uuid.UUID, adminID uuid.UUID, reason string) error {
	// Log the action
	err := s.logAdminAction(ctx, adminID, "delete_user", userID, reason)
	if err != nil {
		return fmt.Errorf("failed to log admin action: %w", err)
	}

	// Delete user and all associated data
	_, err = s.pool.Exec(ctx, `
		DELETE FROM users 
		WHERE id = $1
	`, userID)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	return nil
}

// DeleteRelay permanently deletes a relay
func (s *AdminService) DeleteRelay(ctx context.Context, relayID uuid.UUID, adminID uuid.UUID, reason string) error {
	// Log the action
	err := s.logAdminAction(ctx, adminID, "delete_relay", relayID, reason)
	if err != nil {
		return fmt.Errorf("failed to log admin action: %w", err)
	}

	// Delete relay and all associated data
	_, err = s.pool.Exec(ctx, `
		DELETE FROM relays 
		WHERE id = $1
	`, relayID)
	if err != nil {
		return fmt.Errorf("failed to delete relay: %w", err)
	}

	return nil
}

// GetAdminActions retrieves admin action history
func (s *AdminService) GetAdminActions(ctx context.Context, limit int) ([]AdminAction, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, admin_id, action, target_id, reason, created_at
		FROM admin_actions
		ORDER BY created_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query admin actions: %w", err)
	}
	defer rows.Close()

	var actions []AdminAction
	for rows.Next() {
		var action AdminAction
		err := rows.Scan(
			&action.ID,
			&action.AdminID,
			&action.Action,
			&action.TargetID,
			&action.Reason,
			&action.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan admin action: %w", err)
		}
		actions = append(actions, action)
	}

	return actions, nil
}

// AdminAction represents an admin action log entry
type AdminAction struct {
	ID        int64     `json:"id"`
	AdminID   uuid.UUID `json:"admin_id"`
	Action    string    `json:"action"`
	TargetID  uuid.UUID `json:"target_id"`
	Reason    string    `json:"reason"`
	CreatedAt time.Time `json:"created_at"`
}

// logAdminAction logs an admin action to the database
func (s *AdminService) logAdminAction(ctx context.Context, adminID uuid.UUID, action string, targetID uuid.UUID, reason string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO admin_actions (admin_id, action, target_id, reason, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`, adminID, action, targetID, reason, time.Now())
	return err
}

// GetSystemStats returns system statistics
func (s *AdminService) GetSystemStats(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Total users
	var totalUsers int
	err := s.pool.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&totalUsers)
	if err != nil {
		return nil, fmt.Errorf("failed to get total users: %w", err)
	}
	stats["total_users"] = totalUsers

	// Active users
	var activeUsers int
	err = s.pool.QueryRow(ctx, "SELECT COUNT(*) FROM users WHERE status = 'verified'").Scan(&activeUsers)
	if err != nil {
		return nil, fmt.Errorf("failed to get active users: %w", err)
	}
	stats["active_users"] = activeUsers

	// Suspended users
	var suspendedUsers int
	err = s.pool.QueryRow(ctx, "SELECT COUNT(*) FROM users WHERE status = 'suspended'").Scan(&suspendedUsers)
	if err != nil {
		return nil, fmt.Errorf("failed to get suspended users: %w", err)
	}
	stats["suspended_users"] = suspendedUsers

	// Total relays
	var totalRelays int
	err = s.pool.QueryRow(ctx, "SELECT COUNT(*) FROM relays").Scan(&totalRelays)
	if err != nil {
		return nil, fmt.Errorf("failed to get total relays: %w", err)
	}
	stats["total_relays"] = totalRelays

	// Active relays
	var activeRelays int
	err = s.pool.QueryRow(ctx, "SELECT COUNT(*) FROM relays WHERE status = 'active'").Scan(&activeRelays)
	if err != nil {
		return nil, fmt.Errorf("failed to get active relays: %w", err)
	}
	stats["active_relays"] = activeRelays

	// Total envelopes
	var totalEnvelopes int
	err = s.pool.QueryRow(ctx, "SELECT COUNT(*) FROM envelopes").Scan(&totalEnvelopes)
	if err != nil {
		return nil, fmt.Errorf("failed to get total envelopes: %w", err)
	}
	stats["total_envelopes"] = totalEnvelopes

	// Envelopes today
	var envelopesToday int
	err = s.pool.QueryRow(ctx, "SELECT COUNT(*) FROM envelopes WHERE queued_at >= CURRENT_DATE").Scan(&envelopesToday)
	if err != nil {
		return nil, fmt.Errorf("failed to get envelopes today: %w", err)
	}
	stats["envelopes_today"] = envelopesToday

	// Database size
	var dbSize string
	err = s.pool.QueryRow(ctx, "SELECT pg_size_pretty(pg_database_size(current_database()))").Scan(&dbSize)
	if err != nil {
		return nil, fmt.Errorf("failed to get database size: %w", err)
	}
	stats["database_size"] = dbSize

	return stats, nil
}
