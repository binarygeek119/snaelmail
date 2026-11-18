package auth

import "testing"

func TestValidateHandle(t *testing.T) {
	tests := []struct {
		name    string
		handle  string
		wantErr bool
	}{
		{"valid simple", "alice", false},
		{"valid with dots", "alice.bob", false},
		{"valid with dashes", "alice-bob", false},
		{"valid with underscores", "alice_bob", false},
		{"valid numbers", "alice123", false},
		{"valid min length", "abc", false},
		{"valid max length", "abcdefghijklmnopqrstuvwx", false}, // 24 chars
		{"too short", "ab", true},
		{"too long", "abcdefghijklmnopqrstuvwxy", true}, // 25 chars
		{"invalid start", "-alice", true},
		{"invalid end", "alice-", true},
		{"invalid chars", "alice!", true},
		{"consecutive dots", "alice..bob", true},
		{"uppercase", "Alice", true},
		{"reserved: admin", "admin", true},
		{"reserved: postmaster", "postmaster", true},
		{"reserved: root", "root", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateHandle(tt.handle)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateHandle(%q) error = %v, wantErr %v", tt.handle, err, tt.wantErr)
			}
		})
	}
}

func TestValidatePassword(t *testing.T) {
	common := map[string]bool{"password123": true}

	tests := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{"valid", "securepassword", false},
		{"too short", "short", true},
		{"exactly 10", "1234567890", false},
		{"common password", "password123", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePassword(tt.password, common)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePassword(%q) error = %v, wantErr %v", tt.password, err, tt.wantErr)
			}
		})
	}
}

func TestHashAndVerifyPassword(t *testing.T) {
	password := "mysecretpassword123"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	ok, err := VerifyPassword(password, hash)
	if err != nil {
		t.Fatalf("VerifyPassword: %v", err)
	}
	if !ok {
		t.Error("VerifyPassword returned false for correct password")
	}

	ok, err = VerifyPassword("wrongpassword", hash)
	if err != nil {
		t.Fatalf("VerifyPassword: %v", err)
	}
	if ok {
		t.Error("VerifyPassword returned true for wrong password")
	}
}

func TestGenerateCode(t *testing.T) {
	code1, err := GenerateCode()
	if err != nil {
		t.Fatalf("GenerateCode: %v", err)
	}
	if len(code1) != 8 {
		t.Errorf("code length = %d, want 8", len(code1))
	}

	code2, err := GenerateCode()
	if err != nil {
		t.Fatalf("GenerateCode: %v", err)
	}
	if code1 == code2 {
		t.Error("two codes should not be identical")
	}
}

func TestGenerateToken(t *testing.T) {
	token1, err := GenerateToken(32)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	if len(token1) != 64 { // 32 bytes = 64 hex chars
		t.Errorf("token length = %d, want 64", len(token1))
	}

	token2, err := GenerateToken(32)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	if token1 == token2 {
		t.Error("two tokens should not be identical")
	}
}

func TestIsReservedRelaySlug(t *testing.T) {
	tests := []struct {
		slug string
		want bool
	}{
		{"po-newyork-us", true},
		{"po-london-gb", true},
		{"po-tokyo-jp", true},
		{"alice", false},
		{"admin", false},
		{"po-", false},
		{"po-newyork", false},
	}

	for _, tt := range tests {
		t.Run(tt.slug, func(t *testing.T) {
			got := IsReservedRelaySlug(tt.slug)
			if got != tt.want {
				t.Errorf("IsReservedRelaySlug(%q) = %v, want %v", tt.slug, got, tt.want)
			}
		})
	}
}
