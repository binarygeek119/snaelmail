package main

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"snailmail/internal/store"
)

// handleGetTracking handles GET /api/tracking/:envelope_id
func (app *appState) handleGetTracking(w http.ResponseWriter, r *http.Request) {
	// Get user ID from Authorization header
	userIDStr := r.Header.Get("Authorization")
	userIDStr = strings.TrimPrefix(userIDStr, "Bearer ")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid user_id"})
		return
	}

	// Get envelope ID from URL path
	path := r.URL.Path
	parts := strings.Split(path, "/")
	if len(parts) < 4 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing envelope_id"})
		return
	}
	envelopeIDStr := parts[3]
	envelopeID, err := uuid.Parse(envelopeIDStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid envelope_id"})
		return
	}

	// Get envelope
	envelope, err := app.store.GetEnvelopeByID(r.Context(), envelopeID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get envelope"})
		return
	}
	if envelope == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "envelope not found"})
		return
	}

	// Verify user has access (is sender or recipient)
	if (envelope.SenderID != nil && *envelope.SenderID != userID) &&
		(envelope.RecipientID != nil && *envelope.RecipientID != userID) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "access denied"})
		return
	}

	// Get tracking events
	events, err := app.store.GetTrackingEvents(r.Context(), envelopeID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get tracking events"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"envelope_id": envelopeID,
		"tracking_id": envelope.TrackingID,
		"events":      events,
	})
}

// handleShareTracking handles POST /api/tracking/:envelope_id/share
func (app *appState) handleShareTracking(w http.ResponseWriter, r *http.Request) {
	// Get user ID from Authorization header
	userIDStr := r.Header.Get("Authorization")
	userIDStr = strings.TrimPrefix(userIDStr, "Bearer ")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid user_id"})
		return
	}

	// Get envelope ID from URL path
	path := r.URL.Path
	parts := strings.Split(path, "/")
	if len(parts) < 5 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing envelope_id"})
		return
	}
	envelopeIDStr := parts[3]
	envelopeID, err := uuid.Parse(envelopeIDStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid envelope_id"})
		return
	}

	// Get envelope
	envelope, err := app.store.GetEnvelopeByID(r.Context(), envelopeID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get envelope"})
		return
	}
	if envelope == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "envelope not found"})
		return
	}

	// Verify user is sender
	if envelope.SenderID == nil || *envelope.SenderID != userID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "only sender can share tracking"})
		return
	}

	// Share tracking with recipient
	if err := app.store.ShareTrackingWithRecipient(r.Context(), envelopeID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to share tracking"})
		return
	}

	// Create notification for recipient if recipient exists
	if envelope.RecipientID != nil {
		notification := &store.Notification{
			UserID:     *envelope.RecipientID,
			EnvelopeID: envelopeID,
			Type:       "tracking_shared",
			Message:    "Tracking number shared for your message",
		}
		if err := app.store.CreateNotification(r.Context(), notification); err != nil {
			// Log error but don't fail the request
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "success"})
}

// handleGetNotifications handles GET /api/notifications
func (app *appState) handleGetNotifications(w http.ResponseWriter, r *http.Request) {
	// Get user ID from Authorization header
	userIDStr := r.Header.Get("Authorization")
	userIDStr = strings.TrimPrefix(userIDStr, "Bearer ")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid user_id"})
		return
	}

	// Get notifications
	notifications, err := app.store.GetNotifications(r.Context(), userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get notifications"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"notifications": notifications,
	})
}

// handleMarkNotificationRead handles POST /api/notifications/:id/read
func (app *appState) handleMarkNotificationRead(w http.ResponseWriter, r *http.Request) {
	// Get user ID from Authorization header
	userIDStr := r.Header.Get("Authorization")
	userIDStr = strings.TrimPrefix(userIDStr, "Bearer ")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid user_id"})
		return
	}

	// Parse request body
	var req struct {
		NotificationID int64 `json:"notification_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	// Mark notification as read
	if err := app.store.MarkNotificationRead(r.Context(), userID, req.NotificationID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to mark notification as read"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "success"})
}
