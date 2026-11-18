// Package relayreg implements relay registration, hostname validation,
// and TXT record verification per §11 of the spec.
package relayreg

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"
)

// HostnameRE matches the relay hostname pattern per §2.
var HostnameRE = regexp.MustCompile(`^po-[a-z0-9]{2,24}-[a-z]{2}$`)

// TXTRE matches the TXT record format per §11.
var (
	errInvalidHostname = errors.New("relayreg: hostname must match ^po-[a-z0-9]{2,24}-[a-z]{2}$")
	errInvalidTXT      = errors.New("relayreg: TXT record format invalid")
	errVersionMismatch = errors.New("relayreg: TXT version must be 1")
	errTokenMissing    = errors.New("relayreg: token missing")
	errKeyMissing      = errors.New("relayreg: key missing")
	errKeyPrefix       = errors.New("relayreg: key must start with ed25519:")
	errKeyLength       = errors.New("relayreg: key must be 32 bytes")
	errWellKnown       = errors.New("relayreg: well-known token mismatch")
)

// ValidateHostname checks a relay hostname.
func ValidateHostname(hostname string) error {
	if !HostnameRE.MatchString(hostname) {
		return errInvalidHostname
	}
	return nil
}

// ParsedTXT is the parsed form of a relay's DNS TXT record.
type ParsedTXT struct {
	Version int
	Token   string
	Key     ed25519.PublicKey
}

// ParseTXT parses a TXT record of the form:
//
//	v=1; token=<token>; key=ed25519:<base64>
func ParseTXT(txt string) (*ParsedTXT, error) {
	p := &ParsedTXT{}
	parts := splitSemicolon(txt)

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		eq := strings.Index(part, "=")
		if eq < 0 {
			return nil, errInvalidTXT
		}
		key := strings.TrimSpace(part[:eq])
		val := strings.TrimSpace(part[eq+1:])

		switch key {
		case "v":
			if val != "1" {
				return nil, errVersionMismatch
			}
			p.Version = 1
		case "token":
			p.Token = val
		case "key":
			if !strings.HasPrefix(val, "ed25519:") {
				return nil, errKeyPrefix
			}
			b64 := strings.TrimPrefix(val, "ed25519:")
			decoded, err := base64.StdEncoding.DecodeString(b64)
			if err != nil {
				return nil, fmt.Errorf("relayreg: key base64: %w", err)
			}
			if len(decoded) != ed25519.PublicKeySize {
				return nil, errKeyLength
			}
			p.Key = ed25519.PublicKey(decoded)
		}
	}

	if p.Version != 1 {
		return nil, errVersionMismatch
	}
	if p.Token == "" {
		return nil, errTokenMissing
	}
	if p.Key == nil {
		return nil, errKeyMissing
	}
	return p, nil
}

// FormatTXT produces the canonical TXT record format from a parsed record.
func FormatTXT(p *ParsedTXT) string {
	keyB64 := base64.StdEncoding.EncodeToString(p.Key)
	return fmt.Sprintf("v=1; token=%s; key=ed25519:%s", p.Token, keyB64)
}

// DNSResolver is the interface for looking up DNS TXT records.
// Using an interface allows tests to provide a mock implementation.
type DNSResolver interface {
	LookupTXT(ctx context.Context, hostname string) ([]string, error)
}

// NetResolver uses the system DNS resolver.
type NetResolver struct{}

func (r *NetResolver) LookupTXT(ctx context.Context, hostname string) ([]string, error) {
	return net.DefaultResolver.LookupTXT(ctx, hostname)
}

// FetchRelayTXT looks up the relay's TXT record and parses it.
func FetchRelayTXT(ctx context.Context, resolver DNSResolver, hostname string) (*ParsedTXT, error) {
	records, err := resolver.LookupTXT(ctx, hostname)
	if err != nil {
		return nil, fmt.Errorf("relayreg: DNS lookup: %w", err)
	}
	for _, r := range records {
		if strings.Contains(r, "v=1") {
			return ParseTXT(r)
		}
	}
	return nil, errInvalidTXT
}

// HTTPChecker is the interface for fetching the relay's well-known page.
type HTTPChecker interface {
	GetWellKnown(ctx context.Context, hostname string) (string, error)
}

// Verify checks the full registration flow per §11:
//  1. TXT record parses and contains v=1
//  2. Token matches the unexpired challenge
//  3. Key fingerprint matches the registered key
//  4. /.well-known/snailmail-relay returns the same token
func Verify(ctx context.Context, resolver DNSResolver, http HTTPChecker, hostname string, expectedToken string, expectedKeyFpr []byte) error {
	// 1+2. Fetch and validate TXT record
	parsed, err := FetchRelayTXT(ctx, resolver, hostname)
	if err != nil {
		return err
	}

	// 2. Token match
	if parsed.Token != expectedToken {
		return fmt.Errorf("relayreg: token mismatch: got %q, want %q", parsed.Token, expectedToken)
	}

	// 3. Key fingerprint match
	keyFpr := keyFingerprint(parsed.Key)
	if !bytesEqual(keyFpr, expectedKeyFpr) {
		return fmt.Errorf("relayreg: key fingerprint mismatch")
	}

	// 4. Well-known check
	if http != nil {
		wkToken, err := http.GetWellKnown(ctx, hostname)
		if err != nil {
			return fmt.Errorf("relayreg: well-known fetch: %w", err)
		}
		if wkToken != expectedToken {
			return errWellKnown
		}
	}

	return nil
}

// KeyFingerprint returns the SHA256 fingerprint of an Ed25519 public key.
func KeyFingerprint(pub ed25519.PublicKey) []byte {
	return keyFingerprint(pub)
}

func keyFingerprint(pub ed25519.PublicKey) []byte {
	h := sha256.Sum256(pub)
	return h[:]
}

// splitSemicolon splits on semicolons but respects quoted strings.
func splitSemicolon(s string) []string {
	var parts []string
	var cur strings.Builder
	inQuote := false
	for _, r := range s {
		switch {
		case r == '"':
			inQuote = !inQuote
			cur.WriteRune(r)
		case r == ';' && !inQuote:
			parts = append(parts, cur.String())
			cur.Reset()
		default:
			cur.WriteRune(r)
		}
	}
	if cur.Len() > 0 {
		parts = append(parts, cur.String())
	}
	return parts
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
