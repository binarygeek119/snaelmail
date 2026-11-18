package relayreg

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"testing"
)

func TestValidateHostname(t *testing.T) {
	tests := []struct {
		hostname string
		wantErr  bool
	}{
		{"po-london-gb", false},
		{"po-newyork-us", false},
		{"po-tokyo-jp", false},
		{"po-paris-fr", false},
		{"po-berlin-de", false},
		{"po-ab-us", false}, // minimum city length
		{"po-abcdefghijklmnopqrstuvwx-us", false}, // maximum city length (24)

		// Invalid cases
		{"london-gb", true},        // missing po- prefix
		{"po-London-gb", true},     // uppercase
		{"po-london-GB", true},     // uppercase country
		{"po-london-g", true},      // country too short
		{"po-london-gbr", true},    // country too long
		{"po-london-gb-extra", true}, // extra suffix
		{"po--london-gb", true},    // empty city
		{"po-abcdefghijklmnopqrstuvwxy-us", true}, // city too long (25)
		{"", true},
		{"po-", true},
		{"po-us", true},
	}

	for _, tt := range tests {
		t.Run(tt.hostname, func(t *testing.T) {
			err := ValidateHostname(tt.hostname)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateHostname(%q) error = %v, wantErr %v", tt.hostname, err, tt.wantErr)
			}
		})
	}
}

func TestParseTXT(t *testing.T) {
	// Generate a test key
	pub, _, _ := ed25519.GenerateKey(nil)
	keyB64 := base64.StdEncoding.EncodeToString(pub)

	tests := []struct {
		name    string
		txt     string
		wantErr bool
		check   func(*ParsedTXT) error
	}{
		{
			name:    "valid TXT record",
			txt:     "v=1; token=abc123; key=ed25519:" + keyB64,
			wantErr: false,
			check: func(p *ParsedTXT) error {
				if p.Version != 1 {
					return errors.New("version mismatch")
				}
				if p.Token != "abc123" {
					return errors.New("token mismatch")
				}
				if !bytesEqual(p.Key, pub) {
					return errors.New("key mismatch")
				}
				return nil
			},
		},
		{
			name:    "extra whitespace",
			txt:     "  v=1 ;  token=xyz  ;  key=ed25519:" + keyB64 + "  ",
			wantErr: false,
			check: func(p *ParsedTXT) error {
				if p.Token != "xyz" {
					return errors.New("token not trimmed")
				}
				return nil
			},
		},
		{
			name:    "wrong version",
			txt:     "v=2; token=abc; key=ed25519:" + keyB64,
			wantErr: true,
		},
		{
			name:    "missing token",
			txt:     "v=1; key=ed25519:" + keyB64,
			wantErr: true,
		},
		{
			name:    "missing key",
			txt:     "v=1; token=abc",
			wantErr: true,
		},
		{
			name:    "wrong key prefix",
			txt:     "v=1; token=abc; key=rsa:" + keyB64,
			wantErr: true,
		},
		{
			name:    "invalid base64",
			txt:     "v=1; token=abc; key=ed25519:not-valid-base64!@#$",
			wantErr: true,
		},
		{
			name:    "key too short",
			txt:     "v=1; token=abc; key=ed25519:" + base64.StdEncoding.EncodeToString([]byte("short")),
			wantErr: true,
		},
		{
			name:    "empty string",
			txt:     "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := ParseTXT(tt.txt)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseTXT() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && tt.check != nil {
				if err := tt.check(parsed); err != nil {
					t.Errorf("check failed: %v", err)
				}
			}
		})
	}
}

func TestFormatTXT(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(nil)
	original := &ParsedTXT{
		Version: 1,
		Token:   "testtoken",
		Key:     pub,
	}

	formatted := FormatTXT(original)

	// Parse it back
	parsed, err := ParseTXT(formatted)
	if err != nil {
		t.Fatalf("FormatTXT produced invalid output: %v", err)
	}

	if parsed.Version != original.Version {
		t.Errorf("version mismatch: got %d, want %d", parsed.Version, original.Version)
	}
	if parsed.Token != original.Token {
		t.Errorf("token mismatch: got %q, want %q", parsed.Token, original.Token)
	}
	if !bytesEqual(parsed.Key, original.Key) {
		t.Error("key mismatch after round-trip")
	}
}

// MockDNSResolver for testing
type MockDNSResolver struct {
	records map[string][]string
	err     error
}

func (m *MockDNSResolver) LookupTXT(ctx context.Context, hostname string) ([]string, error) {
	if m.err != nil {
		return nil, m.err
	}
	if records, ok := m.records[hostname]; ok {
		return records, nil
	}
	return nil, errors.New("no records")
}

