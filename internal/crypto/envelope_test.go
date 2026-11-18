package crypto

import "testing"

func TestGenerateKeyPair(t *testing.T) {
	pub, priv, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	if len(pub) != KeySize {
		t.Errorf("pub key length = %d, want %d", len(pub), KeySize)
	}
	if len(priv) != KeySize {
		t.Errorf("priv key length = %d, want %d", len(priv), KeySize)
	}
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	senderPub, senderPriv, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("sender keypair: %v", err)
	}
	_ = senderPub // not used in encrypt

	recipientPub, recipientPriv, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("recipient keypair: %v", err)
	}

	plaintext := []byte("Hello, this is a test message for envelope encryption.")

	ciphertext, err := Encrypt(plaintext, recipientPub, senderPriv)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	// Ciphertext should be larger than plaintext (ephPub + nonce + tag)
	expectedMinLen := KeySize + NonceSize + 16 // 16 byte tag
	if len(ciphertext) < expectedMinLen {
		t.Errorf("ciphertext length = %d, want >= %d", len(ciphertext), expectedMinLen)
	}

	decrypted, err := Decrypt(ciphertext, recipientPriv)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if string(decrypted) != string(plaintext) {
		t.Errorf("decrypted = %q, want %q", decrypted, plaintext)
	}
}

func TestDecryptWrongKey(t *testing.T) {
	_, senderPriv, _ := GenerateKeyPair()
	recipientPub, recipientPriv, _ := GenerateKeyPair()
	wrongPub, wrongPriv, _ := GenerateKeyPair()
	_ = wrongPub
	_ = wrongPriv

	plaintext := []byte("secret message")
	ciphertext, _ := Encrypt(plaintext, recipientPub, senderPriv)

	// Try to decrypt with wrong key
	_, err := Decrypt(ciphertext, wrongPriv)
	if err == nil {
		t.Error("Decrypt should fail with wrong key")
	}
	if err != ErrDecrypt {
		t.Errorf("Decrypt error = %v, want ErrDecrypt", err)
	}

	// Correct key should work
	decrypted, err := Decrypt(ciphertext, recipientPriv)
	if err != nil {
		t.Fatalf("Decrypt with correct key: %v", err)
	}
	if string(decrypted) != string(plaintext) {
		t.Errorf("decrypted = %q, want %q", decrypted, plaintext)
	}
}

func TestEncryptInvalidKeyLength(t *testing.T) {
	_, senderPriv, _ := GenerateKeyPair()
	shortPub := make([]byte, 16) // wrong length

	_, err := Encrypt([]byte("test"), shortPub, senderPriv)
	if err != ErrInvalidKey {
		t.Errorf("Encrypt with short pub key: got %v, want ErrInvalidKey", err)
	}

	shortPriv := make([]byte, 16)
	recipientPub, _, _ := GenerateKeyPair()
	_, err = Encrypt([]byte("test"), recipientPub, shortPriv)
	if err != ErrInvalidKey {
		t.Errorf("Encrypt with short priv key: got %v, want ErrInvalidKey", err)
	}
}

func TestDecryptInvalidCiphertext(t *testing.T) {
	_, recipientPriv, _ := GenerateKeyPair()

	// Too short
	_, err := Decrypt([]byte("short"), recipientPriv)
	if err != ErrInvalidCtxt {
		t.Errorf("Decrypt with short ciphertext: got %v, want ErrInvalidCtxt", err)
	}

	// Invalid key length
	_, err = Decrypt(make([]byte, 100), make([]byte, 16))
	if err != ErrInvalidKey {
		t.Errorf("Decrypt with short key: got %v, want ErrInvalidKey", err)
	}
}

func TestEncryptDeterministicNonce(t *testing.T) {
	// Nonce should be random, so two encryptions should differ
	_, senderPriv, _ := GenerateKeyPair()
	recipientPub, _, _ := GenerateKeyPair()

	plaintext := []byte("same message")
	ct1, _ := Encrypt(plaintext, recipientPub, senderPriv)
	ct2, _ := Encrypt(plaintext, recipientPub, senderPriv)

	if string(ct1) == string(ct2) {
		t.Error("two encryptions should not be identical (nonce should be random)")
	}
}

func TestDecryptTamperedCiphertext(t *testing.T) {
	_, senderPriv, _ := GenerateKeyPair()
	recipientPub, recipientPriv, _ := GenerateKeyPair()

	plaintext := []byte("important message")
	ciphertext, _ := Encrypt(plaintext, recipientPub, senderPriv)

	// Tamper with ciphertext
	tampered := make([]byte, len(ciphertext))
	copy(tampered, ciphertext)
	tampered[len(tampered)-1] ^= 0xff

	_, err := Decrypt(tampered, recipientPriv)
	if err == nil {
		t.Error("Decrypt should fail with tampered ciphertext")
	}
}

func TestEncryptEmptyPlaintext(t *testing.T) {
	_, senderPriv, _ := GenerateKeyPair()
	recipientPub, recipientPriv, _ := GenerateKeyPair()

	ciphertext, err := Encrypt([]byte{}, recipientPub, senderPriv)
	if err != nil {
		t.Fatalf("Encrypt empty plaintext: %v", err)
	}

	decrypted, err := Decrypt(ciphertext, recipientPriv)
	if err != nil {
		t.Fatalf("Decrypt empty plaintext: %v", err)
	}
	if len(decrypted) != 0 {
		t.Errorf("decrypted length = %d, want 0", len(decrypted))
	}
}
