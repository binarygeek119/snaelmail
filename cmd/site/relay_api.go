package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	"snailmail/internal/auth"
	"snailmail/internal/relayreg"
	"snailmail/internal/relayversion"

	"github.com/google/uuid"
)

// apiCreateRelay handles POST /api/relays
func (app *appState) apiCreateRelay(w http.ResponseWriter, r *http.Request) {
	if !app.requireRelayVersion(w, r) {
		return
	}
	var req struct {
		Hostname      string  `json:"hostname"`
		City          string  `json:"city"`
		Region        string  `json:"region"`
		CC            string  `json:"cc"`
		Lat           float64 `json:"lat"`
		Lon           float64 `json:"lon"`
		TZ            string  `json:"tz"`
		Pubkey        string  `json:"pubkey"` // base64-encoded Ed25519 public key
		StampSVG      string  `json:"stamp_svg"`
		OwnerUsername string  `json:"owner_username"`
		OwnerAPIKey   string  `json:"owner_api_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if strings.TrimSpace(req.OwnerUsername) == "" || strings.TrimSpace(req.OwnerAPIKey) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "owner_username and owner_api_key are required"})
		return
	}
	owner, err := app.store.GetUserByAPIKey(r.Context(), strings.TrimSpace(req.OwnerUsername), strings.TrimSpace(req.OwnerAPIKey))
	if err != nil || owner == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid username or api key"})
		return
	}

	// Validate hostname
	if err := relayreg.ValidateHostname(req.Hostname); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if strings.TrimSpace(req.City) == "" || strings.TrimSpace(req.Region) == "" || strings.TrimSpace(req.CC) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "city, region, and country code are required"})
		return
	}
	if strings.EqualFold(strings.TrimSpace(req.Region), "internet") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "region must be a real location, not 'Internet'"})
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
	ownerID := owner.ID
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
		&ownerID,
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

func (app *appState) requireRelayVersion(w http.ResponseWriter, r *http.Request) bool {
	minV := strings.TrimSpace(os.Getenv("SMAIL_MIN_RELAY_VERSION"))
	if minV == "" || minV == "0.0.0" {
		return true
	}
	rv := strings.TrimSpace(r.Header.Get("X-Relay-Version"))
	if rv == "" {
		writeJSON(w, http.StatusUpgradeRequired, map[string]string{
			"error":           "missing X-Relay-Version header; relay version is below required minimum or not reported",
			"minimum_version": minV,
		})
		return false
	}
	ok, err := relayversion.AtLeast(rv, minV)
	if err != nil || !ok {
		writeJSON(w, http.StatusUpgradeRequired, map[string]string{
			"error":           "relay version is below minimum required version; update relay to connect",
			"minimum_version": minV,
			"your_version":    rv,
		})
		return false
	}
	return true
}

// apiRelayRouter routes /api/relays/:id/* to the appropriate handler
func (app *appState) apiUpdateRelayProfile(w http.ResponseWriter, r *http.Request, relayID uuid.UUID) {
	if !app.requireRelayVersion(w, r) { return }
	var req struct {
		City          string  `json:"city"`
		Region        string  `json:"region"`
		CC            string  `json:"cc"`
		Lat           float64 `json:"lat"`
		Lon           float64 `json:"lon"`
		TZ            string  `json:"tz"`
		OwnerUsername string  `json:"owner_username"`
		OwnerAPIKey   string  `json:"owner_api_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"}); return
	}
	if strings.TrimSpace(req.OwnerUsername) == "" || strings.TrimSpace(req.OwnerAPIKey) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "owner_username and owner_api_key are required"}); return
	}
	owner, err := app.store.GetUserByAPIKey(r.Context(), strings.TrimSpace(req.OwnerUsername), strings.TrimSpace(req.OwnerAPIKey))
	if err != nil || owner == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid username or api key"}); return
	}
	if strings.TrimSpace(req.City)=="" || strings.TrimSpace(req.Region)=="" || strings.TrimSpace(req.CC)=="" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error":"city, region, and country code are required"}); return
	}
	if strings.EqualFold(strings.TrimSpace(req.Region), "internet") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error":"region must be a real location"}); return
	}
	if req.TZ == "" { req.TZ = "UTC" }
	if err := app.store.UpdateRelayLocation(r.Context(), relayID, req.City, req.Region, req.CC, req.Lat, req.Lon, req.TZ); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()}); return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status":"ok"})
}

