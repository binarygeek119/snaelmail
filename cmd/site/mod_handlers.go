package main

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
)

func (app *appState) handleModRelays(w http.ResponseWriter, r *http.Request) {
	_, user := app.requireModOrAdmin(w, r)
	if user == nil { return }
	relays, err := app.store.GetAllRelays(r.Context())
	if err != nil { writeJSON(w, 500, map[string]string{"error": err.Error()}); return }
	writeJSON(w, 200, map[string]any{"relays": relays})
}

func (app *appState) handleModApproveRelay(w http.ResponseWriter, r *http.Request) {
	_, user := app.requireModOrAdmin(w, r)
	if user == nil { return }
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil { writeJSON(w, 400, map[string]string{"error":"invalid id"}); return }
	if err := app.store.ActivateRelay(r.Context(), id); err != nil { writeJSON(w, 500, map[string]string{"error": err.Error()}); return }
	writeJSON(w, 200, map[string]string{"status":"ok"})
}

func (app *appState) handleModSuspendRelay(w http.ResponseWriter, r *http.Request) {
	_, user := app.requireModOrAdmin(w, r)
	if user == nil { return }
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil { writeJSON(w, 400, map[string]string{"error":"invalid id"}); return }
	var req struct { Duration string `json:"duration"` }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil { writeJSON(w,400,map[string]string{"error":"invalid JSON"}); return }
	var until *time.Time
	now := time.Now().UTC()
	switch req.Duration {
	case "1d": t := now.Add(24*time.Hour); until = &t
	case "15d": t := now.Add(15*24*time.Hour); until = &t
	case "30d": t := now.Add(30*24*time.Hour); until = &t
	case "60d": t := now.Add(60*24*time.Hour); until = &t
	case "1y": t := now.AddDate(1,0,0); until = &t
	case "2y": t := now.AddDate(2,0,0); until = &t
	case "life": until = nil // indefinite
	default: writeJSON(w,400,map[string]string{"error":"invalid duration"}); return
	}
	if err := app.store.SuspendRelayUntil(r.Context(), id, until); err != nil { writeJSON(w,500,map[string]string{"error":err.Error()}); return }
	writeJSON(w, 200, map[string]string{"status":"ok"})
}

func (app *appState) handleModUnsuspendRelay(w http.ResponseWriter, r *http.Request) {
	_, user := app.requireModOrAdmin(w, r)
	if user == nil { return }
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil { writeJSON(w, 400, map[string]string{"error":"invalid id"}); return }
	if err := app.store.UnsuspendRelay(r.Context(), id); err != nil { writeJSON(w,500,map[string]string{"error":err.Error()}); return }
	writeJSON(w, 200, map[string]string{"status":"ok"})
}

func (app *appState) handleModStamps(w http.ResponseWriter, r *http.Request) {
	_, user := app.requireModOrAdmin(w, r)
	if user == nil { return }
	stamps, err := app.store.ListPendingStampVersions(r.Context())
	if err != nil { writeJSON(w,500,map[string]string{"error":err.Error()}); return }
	writeJSON(w,200,map[string]any{"stamps":stamps})
}

func (app *appState) handleModApproveStamp(w http.ResponseWriter, r *http.Request) {
	_, user := app.requireModOrAdmin(w, r)
	if user == nil { return }
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil { writeJSON(w,400,map[string]string{"error":"invalid id"}); return }
	var req struct { MakeCurrent bool `json:"make_current"` }
	_ = json.NewDecoder(r.Body).Decode(&req)
	sv, err := app.store.GetStampVersion(r.Context(), id)
	if err != nil || sv == nil { writeJSON(w,404,map[string]string{"error":"stamp not found"}); return }
	cur, _ := app.store.GetCurrentStampVersion(r.Context(), sv.RelayID)
	makeCurrent := req.MakeCurrent || cur == nil
	if err := app.store.ApproveStampVersion(r.Context(), id, user.ID, makeCurrent); err != nil { writeJSON(w,500,map[string]string{"error":err.Error()}); return }
	writeJSON(w,200,map[string]string{"status":"ok"})
}

func (app *appState) handleModRejectStamp(w http.ResponseWriter, r *http.Request) {
	_, user := app.requireModOrAdmin(w, r)
	if user == nil { return }
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil { writeJSON(w,400,map[string]string{"error":"invalid id"}); return }
	if err := app.store.RejectStampVersion(r.Context(), id, user.ID); err != nil { writeJSON(w,500,map[string]string{"error":err.Error()}); return }
	writeJSON(w,200,map[string]string{"status":"ok"})
}

func (app *appState) handleModRelayAssets(w http.ResponseWriter, r *http.Request) {
	_, user := app.requireModOrAdmin(w, r)
	if user == nil { return }
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil { writeJSON(w,400,map[string]string{"error":"invalid id"}); return }
	assets, err := app.store.ListPageAssets(r.Context(), id)
	if err != nil { writeJSON(w,500,map[string]string{"error":err.Error()}); return }
	writeJSON(w,200,map[string]any{"assets":assets})
}

func (app *appState) handleModDeleteAsset(w http.ResponseWriter, r *http.Request) {
	_, user := app.requireModOrAdmin(w, r)
	if user == nil { return }
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil { writeJSON(w,400,map[string]string{"error":"invalid id"}); return }
	filename := r.PathValue("filename")
	if filename == "" { writeJSON(w,400,map[string]string{"error":"missing filename"}); return }
	if err := app.store.DeletePageAsset(r.Context(), id, filename); err != nil { writeJSON(w,500,map[string]string{"error":err.Error()}); return }
	writeJSON(w,200,map[string]string{"status":"ok"})
}
