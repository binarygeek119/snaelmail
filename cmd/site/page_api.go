package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"

	"snailmail/internal/bundle"
	"snailmail/internal/page"
	"snailmail/internal/render"

	"github.com/google/uuid"
)

// apiPublishPage handles POST /api/relays/:id/publish
func (app *appState) apiPublishPage(w http.ResponseWriter, r *http.Request, relayID uuid.UUID) {
	// Decode request body
	var req struct {
		Manifest page.Manifest `json:"manifest"`
		Images   []struct {
			Filename string `json:"filename"`
			Data     string `json:"data"` // base64-encoded
		} `json:"images"`
		Logo *struct {
			Filename string `json:"filename"`
			Data     string `json:"data"` // base64-encoded
		} `json:"logo"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	// Validate manifest
	if err := page.Validate(&req.Manifest); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("manifest validation: %v", err)})
		return
	}

	// Fetch relay
	relay, err := app.store.GetRelayByID(r.Context(), relayID)
	if err != nil {
		log.Printf("fetch relay: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}
	if relay == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "relay not found"})
		return
	}

	// Check relay is active
	if relay.Status != "active" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "relay must be active to publish pages"})
		return
	}

	// Create bundle
	b, err := bundle.Create(relayID.String(), req.Manifest, app.siteKey)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create bundle"})
		return
	}

	// Add images
	for _, img := range req.Images {
		data, err := base64.StdEncoding.DecodeString(img.Data)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("invalid base64 for %s", img.Filename)})
			return
		}
		if err := b.AddImage(img.Filename, data); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("invalid image %s: %v", img.Filename, err)})
			return
		}
	}

	// Add logo
	if req.Logo != nil {
		data, err := base64.StdEncoding.DecodeString(req.Logo.Data)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid base64 for logo"})
			return
		}
		if err := b.SetLogo(data, req.Logo.Filename); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("invalid logo: %v", err)})
			return
		}
	}

	// Re-encode images
	if err := b.ReencodeImages(); err != nil {
		log.Printf("reencode images: %v", err)
		// Non-fatal, continue
	}

	// Sign manifest
	sig, err := b.Sign()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to sign manifest"})
		return
	}

	// Determine status based on review history
	reviewClean, err := app.store.GetRelayReviewClean(r.Context(), relayID)
	if err != nil {
		log.Printf("get review_clean: %v", err)
		reviewClean = 0
	}

	status := "live"

	// Store in database
	p, err := app.store.CreatePage(r.Context(), relayID, req.Manifest, sig, status)
	if err != nil {
		log.Printf("store page: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}

	// If status is live and review was needed, increment review_clean
	if status == "live" && reviewClean < 2 {
		if err := app.store.IncrementReviewClean(r.Context(), relayID); err != nil {
			log.Printf("update review_clean: %v", err)
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   status,
		"revision": p.Revision,
		"message":  fmt.Sprintf("page %s (revision %d)", status, p.Revision),
	})
}

// apiOpenPostOffice handles GET /open-post-offices/:hostname
func (app *appState) apiOpenPostOffice(w http.ResponseWriter, r *http.Request) {
	hostname := strings.TrimPrefix(r.URL.Path, "/open-post-offices/")

	// Check if this is an asset request
	if strings.Contains(hostname, "/images/") {
		parts := strings.SplitN(hostname, "/images/", 2)
		if len(parts) == 2 {
			app.servePageAsset(w, r, parts[0], parts[1])
			return
		}
	}

	// Fetch relay
	relay, err := app.store.GetRelayByHostname(r.Context(), hostname)
	if err != nil {
		log.Printf("fetch relay: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}
	if relay == nil {
		http.NotFound(w, r)
		return
	}

	relayInfo := render.RelayInfo{
		Hostname:     relay.Hostname,
		City:         relay.City,
		Region:       relay.Region,
		CC:           relay.CC,
		Lat:          relay.Lat,
		Lon:          relay.Lon,
		OpenedAt:     relay.OpenedAt,
		LettersCount: relay.LettersPassed,
		StampSVG:     template.HTML(relay.StampSVG),
	}

	// Fetch page
	p, err := app.store.GetPageByRelayID(r.Context(), relay.ID)
	if err != nil {
		log.Printf("fetch page: %v", err)
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	if p == nil || p.Status != "live" {
		// Render fallback
		html, err := render.RenderFallback(relayInfo)
		if err != nil {
			log.Printf("render fallback: %v", err)
			http.Error(w, "render error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(html)
		return
	}

	// Render page
	html, err := render.RenderPage(p.Manifest, relayInfo)
	if err != nil {
		log.Printf("render page: %v", err)
		http.Error(w, "render error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(html)
}

// servePageAsset serves page assets (images, logo)
func (app *appState) servePageAsset(w http.ResponseWriter, r *http.Request, hostname, filename string) {
	relay, err := app.store.GetRelayByHostname(r.Context(), hostname)
	if err != nil || relay == nil {
		http.NotFound(w, r)
		return
	}

	data, err := app.store.GetPageAsset(r.Context(), relay.ID, filename)
	if err != nil || len(data) == 0 {
		http.NotFound(w, r)
		return
	}

	ct := "application/octet-stream"
	switch {
	case strings.HasSuffix(strings.ToLower(filename), ".svg"):
		ct = "image/svg+xml"
	case strings.HasSuffix(strings.ToLower(filename), ".png"):
		ct = "image/png"
	case strings.HasSuffix(strings.ToLower(filename), ".jpg"), strings.HasSuffix(strings.ToLower(filename), ".jpeg"):
		ct = "image/jpeg"
	case strings.HasSuffix(strings.ToLower(filename), ".webp"):
		ct = "image/webp"
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_, _ = w.Write(data)
}

// apiPreviewPage handles POST /api/relays/:id/preview
func (app *appState) apiPreviewPage(w http.ResponseWriter, r *http.Request, relayID uuid.UUID) {
	// Decode request body
	var req struct {
		Manifest page.Manifest `json:"manifest"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	// Validate manifest
	if err := page.Validate(&req.Manifest); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("manifest validation: %v", err)})
		return
	}

	// Fetch relay
	relay, err := app.store.GetRelayByID(r.Context(), relayID)
	if err != nil {
		log.Printf("fetch relay: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}
	if relay == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "relay not found"})
		return
	}

	relayInfo := render.RelayInfo{
		Hostname:     relay.Hostname,
		City:         relay.City,
		Region:       relay.Region,
		CC:           relay.CC,
		Lat:          relay.Lat,
		Lon:          relay.Lon,
		OpenedAt:     relay.OpenedAt,
		LettersCount: relay.LettersPassed,
		StampSVG:     template.HTML(relay.StampSVG),
	}

	// Render preview
	html, err := render.RenderPage(req.Manifest, relayInfo)
	if err != nil {
		log.Printf("render preview: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "render error"})
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(html)
}

// apiApprovePage handles POST /api/admin/pages/:id/approve
func (app *appState) apiApprovePage(w http.ResponseWriter, r *http.Request, pageID int64) {
	ctx := r.Context()

	// Fetch page
	p, err := app.store.GetPageByID(ctx, pageID)
	if err != nil {
		log.Printf("fetch page: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}
	if p == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "page not found"})
		return
	}

	if p.Status != "review" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "page is not in review"})
		return
	}

	// Approve page
	note := "Approved"
	if err := app.store.UpdatePageStatus(ctx, pageID, "live", &note); err != nil {
		log.Printf("approve page: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}

	// Increment review_clean for relay
	if err := app.store.IncrementReviewClean(ctx, p.RelayID); err != nil {
		log.Printf("update review_clean: %v", err)
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "approved"})
}

