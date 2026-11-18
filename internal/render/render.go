// Package render ties together chain verification and composite rendering.
// It performs render-time verification of stamps and produces the final
// postmark composite image. Unverifiable stamps render gray but never
// block delivery (§1 invariants).
package render

import (
	"crypto/ed25519"

	"snailmail/internal/stamp"
)

// RenderResult is the output of rendering a letter's postmarks.
type RenderResult struct {
	// CompositePNG is the final composite image bytes.
	CompositePNG []byte
	// AllVerified is true when every stamp in the chain verified.
	AllVerified bool
	// Errors holds per-hop verification errors (nil for OK hops).
	Errors []error
}

// Options controls render-time verification.
type Options struct {
	SiteKey ed25519.PublicKey
}

// Render verifies the chain and produces the composite image.
// Per §1: "Unverifiable stamps render gray but never block delivery."
func Render(siteKey ed25519.PublicKey, stamps []*stamp.Stamp, stampPNGs [][]byte) (*RenderResult, error) {
	// 1. Verify the full chain
	chainErr := stamp.VerifyChain(siteKey, stamps)

	// 2. Determine per-hop verification status
	errors := make([]error, len(stamps))
	allOK := chainErr == nil

	if chainErr != nil {
		// If chain is broken, mark all hops as unverified
		for i := range stamps {
			errors[i] = chainErr
		}
	} else {
		// Per-hop individual verification
		for i, s := range stamps {
			if err := stamp.VerifyStamp(siteKey, s); err != nil {
				errors[i] = err
				allOK = false
			}
		}
	}

	// 3. Build composite slots
	slots := make([]stamp.Slot, len(stamps))
	for i, s := range stamps {
		var stampPNG []byte
		if i < len(stampPNGs) {
			stampPNG = stampPNGs[i]
		}
		slots[i] = stamp.Slot{
			StampPNG: stampPNG,
			Date:     s.StampAt,
			City:     s.City,
			Idx:      s.Idx,
			Total:    len(stamps),
			Verified: errors[i] == nil,
		}
	}

	// 4. Render composite
	composite, err := stamp.Composite(slots)
	if err != nil {
		return nil, err
	}

	return &RenderResult{
		CompositePNG: composite,
		AllVerified:  allOK,
		Errors:       errors,
	}, nil
}