func (app *appState) apiRelayRouter(w http.ResponseWriter, r *http.Request) {
	if !app.requireRelayVersion(w, r) {
		return
	}
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

	if action == "stamps" && r.Method == http.MethodGet {
		app.apiListRelayStamps(w, r, id)
		return
	}

	if !app.requireRelayVersion(w, r) {
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
	case "update":
		app.apiUpdateRelayProfile(w, r, id)
	case "stamps":
		if r.Method == http.MethodPost {
			app.apiSubmitRelayStamp(w, r, id)
		} else {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		}
	case "retire":
		app.apiRetireRelay(w, r)
	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	}
}

// apiMyRelays handles GET /api/postmaster/relays
func (app *appState) apiMyRelays(w http.ResponseWriter, r *http.Request) {
	_, user := app.validateSession(w, r)
	if user == nil { return }
	relays, err := app.store.GetRelaysByOwnerID(r.Context(), user.ID)
	if err != nil { writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()}); return }
	writeJSON(w, http.StatusOK, map[string]any{"relays": relays})
}

func (app *appState) apiPostmasterSetCurrentStamp(w http.ResponseWriter, r *http.Request) {
	_, user := app.validateSession(w, r)
	if user == nil { return }
	relayID, err := uuid.Parse(r.PathValue("id"))
	if err != nil { writeJSON(w,400,map[string]string{"error":"invalid relay id"}); return }
	stampID, err := uuid.Parse(r.PathValue("stampId"))
	if err != nil { writeJSON(w,400,map[string]string{"error":"invalid stamp id"}); return }
	relay, err := app.store.GetRelayByID(r.Context(), relayID)
	if err != nil || relay == nil { writeJSON(w,404,map[string]string{"error":"relay not found"}); return }
	if relay.OwnerUserID == nil || *relay.OwnerUserID != user.ID {
		// allow admin too
		if !user.IsAdmin { writeJSON(w,403,map[string]string{"error":"forbidden"}); return }
	}
	if err := app.store.SetCurrentStampVersion(r.Context(), relayID, stampID); err != nil { writeJSON(w,400,map[string]string{"error":err.Error()}); return }
	writeJSON(w,200,map[string]string{"status":"ok"})
}

func (app *appState) apiSubmitRelayStamp(w http.ResponseWriter, r *http.Request, relayID uuid.UUID) {
	var req struct {
		StampSVG       string  `json:"stamp_svg"`
		StampPNGBase64 string  `json:"stamp_png_base64"`
		Event          *string `json:"event"`
		Honor          *string `json:"honor"`
		Info           *string `json:"info"`
		StampDate      string  `json:"stamp_date"` // YYYY-MM-DD
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil { writeJSON(w,400,map[string]string{"error":"invalid JSON"}); return }
	relay, err := app.store.GetRelayByID(r.Context(), relayID)
	if err != nil || relay == nil { writeJSON(w,404,map[string]string{"error":"relay not found"}); return }
	format := ""; var svg string; var png []byte
	if strings.TrimSpace(req.StampSVG) != "" { format="svg"; svg=req.StampSVG }
	if req.StampPNGBase64 != "" { b, err := base64.StdEncoding.DecodeString(req.StampPNGBase64); if err != nil { writeJSON(w,400,map[string]string{"error":"invalid stamp_png_base64"}); return }; png=b; if format=="" { format="png" } }
	if format == "" { writeJSON(w,400,map[string]string{"error":"no stamp data"}); return }
	stampDate := time.Now().UTC()
	clean := func(p *string) *string { if p==nil { return nil }; v:=strings.TrimSpace(*p); if v=="" { return nil }; return &v }
	sv, err := app.store.CreateStampVersion(r.Context(), relayID, format, svg, png, stampDate, clean(req.Event), clean(req.Honor), clean(req.Info))
	if err != nil { writeJSON(w,500,map[string]string{"error":err.Error()}); return }
	writeJSON(w, 201, map[string]any{"status":"pending_approval","stamp": sv})
}

func (app *appState) apiListRelayStamps(w http.ResponseWriter, r *http.Request, relayID uuid.UUID) {
	stamps, err := app.store.ListRelayStampVersions(r.Context(), relayID)
	if err != nil { writeJSON(w,500,map[string]string{"error":err.Error()}); return }
	// hide rejected from public-ish listing? keep simple
	writeJSON(w, 200, map[string]any{"stamps": stamps})
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
