package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"snailmail/internal/auth"
	"snailmail/internal/relayreg"

	"github.com/google/uuid"
)

// apiCreateRelay handles POST /api/relays
func (app *appState) apiCreateRelay(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Hostname string  `json:"hostname"`
		City     string  `json:"city"`
		Region   string  `json:"region"`
		CC       string  `json:"cc"`
		Lat      float64 `json:"lat"`
		Lon      float64 `json:"lon"`
		TZ       string  `json:"tz"`
		Pubkey   string  `json:"pubkey"` // base64-encoded Ed25519 public key
		StampSVG string  `json:"stamp_svg"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	// Validate hostname
	if err := relayreg.ValidateHostname(req.Hostname); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	// Check hostname uniqueness
	existing, err := app.store.GetRelayByHostname(r.Context(), req.Hostname)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}
	if existing != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "hostname already registered"})
		return
	}

	// Decode public key
	pubkeyBytes, err := base64.StdEncoding.DecodeString(req.Pubkey)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid pubkey base64"})
		return
	}
	if len(pubkeyBytes) != ed25519.PublicKeySize {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "pubkey must be 32 bytes"})
		return
	}

	// Compute key fingerprint
	keyFpr := relayreg.KeyFingerprint(ed25519.PublicKey(pubkeyBytes))

	// For now, use placeholder stamp PNG and SHA256
	// In a real system, we'd render the SVG to PNG and compute SHA256
	stampPNG := []byte{} // placeholder
	stampSHA256 := []byte{} // placeholder

	// Create relay
	relay, err := app.store.CreateRelay(
		r.Context(),
		req.Hostname,
		pubkeyBytes,
		keyFpr,
		req.City,
		req.Region,
		req.CC,
		req.Lat,
		req.Lon,
		req.TZ,
		req.StampSVG,
		stampPNG,
		stampSHA256,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create relay"})
		return
	}

	// Generate challenge token
	token, err := auth.GenerateToken(16)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to generate token"})
		return
	}

	// Create challenge (1 hour expiry)
	expiresAt := time.Now().Add(1 * time.Hour)
	if err := app.store.CreateRelayChallenge(r.Context(), relay.ID, token, expiresAt); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create challenge"})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"relay_id": relay.ID,
		"hostname": relay.Hostname,
		"status":   relay.Status,
		"token":    token,
		"instructions": map[string]string{
			"txt_record": "Add this TXT record to your hostname:",
			"format":     "v=1; token=" + token + "; key=ed25519:" + req.Pubkey,
		},
	})
}

// apiGetRelayChallenge handles GET /api/relays/:id/challenge
func (app *appState) apiGetRelayChallenge(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/api/relays/")
	idStr = strings.TrimSuffix(idStr, "/challenge")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid relay id"})
		return
	}

	challenge, err := app.store.GetRelayChallenge(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}
	if challenge == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no active challenge"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"token":      challenge.Token,
		"expires_at": challenge.ExpiresAt,
	})
}

// apiVerifyRelay handles POST /api/relays/:id/verify
func (app *appState) apiVerifyRelay(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/api/relays/")
	idStr = strings.TrimSuffix(idStr, "/verify")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid relay id"})
		return
	}

	// Fetch relay
	relay, err := app.store.GetRelayByID(r.Context(), id)
	if err != nil || relay == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "relay not found"})
		return
	}

	// Fetch challenge
	challenge, err := app.store.GetRelayChallenge(r.Context(), id)
	if err != nil || challenge == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no active challenge"})
		return
	}

	// Verify DNS TXT record
	dnsResolver := &relayreg.NetResolver{}
	if err := relayreg.Verify(r.Context(), dnsResolver, nil, relay.Hostname, challenge.Token, relay.KeyFpr); err != nil {
		// Verification failed
		if err := app.store.UpdateRelayStatus(r.Context(), id, "unverified"); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update status"})
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error":   "verification failed",
			"details": err.Error(),
		})
		return
	}

	// Success: activate relay
	if err := app.store.ActivateRelay(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to activate"})
		return
	}

	// Update last TXT check
	if err := app.store.UpdateRelayTXTCheck(r.Context(), id); err != nil {
		// Non-fatal
	}

	// Delete challenge
	if err := app.store.DeleteRelayChallenge(r.Context(), id, challenge.Token); err != nil {
		// Non-fatal
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "active",
		"message": "relay verified and activated",
	})
}

// apiPauseRelay handles POST /api/relay/:id/pause
func (app *appState) apiPauseRelay(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/api/relay/")
	idStr = strings.TrimSuffix(idStr, "/pause")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid relay id"})
		return
	}

	var req struct {
		Paused bool `json:"paused"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	if err := app.store.SetRelayPaused(r.Context(), id, req.Paused); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update pause"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"paused": req.Paused,
	})
}

