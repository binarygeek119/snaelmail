package plate

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"testing"
)

func testKey(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return pub, priv
}

func sampleData() Data {
	return Data{
		V:     1,
		Relay: "00000000-0000-0000-0000-000000000001",
		Loc:   Loc{Lat: 40.7128, Lon: -74.006, City: "New York", CC: "US"},
		StampSHA256: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		Issued: "2026-11-18T12:00:00Z",
	}
}

func TestCanonicalDeterministic(t *testing.T) {
	d := sampleData()
	c1, err := Canonical(d)
	if err != nil {
		t.Fatal(err)
	}
	c2, err := Canonical(d)
	if err != nil {
		t.Fatal(err)
	}
	if string(c1) != string(c2) {
		t.Errorf("canonical not deterministic:\n  %s\n  %s", c1, c2)
	}
	// Keys should be sorted
	want := `{"issued":"2026-11-18T12:00:00Z","loc":{"cc":"US","city":"New York","lat":40.7128,"lon":-74.006},"relay":"00000000-0000-0000-0000-000000000001","stamp_sha256":"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855","v":1}`
	if string(c1) != want {
		t.Errorf("canonical:\n  got  %s\n  want %s", c1, want)
	}
}

func TestSignAndVerify(t *testing.T) {
	pub, priv := testKey(t)
	d := sampleData()

	p, err := Sign(priv, d)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Sig) != ed25519.SignatureSize {
		t.Errorf("signature length = %d, want %d", len(p.Sig), ed25519.SignatureSize)
	}
	if err := Verify(pub, d, p.Sig); err != nil {
		t.Errorf("Verify: %v", err)
	}
}

func TestVerifyRejectsBadSignature(t *testing.T) {
	pub, priv := testKey(t)
	d := sampleData()

	p, err := Sign(priv, d)
	if err != nil {
		t.Fatal(err)
	}

	// Corrupt one byte
	badSig := make([]byte, len(p.Sig))
	copy(badSig, p.Sig)
	badSig[0] ^= 0xff
	if err := Verify(pub, d, badSig); err == nil {
		t.Error("Verify should reject corrupted signature")
	}
}

func TestVerifyRejectsWrongKey(t *testing.T) {
	_, priv := testKey(t)
	pub2, _ := testKey(t)
	d := sampleData()

	p, err := Sign(priv, d)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(pub2, d, p.Sig); err == nil {
		t.Error("Verify should reject wrong public key")
	}
}

func TestVerifyRejectsTamperedData(t *testing.T) {
	pub, priv := testKey(t)
	d := sampleData()

	p, err := Sign(priv, d)
	if err != nil {
		t.Fatal(err)
	}

	// Tamper with city
	d2 := d
	d2.Loc.City = "Tampered"
	if err := Verify(pub, d2, p.Sig); err == nil {
		t.Error("Verify should reject tampered data")
	}
}

func TestHash(t *testing.T) {
	d := sampleData()
	h1, err := Hash(d)
	if err != nil {
		t.Fatal(err)
	}
	h2, err := Hash(d)
	if err != nil {
		t.Fatal(err)
	}
	if h1 != h2 {
		t.Error("hash not deterministic")
	}

	// Tamper should change hash
	d2 := d
	d2.Loc.Lat = 99.9
	h3, _ := Hash(d2)
	if h1 == h3 {
		t.Error("hash unchanged after tamper")
	}
}

func TestHashHex(t *testing.T) {
	d := sampleData()
	h, err := HashHex(d)
	if err != nil {
		t.Fatal(err)
	}
	if len(h) != 64 {
		t.Errorf("hex hash length = %d, want 64", len(h))
	}
	if _, err := hex.DecodeString(h); err != nil {
		t.Errorf("hex hash invalid: %v", err)
	}
}

func TestSignRejectsWrongVersion(t *testing.T) {
	_, priv := testKey(t)
	d := sampleData()
	d.V = 99
	_, err := Sign(priv, d)
	if err != ErrVersion {
		t.Errorf("Sign with bad version: got %v, want ErrVersion", err)
	}
}

func TestParseRoundTrip(t *testing.T) {
	_, priv := testKey(t)
	d := sampleData()

	p, err := Sign(priv, d)
	if err != nil {
		t.Fatal(err)
	}
	js, err := Encode(p.Data)
	if err != nil {
		t.Fatal(err)
	}

	parsed, err := Parse(js, p.Sig)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Data.V != d.V || parsed.Data.Relay != d.Relay || parsed.Data.Loc.City != d.Loc.City {
		t.Errorf("parse roundtrip lost data: %+v", parsed.Data)
	}
}

func TestKeyRotationSelfVerifying(t *testing.T) {
	// Simulates the key rotation flow: the new key must sign a plate
	// with the same stamp_sha256 but new issued time; the old key's
	// signature must not verify.
	pubOld, privOld := testKey(t)
	pubNew, privNew := testKey(t)

	d := sampleData()
	pOld, _ := Sign(privOld, d)

	// Old signature verifies with old key
	if err := Verify(pubOld, d, pOld.Sig); err != nil {
		t.Fatal(err)
	}
	// Old signature does NOT verify with new key
	if err := Verify(pubNew, d, pOld.Sig); err == nil {
		t.Error("old sig should not verify with new key")
	}

	// Rotate: re-sign with new key, bump issued
	d.Issued = "2026-11-19T12:00:00Z"
	pNew, _ := Sign(privNew, d)
	if err := Verify(pubNew, d, pNew.Sig); err != nil {
		t.Errorf("new sig should verify with new key: %v", err)
	}
}
