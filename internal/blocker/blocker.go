// Package blocker maps relay routing blockers to fix links and descriptions.
package blocker

import (
	"fmt"

	"snailmail/internal/store"
)

// Fix describes how to resolve a blocker.
type Fix struct {
	// Description explains the problem.
	Description string `json:"description"`
	// Action is the HTTP endpoint to fix the issue.
	Action string `json:"action"`
	// Method is the HTTP method (POST, PUT, etc.).
	Method string `json:"method"`
}

// Registry maps blocker types to their fixes.
type Registry struct {
	baseURL string
}

// NewRegistry creates a blocker registry with the given base URL for fix links.
func NewRegistry(baseURL string) *Registry {
	return &Registry{baseURL: baseURL}
}

// GetFix returns the fix for a blocker type.
func (r *Registry) GetFix(relayID string, b store.Blocker) *Fix {
	switch b {
	case store.BlockerStatus:
		return &Fix{
			Description: "Relay is not active. Complete verification to activate.",
			Action:      fmt.Sprintf("%s/api/relay/%s/verify", r.baseURL, relayID),
			Method:      "POST",
		}
	case store.BlockerPaused:
		return &Fix{
			Description: "Relay is paused. Unpause to resume routing.",
			Action:      fmt.Sprintf("%s/api/relay/%s/pause", r.baseURL, relayID),
			Method:      "POST",
		}
	case store.BlockerPlate:
		return &Fix{
			Description: "Relay plate is missing or invalid. Upload a valid plate.",
			Action:      fmt.Sprintf("%s/api/relay/%s/plate", r.baseURL, relayID),
			Method:      "PUT",
		}
	case store.BlockerTXT:
		return &Fix{
			Description: "DNS TXT record is stale or missing. Re-verify to refresh.",
			Action:      fmt.Sprintf("%s/api/relay/%s/recheck", r.baseURL, relayID),
			Method:      "POST",
		}
	case store.BlockerComplaints:
		return &Fix{
			Description: "Too many complaints in the last 30 days. Review and resolve issues.",
			Action:      fmt.Sprintf("%s/api/relay/%s/complaints", r.baseURL, relayID),
			Method:      "GET",
		}
	default:
		return &Fix{
			Description: "Unknown blocker.",
			Action:      "",
			Method:      "",
		}
	}
}

// RenderBlockers returns a list of fixes for all blockers on a relay.
func (r *Registry) RenderBlockers(relayID string, blockers []store.Blocker) []*Fix {
	fixes := make([]*Fix, 0, len(blockers))
	for _, b := range blockers {
		fixes = append(fixes, r.GetFix(relayID, b))
	}
	return fixes
}
