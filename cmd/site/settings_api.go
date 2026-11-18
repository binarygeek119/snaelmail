package main

import (
	"encoding/json"
	"net/http"

	"snailmail/internal/store"
)

// handleGetSettings handles GET /api/settings
func (app *appState) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	// Validate session
	_, user := app.validateSession(w, r)
	if user == nil {
		return
	}

	// Get user settings
	settings, err := app.store.GetUserSettings(r.Context(), user.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get settings"})
		return
	}

	writeJSON(w, http.StatusOK, settings)
}

// handleUpdateSettings handles PUT /api/settings
func (app *appState) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	// Validate session
	_, user := app.validateSession(w, r)
	if user == nil {
		return
	}

	// Parse request body
	var req store.UserSettings
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	// Set user ID from authenticated user
	req.UserID = user.ID

	// Update settings
	if err := app.store.UpdateUserSettings(r.Context(), &req); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update settings"})
		return
	}

	// Return updated settings
	settings, err := app.store.GetUserSettings(r.Context(), user.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get updated settings"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"message":  "Settings updated successfully",
		"settings": settings,
	})
}
