package stamp

import (
	"bytes"
	"image/png"
	"testing"
	"time"
)

func TestCompositeDeterministic(t *testing.T) {
	stub1 := MakeStubStampPNG("NYC")
	stub2 := MakeStubStampPNG("CHI")
	stub3 := MakeStubStampPNG("MTL")

	base := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
	slots := []Slot{
		{StampPNG: stub1, Date: base, City: "New York", Idx: 0, Total: 3, Verified: true},
		{StampPNG: stub2, Date: base.Add(2 * time.Hour), City: "Chicago", Idx: 1, Total: 3, Verified: true},
		{StampPNG: stub3, Date: base.Add(5 * time.Hour), City: "Montreal", Idx: 2, Total: 3, Verified: true},
	}

	out1, err := Composite(slots)
	if err != nil {
		t.Fatal(err)
	}
	out2, err := Composite(slots)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(out1, out2) {
		t.Errorf("composite not deterministic: len1=%d len2=%d", len(out1), len(out2))
	}

	// Must be valid PNG
	if _, err := png.Decode(bytes.NewReader(out1)); err != nil {
		t.Errorf("composite not valid PNG: %v", err)
	}
}

func TestCompositeTwoRunsIdentical(t *testing.T) {
	// The spec requires: same inputs → byte-identical output
	stub := MakeStubStampPNG("TEST")
	base := time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)

	slots := []Slot{
		{StampPNG: stub, Date: base, City: "TestCity", Idx: 0, Total: 1, Verified: true},
	}

	out1, _ := Composite(slots)
	out2, _ := Composite(slots)

	if !bytes.Equal(out1, out2) {
		t.Errorf("two runs differ: %d bytes vs %d bytes", len(out1), len(out2))
	}
}

func TestCompositeUnverifiedGray(t *testing.T) {
	stub := MakeStubStampPNG("BAD")
	base := time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)

	slotsVerified := []Slot{
		{StampPNG: stub, Date: base, City: "City", Idx: 0, Total: 1, Verified: true},
	}
	slotsUnverified := []Slot{
		{StampPNG: stub, Date: base, City: "City", Idx: 0, Total: 1, Verified: false},
	}

	out1, err := Composite(slotsVerified)
	if err != nil {
		t.Fatal(err)
	}
	out2, err := Composite(slotsUnverified)
	if err != nil {
		t.Fatal(err)
	}

	// They must differ (gray vs color)
	if bytes.Equal(out1, out2) {
		t.Error("verified and unverified should produce different output")
	}
}

func TestCompositeInvalidStampPNG(t *testing.T) {
	// Invalid stamp should not crash, should render gray placeholder
	slots := []Slot{
		{StampPNG: []byte("not a png"), Date: time.Now(), City: "Bad", Idx: 0, Total: 1, Verified: true},
	}
	out, err := Composite(slots)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) == 0 {
		t.Error("composite should produce output even with invalid stamp")
	}
}

func TestCompositeMaxSlots(t *testing.T) {
	// More than 6 slots should be truncated
	slots := make([]Slot, 10)
	for i := range slots {
		slots[i] = Slot{
			StampPNG: MakeStubStampPNG("X"),
			Date:     time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC),
			City:     "City",
			Idx:      i,
			Total:    10,
			Verified: true,
		}
	}
	out, err := Composite(slots)
	if err != nil {
		t.Fatal(err)
	}
	// Should still be valid
	if _, err := png.Decode(bytes.NewReader(out)); err != nil {
		t.Errorf("overflow composite not valid PNG: %v", err)
	}
}

func TestCompositeEmpty(t *testing.T) {
	out, err := Composite(nil)
	if err != nil {
		t.Fatal(err)
	}
	// Should produce a valid blank canvas
	if _, err := png.Decode(bytes.NewReader(out)); err != nil {
		t.Errorf("empty composite not valid PNG: %v", err)
	}
}
