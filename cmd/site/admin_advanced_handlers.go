package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
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
	_, user := app.requireAdmin(w, r)
	if user == nil {
		return
	}

	userIDStr := r.PathValue("id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid user_id"})
		return
	}

	var req struct {
		Reason   string `json:"reason"`
		Duration string `json:"duration"` // "1d", "15d", "30d", "60d", "1y", "forever"
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	// Parse duration
	var duration time.Duration
	switch req.Duration {
	case "1d":
		duration = 24 * time.Hour
	case "15d":
		duration = 15 * 24 * time.Hour
	case "30d":
		duration = 30 * 24 * time.Hour
	case "60d":
		duration = 60 * 24 * time.Hour
	case "1y":
		duration = 365 * 24 * time.Hour
	case "forever":
		duration = 0
	default:
		duration = 24 * time.Hour // Default to 1 day
	}

	adminService := admin.NewAdminService(app.store, app.store.Pool())
	err = adminService.SuspendUserWithDuration(r.Context(), userID, user.ID, req.Reason, duration)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "success"})
}

// handleAdminUnsuspendUser handles POST /api/admin/users/:id/unsuspend
func (app *appState) handleAdminUnsuspendUser(w http.ResponseWriter, r *http.Request) {
	_, user := app.requireAdmin(w, r)
	if user == nil {
		return
	}

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
	err = adminService.UnsuspendUser(r.Context(), userID, user.ID, req.Reason)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "success"})
}
func (app *appState) handleAdminDeleteUser(w http.ResponseWriter, r *http.Request) {
	_, user := app.requireAdmin(w, r)
	if user == nil {
		return
	}

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
	err = adminService.DeleteUser(r.Context(), userID, user.ID, req.Reason)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "success"})
}

// handleAdminSuspendRelay handles POST /api/admin/relays/:id/suspend
func (app *appState) handleAdminSuspendRelay(w http.ResponseWriter, r *http.Request) {
	_, user := app.requireAdmin(w, r)
	if user == nil {
		return
	}

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
	err = adminService.SuspendRelay(r.Context(), relayID, user.ID, req.Reason)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "success"})
}

// handleAdminUnsuspendRelay handles POST /api/admin/relays/:id/unsuspend
func (app *appState) handleAdminUnsuspendRelay(w http.ResponseWriter, r *http.Request) {
	_, user := app.requireAdmin(w, r)
	if user == nil {
		return
	}

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
	err = adminService.UnsuspendRelay(r.Context(), relayID, user.ID, req.Reason)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "success"})
}

// handleAdminDeleteRelay handles DELETE /api/admin/relays/:id
func (app *appState) handleAdminDeleteRelay(w http.ResponseWriter, r *http.Request) {
	_, user := app.requireAdmin(w, r)
	if user == nil {
		return
	}

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
	err = adminService.DeleteRelay(r.Context(), relayID, user.ID, req.Reason)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "success"})
}

// handleAdminActivateRelay handles POST /api/admin/relays/{id}/activate
func (app *appState) handleAdminActivateRelay(w http.ResponseWriter, r *http.Request) {
	_, user := app.requireAdmin(w, r)
	if user == nil {
		return
	}

	relayID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid relay_id"})
		return
	}

	if err := app.store.ActivateRelay(r.Context(), relayID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "success"})
}

