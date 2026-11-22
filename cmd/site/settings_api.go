package main

import (
	"encoding/json"
	"log"
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

	writeJSON(w, http.StatusOK, map[string]any{
		"settings": settings,
		"city":     user.City,
		"state":    user.State,
		"country":  user.Country,
	})
}

// handleUpdateSettings handles PUT /api/settings
func (app *appState) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	// Validate session
	_, user := app.validateSession(w, r)
	if user == nil {
		return
	}

	// Parse request body
	var req struct {
		store.UserSettings
		City    *string `json:"city"`
		State   *string `json:"state"`
		Country *string `json:"country"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	// Set user ID from authenticated user
	req.UserSettings.UserID = user.ID

	// Set defaults for empty fields
	if req.Theme == "" {
		req.Theme = "light"
	}
	if req.DefaultView == "" {
		req.DefaultView = "inbox"
	}
	if req.IMAPHost == "" {
		req.IMAPHost = "imap.snaelmail.org"
	}
	if req.IMAPPort == 0 {
		req.IMAPPort = 993
	}
	if req.POPHost == "" {
		req.POPHost = "pop.snaelmail.org"
	}
	if req.POPPort == 0 {
		req.POPPort = 995
	}

	// Update settings
	if err := app.store.UpdateUserSettings(r.Context(), &req.UserSettings); err != nil {
		log.Printf("settings update error: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update settings: " + err.Error()})
		return
	}
	if err := app.store.UpdateUserLocation(r.Context(), user.ID, req.City, req.State, req.Country); err != nil {
		log.Printf("location update error: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update location"})
		return
	}

	// Return updated settings
	settings, err := app.store.GetUserSettings(r.Context(), user.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get updated settings"})
		return
	}
	updatedUser, _ := app.store.GetUserByID(r.Context(), user.ID)

	resp := map[string]any{
		"message":  "Settings updated successfully",
		"settings": settings,
	}
	if updatedUser != nil {
		resp["city"] = updatedUser.City
		resp["state"] = updatedUser.State
		resp["country"] = updatedUser.Country
	}
	writeJSON(w, http.StatusOK, resp)
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

// handleGenerateAPIKey handles POST /api/settings/api-key
func (app *appState) handleGenerateAPIKey(w http.ResponseWriter, r *http.Request) {
	_, user := app.validateSession(w, r)
	if user == nil { return }
	key, err := auth.GenerateToken(32)
	if err != nil { writeJSON(w, 500, map[string]string{"error":"failed to generate key"}); return }
	if err := app.store.SetUserAPIKey(r.Context(), user.ID, key); err != nil {
		writeJSON(w, 500, map[string]string{"error":"failed to save key"}); return
	}
	writeJSON(w, http.StatusOK, map[string]string{"api_key": key})
}

// handleGetAPIKey handles GET /api/settings/api-key
func (app *appState) handleGetAPIKey(w http.ResponseWriter, r *http.Request) {
	_, user := app.validateSession(w, r)
	if user == nil { return }
	key, err := app.store.GetUserAPIKey(r.Context(), user.ID)
	if err != nil { writeJSON(w, 500, map[string]string{"error":"failed to get key"}); return }
	writeJSON(w, http.StatusOK, map[string]any{"api_key": key})
}
