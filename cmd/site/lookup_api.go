package main

import (
	"encoding/json"
	"net/http"

	"snailmail/internal/lookup"
)

// handleRelaySearch handles GET /api/relays/search
func (app *appState) handleRelaySearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	city := r.URL.Query().Get("city")
	region := r.URL.Query().Get("region")
	country := r.URL.Query().Get("country")
	status := r.URL.Query().Get("status")

	filter := lookup.RelayFilter{
		City:    city,
		Region:  region,
		Country: country,
		Status:  status,
		Limit:   50,
		Offset:  0,
	}

	results, err := lookup.SearchRelays(r.Context(), app.store.Pool(), filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

// handleUserSearch handles GET /api/users/search (admin only)
func (app *appState) handleUserSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// TODO: Add admin authentication check

	handle := r.URL.Query().Get("handle")
	status := r.URL.Query().Get("status")

	filter := lookup.UserFilter{
		Handle: handle,
		Status: status,
		Limit:  50,
		Offset: 0,
	}

	results, err := lookup.SearchUsers(r.Context(), app.store.Pool(), filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

// handleLetterSearch handles GET /api/letters/search (admin only)
func (app *appState) handleLetterSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// TODO: Add admin authentication check

	status := r.URL.Query().Get("status")

	filter := lookup.LetterFilter{
		Status: status,
		Limit:  50,
		Offset: 0,
	}

	results, err := lookup.SearchLetters(r.Context(), app.store.Pool(), filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}