// handleAdminUpdateRelayLocation handles POST /api/admin/relays/{id}/location
func (app *appState) handleAdminUpdateRelayLocation(w http.ResponseWriter, r *http.Request) {
	_, user := app.requireAdmin(w, r)
	if user == nil {
		return
	}
	relayID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid relay_id"})
		return
	}
	var req struct {
		City    string  `json:"city"`
		Region  string  `json:"region"`
		CC      string  `json:"cc"`
		Lat     float64 `json:"lat"`
		Lon     float64 `json:"lon"`
		TZ      string  `json:"tz"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if req.TZ == "" { req.TZ = "UTC" }
	if err := app.store.UpdateRelayLocation(r.Context(), relayID, req.City, req.Region, req.CC, req.Lat, req.Lon, req.TZ); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "success"})
}

// handleAdminGetRelayPage handles GET /api/admin/relays/{id}/page
func (app *appState) handleAdminGetRelayPage(w http.ResponseWriter, r *http.Request) {
	_, user := app.requireAdmin(w, r)
	if user == nil {
		return
	}
	relayID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid relay_id"})
		return
	}
	p, err := app.store.GetPageByRelayID(r.Context(), relayID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if p == nil {
		writeJSON(w, http.StatusOK, map[string]any{"page": nil})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"page": p})
}

// handleAdminUpdateRelayStamp handles POST /api/admin/relays/{id}/stamp
func (app *appState) handleAdminUpdateRelayStamp(w http.ResponseWriter, r *http.Request) {
	_, user := app.requireAdmin(w, r)
	if user == nil {
		return
	}
	relayID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid relay_id"})
		return
	}
	var req struct {
		StampSVG       string `json:"stamp_svg"`
		StampPNGBase64 string `json:"stamp_png_base64"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	var stampPNG []byte
	if req.StampPNGBase64 != "" {
		stampPNG, err = base64.StdEncoding.DecodeString(req.StampPNGBase64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid stamp_png_base64"})
			return
		}
	}
	sumSrc := []byte(req.StampSVG)
	if len(stampPNG) > 0 && req.StampSVG == "" {
		sumSrc = stampPNG
	}
	sha := sha256.Sum256(sumSrc)
	if err := app.store.UpdateRelayStamp(r.Context(), relayID, req.StampSVG, stampPNG, sha); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "success"})
}

// handleAdminActions handles GET /api/admin/actions
func (app *appState) handleAdminActions(w http.ResponseWriter, r *http.Request) {
	_, user := app.requireAdmin(w, r)
	if user == nil {
		return
	}

	adminService := admin.NewAdminService(app.store, app.store.Pool())
	actions, err := adminService.GetAdminActions(r.Context(), 100)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get actions"})
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

// handleAdminRemoveRelay handles POST /api/admin/relays/{id}/remove
func (app *appState) handleAdminRemoveRelay(w http.ResponseWriter, r *http.Request) {
	_, user := app.requireAdmin(w, r)
	if user == nil { return }
	relayID, err := uuid.Parse(r.PathValue("id"))
	if err != nil { writeJSON(w, http.StatusBadRequest, map[string]string{"error":"invalid relay_id"}); return }
	if err := app.store.RetireRelay(r.Context(), relayID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()}); return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status":"success"})
}

func (app *appState) handleAdminListStamps(w http.ResponseWriter, r *http.Request) {
	_, user := app.requireAdmin(w, r)
	if user == nil { return }
	stamps, err := app.store.ListPendingStampVersions(r.Context())
	if err != nil { writeJSON(w, 500, map[string]string{"error": err.Error()}); return }
	writeJSON(w, 200, map[string]any{"stamps": stamps})
}

func (app *appState) handleAdminApproveStamp(w http.ResponseWriter, r *http.Request) {
	_, user := app.requireAdmin(w, r)
	if user == nil { return }
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil { writeJSON(w, 400, map[string]string{"error": "invalid id"}); return }
	var req struct { MakeCurrent bool `json:"make_current"` }
	_ = json.NewDecoder(r.Body).Decode(&req)
	// auto-make current if no current exists
	sv, err := app.store.GetStampVersion(r.Context(), id)
	if err != nil || sv == nil { writeJSON(w, 404, map[string]string{"error": "stamp not found"}); return }
	cur, _ := app.store.GetCurrentStampVersion(r.Context(), sv.RelayID)
	makeCurrent := req.MakeCurrent || cur == nil
	if err := app.store.ApproveStampVersion(r.Context(), id, user.ID, makeCurrent); err != nil { writeJSON(w, 500, map[string]string{"error": err.Error()}); return }
	writeJSON(w, 200, map[string]string{"status":"ok"})
}

func (app *appState) handleAdminRejectStamp(w http.ResponseWriter, r *http.Request) {
	_, user := app.requireAdmin(w, r)
	if user == nil { return }
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil { writeJSON(w, 400, map[string]string{"error": "invalid id"}); return }
	if err := app.store.RejectStampVersion(r.Context(), id, user.ID); err != nil { writeJSON(w, 500, map[string]string{"error": err.Error()}); return }
	writeJSON(w, 200, map[string]string{"status":"ok"})
}
