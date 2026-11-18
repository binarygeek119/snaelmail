package blocker

import (
	"strings"
	"testing"

	"snailmail/internal/store"
)

func TestRegistryGetFix(t *testing.T) {
	r := NewRegistry("http://localhost:8477")
	relayID := "test-relay-id"

	tests := []struct {
		blocker  store.Blocker
		wantDesc string
		wantURL  string
		wantMeth string
	}{
		{
			blocker:  store.BlockerStatus,
			wantDesc: "not active",
			wantURL:  "/verify",
			wantMeth: "POST",
		},
		{
			blocker:  store.BlockerPaused,
			wantDesc: "paused",
			wantURL:  "/pause",
			wantMeth: "POST",
		},
		{
			blocker:  store.BlockerPlate,
			wantDesc: "plate",
			wantURL:  "/plate",
			wantMeth: "PUT",
		},
		{
			blocker:  store.BlockerTXT,
			wantDesc: "TXT",
			wantURL:  "/recheck",
			wantMeth: "POST",
		},
		{
			blocker:  store.BlockerComplaints,
			wantDesc: "complaints",
			wantURL:  "/complaints",
			wantMeth: "GET",
		},
	}

	for _, tt := range tests {
		t.Run(store.BlockerName(tt.blocker), func(t *testing.T) {
			fix := r.GetFix(relayID, tt.blocker)
			if fix == nil {
				t.Fatal("GetFix returned nil")
			}
			if !strings.Contains(fix.Description, tt.wantDesc) {
				t.Errorf("description %q does not contain %q", fix.Description, tt.wantDesc)
			}
			if !strings.Contains(fix.Action, tt.wantURL) {
				t.Errorf("action %q does not contain %q", fix.Action, tt.wantURL)
			}
			if !strings.Contains(fix.Action, relayID) {
				t.Errorf("action %q does not contain relayID %q", fix.Action, relayID)
			}
			if fix.Method != tt.wantMeth {
				t.Errorf("method = %q, want %q", fix.Method, tt.wantMeth)
			}
		})
	}
}

func TestRegistryUnknownBlocker(t *testing.T) {
	r := NewRegistry("http://localhost:8477")
	fix := r.GetFix("id", store.Blocker(999))
	if fix == nil {
		t.Fatal("GetFix returned nil for unknown blocker")
	}
	if !strings.Contains(fix.Description, "Unknown") {
		t.Errorf("unknown blocker description should mention 'Unknown': %q", fix.Description)
	}
}

func TestRenderBlockers(t *testing.T) {
	r := NewRegistry("http://localhost:8477")
	blockers := []store.Blocker{
		store.BlockerStatus,
		store.BlockerPaused,
		store.BlockerTXT,
	}

	fixes := r.RenderBlockers("relay-1", blockers)
	if len(fixes) != len(blockers) {
		t.Fatalf("RenderBlockers returned %d fixes, want %d", len(fixes), len(blockers))
	}

	// Verify each fix is non-nil and has expected fields
	for i, fix := range fixes {
		if fix == nil {
			t.Errorf("fix[%d] is nil", i)
			continue
		}
		if fix.Description == "" {
			t.Errorf("fix[%d].Description is empty", i)
		}
		if fix.Action == "" {
			t.Errorf("fix[%d].Action is empty", i)
		}
		if fix.Method == "" {
			t.Errorf("fix[%d].Method is empty", i)
		}
	}
}

func TestRenderBlockersEmpty(t *testing.T) {
	r := NewRegistry("http://localhost:8477")
	fixes := r.RenderBlockers("relay-1", nil)
	if len(fixes) != 0 {
		t.Errorf("RenderBlockers(nil) returned %d fixes, want 0", len(fixes))
	}
}

func TestBaseURLIncluded(t *testing.T) {
	r := NewRegistry("https://snailmail.example.com")
	fix := r.GetFix("abc", store.BlockerStatus)
	if !strings.HasPrefix(fix.Action, "https://snailmail.example.com") {
		t.Errorf("action should start with base URL: %q", fix.Action)
	}
}