// apiRecheckRelay handles POST /api/relay/:id/recheck
func (app *appState) apiRecheckRelay(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/api/relay/")
	idStr = strings.TrimSuffix(idStr, "/recheck")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid relay id"})
		return
	}

	relay, err := app.store.GetRelayByID(r.Context(), id)
	if err != nil || relay == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "relay not found"})
		return
	}

	// Re-verify DNS TXT record
	dnsResolver := &relayreg.NetResolver{}
	challenge, _ := app.store.GetRelayChallenge(r.Context(), id)

	if challenge != nil {
		if err := relayreg.Verify(r.Context(), dnsResolver, nil, relay.Hostname, challenge.Token, relay.KeyFpr); err != nil {
			// Recheck failed
			if err := app.store.UpdateRelayStatus(r.Context(), id, "unverified"); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update status"})
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"status": "unverified",
				"error":  err.Error(),
			})
			return
		}
	}

	// Update last TXT check
	if err := app.store.UpdateRelayTXTCheck(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status": relay.Status,
	})
}

// apiRetireRelay handles POST /api/relay/:id/retire
func (app *appState) apiRetireRelay(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/api/relay/")
	idStr = strings.TrimSuffix(idStr, "/retire")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid relay id"})
		return
	}

	var req struct {
		ConfirmHostname string `json:"confirm_hostname"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	relay, err := app.store.GetRelayByID(r.Context(), id)
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

	if err := app.store.RetireRelay(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to retire relay"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status": "retired",
	})
}

// apiRelayRouter routes /api/relays/:id/* to the appropriate handler
func (app *appState) apiRelayRouter(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	parts := strings.Split(strings.TrimPrefix(path, "/api/relays/"), "/")

	if len(parts) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing relay id"})
		return
	}

	idStr := parts[0]
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}

	_ = idStr
	_ = action

	switch action {
	case "challenge":
		app.apiGetRelayChallenge(w, r)
	case "verify":
		app.apiVerifyRelay(w, r)
	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	}
}

// apiRelayActionRouter routes /api/relay/:id/* to the appropriate handler
func (app *appState) apiRelayActionRouter(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	parts := strings.Split(strings.TrimPrefix(path, "/api/relay/"), "/")

	if len(parts) < 2 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing action"})
		return
	}

	idStr := parts[0]
	action := parts[1]

	id, err := uuid.Parse(idStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid relay id"})
		return
	}

	switch action {
	case "pause":
		app.apiPauseRelay(w, r)
	case "recheck":
		app.apiRecheckRelay(w, r)
	case "publish":
		app.apiPublishPage(w, r, id)
	case "preview":
		app.apiPreviewPage(w, r, id)
	case "retire":
		app.apiRetireRelay(w, r)
	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	}
}

// apiOpenPostOffices handles GET /open-post-offices
func (app *appState) apiOpenPostOffices(w http.ResponseWriter, r *http.Request) {
	relays, err := app.store.ListActiveRelays(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}

	var list []map[string]any
	for _, relay := range relays {
		list = append(list, map[string]any{
			"id":               relay.ID,
			"hostname":         relay.Hostname,
			"city":             relay.City,
			"region":           relay.Region,
			"cc":               relay.CC,
			"central":          relay.Central,
			"lat":              relay.Lat,
			"lon":              relay.Lon,
			"opened_at":        relay.OpenedAt,
			"letters_passed":   relay.LettersPassed,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"relays": list,
		"count":  len(list),
	})
}

// apiOpenPostOffice handles GET /open-post-offices/:slug
// For later: apiLoginChallenge, apiLogin, apiDashboard, apiStream, etc.
