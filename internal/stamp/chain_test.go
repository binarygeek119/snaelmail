package stamp

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"testing"
	"time"
)

func testKey(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return pub, priv
}

func plateSHA(seed byte) [32]byte {
	data := []byte{seed}
	return sha256.Sum256(data)
}

func sampleHops() []HopSpec {
	base := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
	var r1, r2, r3 [16]byte
	r1[15] = 1
	r2[15] = 2
	r3[15] = 3
	return []HopSpec{
		{Idx: 0, RelayID: r1, StampAt: base, PlateSHA: plateSHA(1), City: "New York", CC: "US"},
		{Idx: 1, RelayID: r2, StampAt: base.Add(2 * time.Hour), PlateSHA: plateSHA(2), City: "Chicago", CC: "US"},
		{Idx: 2, RelayID: r3, StampAt: base.Add(5 * time.Hour), PlateSHA: plateSHA(3), City: "Montreal", CC: "CA"},
	}
}

func TestBuildAndVerifyChain(t *testing.T) {
	pub, priv := testKey(t)
	hops := sampleHops()

	stamps, err := BuildChain(priv, hops)
	if err != nil {
		t.Fatal(err)
	}
	if len(stamps) != 3 {
		t.Fatalf("got %d stamps, want 3", len(stamps))
	}

	if err := VerifyChain(pub, stamps); err != nil {
		t.Errorf("VerifyChain: %v", err)
	}
}

func TestChainDeterministic(t *testing.T) {
	_, priv := testKey(t)
	hops := sampleHops()

	s1, err := BuildChain(priv, hops)
	if err != nil {
		t.Fatal(err)
	}
	s2, err := BuildChain(priv, hops)
	if err != nil {
		t.Fatal(err)
	}

	if len(s1) != len(s2) {
		t.Fatalf("length mismatch: %d vs %d", len(s1), len(s2))
	}

	for i := range s1 {
		enc1, _ := s1[i].Encode()
		enc2, _ := s2[i].Encode()
		if string(enc1) != string(enc2) {
			t.Errorf("stamp %d not deterministic:\n  %s\n  %s", i, enc1, enc2)
		}
		if s1[i].PrevHash != s2[i].PrevHash {
			t.Errorf("stamp %d prev_hash differs", i)
		}
		if string(s1[i].Sig) != string(s2[i].Sig) {
			t.Errorf("stamp %d signature differs", i)
		}
	}
}

func TestChainTwoRunsIdentical(t *testing.T) {
	// The spec requires identical chain across two runs
	pub, priv := testKey(t)
	hops := sampleHops()

	stamps1, _ := BuildChain(priv, hops)
	stamps2, _ := BuildChain(priv, hops)

	// Verify both chains against each other's output
	if err := VerifyChain(pub, stamps1); err != nil {
		t.Errorf("chain 1: %v", err)
	}
	if err := VerifyChain(pub, stamps2); err != nil {
		t.Errorf("chain 2: %v", err)
	}

	// And byte-identical
	for i := range stamps1 {
		e1, _ := stamps1[i].Encode()
		e2, _ := stamps2[i].Encode()
		if string(e1) != string(e2) {
			t.Errorf("run 1 vs run 2: stamp %d differs", i)
		}
	}
}

func TestChainRejectsTamperedTimestamp(t *testing.T) {
	pub, priv := testKey(t)
	hops := sampleHops()

	stamps, _ := BuildChain(priv, hops)

	// Tamper: change a middle stamp's timestamp
	stamps[1].StampAt = stamps[1].StampAt.Add(time.Hour)

	if err := VerifyChain(pub, stamps); err == nil {
		t.Error("should reject tampered timestamp")
	}
}

func TestChainRejectsReorderedStamps(t *testing.T) {
	pub, priv := testKey(t)
	hops := sampleHops()

	stamps, _ := BuildChain(priv, hops)

	// Swap stamps 0 and 1 (keeping their internal data)
	stamps[0], stamps[1] = stamps[1], stamps[0]

	if err := VerifyChain(pub, stamps); err == nil {
		t.Error("should reject reordered stamps")
	}
}

