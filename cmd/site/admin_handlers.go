package main

import (
	"net/http"
	"strings"

	"github.com/google/uuid"
)

// handleAdminPanel serves the admin panel page
func (app *appState) handleAdminPanel(w http.ResponseWriter, r *http.Request) {
	// Check if user is admin
	userIDStr := r.Header.Get("Authorization")
	userIDStr = strings.TrimPrefix(userIDStr, "Bearer ")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid user_id"})
		return
	}

	user, err := app.store.GetUserByID(r.Context(), userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get user"})
		return
	}
	if user == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
		return
	}

	// Check if user is admin (you'll need to add an IsAdmin field to the User struct)
	// For now, check if user handle is "admin"
	if user.Handle != "admin" {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "admin access required"})
		return
	}

	// Serve admin page
	http.ServeFile(w, r, "web/site/admin.html")
}

// handleAdminUsers returns all users
func (app *appState) handleAdminUsers(w http.ResponseWriter, r *http.Request) {
	// Check if user is admin
	userIDStr := r.Header.Get("Authorization")
	userIDStr = strings.TrimPrefix(userIDStr, "Bearer ")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid user_id"})
		return
	}

	user, err := app.store.GetUserByID(r.Context(), userID)
	if err != nil || user == nil || user.Handle != "admin" {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "admin access required"})
		return
	}

	// Get all users
	rows, err := app.store.Pool().Query(r.Context(), `
		SELECT id, handle, address, status, created_at
		FROM users
		ORDER BY created_at DESC
		LIMIT 100
	`)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get users"})
		return
	}
	defer rows.Close()

	var users []map[string]any
	for rows.Next() {
		var id uuid.UUID
		var handle, address, status string
		var createdAt string

		if err := rows.Scan(&id, &handle, &address, &status, &createdAt); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to scan users"})
			return
		}

		users = append(users, map[string]any{
			"id":         id,
			"handle":     handle,
			"address":    address,
			"status":     status,
			"created_at": createdAt,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{"users": users})
}

// handleAdminRelays returns all relays
func (app *appState) handleAdminRelays(w http.ResponseWriter, r *http.Request) {
	// Check if user is admin
	userIDStr := r.Header.Get("Authorization")
	userIDStr = strings.TrimPrefix(userIDStr, "Bearer ")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid user_id"})
		return
	}

	user, err := app.store.GetUserByID(r.Context(), userID)
	if err != nil || user == nil || user.Handle != "admin" {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "admin access required"})
		return
	}

	// Get all relays
	rows, err := app.store.Pool().Query(r.Context(), `
		SELECT id, hostname, city, region, status, letters_passed
		FROM relays
		ORDER BY letters_passed DESC
		LIMIT 100
	`)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get relays"})
		return
	}
	defer rows.Close()

	var relays []map[string]any
	for rows.Next() {
		var id uuid.UUID
		var hostname, city, region, status string
		var lettersPassed int64

		if err := rows.Scan(&id, &hostname, &city, &region, &status, &lettersPassed); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to scan relays"})
			return
		}

		relays = append(relays, map[string]any{
			"id":              id,
			"hostname":        hostname,
			"city":            city,
			"region":          region,
			"status":          status,
			"letters_passed":  lettersPassed,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{"relays": relays})
}

// handleAdminLetters returns all letters
func (app *appState) handleAdminLetters(w http.ResponseWriter, r *http.Request) {
	// Check if user is admin
	userIDStr := r.Header.Get("Authorization")
	userIDStr = strings.TrimPrefix(userIDStr, "Bearer ")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid user_id"})
		return
	}

	user, err := app.store.GetUserByID(r.Context(), userID)
	if err != nil || user == nil || user.Handle != "admin" {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "admin access required"})
		return
	}

	// Get all letters
	rows, err := app.store.Pool().Query(r.Context(), `
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
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get letters"})
		return
	}
	defer rows.Close()

	var letters []map[string]any
	for rows.Next() {
		var id uuid.UUID
		var subject, state string
		var queuedAt string
		var senderAddress, recipientAddress *string

		if err := rows.Scan(&id, &subject, &state, &queuedAt, &senderAddress, &recipientAddress); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to scan letters"})
			return
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

	writeJSON(w, http.StatusOK, map[string]any{"letters": letters})
}

// handleUserMe returns the current user's information
func (app *appState) handleUserMe(w http.ResponseWriter, r *http.Request) {
	userIDStr := r.Header.Get("Authorization")
	userIDStr = strings.TrimPrefix(userIDStr, "Bearer ")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid user_id"})
		return
	}

	user, err := app.store.GetUserByID(r.Context(), userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get user"})
		return
	}
	if user == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":         user.ID,
		"handle":     user.Handle,
		"address":    user.Address,
		"is_admin":   user.Handle == "admin",
	})
}
