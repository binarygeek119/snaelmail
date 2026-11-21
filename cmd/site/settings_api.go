package main

import (
	"encoding/json"
	"net/http"

	"snailmail/internal/auth"
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

// handleChangePassword handles POST /api/change-password
func (app *appState) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	// Validate session
	_, user := app.validateSession(w, r)
	if user == nil {
		return
	}

	// Parse request body
	var req struct {
		OldPassword     string `json:"old_password"`
		NewPassword     string `json:"new_password"`
		ConfirmPassword string `json:"confirm_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	// Validate new passwords match
	if req.NewPassword != req.ConfirmPassword {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "new passwords do not match"})
		return
	}

	// Validate new password length
	if len(req.NewPassword) < 10 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "new password must be at least 10 characters"})
		return
	}

	// Verify old password
	valid, err := auth.VerifyPassword(req.OldPassword, user.PassHash)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "password verification failed"})
		return
	}
	if !valid {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "old password is incorrect"})
		return
	}

	// Hash new password
	newHash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "password hashing failed"})
		return
	}

	// Update password in database
	if err := app.store.UpdateUserPassword(r.Context(), user.ID, newHash); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update password"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "Password changed successfully"})
}
