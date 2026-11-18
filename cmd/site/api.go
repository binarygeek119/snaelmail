package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"snailmail/internal/auth"
	"snailmail/internal/store"

	"github.com/google/uuid"
)

// apiSignup handles POST /api/signup
func (app *appState) apiSignup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Handle      string     `json:"handle"`
		Password    string     `json:"password"`
		Email       string     `json:"email"`
		DisplayName *string    `json:"display_name,omitempty"`
		HomeRelayID *uuid.UUID `json:"home_relay_id,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	// Validate handle
	if err := auth.ValidateHandle(req.Handle); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	// Validate email
	if err := auth.ValidateEmail(req.Email); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	// Check handle availability
	existing, err := app.store.GetUserByHandle(r.Context(), req.Handle)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}
	if existing != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "handle already taken"})
		return
	}

	// Check email availability
	existingByEmail, err := app.store.GetUserByEmail(r.Context(), req.Email)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}
	if existingByEmail != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "email already registered"})
		return
	}

	// Validate password
	if err := auth.ValidatePassword(req.Password, nil); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	// Hash password
	passHash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "password hashing failed"})
		return
	}

	// Build address
	address := strings.ToLower(req.Handle) + "@" + app.cfg.MailDomain

	// Create user
	email := req.Email
	user, err := app.store.CreateUser(r.Context(), req.Handle, address, passHash, &email, req.DisplayName, req.HomeRelayID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create user"})
		return
	}

	// Generate verification code
	code, err := auth.GenerateCode()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to generate code"})
		return
	}

	// Create verification record (24h expiry)
	expiresAt := time.Now().Add(24 * time.Hour)
	if _, err := app.store.CreateVerification(r.Context(), user.ID, code, "verify", expiresAt); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create verification"})
		return
	}

	// In a real system, we'd queue an internal letter with the code
	// For now, return it directly (dev mode)
	var emailStr string
	if user.Email != nil {
		emailStr = *user.Email
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"user_id":          user.ID,
		"handle":           user.Handle,
		"address":          user.Address,
		"email":            emailStr,
		"status":           user.Status,
		"verification_code": code, // Remove in production
	})
}

// apiVerify handles POST /api/verify
func (app *appState) apiVerify(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID string `json:"user_id"`
		Code   string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid user_id"})
		return
	}

	// Consume verification
	if err := app.store.ConsumeVerification(r.Context(), userID, req.Code, "verify"); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	// Mark user as verified
	if err := app.store.VerifyUser(r.Context(), userID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to verify user"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "verified"})
}

// apiSession handles POST /api/session (login)
func (app *appState) apiSession(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Handle   string `json:"handle"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	// Fetch user
	user, err := app.store.GetUserByHandle(r.Context(), req.Handle)
	if err != nil || user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}

	// Verify password
	ok, err := auth.VerifyPassword(req.Password, user.PassHash)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "password verification failed"})
		return
	}
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}

	// Generate session token
	token, err := auth.GenerateToken(32)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to generate token"})
		return
	}

	// Store session in database (expires in 24 hours)
	expiresAt := time.Now().Add(24 * time.Hour)
	if err := app.store.CreateSession(r.Context(), user.ID, token, expiresAt); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create session"})
		return
	}

	// Return token to client
	response := map[string]any{
		"user_id":  user.ID,
		"handle":   user.Handle,
		"address":  user.Address,
		"token":    token,
		"is_admin": user.IsAdmin,
	}

	// Include email if it exists
	if user.Email != nil {
		response["email"] = *user.Email
	}

	writeJSON(w, http.StatusOK, response)
}

