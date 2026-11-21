package dkim

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"os"
	"strings"
	"time"
)

// Signer handles DKIM signing of emails
type Signer struct {
	privateKey *rsa.PrivateKey
	domain     string
	selector   string
}

// NewSigner creates a new DKIM signer from a PEM file
func NewSigner(privateKeyPath, domain, selector string) (*Signer, error) {
	data, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("read private key: %w", err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		pkcs8Key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse private key: %w", err)
		}
		rsaKey, ok := pkcs8Key.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("not an RSA private key")
		}
		key = rsaKey
	}

	return &Signer{
		privateKey: key,
		domain:     domain,
		selector:   selector,
	}, nil
}

// Sign signs an email message and returns the DKIM-Signature header value
func (s *Signer) Sign(from, to, subject string, body string) (string, error) {
	// Canonicalize body
	canonBody := canonicalizeBody(body)

	// Hash body (SHA-256)
	bodyHash := sha256.Sum256([]byte(canonBody))
	bodyHashB64 := base64.StdEncoding.EncodeToString(bodyHash[:])

	// Build headers to sign
	headersToSign := fmt.Sprintf("from:%s\r\nto:%s\r\nsubject:%s\r\ndate:%s",
		strings.ToLower(from), strings.ToLower(to), subject, time.Now().UTC().Format(time.RFC1123Z))

	// DKIM signature header template (without b= value)
	sigTemplate := fmt.Sprintf("v=1; a=rsa-sha256; c=simple/simple; d=%s; s=%s; "+
		"t=%d; h=from:to:subject:date; bh=%s; b=",
		s.domain, s.selector, time.Now().Unix(), bodyHashB64)

	// Data to sign = sig header + signed headers
	dataToSign := sigTemplate + "\r\n" + headersToSign

	// Hash and sign
	hash := sha256.Sum256([]byte(dataToSign))
	signature, err := rsa.SignPKCS1v15(nil, s.privateKey, crypto.SHA256, hash[:])
	if err != nil {
		return "", fmt.Errorf("sign: %w", err)
	}

	sigB64 := base64.StdEncoding.EncodeToString(signature)

	fullSig := fmt.Sprintf("v=1; a=rsa-sha256; c=simple/simple; d=%s; s=%s; "+
		"t=%d; h=from:to:subject:date; bh=%s; b=%s",
		s.domain, s.selector, time.Now().Unix(), bodyHashB64, sigB64)

	return fullSig, nil
}

// canonicalizeBody applies simple body canonicalization
func canonicalizeBody(body string) string {
	body = strings.TrimRight(body, "\r\n")
	if body != "" {
		body += "\r\n"
	}
	return body
}

// GenerateKeyPair generates a new RSA key pair and saves to files
func GenerateKeyPair(privatePath, publicPath string, bits int) error {
	key, err := rsa.GenerateKey(nil, bits)
	if err != nil {
		return err
	}

	// Save private key
	privFile, err := os.Create(privatePath)
	if err != nil {
		return err
	}
	defer privFile.Close()
	pem.Encode(privFile, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})

	// Save public key
	pubBytes, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return err
	}
	pubFile, err := os.Create(publicPath)
	if err != nil {
		return err
	}
	defer pubFile.Close()
	pem.Encode(pubFile, &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubBytes,
	})

	return nil
}
