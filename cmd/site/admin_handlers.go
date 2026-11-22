package main

import (
	"net/http"
	"strings"

	"snailmail/internal/store"
)

// validateSession extracts and validates the session token from the Authorization header
// Returns the session and user, or writes an error response and returns nil
func (app *appState) validateSession(w http.ResponseWriter, r *http.Request) (*store.Session, *store.User) {
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if token == "" {
		token = r.URL.Query().Get("token")
	}
	if token == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing authorization token"})
		return nil, nil
	}

	session, err := app.store.GetSession(r.Context(), token)
	if err != nil || session == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid or expired session"})
		return nil, nil
	}

	user, err := app.store.GetUserByID(r.Context(), session.UserID)
	if err != nil || user == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
		return nil, nil
	}

	return session, user
}

// requireAdmin validates that the current session belongs to an admin user
// Returns the session and user if authorized, or writes an error response and returns nil
func (app *appState) requireAdmin(w http.ResponseWriter, r *http.Request) (*store.Session, *store.User) {
	session, user := app.validateSession(w, r)
	if session == nil || user == nil {
		return nil, nil
	}

	if !user.IsAdmin {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "admin access required"})
		return nil, nil
	}

	return session, user
}

func (app *appState) requireModOrAdmin(w http.ResponseWriter, r *http.Request) (*store.Session, *store.User) {
	session, user := app.validateSession(w, r)
	if session == nil || user == nil {
		return nil, nil
	}
	if !user.IsAdmin && !user.IsMod {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "moderator access required"})
		return nil, nil
	}
	return session, user
}

// handleAdminPanel serves the admin panel page
func (app *appState) handleAdminPanel(w http.ResponseWriter, r *http.Request) {
	_, user := app.requireAdmin(w, r)
	if user == nil {
		return
	}

	// Serve admin page
	http.ServeFile(w, r, "web/site/admin.html")
}

// handleAdminUsers returns all users
func (app *appState) handleAdminUsers(w http.ResponseWriter, r *http.Request) {
	_, user := app.requireAdmin(w, r)
	if user == nil {
		return
	}

	// Get all users
	users, err := app.store.GetAllUsers(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get users"})
		return
	}

	// Filter out sensitive fields
	safeUsers := make([]map[string]any, len(users))
	for i, u := range users {
		safeUsers[i] = map[string]any{
			"id":           u.ID,
			"handle":       u.Handle,
			"address":      u.Address,
			"display_name": u.DisplayName,
			"status":       u.Status,
			"is_admin":     u.IsAdmin,
			"created_at":   u.CreatedAt,
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{"users": safeUsers})
}

// handleAdminRelays returns all relays
func (app *appState) handleAdminRelays(w http.ResponseWriter, r *http.Request) {
	_, user := app.requireAdmin(w, r)
	if user == nil {
		return
	}

	// Get all relays
	relays, err := app.store.GetAllRelays(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get relays"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"relays": relays})
}

// handleAdminLetters returns all letters
func (app *appState) handleAdminLetters(w http.ResponseWriter, r *http.Request) {
	_, user := app.requireAdmin(w, r)
	if user == nil {
		return
	}

	// Get all letters
	letters, err := app.store.GetAllLetters(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get letters"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"letters": letters})
}

// handleUserMe returns the current user's information
func (app *appState) handleUserMe(w http.ResponseWriter, r *http.Request) {
	_, user := app.validateSession(w, r)
	if user == nil {
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":         user.ID,
		"handle":     user.Handle,
		"address":    user.Address,
		"is_admin":   user.IsAdmin,
		"is_mod":     user.IsMod,
	})
}
