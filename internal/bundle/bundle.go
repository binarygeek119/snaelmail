// Package bundle handles page bundling: manifest.json + images + signature.
package bundle

import (
	"archive/zip"
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"

	"snailmail/internal/page"
)

// Bundle represents a page bundle.
type Bundle struct {
	RelayID  string
	Manifest page.Manifest
	Images   map[string][]byte // filename -> bytes
	Logo     []byte
	SiteKey  ed25519.PrivateKey
}

// Create creates a new bundle from a manifest.
func Create(relayID string, m page.Manifest, siteKey ed25519.PrivateKey) (*Bundle, error) {
	b := &Bundle{
		RelayID:  relayID,
		Manifest: m,
		Images:   make(map[string][]byte),
		SiteKey:  siteKey,
	}
	return b, nil
}

// AddImage adds an image to the bundle.
func (b *Bundle) AddImage(filename string, data []byte) error {
	// Validate it's a valid image
	_, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("invalid image: %w", err)
	}
	b.Images[filename] = data
	return nil
}

// SetLogo sets the logo (PNG or SVG).
func (b *Bundle) SetLogo(data []byte, filename string) error {
	if strings.HasSuffix(filename, ".svg") {
		// SVG: just store as-is
		b.Logo = data
	} else {
		// PNG: validate
		_, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			return fmt.Errorf("invalid logo PNG: %w", err)
		}
		b.Logo = data
	}
	return nil
}

// Sign signs the manifest and returns the signature.
func (b *Bundle) Sign() ([]byte, error) {
	data, err := json.Marshal(b.Manifest)
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(data)
	sig := ed25519.Sign(b.SiteKey, hash[:])
	return sig, nil
}

// Export writes the bundle to a zip file.
func (b *Bundle) Export(w io.Writer) error {
	sig, err := b.Sign()
	if err != nil {
		return err
	}

	zw := zip.NewWriter(w)
	defer zw.Close()

	// Write manifest.json
	manifestJSON, err := json.MarshalIndent(b.Manifest, "", "  ")
	if err != nil {
		return err
	}
	f, err := zw.Create("manifest.json")
	if err != nil {
		return err
	}
	if _, err := f.Write(manifestJSON); err != nil {
		return err
	}

	// Write signature
	f, err = zw.Create("signature")
	if err != nil {
		return err
	}
	if _, err := f.Write(sig); err != nil {
		return err
	}

	// Write images
	for filename, data := range b.Images {
		f, err := zw.Create(filepath.Join("images", filename))
		if err != nil {
			return err
		}
		if _, err := f.Write(data); err != nil {
			return err
		}
	}

	// Write logo
	if b.Logo != nil {
		ext := ".png"
		if bytes.HasPrefix(b.Logo, []byte("<svg")) || bytes.HasPrefix(b.Logo, []byte("<?xml")) {
			ext = ".svg"
		}
		f, err := zw.Create("logo" + ext)
		if err != nil {
			return err
		}
		if _, err := f.Write(b.Logo); err != nil {
			return err
		}
	}

	return nil
}

// Import reads a bundle from a zip file.
func Import(r io.ReaderAt, size int64) (*Bundle, []byte, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, nil, err
	}

	b := &Bundle{
		Images: make(map[string][]byte),
	}

	var manifestData []byte
	var sigData []byte

	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			return nil, nil, err
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, nil, err
		}

		switch f.Name {
		case "manifest.json":
			manifestData = data
		case "signature":
			sigData = data
		case "logo.png", "logo.svg":
			b.Logo = data
		default:
			if strings.HasPrefix(f.Name, "images/") {
				filename := strings.TrimPrefix(f.Name, "images/")
				b.Images[filename] = data
			}
		}
	}

	if manifestData == nil {
		return nil, nil, fmt.Errorf("manifest.json not found")
	}
	if sigData == nil {
		return nil, nil, fmt.Errorf("signature not found")
	}

	if err := json.Unmarshal(manifestData, &b.Manifest); err != nil {
		return nil, nil, err
	}

	return b, sigData, nil
}

// VerifySignature verifies the bundle signature.
func VerifySignature(m page.Manifest, sig []byte, pubKey ed25519.PublicKey) error {
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	hash := sha256.Sum256(data)
	if !ed25519.Verify(pubKey, hash[:], sig) {
		return fmt.Errorf("invalid signature")
	}
	return nil
}

// ReencodeImages re-encodes images to WebP at two sizes: 1600x1000 and 800x500.
// For now, we'll just resize PNGs since WebP encoding requires external libs.
func (b *Bundle) ReencodeImages() error {
	for filename, data := range b.Images {
		img, _, err := image.Decode(bytes.NewReader(data))
		if err != nil {
			return fmt.Errorf("decode %s: %w", filename, err)
		}

		// Create two sizes
		large := resizeImage(img, 1600, 1000)
		small := resizeImage(img, 800, 500)

		// Encode as PNG (WebP requires external libs)
		var buf bytes.Buffer
		if err := png.Encode(&buf, large); err != nil {
			return err
		}
		b.Images[filename] = buf.Bytes()

		// Store small version with -small suffix
		base := strings.TrimSuffix(filename, filepath.Ext(filename))
		ext := filepath.Ext(filename)
		smallName := base + "-small" + ext

		buf.Reset()
		if err := png.Encode(&buf, small); err != nil {
			return err
		}
		b.Images[smallName] = buf.Bytes()
	}
	return nil
}

// resizeImage resizes an image to fit within maxW x maxH while maintaining aspect ratio.
func resizeImage(img image.Image, maxW, maxH int) image.Image {
	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()

	// Calculate scale to fit within maxW x maxH
	scale := 1.0
	if w > maxW {
		scale = float64(maxW) / float64(w)
	}
	if float64(h)*scale > float64(maxH) {
		scale = float64(maxH) / float64(h)
	}

	newW := int(float64(w) * scale)
	newH := int(float64(h) * scale)

	// Simple nearest-neighbor resize
	dst := image.NewRGBA(image.Rect(0, 0, newW, newH))
	for y := 0; y < newH; y++ {
		for x := 0; x < newW; x++ {
			srcX := int(float64(x) / scale)
			srcY := int(float64(y) / scale)
			dst.Set(x, y, img.At(srcX, srcY))
		}
	}
	return dst
}

// LoadJPEG loads a JPEG file (helper for testing).
func LoadJPEG(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

// EncodeJPEG encodes an image as JPEG (helper for testing).
func EncodeJPEG(img image.Image, quality int) ([]byte, error) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