func TestChainRejectsDroppedStamp(t *testing.T) {
	pub, priv := testKey(t)
	hops := sampleHops()

	stamps, _ := BuildChain(priv, hops)

	// Drop the middle stamp
	stamps = []*Stamp{stamps[0], stamps[2]}

	if err := VerifyChain(pub, stamps); err == nil {
		t.Error("should reject dropped stamp")
	}
}

func TestChainRejectsInsertedStamp(t *testing.T) {
	pub, priv := testKey(t)
	hops := sampleHops()

	stamps, _ := BuildChain(priv, hops)

	// Insert a bogus stamp in the middle
	var r4 [16]byte
	r4[15] = 99
	bogus := &Stamp{
		Idx:      1,
		RelayID:  r4,
		StampAt:  stamps[0].StampAt.Add(time.Minute),
		PlateSHA: plateSHA(99),
		City:     "Fake",
		CC:       "XX",
		PrevHash: stamps[0].PrevHash, // wrong linkage
		Sig:      ed25519.Sign(priv, stamps[0].PrevHash[:]),
	}

	stamps = []*Stamp{stamps[0], bogus, stamps[2]}

	if err := VerifyChain(pub, stamps); err == nil {
		t.Error("should reject inserted stamp")
	}
}

func TestChainRejectsWrongKey(t *testing.T) {
	_, priv := testKey(t)
	pub2, _ := testKey(t)
	hops := sampleHops()

	stamps, _ := BuildChain(priv, hops)

	if err := VerifyChain(pub2, stamps); err == nil {
		t.Error("should reject wrong public key")
	}
}

func TestChainRejectsBadIndex(t *testing.T) {
	pub, priv := testKey(t)
	hops := sampleHops()

	stamps, _ := BuildChain(priv, hops)

	// Change idx without re-signing (signature covers prev_hash, not idx directly,
	// but VerifyChain checks idx matches position)
	stamps[1].Idx = 99

	if err := VerifyChain(pub, stamps); err == nil {
		t.Error("should reject bad index")
	}
}

func TestEmptyChain(t *testing.T) {
	pub, _ := testKey(t)
	if err := VerifyChain(pub, nil); err != ErrNoStamps {
		t.Errorf("empty chain: got %v, want ErrNoStamps", err)
	}
}

func TestComputePrevHashDeterministic(t *testing.T) {
	var r [16]byte
	r[15] = 42
	ts := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)

	h1 := ComputePrevHash(Genesis, r, ts, 0)
	h2 := ComputePrevHash(Genesis, r, ts, 0)
	if h1 != h2 {
		t.Error("ComputePrevHash not deterministic")
	}

	// Different idx → different hash
	h3 := ComputePrevHash(Genesis, r, ts, 1)
	if h1 == h3 {
		t.Error("different idx should produce different hash")
	}

	// Different timestamp → different hash
	h4 := ComputePrevHash(Genesis, r, ts.Add(time.Second), 0)
	if h1 == h4 {
		t.Error("different timestamp should produce different hash")
	}

	// Different relay → different hash
	var r2 [16]byte
	r2[15] = 43
	h5 := ComputePrevHash(Genesis, r2, ts, 0)
	if h1 == h5 {
		t.Error("different relay should produce different hash")
	}
}

func TestSingleHopChain(t *testing.T) {
	pub, priv := testKey(t)
	var r [16]byte
	r[15] = 1
	ts := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)

	hops := []HopSpec{
		{Idx: 0, RelayID: r, StampAt: ts, PlateSHA: plateSHA(1), City: "Home", CC: "US"},
	}
	stamps, err := BuildChain(priv, hops)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyChain(pub, stamps); err != nil {
		t.Errorf("single hop chain: %v", err)
	}
}
