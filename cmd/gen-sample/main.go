package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"log"
	"os"
	"time"

	"snailmail/internal/render"
	"snailmail/internal/stamp"
)

func main() {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		log.Fatal(err)
	}

	base := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
	var r1, r2, r3 [16]byte
	r1[15] = 1
	r2[15] = 2
	r3[15] = 3

	hops := []stamp.HopSpec{
		{Idx: 0, RelayID: r1, StampAt: base, PlateSHA: [32]byte{1}, City: "New York", CC: "US"},
		{Idx: 1, RelayID: r2, StampAt: base.Add(4 * time.Hour), PlateSHA: [32]byte{2}, City: "Chicago", CC: "US"},
		{Idx: 2, RelayID: r3, StampAt: base.Add(9 * time.Hour), PlateSHA: [32]byte{3}, City: "Montreal", CC: "CA"},
	}

	stamps, err := stamp.BuildChain(priv, hops)
	if err != nil {
		log.Fatal(err)
	}

	stubPNGs := [][]byte{
		stamp.MakeStubStampPNG("NYC"),
		stamp.MakeStubStampPNG("CHI"),
		stamp.MakeStubStampPNG("MTL"),
	}

	result, err := render.Render(pub, stamps, stubPNGs)
	if err != nil {
		log.Fatal(err)
	}

	if err := os.WriteFile("testdata/sample-composite.png", result.CompositePNG, 0644); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Generated sample composite: %d bytes, all_verified=%v\n", len(result.CompositePNG), result.AllVerified)
}
