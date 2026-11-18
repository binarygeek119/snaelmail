package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"snailmail/internal/relayreg"
)

// keygenCmd generates a new Ed25519 keypair for a relay operator.
func keygenCmd(args []string) error {
	// Generate key
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("keygen: %w", err)
	}

	// Compute fingerprint
	fpr := relayreg.KeyFingerprint(pub)
	fprHex := fmt.Sprintf("%x", fpr)

	// Encode
	pubB64 := base64.StdEncoding.EncodeToString(pub)
	privHex := fmt.Sprintf("%x", priv.Seed())

	// Determine output path
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	keyDir := filepath.Join(home, ".snailmail")
	if err := os.MkdirAll(keyDir, 0700); err != nil {
		return fmt.Errorf("keygen: mkdir: %w", err)
	}

	keyPath := filepath.Join(keyDir, "relay.key")
	if err := os.WriteFile(keyPath, []byte(privHex+"\n"), 0600); err != nil {
		return fmt.Errorf("keygen: write key: %w", err)
	}

	pubPath := filepath.Join(keyDir, "relay.pub")
	if err := os.WriteFile(pubPath, []byte(pubB64+"\n"), 0644); err != nil {
		return fmt.Errorf("keygen: write pubkey: %w", err)
	}

	// Print summary
	summary := map[string]string{
		"key_file":     keyPath,
		"pubkey_file":  pubPath,
		"pubkey_b64":   pubB64,
		"fingerprint":  fprHex,
	}
	out, _ := json.MarshalIndent(summary, "", "  ")
	fmt.Println(string(out))
	return nil
}

// loginCmd simulates the operator login flow.
// In a real system, this would fetch a challenge from the site,
// sign it with the relay's private key, and POST back.
func loginCmd(args []string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("login: %w", err)
	}

	keyPath := filepath.Join(home, ".snailmail", "relay.key")
	keyHex, err := os.ReadFile(keyPath)
	if err != nil {
		return fmt.Errorf("login: read key: %w (run keygen first)", err)
	}

	seed := make([]byte, 32)
	_, err = fmt.Sscanf(string(keyHex), "%x", &seed)
	if err != nil {
		// Try manual hex decode
		for i := 0; i < 32; i++ {
			fmt.Sscanf(string(keyHex)[i*2:i*2+2], "%02x", &seed[i])
		}
	}

	priv := ed25519.NewKeyFromSeed(seed)
	pub := priv.Public().(ed25519.PublicKey)
	pubB64 := base64.StdEncoding.EncodeToString(pub)

	fmt.Printf("Relay pubkey: %s\n", pubB64)
	fmt.Printf("Fingerprint:  %x\n", relayreg.KeyFingerprint(pub))
	fmt.Println("Login not yet implemented in v1.")
	return nil
}