// MockHTTPChecker for testing
type MockHTTPChecker struct {
	token string
	err   error
}

func (m *MockHTTPChecker) GetWellKnown(ctx context.Context, hostname string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.token, nil
}

func TestFetchRelayTXT(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(nil)
	keyB64 := base64.StdEncoding.EncodeToString(pub)

	mock := &MockDNSResolver{
		records: map[string][]string{
			"po-london-gb.snailmail.test": {"v=1; token=abc123; key=ed25519:" + keyB64},
		},
	}

	parsed, err := FetchRelayTXT(context.Background(), mock, "po-london-gb.snailmail.test")
	if err != nil {
		t.Fatalf("FetchRelayTXT failed: %v", err)
	}

	if parsed.Token != "abc123" {
		t.Errorf("token mismatch: got %q, want %q", parsed.Token, "abc123")
	}
	if !bytesEqual(parsed.Key, pub) {
		t.Error("key mismatch")
	}
}

func TestVerify(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(nil)
	keyB64 := base64.StdEncoding.EncodeToString(pub)
	keyFpr := KeyFingerprint(pub)

	t.Run("successful verification", func(t *testing.T) {
		dnsMock := &MockDNSResolver{
			records: map[string][]string{
				"po-london-gb.test": {"v=1; token=challenge123; key=ed25519:" + keyB64},
			},
		}
		httpMock := &MockHTTPChecker{token: "challenge123"}

		err := Verify(context.Background(), dnsMock, httpMock, "po-london-gb.test", "challenge123", keyFpr)
		if err != nil {
			t.Errorf("Verify failed: %v", err)
		}
	})

	t.Run("token mismatch in TXT", func(t *testing.T) {
		dnsMock := &MockDNSResolver{
			records: map[string][]string{
				"po-london-gb.test": {"v=1; token=wrongtoken; key=ed25519:" + keyB64},
			},
		}

		err := Verify(context.Background(), dnsMock, nil, "po-london-gb.test", "challenge123", keyFpr)
		if err == nil {
			t.Error("expected error for token mismatch")
		}
	})

	t.Run("key fingerprint mismatch", func(t *testing.T) {
		otherPub, _, _ := ed25519.GenerateKey(nil)
		otherB64 := base64.StdEncoding.EncodeToString(otherPub)

		dnsMock := &MockDNSResolver{
			records: map[string][]string{
				"po-london-gb.test": {"v=1; token=challenge123; key=ed25519:" + otherB64},
			},
		}

		err := Verify(context.Background(), dnsMock, nil, "po-london-gb.test", "challenge123", keyFpr)
		if err == nil {
			t.Error("expected error for key mismatch")
		}
	})

	t.Run("well-known token mismatch", func(t *testing.T) {
		dnsMock := &MockDNSResolver{
			records: map[string][]string{
				"po-london-gb.test": {"v=1; token=challenge123; key=ed25519:" + keyB64},
			},
		}
		httpMock := &MockHTTPChecker{token: "wrongtoken"}

		err := Verify(context.Background(), dnsMock, httpMock, "po-london-gb.test", "challenge123", keyFpr)
		if err == nil {
			t.Error("expected error for well-known mismatch")
		}
	})

	t.Run("DNS lookup failure", func(t *testing.T) {
		dnsMock := &MockDNSResolver{err: errors.New("DNS timeout")}

		err := Verify(context.Background(), dnsMock, nil, "po-london-gb.test", "challenge123", keyFpr)
		if err == nil {
			t.Error("expected error for DNS failure")
		}
	})

	t.Run("no TXT records", func(t *testing.T) {
		dnsMock := &MockDNSResolver{
			records: map[string][]string{
				"po-london-gb.test": {},
			},
		}

		err := Verify(context.Background(), dnsMock, nil, "po-london-gb.test", "challenge123", keyFpr)
		if err == nil {
			t.Error("expected error for no TXT records")
		}
	})
}

func TestKeyFingerprint(t *testing.T) {
	pub1, _, _ := ed25519.GenerateKey(nil)
	pub2, _, _ := ed25519.GenerateKey(nil)

	fpr1 := KeyFingerprint(pub1)
	fpr2 := KeyFingerprint(pub2)

	if len(fpr1) != 32 {
		t.Errorf("fingerprint length = %d, want 32", len(fpr1))
	}

	if bytesEqual(fpr1, fpr2) {
		t.Error("different keys should have different fingerprints")
	}

	// Same key should produce same fingerprint
	fpr1Again := KeyFingerprint(pub1)
	if !bytesEqual(fpr1, fpr1Again) {
		t.Error("same key should produce same fingerprint")
	}
}
