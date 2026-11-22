package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"snailmail/internal/sse"
	"snailmail/internal/stats"
	"snailmail/internal/store"
	"snailmail/internal/worker"

	"github.com/google/uuid"
)

func (app *relayApp) setupRoutes(mux *http.ServeMux) {
	// Web UI
	mux.HandleFunc("GET /{$}", app.setupUIHandler)
	mux.HandleFunc("GET /api/setup/status", app.setupStatusHandler)
	mux.HandleFunc("POST /api/setup/keygen", app.setupKeygenHandler)
	mux.HandleFunc("POST /api/setup/register", app.setupRegisterHandler)
	mux.HandleFunc("GET /api/setup/images", app.setupListImagesHandler)
	mux.HandleFunc("POST /api/setup/images", app.setupUploadImageHandler)
	mux.HandleFunc("GET /api/setup/images/{name}", app.setupGetImageHandler)
	mux.HandleFunc("DELETE /api/setup/images/{name}", app.setupDeleteImageHandler)
	mux.HandleFunc("POST /api/setup/images/hero", app.setupImageHeroHandler)
	mux.HandleFunc("POST /api/setup/stamp", app.setupUploadStampHandler)
	mux.HandleFunc("GET /api/setup/stamp/preview", app.setupStampPreviewHandler)
	mux.HandleFunc("POST /api/setup/publish", app.setupPublishHandler)
	mux.HandleFunc("POST /api/setup/copy-central", app.setupCopyCentralHandler)
	mux.HandleFunc("GET /preview", app.setupPreviewHandler)
	mux.HandleFunc("GET /preview/images/{name}", app.setupPreviewImageHandler)

	// Health checks
	mux.HandleFunc("GET /healthz", app.healthz)
	mux.HandleFunc("GET /readyz", app.readyz)

	// Dashboard
	mux.HandleFunc("GET /stats", app.statsHandler)
	mux.HandleFunc("GET /blockers", app.blockersHandler)

	// SSE stream
	mux.HandleFunc("GET /stream", app.broker.Handler())

	// Controls
	mux.HandleFunc("POST /pause", app.pauseHandler)
	mux.HandleFunc("POST /unpause", app.unpauseHandler)
	mux.HandleFunc("POST /recheck", app.recheckHandler)
	mux.HandleFunc("POST /retire", app.retireHandler)
}

func (app *relayApp) requireStore(w http.ResponseWriter) bool {
	if app.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database not configured; set SMAIL_DSN"})
		return false
	}
	return true
}

func (app *relayApp) healthz(w http.ResponseWriter, r *http.Request) {
	if app.store != nil {
		if err := app.store.Health(r.Context()); err != nil {
			http.Error(w, "db down", http.StatusServiceUnavailable)
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "db": func() string { if app.store==nil { return "not_configured" }; return "ok" }()})
}

func (app *relayApp) readyz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (app *relayApp) statsHandler(w http.ResponseWriter, r *http.Request) {
	if !app.requireStore(w) { return }
	relayID, err := app.requireRelayID(w, r)
	if err != nil {
		return
	}

	window := r.URL.Query().Get("window")
	if window == "" {
		window = "today"
	}

	var from, to time.Time
	now := time.Now().UTC()
	today := now.Truncate(24 * time.Hour)

	switch window {
	case "today":
		from = today
		to = today.Add(24 * time.Hour)
	case "7d":
		from = today.AddDate(0, 0, -6)
		to = today.Add(24 * time.Hour)
	case "30d":
		from = today.AddDate(0, 0, -29)
		to = today.Add(24 * time.Hour)
	case "lifetime":
		from = time.Time{}
		to = now.Add(24 * time.Hour)
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid window"})
		return
	}

	summary, err := stats.Window(r.Context(), app.store.Pool(), relayID, from, to)
	if err != nil {
		log.Printf("stats: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "stats query failed"})
		return
	}

	// Get daily rows for sparkline
	var daily []stats.DailyRow
	if window == "7d" || window == "30d" {
		daily, err = stats.Daily(r.Context(), app.store.Pool(), relayID, from, to)
		if err != nil {
			log.Printf("stats daily: %v", err)
			// Non-fatal
		}
	}

	// Get queue depth
	queueDepth, _ := stats.QueueDepth(r.Context(), app.store.Pool())

	writeJSON(w, http.StatusOK, map[string]any{
		"relay_id":    relayID,
		"window":      window,
		"summary":     summary,
		"daily":       daily,
		"queue_depth": queueDepth,
	})
}

func (app *relayApp) blockersHandler(w http.ResponseWriter, r *http.Request) {
	if !app.requireStore(w) { return }
	relayID, err := app.requireRelayID(w, r)
	if err != nil {
		return
	}

	relay, err := app.store.GetRelayByID(r.Context(), relayID)
	if err != nil {
		log.Printf("blockers: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db error"})
		return
	}
	if relay == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "relay not found"})
		return
	}

	ok, blockers := store.Routable(relay, 5) // 5 = complaint threshold
	fixes := app.blockerRegistry.RenderBlockers(relayID.String(), blockers)

	writeJSON(w, http.StatusOK, map[string]any{
		"relay_id":  relayID,
		"routable":  ok,
		"blockers":  blockers,
		"fixes":     fixes,
	})
}

