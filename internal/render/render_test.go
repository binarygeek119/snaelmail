package render

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"

	"snailmail/internal/stamp"
)

func testKey(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return pub, priv
}

func TestRenderVerifiedChain(t *testing.T) {
	pub, priv := testKey(t)

	hops := []stamp.HopSpec{
		{Idx: 0, RelayID: [16]byte{15: 1}, StampAt: time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC), PlateSHA: [32]byte{1}, City: "NYC", CC: "US"},
		{Idx: 1, RelayID: [16]byte{15: 2}, StampAt: time.Date(2026, 11, 18, 14, 0, 0, 0, time.UTC), PlateSHA: [32]byte{2}, City: "CHI", CC: "US"},
	}

	stamps, err := stamp.BuildChain(priv, hops)
	if err != nil {
		t.Fatal(err)
	}

	stubPNGs := [][]byte{
		stamp.MakeStubStampPNG("NYC"),
		stamp.MakeStubStampPNG("CHI"),
	}

	result, err := Render(pub, stamps, stubPNGs)
	if err != nil {
		t.Fatal(err)
	}

	if !result.AllVerified {
		t.Error("expected AllVerified=true for valid chain")
	}
	if len(result.CompositePNG) == 0 {
		t.Error("composite PNG is empty")
	}
	for i, e := range result.Errors {
		if e != nil {
			t.Errorf("hop %d has error: %v", i, e)
		}
	}
}

func TestRenderUnverifiedChainGray(t *testing.T) {
	_, priv := testKey(t)

	hops := []stamp.HopSpec{
		{Idx: 0, RelayID: [16]byte{15: 1}, StampAt: time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC), PlateSHA: [32]byte{1}, City: "NYC", CC: "US"},
	}

	stamps, _ := stamp.BuildChain(priv, hops)
	stubPNGs := [][]byte{stamp.MakeStubStampPNG("NYC")}

	// Verify with wrong key
	pub2, _ := testKey(t)
	result, err := Render(pub2, stamps, stubPNGs)
	if err != nil {
		t.Fatal(err)
	}

	if result.AllVerified {
		t.Error("expected AllVerified=false for wrong key")
	}
	// Per §1: delivery proceeds regardless — Render must not return error
	if len(result.CompositePNG) == 0 {
		t.Error("unverified chain should still produce composite")
	}
}

func TestRenderTamperedChainDoesNotBlock(t *testing.T) {
	pub, priv := testKey(t)

	hops := []stamp.HopSpec{
		{Idx: 0, RelayID: [16]byte{15: 1}, StampAt: time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC), PlateSHA: [32]byte{1}, City: "NYC", CC: "US"},
		{Idx: 1, RelayID: [16]byte{15: 2}, StampAt: time.Date(2026, 11, 18, 14, 0, 0, 0, time.UTC), PlateSHA: [32]byte{2}, City: "CHI", CC: "US"},
	}

	stamps, _ := stamp.BuildChain(priv, hops)

	// Tamper: corrupt a stamp's signature
	stamps[1].Sig[0] ^= 0xff

	stubPNGs := [][]byte{
		stamp.MakeStubStampPNG("NYC"),
		stamp.MakeStubStampPNG("CHI"),
	}

	result, err := Render(pub, stamps, stubPNGs)
	if err != nil {
		t.Fatal(err)
	}

	if result.AllVerified {
		t.Error("tampered chain should not verify")
	}
	if len(result.CompositePNG) == 0 {
		t.Error("tampered chain must still produce composite (delivery proceeds)")
	}
}

func TestRenderDeterministic(t *testing.T) {
	pub, priv := testKey(t)

	hops := []stamp.HopSpec{
		{Idx: 0, RelayID: [16]byte{15: 1}, StampAt: time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC), PlateSHA: [32]byte{1}, City: "NYC", CC: "US"},
	}

	stamps, _ := stamp.BuildChain(priv, hops)
	stubPNGs := [][]byte{stamp.MakeStubStampPNG("NYC")}

	r1, _ := Render(pub, stamps, stubPNGs)
	r2, _ := Render(pub, stamps, stubPNGs)

	if string(r1.CompositePNG) != string(r2.CompositePNG) {
		t.Error("render not deterministic")
	}
	if r1.AllVerified != r2.AllVerified {
		t.Error("verification result not deterministic")
	}
}
