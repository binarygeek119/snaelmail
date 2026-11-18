// Package lookup provides search and filtering for relays, users, and letters.
package lookup

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RelayFilter defines search criteria for relays.
type RelayFilter struct {
	City    string
	Region  string
	Country string
	Status  string
	Paused  *bool
	Limit   int
	Offset  int
}

// UserFilter defines search criteria for users.
type UserFilter struct {
	Handle  string
	Status  string
	Limit   int
	Offset  int
}

// LetterFilter defines search criteria for letters.
type LetterFilter struct {
	SenderID    *uuid.UUID
	RecipientID *uuid.UUID
	RelayID     *uuid.UUID
	Status      string
	Limit       int
	Offset      int
}

// RelayResult represents a relay search result.
type RelayResult struct {
	ID            uuid.UUID `json:"id"`
	Hostname      string    `json:"hostname"`
	City          string    `json:"city"`
	Region        string    `json:"region"`
	Country       string    `json:"country"`
	Status        string    `json:"status"`
	Paused        bool      `json:"paused"`
	LettersPassed int64     `json:"letters_passed"`
}

// UserResult represents a user search result.
type UserResult struct {
	ID        uuid.UUID `json:"id"`
	Handle    string    `json:"handle"`
	Address   string    `json:"address"`
	Status    string    `json:"status"`
	CreatedAt string    `json:"created_at"`
}

// LetterResult represents a letter search result.
type LetterResult struct {
	ID         uuid.UUID `json:"id"`
	SenderID   uuid.UUID `json:"sender_id"`
	RecipientID uuid.UUID `json:"recipient_id"`
	Subject    string    `json:"subject"`
	Status     string    `json:"status"`
	CreatedAt  string    `json:"created_at"`
	DeliveredAt *string  `json:"delivered_at,omitempty"`
}

// SearchRelays searches for relays matching the filter.
func SearchRelays(ctx context.Context, pool *pgxpool.Pool, filter RelayFilter) ([]RelayResult, error) {
	var conditions []string
	var args []interface{}
	argNum := 1

	if filter.City != "" {
		conditions = append(conditions, fmt.Sprintf("city ILIKE $%d", argNum))
		args = append(args, "%"+filter.City+"%")
		argNum++
	}

	if filter.Region != "" {
		conditions = append(conditions, fmt.Sprintf("region ILIKE $%d", argNum))
		args = append(args, "%"+filter.Region+"%")
		argNum++
	}

	if filter.Country != "" {
		conditions = append(conditions, fmt.Sprintf("country = $%d", argNum))
		args = append(args, filter.Country)
		argNum++
	}

	if filter.Status != "" {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argNum))
		args = append(args, filter.Status)
		argNum++
	}

	if filter.Paused != nil {
		conditions = append(conditions, fmt.Sprintf("paused = $%d", argNum))
		args = append(args, *filter.Paused)
		argNum++
	}

	query := `SELECT id, hostname, city, region, country, status, paused, letters_passed FROM relays`
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY letters_passed DESC"

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argNum)
		args = append(args, filter.Limit)
		argNum++
	}

	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argNum)
		args = append(args, filter.Offset)
		argNum++
	}

	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []RelayResult
	for rows.Next() {
		var r RelayResult
		if err := rows.Scan(&r.ID, &r.Hostname, &r.City, &r.Region, &r.Country, &r.Status, &r.Paused, &r.LettersPassed); err != nil {
			return nil, err
		}
		results = append(results, r)
	}

	return results, rows.Err()
}

// SearchUsers searches for users matching the filter.
func SearchUsers(ctx context.Context, pool *pgxpool.Pool, filter UserFilter) ([]UserResult, error) {
	var conditions []string
	var args []interface{}
	argNum := 1

	if filter.Handle != "" {
		conditions = append(conditions, fmt.Sprintf("handle ILIKE $%d", argNum))
		args = append(args, "%"+filter.Handle+"%")
		argNum++
	}

	if filter.Status != "" {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argNum))
		args = append(args, filter.Status)
		argNum++
	}

	query := `SELECT id, handle, address, status, created_at FROM users`
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY created_at DESC"

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argNum)
		args = append(args, filter.Limit)
		argNum++
	}

	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argNum)
		args = append(args, filter.Offset)
		argNum++
	}

	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []UserResult
	for rows.Next() {
		var u UserResult
		if err := rows.Scan(&u.ID, &u.Handle, &u.Address, &u.Status, &u.CreatedAt); err != nil {
			return nil, err
		}
		results = append(results, u)
	}

	return results, rows.Err()
}

// SearchLetters searches for letters matching the filter.
func SearchLetters(ctx context.Context, pool *pgxpool.Pool, filter LetterFilter) ([]LetterResult, error) {
	var conditions []string
	var args []interface{}
	argNum := 1

	if filter.SenderID != nil {
		conditions = append(conditions, fmt.Sprintf("sender_id = $%d", argNum))
		args = append(args, *filter.SenderID)
		argNum++
	}

	if filter.RecipientID != nil {
		conditions = append(conditions, fmt.Sprintf("recipient_id = $%d", argNum))
		args = append(args, *filter.RecipientID)
		argNum++
	}

	if filter.RelayID != nil {
		conditions = append(conditions, fmt.Sprintf(`
			id IN (SELECT letter_id FROM letter_stamps WHERE relay_id = $%d)
		`, argNum))
		args = append(args, *filter.RelayID)
		argNum++
	}

	if filter.Status != "" {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argNum))
		args = append(args, filter.Status)
		argNum++
	}

	query := `SELECT id, sender_id, recipient_id, subject, status, created_at, delivered_at FROM letters`
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY created_at DESC"

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argNum)
		args = append(args, filter.Limit)
		argNum++
	}

	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argNum)
		args = append(args, filter.Offset)
		argNum++
	}

	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []LetterResult
	for rows.Next() {
		var l LetterResult
		var deliveredAt sql.NullString
		if err := rows.Scan(&l.ID, &l.SenderID, &l.RecipientID, &l.Subject, &l.Status, &l.CreatedAt, &deliveredAt); err != nil {
			return nil, err
		}
		if deliveredAt.Valid {
			l.DeliveredAt = &deliveredAt.String
		}
		results = append(results, l)
	}

	return results, rows.Err()
}