func (app *relayApp) pauseHandler(w http.ResponseWriter, r *http.Request) {
	if !app.requireStore(w) { return }
	relayID, err := app.requireRelayID(w, r)
	if err != nil {
		return
	}

	if err := app.store.SetRelayPaused(r.Context(), relayID, true); err != nil {
		log.Printf("pause: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db error"})
		return
	}

	// Broadcast status change via SSE
	app.broker.Broadcast(sse.Event{
		Name: "status",
		Data: mustJSON(map[string]any{
			"relay_id": relayID,
			"paused":   true,
		}),
	})

	writeJSON(w, http.StatusOK, map[string]string{"status": "paused"})
}

func (app *relayApp) unpauseHandler(w http.ResponseWriter, r *http.Request) {
	if !app.requireStore(w) { return }
	relayID, err := app.requireRelayID(w, r)
	if err != nil {
		return
	}

	if err := app.store.SetRelayPaused(r.Context(), relayID, false); err != nil {
		log.Printf("unpause: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db error"})
		return
	}

	app.broker.Broadcast(sse.Event{
		Name: "status",
		Data: mustJSON(map[string]any{
			"relay_id": relayID,
			"paused":   false,
		}),
	})

	writeJSON(w, http.StatusOK, map[string]string{"status": "unpaused"})
}

func (app *relayApp) recheckHandler(w http.ResponseWriter, r *http.Request) {
	if !app.requireStore(w) { return }
	relayID, err := app.requireRelayID(w, r)
	if err != nil {
		return
	}

	// Recheck is implemented in the site's API; here we just update the timestamp
	if err := app.store.UpdateRelayTXTCheck(r.Context(), relayID); err != nil {
		log.Printf("recheck: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db error"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "rechecked"})
}

func (app *relayApp) retireHandler(w http.ResponseWriter, r *http.Request) {
	if !app.requireStore(w) { return }
	relayID, err := app.requireRelayID(w, r)
	if err != nil {
		return
	}

	var req struct {
		ConfirmHostname string `json:"confirm_hostname"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	relay, err := app.store.GetRelayByID(r.Context(), relayID)
	if err != nil || relay == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "relay not found"})
		return
	}

	// Must type the hostname to confirm retirement
	if req.ConfirmHostname != relay.Hostname {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "confirm_hostname must match the relay's hostname",
		})
		return
	}

	if err := app.store.RetireRelay(r.Context(), relayID); err != nil {
		log.Printf("retire: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db error"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "retired"})
}

// requireRelayID extracts the relay_id from query param or returns 400.
func (app *relayApp) requireRelayID(w http.ResponseWriter, r *http.Request) (uuid.UUID, error) {
	idStr := r.URL.Query().Get("relay_id")
	if idStr == "" {
		// Try path param
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/"), "/")
		if len(parts) >= 2 {
			idStr = parts[1]
		}
	}
	if idStr == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "relay_id required"})
		return uuid.Nil, http.ErrBodyNotAllowed
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid relay_id"})
		return uuid.Nil, err
	}
	return id, nil
}

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// startBackgroundServices launches the stamp worker and SSE publisher.
func (app *relayApp) startBackgroundServices(ctx context.Context) {
	if app.store == nil {
		log.Println("relay: background services skipped (database not configured)")
		return
	}
	// Stamp worker: process due hops every 15 seconds
	stampWorker := worker.NewStampWorker(app.store.Pool(), 15*time.Second)
	go stampWorker.Run(ctx)

	// SSE publisher: broadcast stats every 5 seconds
	statsPublisher := sse.NewPublisher(app.broker, 5*time.Second, func(ctx context.Context) (sse.Event, error) {
		queueDepth, err := stats.QueueDepth(ctx, app.store.Pool())
		if err != nil {
			return sse.Event{}, err
		}
		lag, err := stats.DueLagSeconds(ctx, app.store.Pool())
		if err != nil {
			return sse.Event{}, err
		}
		return sse.Event{
			Name: "counter",
			Data: mustJSON(map[string]any{
				"queue_depth":   queueDepth,
				"due_lag_seconds": lag,
			}),
		}, nil
	})
	go statsPublisher.Run(ctx)

	log.Println("relay: background services started")
}