// apiLetters handles POST /api/letters (compose)
func (app *appState) apiLetters(w http.ResponseWriter, r *http.Request) {
	// In a real system, we'd authenticate via session token
	// For now, expect user_id in the request
	var req struct {
		UserID    string `json:"user_id"`
		Recipient string `json:"recipient"` // email address (internal or external)
		Subject   string `json:"subject"`
		Body      string `json:"body"`
		Class     string `json:"class"` // optional: surface, first, air, registered
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid user_id"})
		return
	}

	// Fetch sender
	sender, err := app.store.GetUserByID(r.Context(), userID)
	if err != nil || sender == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
		return
	}

	// Check status
	if sender.Status != "verified" {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "unverified users cannot send"})
		return
	}

	// Check quota
	allowed, remaining, _, err := app.store.CheckQuota(r.Context(), userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "quota check failed"})
		return
	}
	if !allowed {
		writeJSON(w, http.StatusTooManyRequests, map[string]any{
			"error":     "daily quota exceeded",
			"remaining": remaining,
		})
		return
	}

	// Validate recipient is an email address
	if !strings.Contains(req.Recipient, "@") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "recipient must be an email address"})
		return
	}

	// Check if recipient is internal (snaelmail.org) or external
	var recipient *store.User
	var isExternal bool
	if strings.HasSuffix(strings.ToLower(req.Recipient), "@"+app.cfg.MailDomain) {
		// Internal recipient - look up in database
		recipient, err = app.store.GetUserByAddress(r.Context(), req.Recipient)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
			return
		}
		if recipient == nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "recipient not found"})
			return
		}
		isExternal = false
	} else {
		// External recipient - allow sending to any email
		isExternal = true
	}

	// Default to "surface" class if not specified
	mailClass := req.Class
	if mailClass == "" {
		mailClass = "surface"
	}

	// Validate class if provided
	validClasses := map[string]bool{"surface": true, "first": true, "air": true, "registered": true}
	if !validClasses[mailClass] {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid class"})
		return
	}

	// In a real system:
	// 1. Encrypt the letter body
	// 2. Resolve route (for internal) or use SMTP relay (for external)
	// 3. Compute schedule
	// 4. Create envelope with hops
	// 5. Increment sender's quota

	// For now, create a simple envelope
	ciphertext := []byte(req.Body) // Not encrypted yet
	sizeBytes := len(req.Body)
	routeJSON := []byte(`{"hops":[]}`)
	queuedAt := time.Now()
	deliverAt := queuedAt.Add(24 * time.Hour) // Placeholder

	var envelope *store.Envelope
	if isExternal {
		// For external emails, store with external_to field
		externalAddr := req.Recipient
		envelope, err = app.store.CreateEnvelope(
			r.Context(),
			&sender.ID,
			nil, // No recipient_id for external
			"outbound",
			mailClass,
			req.Subject,
			ciphertext,
			sizeBytes,
			routeJSON,
			0, // total_km placeholder
			queuedAt,
			deliverAt,
			&externalAddr,
			nil,
		)
	} else {
		// For internal emails, use recipient_id
		envelope, err = app.store.CreateEnvelope(
			r.Context(),
			&sender.ID,
			&recipient.ID,
			"local",
			mailClass,
			req.Subject,
			ciphertext,
			sizeBytes,
			routeJSON,
			0, // total_km placeholder
			queuedAt,
			deliverAt,
			nil,
			nil,
		)
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create envelope"})
		return
	}

	// Increment quota
	if err := app.store.IncrementLettersToday(r.Context(), userID); err != nil {
		// Log but don't fail
	}

	response := map[string]any{
		"envelope_id": envelope.ID,
		"deliver_at":  envelope.DeliverAt,
		"state":       envelope.State,
	}
	
	if isExternal {
		response["recipient"] = req.Recipient
		response["message"] = "Letter queued for external delivery via SMTP relay"
	}

	writeJSON(w, http.StatusCreated, response)
}

// apiGetLetter handles GET /api/letters/:id
func (app *appState) apiGetLetter(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/api/letters/")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid envelope id"})
		return
	}

	envelope, err := app.store.GetEnvelopeByID(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}
	if envelope == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "envelope not found"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":         envelope.ID,
		"subject":    envelope.Subject,
		"class":      envelope.Class,
		"state":      envelope.State,
		"queued_at":  envelope.QueuedAt,
		"deliver_at": envelope.DeliverAt,
	})
}

// apiInbox handles GET /inbox
func (app *appState) apiInbox(w http.ResponseWriter, r *http.Request) {
	// In a real system, authenticate via session token
	userIDStr := r.URL.Query().Get("user_id")
	if userIDStr == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "user_id required"})
		return
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid user_id"})
		return
	}

	envs, err := app.store.ListInbox(r.Context(), userID, 50)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"inbox": envs})
}

// apiOutbox handles GET /outbox
func (app *appState) apiOutbox(w http.ResponseWriter, r *http.Request) {
	userIDStr := r.URL.Query().Get("user_id")
	if userIDStr == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "user_id required"})
		return
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid user_id"})
		return
	}

	envs, err := app.store.ListOutbox(r.Context(), userID, 50)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"outbox": envs})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}
