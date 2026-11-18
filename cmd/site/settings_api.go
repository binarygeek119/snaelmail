package main

import (
	"encoding/json"
	"net/http"
	"strings"

	"snailmail/internal/store"

	"github.com/google/uuid"
)

// handleGetSettings handles GET /api/settings
func (app *appState) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	// Get user ID from Authorization header (simple auth for now)
	userIDStr := r.Header.Get("Authorization")
	userIDStr = strings.TrimPrefix(userIDStr, "Bearer ")
	
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid user_id"})
		return
	}

	// Get user settings
	settings, err := app.store.GetUserSettings(r.Context(), userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get settings"})
		return
	}

	writeJSON(w, http.StatusOK, settings)
}

// handleUpdateSettings handles PUT /api/settings
func (app *appState) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	// Get user ID from Authorization header (simple auth for now)
	userIDStr := r.Header.Get("Authorization")
	userIDStr = strings.TrimPrefix(userIDStr, "Bearer ")
	
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid user_id"})
		return
	}

	// Parse request body
	var req store.UserSettings
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	// Set user ID from authenticated user
	req.UserID = userID

	// Update settings
	if err := app.store.UpdateUserSettings(r.Context(), &req); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update settings"})
		return
	}

	// Return updated settings
	settings, err := app.store.GetUserSettings(r.Context(), userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get updated settings"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"message":  "Settings updated successfully",
		"settings": settings,
	})
}
