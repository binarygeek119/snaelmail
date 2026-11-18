// Package plate implements operator-signed registration records.
//
// A plate is an Ed25519 signature over canonical JSON of the form:
//
//	{"v":1,"relay":"<uuid>","loc":{"lat":..,"lon":..,"city":"...","cc":".."},
//	 "stamp_sha256":"<hex>","issued":"<rfc3339>"}
//
// Canonical JSON: keys sorted, no whitespace, floats in shortest form.
package plate

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Loc is the location declared in a plate.
type Loc struct {
	Lat  float64 `json:"lat"`
	Lon  float64 `json:"lon"`
	City string  `json:"city"`
	CC   string  `json:"cc"`
}

// Data is the canonical payload of a plate.
type Data struct {
	V           int    `json:"v"`
	Relay       string `json:"relay"` // UUID string
	Loc         Loc    `json:"loc"`
	StampSHA256 string `json:"stamp_sha256"` // hex-encoded
	Issued      string `json:"issued"`       // RFC3339
}

// Plate is a signed Data.
type Plate struct {
	Data Data
	Sig  []byte
}

var (
	ErrInvalidSignature = errors.New("plate: invalid signature")
	ErrInvalidJSON      = errors.New("plate: invalid JSON")
	ErrVersion          = errors.New("plate: unsupported version")
)

// Canonical returns the canonical JSON encoding of d.
func Canonical(d Data) ([]byte, error) {
	// Use json.Marshal which sorts keys for maps. For structs, the key order
	// is declaration order; we re-marshal via a map to ensure sorted keys.
	m := map[string]any{
		"v":            d.V,
		"relay":        d.Relay,
		"loc":          map[string]any{"lat": d.Loc.Lat, "lon": d.Loc.Lon, "city": d.Loc.City, "cc": d.Loc.CC},
		"stamp_sha256": d.StampSHA256,
		"issued":       d.Issued,
	}
	return canonicalJSON(m)
}

// canonicalJSON produces a deterministic JSON encoding with sorted keys and
// no whitespace. Floats are encoded with strconv.FormatFloat 'g' format.
func canonicalJSON(v any) ([]byte, error) {
	// Marshal via standard library into a generic tree, then re-emit sorted.
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var generic any
	if err := json.Unmarshal(raw, &generic); err != nil {
		return nil, err
	}
	return emitSorted(generic), nil
}

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
		// Shortest representation that round-trips
		s := json.Number(fmt.Sprintf("%v", val)).String()
		return []byte(s)
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
		// Sort keys
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

// sortStrings is a simple insertion sort (small arrays, no deps).
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// Sign creates a signed plate from data using the given Ed25519 private key.
func Sign(priv ed25519.PrivateKey, d Data) (*Plate, error) {
	if d.V != 1 {
		return nil, ErrVersion
	}
	canon, err := Canonical(d)
	if err != nil {
		return nil, fmt.Errorf("plate: canonical: %w", err)
	}
	sig := ed25519.Sign(priv, canon)
	return &Plate{Data: d, Sig: sig}, nil
}

// Verify checks that sig is a valid Ed25519 signature over the canonical form
// of d using the given public key.
func Verify(pub ed25519.PublicKey, d Data, sig []byte) error {
	canon, err := Canonical(d)
	if err != nil {
		return fmt.Errorf("plate: canonical: %w", err)
	}
	if !ed25519.Verify(pub, canon, sig) {
		return ErrInvalidSignature
	}
	return nil
}

// Hash returns sha256 of the canonical JSON form.
func Hash(d Data) ([32]byte, error) {
	canon, err := Canonical(d)
	if err != nil {
		return [32]byte{}, err
	}
	return sha256.Sum256(canon), nil
}

// HashHex returns the hex-encoded sha256 of the canonical JSON form.
func HashHex(d Data) (string, error) {
	h, err := Hash(d)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(h[:]), nil
}

// Parse parses plate JSON + signature into a Plate.
func Parse(jsonBytes, sig []byte) (*Plate, error) {
	var d Data
	if err := json.Unmarshal(jsonBytes, &d); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidJSON, err)
	}
	if d.V != 1 {
		return nil, ErrVersion
	}
	return &Plate{Data: d, Sig: sig}, nil
}

// Encode serializes the plate JSON.
func Encode(d Data) ([]byte, error) {
	return Canonical(d)
}

// Now issues a plate at the current time.
func Now(d Data) Data {
	d.Issued = time.Now().UTC().Round(time.Second).Format(time.RFC3339)
	return d
}
