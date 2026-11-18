package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SetupMainPostOffice creates the main site as an official relay
func SetupMainPostOffice() error {
	// Connect to database
	connString := "postgres://snailmail:snailmail_secure_password_change_me@localhost:5432/snailmail"
	ctx := context.Background()
	
	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer pool.Close()

	// Generate Ed25519 keypair for the main post office
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("generate key: %w", err)
	}

	// Save private key
	privKeyPath := "/opt/snailmail/main_post_office.key"
	privKeyBytes := priv.Seed()
	privKeyHex := hex.EncodeToString(privKeyBytes)
	if err := os.WriteFile(privKeyPath, []byte(privKeyHex), 0600); err != nil {
		return fmt.Errorf("save private key: %w", err)
	}

	// Create stamp SVG (placeholder for now)
	stampSVG := `<svg xmlns="http://www.w3.org/2000/svg" width="100" height="100">
		<circle cx="50" cy="50" r="45" fill="#667eea" stroke="#764ba2" stroke-width="3"/>
		<text x="50" y="55" font-size="16" fill="white" text-anchor="middle" font-family="Arial">SNAEL</text>
	</svg>`

	relayID := uuid.New()
	hostname := "main.snaelmail.org"

	// For the main site, we use fixed coordinates
	// Default: New York City area
	lat := 40.7128
	lon := -74.0060
	city := "New York"
	region := "NY"
	cc := "US"
	tz := "America/New_York"

	// Calculate key fingerprint
	keyFpr := sha256.Sum256(pub)
	
	// For stamp_png and stamp_sha256, we'll use placeholders
	// In production, you'd render the SVG to PNG
	stampPNG := []byte{} // placeholder
	stampSHA256 := sha256.Sum256([]byte(stampSVG))

	// Insert relay directly into database
	_, err = pool.Exec(ctx, `
		INSERT INTO relays (
			id, hostname, pubkey, key_fpr, city, region, cc, lat, lon, tz,
			stamp_svg, stamp_png, stamp_sha256, plate_json, plate_sig,
			status, official, paused, opened_at, letters_passed, complaints_30d
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
			$11, $12, $13, $14, $15,
			$16, $17, $18, $19, $20, $21
		)
	`,
		relayID,
		hostname,
		[]byte(pub),
		keyFpr[:],
		city,
		region,
		cc,
		lat,
		lon,
		tz,
		stampSVG,
		stampPNG,
		stampSHA256[:],
		nil, // plate_json
		nil, // plate_sig
		"active",
		true, // official
		false, // not paused
		"now()", // opened_at
		0, // letters_passed
		0, // complaints_30d
	)

	if err != nil {
		return fmt.Errorf("insert relay: %w", err)
	}

	fmt.Printf("✓ Main post office created\n")
	fmt.Printf("  Relay ID: %s\n", relayID)
	fmt.Printf("  Hostname: %s\n", hostname)
	fmt.Printf("  Location: %s, %s, %s\n", city, region, cc)
	fmt.Printf("  Private key: %s\n", privKeyPath)

	return nil
}

func main() {
	if err := SetupMainPostOffice(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
