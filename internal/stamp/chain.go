// Package stamp implements the postmark chain: each hop in a letter's route
// carries a signed stamp that links to the previous hop. The chain proves
// the order and timing of postmarks.
//
// Chain linkage:
//
//	prev_hash = sha256(prev_stamp_json || relay_id || stamp_at_iso || idx)
//	sig       = ed25519(site_key, prev_hash)
//
// The first hop uses a genesis stamp (empty JSON) as prev.
package stamp

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Stamp is a single postmark in a letter's chain.
type Stamp struct {
	Idx        int       `json:"idx"`
	RelayID    [16]byte  `json:"relay_id"` // UUID
	StampAt    time.Time `json:"stamp_at"`
	PlateSHA   [32]byte  `json:"plate_sha"`
	PrevHash   [32]byte  `json:"prev_hash"`
	Sig        []byte    `json:"sig"` // ed25519 signature over prev_hash
	City       string    `json:"city"`
	CC         string    `json:"cc"`
}

// Encode serializes a stamp to canonical JSON for hashing/signing.
func (s *Stamp) Encode() ([]byte, error) {
	// Use a stable structure for hashing.
	m := map[string]any{
		"idx":        s.Idx,
		"relay_id":   hex.EncodeToString(s.RelayID[:]),
		"stamp_at":   s.StampAt.UTC().Format(time.RFC3339),
		"plate_sha":  hex.EncodeToString(s.PlateSHA[:]),
		"prev_hash":  hex.EncodeToString(s.PrevHash[:]),
		"city":       s.City,
		"cc":         s.CC,
	}
	raw, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	// Re-parse into generic tree and emit sorted keys
	var generic any
	if err := json.Unmarshal(raw, &generic); err != nil {
		return nil, err
	}
	return emitSorted(generic), nil
}

// emitSorted produces deterministic JSON with sorted keys.
func emitSorted(v any) []byte {
	switch val := v.(type) {
	case nil:
		return []byte("null")
	case bool:
		if val {
			return []byte("true")
		}
		return []byte("false")
	case float64:
		return []byte(json.Number(fmt.Sprintf("%v", val)).String())
	case string:
		b, _ := json.Marshal(val)
		return b
	case []any:
		out := []byte{'['}
		for i, elem := range val {
			if i > 0 {
				out = append(out, ',')
			}
			out = append(out, emitSorted(elem)...)
		}
		out = append(out, ']')
		return out
	case map[string]any:
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sortStrings(keys)
		out := []byte{'{'}
		for i, k := range keys {
			if i > 0 {
				out = append(out, ',')
			}
			kb, _ := json.Marshal(k)
			out = append(out, kb...)
			out = append(out, ':')
			out = append(out, emitSorted(val[k])...)
		}
		out = append(out, '}')
		return out
	default:
		b, _ := json.Marshal(val)
		return b
	}
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

var (
	ErrChainBroken  = errors.New("stamp: chain linkage broken")
	ErrBadSignature = errors.New("stamp: signature invalid")
	ErrNoStamps     = errors.New("stamp: empty chain")
)

// Genesis is the prev stamp JSON for the first hop.
var Genesis = []byte("{}")

// ComputePrevHash produces the linkage hash for hop idx.
//
//	prev_hash = sha256(prev_stamp_json || relay_id || stamp_at_iso || idx)
func ComputePrevHash(prevStampJSON []byte, relayID [16]byte, stampAt time.Time, idx int) [32]byte {
	h := sha256.New()
	h.Write(prevStampJSON)
	h.Write(relayID[:])
	h.Write([]byte(stampAt.UTC().Format(time.RFC3339)))
	// idx encoded as 4-byte big-endian for determinism
	var buf [4]byte
	buf[0] = byte(idx >> 24)
	buf[1] = byte(idx >> 16)
	buf[2] = byte(idx >> 8)
	buf[3] = byte(idx)
	h.Write(buf[:])
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}

// SignStamp produces a signed stamp for hop idx, given the previous stamp's
// canonical JSON (or Genesis for hop 0), the site private key, and metadata.
func SignStamp(priv ed25519.PrivateKey, prevJSON []byte, relayID [16]byte, stampAt time.Time, plateSHA [32]byte, idx int, city, cc string) (*Stamp, error) {
	prevHash := ComputePrevHash(prevJSON, relayID, stampAt, idx)
	sig := ed25519.Sign(priv, prevHash[:])
	return &Stamp{
		Idx:      idx,
		RelayID:  relayID,
		StampAt:  stampAt,
		PlateSHA: plateSHA,
		PrevHash: prevHash,
		Sig:      sig,
		City:     city,
		CC:       cc,
	}, nil
}

// VerifyStamp checks a single stamp's signature against the site public key.
func VerifyStamp(pub ed25519.PublicKey, s *Stamp) error {
	if !ed25519.Verify(pub, s.PrevHash[:], s.Sig) {
		return ErrBadSignature
	}
	return nil
}

// VerifyChain checks the entire chain: linkage, signatures, and structure.
func VerifyChain(pub ed25519.PublicKey, stamps []*Stamp) error {
	if len(stamps) == 0 {
		return ErrNoStamps
	}
	var prevJSON []byte = Genesis
	for i, s := range stamps {
		// Check idx
		if s.Idx != i {
			return fmt.Errorf("stamp[%d]: idx=%d", i, s.Idx)
		}
		// Check linkage
		expected := ComputePrevHash(prevJSON, s.RelayID, s.StampAt, s.Idx)
		if expected != s.PrevHash {
			return fmt.Errorf("%w at hop %d", ErrChainBroken, i)
		}
		// Check signature
		if err := VerifyStamp(pub, s); err != nil {
			return fmt.Errorf("%w at hop %d: %v", ErrBadSignature, i, err)
		}
		// Update prevJSON for next iteration
		encoded, err := s.Encode()
		if err != nil {
			return fmt.Errorf("stamp[%d]: encode: %w", i, err)
		}
		prevJSON = encoded
	}
	return nil
}

// BuildChain creates a full signed chain from a list of hop specs.
func BuildChain(priv ed25519.PrivateKey, hops []HopSpec) ([]*Stamp, error) {
	stamps := make([]*Stamp, 0, len(hops))
	prevJSON := Genesis
	for _, h := range hops {
		s, err := SignStamp(priv, prevJSON, h.RelayID, h.StampAt, h.PlateSHA, h.Idx, h.City, h.CC)
		if err != nil {
			return nil, fmt.Errorf("hop %d: %w", h.Idx, err)
		}
		encoded, err := s.Encode()
		if err != nil {
			return nil, fmt.Errorf("hop %d: encode: %w", h.Idx, err)
		}
		prevJSON = encoded
		stamps = append(stamps, s)
	}
	return stamps, nil
}

// HopSpec is the input for building a chain.
type HopSpec struct {
	Idx      int
	RelayID  [16]byte
	StampAt  time.Time
	PlateSHA [32]byte
	City     string
	CC       string
}
