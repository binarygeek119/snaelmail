// Package crypto implements envelope encryption using X25519 + XChaCha20-Poly1305 per §10.
package crypto

import (
	"crypto/rand"
	"errors"
	"io"

	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/curve25519"
)

var (
	ErrDecrypt     = errors.New("crypto: decryption failed")
	ErrInvalidKey  = errors.New("crypto: invalid key length")
	ErrInvalidCtxt = errors.New("crypto: ciphertext too short")
)

// KeySize is the size of X25519 keys (32 bytes).
const KeySize = 32

// NonceSize is the size of XChaCha20-Poly1305 nonces (24 bytes).
const NonceSize = chacha20poly1305.NonceSizeX

// GenerateKeyPair generates an X25519 key pair.
func GenerateKeyPair() (publicKey, privateKey []byte, err error) {
	privateKey = make([]byte, KeySize)
	if _, err := rand.Read(privateKey); err != nil {
		return nil, nil, err
	}
	publicKey, err = curve25519.X25519(privateKey, curve25519.Basepoint)
	if err != nil {
		return nil, nil, err
	}
	return publicKey, privateKey, nil
}

// Encrypt encrypts plaintext using ephemeral X25519 + XChaCha20-Poly1305.
//
// Format: ephemeral_pub (32 bytes) || nonce (24 bytes) || ciphertext+tag
//
// The recipient's public key is used to derive a shared secret via X25519.
func Encrypt(plaintext, recipientPub, senderPriv []byte) ([]byte, error) {
	if len(recipientPub) != KeySize {
		return nil, ErrInvalidKey
	}
	if len(senderPriv) != KeySize {
		return nil, ErrInvalidKey
	}

	// Derive shared secret
	shared, err := curve25519.X25519(senderPriv, recipientPub)
	if err != nil {
		return nil, err
	}

	// Generate ephemeral public key for recipient to derive same shared secret
	ephPub, err := curve25519.X25519(senderPriv, curve25519.Basepoint)
	if err != nil {
		return nil, err
	}

	// Generate nonce
	nonce := make([]byte, NonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	// Encrypt with XChaCha20-Poly1305
	aead, err := chacha20poly1305.NewX(shared)
	if err != nil {
		return nil, err
	}
	ciphertext := aead.Seal(nil, nonce, plaintext, nil)

	// Output: ephemeral_pub || nonce || ciphertext
	out := make([]byte, 0, KeySize+NonceSize+len(ciphertext))
	out = append(out, ephPub...)
	out = append(out, nonce...)
	out = append(out, ciphertext...)
	return out, nil
}

// Decrypt decrypts ciphertext produced by Encrypt.
//
// Input format: ephemeral_pub (32 bytes) || nonce (24 bytes) || ciphertext+tag
func Decrypt(ciphertext, recipientPriv []byte) ([]byte, error) {
	if len(recipientPriv) != KeySize {
		return nil, ErrInvalidKey
	}
	if len(ciphertext) < KeySize+NonceSize+chacha20poly1305.Overhead {
		return nil, ErrInvalidCtxt
	}

	ephPub := ciphertext[:KeySize]
	nonce := ciphertext[KeySize : KeySize+NonceSize]
	ct := ciphertext[KeySize+NonceSize:]

	// Derive shared secret
	shared, err := curve25519.X25519(recipientPriv, ephPub)
	if err != nil {
		return nil, err
	}

	// Decrypt
	aead, err := chacha20poly1305.NewX(shared)
	if err != nil {
		return nil, err
	}
	plaintext, err := aead.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, ErrDecrypt
	}
	return plaintext, nil
}
