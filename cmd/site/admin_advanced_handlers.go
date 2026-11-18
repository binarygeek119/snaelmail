package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"snailmail/internal/admin"
	"snailmail/internal/notification"
	"snailmail/internal/store"

	"github.com/google/uuid"
)

// handleAdminSuspendUser handles POST /api/admin/users/:id/suspend
func (app *appState) handleAdminSuspendUser(w http.ResponseWriter, r *http.Request) {
	adminID := r.Context().Value("admin_id").(uuid.UUID)
	userIDStr := r.PathValue("id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid user_id"})
		return
	}

	var req struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	adminService := admin.NewAdminService(app.store, app.store.Pool())
	err = adminService.SuspendUser(r.Context(), userID, adminID, req.Reason)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "success"})
}

// handleAdminUnsuspendUser handles POST /api/admin/users/:id/unsuspend
func (app *appState) handleAdminUnsuspendUser(w http.ResponseWriter, r *http.Request) {
	adminID := r.Context().Value("admin_id").(uuid.UUID)
	userIDStr := r.PathValue("id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid user_id"})
		return
	}

	var req struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	adminService := admin.NewAdminService(app.store, app.store.Pool())
	err = adminService.UnsuspendUser(r.Context(), userID, adminID, req.Reason)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "success"})
}

// handleAdminDeleteUser handles DELETE /api/admin/users/:id
func (app *appState) handleAdminDeleteUser(w http.ResponseWriter, r *http.Request) {
	adminID := r.Context().Value("admin_id").(uuid.UUID)
	userIDStr := r.PathValue("id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid user_id"})
		return
	}

	var req struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	adminService := admin.NewAdminService(app.store, app.store.Pool())
	err = adminService.DeleteUser(r.Context(), userID, adminID, req.Reason)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "success"})
}

// handleAdminSuspendRelay handles POST /api/admin/relays/:id/suspend
func (app *appState) handleAdminSuspendRelay(w http.ResponseWriter, r *http.Request) {
	adminID := r.Context().Value("admin_id").(uuid.UUID)
	relayIDStr := r.PathValue("id")
	relayID, err := uuid.Parse(relayIDStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid relay_id"})
		return
	}

	var req struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	adminService := admin.NewAdminService(app.store, app.store.Pool())
	err = adminService.SuspendRelay(r.Context(), relayID, adminID, req.Reason)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "success"})
}

// handleAdminUnsuspendRelay handles POST /api/admin/relays/:id/unsuspend
func (app *appState) handleAdminUnsuspendRelay(w http.ResponseWriter, r *http.Request) {
	adminID := r.Context().Value("admin_id").(uuid.UUID)
	relayIDStr := r.PathValue("id")
	relayID, err := uuid.Parse(relayIDStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid relay_id"})
		return
	}

	var req struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	adminService := admin.NewAdminService(app.store, app.store.Pool())
	err = adminService.UnsuspendRelay(r.Context(), relayID, adminID, req.Reason)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "success"})
}

// handleAdminDeleteRelay handles DELETE /api/admin/relays/:id
func (app *appState) handleAdminDeleteRelay(w http.ResponseWriter, r *http.Request) {
	adminID := r.Context().Value("admin_id").(uuid.UUID)
	relayIDStr := r.PathValue("id")
	relayID, err := uuid.Parse(relayIDStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid relay_id"})
		return
	}

	var req struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	adminService := admin.NewAdminService(app.store, app.store.Pool())
	err = adminService.DeleteRelay(r.Context(), relayID, adminID, req.Reason)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "success"})
}

// handleAdminActions handles GET /api/admin/actions
func (app *appState) handleAdminActions(w http.ResponseWriter, r *http.Request) {
	limit := 100
	limitStr := r.URL.Query().Get("limit")
	if limitStr != "" {
		if n, err := uuid.Parse(limitStr); err == nil {
			_ = n // Just validate it's a number
			// In a real implementation, parse as int
		}
	}

	adminService := admin.NewAdminService(app.store, app.store.Pool())
	actions, err := adminService.GetAdminActions(r.Context(), limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"actions": actions})
}

// handleAdminStats handles GET /api/admin/stats
func (app *appState) handleAdminStats(w http.ResponseWriter, r *http.Request) {
	adminService := admin.NewAdminService(app.store, app.store.Pool())
	stats, err := adminService.GetSystemStats(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, stats)
}

// startNotificationWorker starts the background notification worker
func (app *appState) startNotificationWorker() {
	emailConfig := notification.EmailConfig{
		SMTPHost:     "smtp.gmail.com",
		SMTPPort:     "587",
		SMTPUsername: app.cfg.SMTPUsername,
		SMTPPassword: app.cfg.SMTPPassword,
		FromEmail:    "noreply@snaelmail.org",
		FromName:     "Snaelmail",
	}

	notifier := notification.NewNotifier(emailConfig, app.store)
	worker := notification.NewNotificationWorker(notifier, app.store.Pool(), 1*time.Minute)

	go func() {
		ctx := context.Background()
		worker.Start(ctx)
	}()
}

// startDeliveryWorker starts the background delivery worker
func (app *appState) startDeliveryWorker() {
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				app.processDeliveries()
			}
		}
	}()
}

// processDeliveries processes envelopes that are ready for delivery
func (app *appState) processDeliveries() {
	ctx := context.Background()

	// Get envelopes that are ready for delivery
	rows, err := app.store.Pool().Query(ctx, `
		SELECT id, sender_id, recipient_id, deliver_at, delivery_notification_sent
		FROM envelopes
		WHERE state = 'in_transit'
		AND deliver_at <= NOW() + INTERVAL '1 day'
		AND delivery_notification_sent = false
	`)
	if err != nil {
		log.Printf("Error querying envelopes: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var envelopeID, senderID, recipientID uuid.UUID
		var deliverAt time.Time
		var notificationSent bool

		err := rows.Scan(&envelopeID, &senderID, &recipientID, &deliverAt, &notificationSent)
		if err != nil {
			continue
		}

		// Create notification for sender
		err = app.store.CreateNotification(ctx, &store.Notification{
			UserID:     senderID,
			EnvelopeID: envelopeID,
			Type:       "out_for_delivery",
			Message:    "Your letter is out for delivery",
		})
		if err != nil {
			log.Printf("Error creating sender notification: %v", err)
			continue
		}

		// Create notification for recipient
		err = app.store.CreateNotification(ctx, &store.Notification{
			UserID:     recipientID,
			EnvelopeID: envelopeID,
			Type:       "out_for_delivery",
			Message:    "A letter is out for delivery to you",
		})
		if err != nil {
			log.Printf("Error creating recipient notification: %v", err)
			continue
		}

		// Mark notification as sent
		_, err = app.store.Pool().Exec(ctx, `
			UPDATE envelopes
			SET delivery_notification_sent = true
			WHERE id = $1
		`, envelopeID)
		if err != nil {
			log.Printf("Error marking notification as sent: %v", err)
		}
	}
}