// apiRejectPage handles POST /api/admin/pages/:id/reject
func (app *appState) apiRejectPage(w http.ResponseWriter, r *http.Request, pageID int64) {
	ctx := r.Context()

	var req struct {
		Note string `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	// Fetch page
	p, err := app.store.GetPageByID(ctx, pageID)
	if err != nil {
		log.Printf("fetch page: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}
	if p == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "page not found"})
		return
	}

	if p.Status != "review" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "page is not in review"})
		return
	}

	// Reject page
	if err := app.store.UpdatePageStatus(ctx, pageID, "rejected", &req.Note); err != nil {
		log.Printf("reject page: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "rejected"})
}

// apiAdminPageRouter routes /api/admin/pages/:id/* to admin handlers
func (app *appState) apiAdminPageRouter(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	parts := strings.Split(strings.TrimPrefix(path, "/api/admin/pages/"), "/")

	if len(parts) < 2 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing action"})
		return
	}

	pageIDStr := parts[0]
	action := parts[1]

	var pageID int64
	if _, err := fmt.Sscanf(pageIDStr, "%d", &pageID); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid page id"})
		return
	}

	switch action {
	case "approve":
		app.apiApprovePage(w, r, pageID)
	case "reject":
		app.apiRejectPage(w, r, pageID)
	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	}
}
