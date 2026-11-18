// Package auth implements handle validation, password hashing, and session
// token generation per §6 of the spec.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

// §6 handle regex: ^[a-z0-9](?:[a-z0-9._-]{1,22})[a-z0-9]$
var handleRe = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9._-]{1,22})[a-z0-9]$`)

// §6 reserved handles (lowercased).
var reservedHandles = map[string]bool{
	"postmaster": true, "admin": true, "root": true, "abuse": true,
	"noreply": true, "mailer-daemon": true, "hostmaster": true,
	"webmaster": true, "security": true, "support": true, "billing": true,
	"dmarc": true, "plus": true,
}

var (
	ErrHandleTooShort       = errors.New("auth: handle must be 3-24 characters")
	ErrHandleTooLong        = errors.New("auth: handle must be 3-24 characters")
	ErrHandleInvalidChars   = errors.New("auth: handle may only contain [a-z0-9._-]")
	ErrHandleStartEnd       = errors.New("auth: handle must start and end with [a-z0-9]")
	ErrHandleConsecutiveDot = errors.New("auth: handle may not contain consecutive dots")
	ErrHandleReserved       = errors.New("auth: handle is reserved")
	ErrPasswordTooShort     = errors.New("auth: password must be at least 10 characters")
	ErrPasswordCommon       = errors.New("auth: password is too common")
	ErrEmailInvalid         = errors.New("auth: invalid email address")
	ErrEmailDomainNotAllowed = errors.New("auth: email domain not supported")
)

// Allowed email domains for signup
var allowedEmailDomains = map[string]bool{
	"gmail.com":    true,
	"googlemail.com": true,
	"outlook.com":  true,
	"hotmail.com":  true,
	"live.com":     true,
	"yahoo.com":    true,
	"aol.com":      true,
}

// ValidateHandle checks a handle against §6 rules.
func ValidateHandle(handle string) error {
	if utf8.RuneCountInString(handle) < 3 {
		return ErrHandleTooShort
	}
	if utf8.RuneCountInString(handle) > 24 {
		return ErrHandleTooLong
	}
	if !handleRe.MatchString(handle) {
		if !regexp.MustCompile(`^[a-z0-9._-]+$`).MatchString(handle) {
			return ErrHandleInvalidChars
		}
		return ErrHandleStartEnd
	}
	if strings.Contains(handle, "..") {
		return ErrHandleConsecutiveDot
	}
	if reservedHandles[handle] {
		return ErrHandleReserved
	}
	return nil
}

// IsReservedRelaySlug checks if a handle matches any relay hostname slug.
// This is checked at the API layer with a DB query; we expose a helper here.
func IsReservedRelaySlug(slug string) bool {
	// Match the relay hostname pattern: ^po-[a-z0-9]{2,24}-[a-z]{2}$
	matched, _ := regexp.MatchString(`^po-[a-z0-9]{2,24}-[a-z]{2}$`, slug)
	return matched
}

// ValidatePassword checks §6 password rules.
func ValidatePassword(password string, commonPasswords map[string]bool) error {
	if len(password) < 10 {
		return ErrPasswordTooShort
	}
	if commonPasswords != nil {
		if commonPasswords[password] {
			return ErrPasswordCommon
		}
	}
	return nil
}

// Argon2id parameters per §6: m=64MB, t=3, p=4.
const (
	argon2Time    = 3
	argon2Memory  = 64 * 1024 // 64 MB in KB
	argon2Threads = 4
	argon2KeyLen  = 32
	saltLen       = 16
)

// HashPassword produces an argon2id hash.
//
// Format: $argon2id$v=19$m=65536,t=3,p=4$<salt_hex>$<hash_hex>
func HashPassword(password string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("auth: salt: %w", err)
	}
	hash := argon2.IDKey([]byte(password), salt, argon2Time, argon2Memory, argon2Threads, argon2KeyLen)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		argon2Memory, argon2Time, argon2Threads,
		hex.EncodeToString(salt),
		hex.EncodeToString(hash),
	), nil
}

// VerifyPassword checks a password against an argon2id hash.
func VerifyPassword(password, hashStr string) (bool, error) {
	// Format: $argon2id$v=19$m=65536,t=3,p=4$<salt_hex>$<hash_hex>
	// Split by $ and validate structure
	parts := strings.Split(hashStr, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return false, fmt.Errorf("auth: malformed hash")
	}

	var version int
	var memory uint32
	var time uint32
	var threads uint32
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return false, fmt.Errorf("auth: malformed version: %w", err)
	}
	if version != 19 {
		return false, fmt.Errorf("auth: unsupported argon2 version %d", version)
	}

	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time, &threads); err != nil {
		return false, fmt.Errorf("auth: malformed params: %w", err)
	}

	salt, err := hex.DecodeString(parts[4])
	if err != nil {
		return false, fmt.Errorf("auth: bad salt hex: %w", err)
	}
	expected, err := hex.DecodeString(parts[5])
	if err != nil {
		return false, fmt.Errorf("auth: bad hash hex: %w", err)
	}

	derived := argon2.IDKey([]byte(password), salt, time, memory, uint8(threads), uint32(len(expected)))
	return subtle.ConstantTimeCompare(derived, expected) == 1, nil
}

// GenerateToken produces a cryptographically random hex token of n bytes.
func GenerateToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// GenerateCode produces an 8-character alphanumeric verification code.
func GenerateCode() (string, error) {
	const chars = "abcdefghjkmnpqrstuvwxyz23456789" // no ambiguous chars
	b := make([]byte, 8)
	for i := range b {
		var buf [1]byte
		if _, err := rand.Read(buf[:]); err != nil {
			return "", err
		}
		b[i] = chars[int(buf[0])%len(chars)]
	}
	return string(b), nil
}

// ValidateEmail checks if an email address is valid and from an allowed domain.
func ValidateEmail(email string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	
	// Basic email format validation
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.MatchString(email) {
		return ErrEmailInvalid
	}
	
	// Extract domain
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return ErrEmailInvalid
	}
	domain := parts[1]
	
	// Check if domain is allowed
	if !allowedEmailDomains[domain] {
		return ErrEmailDomainNotAllowed
	}
	
	return nil
}

// GetEmailDomain extracts the domain from an email address.
func GetEmailDomain(email string) string {
	email = strings.ToLower(strings.TrimSpace(email))
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return ""
	}
	return parts[1]
}

// GetAllowedDomains returns the list of allowed email domains.
func GetAllowedDomains() []string {
	domains := make([]string, 0, len(allowedEmailDomains))
	for domain := range allowedEmailDomains {
		domains = append(domains, domain)
	}
	return domains
}
