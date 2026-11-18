package store

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestRoutable(t *testing.T) {
	now := time.Now()
	recentCheck := now.Add(-1 * time.Hour)
	oldCheck := now.Add(-25 * time.Hour)

	tests := []struct {
		name       string
		relay      *Relay
		threshold  int
		wantOK     bool
		wantBlock  []Blocker
	}{
		{
			name: "fully routable",
			relay: &Relay{
				Status:       "active",
				Paused:       false,
				PlateJSON:    []byte("{}"),
				PlateSig:     []byte("sig"),
				LastTXTCheck: &recentCheck,
				Complaints30d: 0,
			},
			threshold: 5,
			wantOK:    true,
			wantBlock: nil,
		},
		{
			name: "blocked by status",
			relay: &Relay{
				Status:       "pending",
				Paused:       false,
				PlateJSON:    []byte("{}"),
				PlateSig:     []byte("sig"),
				LastTXTCheck: &recentCheck,
				Complaints30d: 0,
			},
			threshold: 5,
			wantOK:    false,
			wantBlock: []Blocker{BlockerStatus},
		},
		{
			name: "blocked by paused",
			relay: &Relay{
				Status:       "active",
				Paused:       true,
				PlateJSON:    []byte("{}"),
				PlateSig:     []byte("sig"),
				LastTXTCheck: &recentCheck,
				Complaints30d: 0,
			},
			threshold: 5,
			wantOK:    false,
			wantBlock: []Blocker{BlockerPaused},
		},
		{
			name: "blocked by no plate",
			relay: &Relay{
				Status:       "active",
				Paused:       false,
				PlateJSON:    nil,
				PlateSig:     nil,
				LastTXTCheck: &recentCheck,
				Complaints30d: 0,
			},
			threshold: 5,
			wantOK:    false,
			wantBlock: []Blocker{BlockerPlate},
		},
		{
			name: "blocked by old TXT check",
			relay: &Relay{
				Status:       "active",
				Paused:       false,
				PlateJSON:    []byte("{}"),
				PlateSig:     []byte("sig"),
				LastTXTCheck: &oldCheck,
				Complaints30d: 0,
			},
			threshold: 5,
			wantOK:    false,
			wantBlock: []Blocker{BlockerTXT},
		},
		{
			name: "blocked by no TXT check",
			relay: &Relay{
				Status:       "active",
				Paused:       false,
				PlateJSON:    []byte("{}"),
				PlateSig:     []byte("sig"),
				LastTXTCheck: nil,
				Complaints30d: 0,
			},
			threshold: 5,
			wantOK:    false,
			wantBlock: []Blocker{BlockerTXT},
		},
		{
			name: "blocked by complaints",
			relay: &Relay{
				Status:       "active",
				Paused:       false,
				PlateJSON:    []byte("{}"),
				PlateSig:     []byte("sig"),
				LastTXTCheck: &recentCheck,
				Complaints30d: 5,
			},
			threshold: 5,
			wantOK:    false,
			wantBlock: []Blocker{BlockerComplaints},
		},
		{
			name: "multiple blockers",
			relay: &Relay{
				Status:       "pending",
				Paused:       true,
				PlateJSON:    nil,
				PlateSig:     nil,
				LastTXTCheck: nil,
				Complaints30d: 10,
			},
			threshold: 5,
			wantOK:    false,
			wantBlock: []Blocker{BlockerStatus, BlockerPaused, BlockerPlate, BlockerTXT, BlockerComplaints},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, blockers := Routable(tt.relay, tt.threshold)
			if ok != tt.wantOK {
				t.Errorf("Routable() ok = %v, want %v", ok, tt.wantOK)
			}
			if len(blockers) != len(tt.wantBlock) {
				t.Errorf("Routable() blockers = %v, want %v", blockers, tt.wantBlock)
			}
			for i, b := range blockers {
				if i < len(tt.wantBlock) && b != tt.wantBlock[i] {
					t.Errorf("blocker[%d] = %v, want %v", i, b, tt.wantBlock[i])
				}
			}
		})
	}
}

func TestRelayToRoute(t *testing.T) {
	id := uuid.New()
	now := time.Now()
	r := &Relay{
		ID:        id,
		Status:    "active",
		Paused:    false,
		PlateJSON: []byte("{}"),
		PlateSig:  []byte("sig"),
		City:      "London",
		CC:        "GB",
		Lat:       51.5074,
		Lon:       -0.1278,
		LastTXTCheck: &now,
	}

	route := RelayToRoute(r)

	if route.ID != [16]byte(id) {
		t.Error("ID mismatch")
	}
	if !route.Active {
		t.Error("should be active")
	}
	if route.Paused {
		t.Error("should not be paused")
	}
	if !route.Plate {
		t.Error("should have plate")
	}
	if route.City != "London" {
		t.Errorf("city = %q, want %q", route.City, "London")
	}
	if route.CC != "GB" {
		t.Errorf("cc = %q, want %q", route.CC, "GB")
	}
}
