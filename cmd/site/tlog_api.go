package main

import (
	"encoding/json"
	"net/http"
	"strconv"

	"snailmail/internal/tlog"
)

// TLog is the global transparency log instance
var TLog = tlog.NewLog()

// handleTLogRoot handles GET /api/tlog/root
func (app *appState) handleTLogRoot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	root := TLog.GetRoot()
	size := TLog.Size()

	response := map[string]interface{}{
		"root": root,
		"size": size,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleTLogProof handles GET /api/tlog/proof/:index
func (app *appState) handleTLogProof(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract index from path
	path := r.URL.Path
	prefix := "/api/tlog/proof/"
	if len(path) <= len(prefix) {
		http.Error(w, "missing index", http.StatusBadRequest)
		return
	}

	indexStr := path[len(prefix):]
	index, err := strconv.ParseInt(indexStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid index", http.StatusBadRequest)
		return
	}

	proof, err := TLog.GetProof(index)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(proof)
}

// handleTLogLeaf handles GET /api/tlog/leaf/:index
func (app *appState) handleTLogLeaf(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract index from path
	path := r.URL.Path
	prefix := "/api/tlog/leaf/"
	if len(path) <= len(prefix) {
		http.Error(w, "missing index", http.StatusBadRequest)
		return
	}

	indexStr := path[len(prefix):]
	index, err := strconv.ParseInt(indexStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid index", http.StatusBadRequest)
		return
	}

	leaf, err := TLog.GetLeaf(index)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(leaf)
}
