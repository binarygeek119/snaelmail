package route

import (
	"testing"
)

func makeRelay(id byte, lat, lon float64, active, paused, plate bool, cc string) *Relay {
	var rid [16]byte
	rid[15] = id
	return &Relay{
		ID:     rid,
		Active: active,
		Paused: paused,
		Plate:  plate,
		City:   "city",
		CC:     cc,
		Lat:    lat,
		Lon:    lon,
	}
}

func TestHaversine(t *testing.T) {
	// London to Paris: ~344 km
	d := Haversine(51.5074, -0.1278, 48.8566, 2.3522)
	if d < 340 || d > 350 {
		t.Errorf("London-Paris = %.1f km, want ~344 km", d)
	}

	// Same point = 0
	d = Haversine(0, 0, 0, 0)
	if d != 0 {
		t.Errorf("same point = %.1f km, want 0", d)
	}

	// Antipodal points: ~20015 km (half circumference)
	d = Haversine(0, 0, 0, 180)
	if d < 20000 || d > 20050 {
		t.Errorf("antipodal = %.1f km, want ~20015 km", d)
	}
}

func TestBearing(t *testing.T) {
	// North
	b := Bearing(0, 0, 10, 0)
	if b > 1 && b < 359 {
		t.Errorf("bearing north = %.1f, want ~0", b)
	}

	// East
	b = Bearing(0, 0, 0, 10)
	if b < 89 || b > 91 {
		t.Errorf("bearing east = %.1f, want ~90", b)
	}

	// South
	b = Bearing(10, 0, 0, 0)
	if b < 179 || b > 181 {
		t.Errorf("bearing south = %.1f, want ~180", b)
	}

	// West
	b = Bearing(0, 10, 0, 0)
	if b < 269 || b > 271 {
		t.Errorf("bearing west = %.1f, want ~270", b)
	}
}

func TestWithinCone(t *testing.T) {
	// Candidate directly on bearing
	ok := WithinCone(0, 0, 10, 0, 0, 30)
	if !ok {
		t.Error("candidate on bearing should be within cone")
	}

	// Candidate 20° off bearing, cone 30°
	ok = WithinCone(0, 0, 10, 5, 0, 30)
	if !ok {
		t.Error("candidate 20° off should be within 30° cone")
	}

	// Candidate 40° off bearing, cone 30°
	ok = WithinCone(0, 0, 10, 10, 0, 30)
	if ok {
		t.Error("candidate 40° off should be outside 30° cone")
	}
}

func TestResolveSameRelay(t *testing.T) {
	r := NewResolver(Config{MaxHops: 6, LocalRadiusKm: 25, ConeDeg: 60, MinProgress: 0.08})
	origin := makeRelay(1, 0, 0, true, false, true, "US")
	hops, err := r.Resolve(origin, origin, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(hops) != 1 {
		t.Errorf("same relay = %d hops, want 1", len(hops))
	}
}

func TestResolveNoRoute(t *testing.T) {
	r := NewResolver(Config{MaxHops: 6, LocalRadiusKm: 25, ConeDeg: 60, MinProgress: 0.08})
	origin := makeRelay(1, 0, 0, true, false, true, "US")
	dest := makeRelay(2, 10, 10, true, false, true, "US")

	// No intermediate relays, but direct route is valid
	all := func() []*Relay { return nil }
	hops, err := r.Resolve(origin, dest, all)
	if err != nil {
		t.Errorf("direct route should work: got %v", err)
	}
	if len(hops) != 2 {
		t.Errorf("direct route: got %d hops, want 2 (origin, dest)", len(hops))
	}
}

func TestResolveTwoHops(t *testing.T) {
	r := NewResolver(Config{MaxHops: 6, LocalRadiusKm: 25, ConeDeg: 60, MinProgress: 0.08})
	origin := makeRelay(1, 0, 0, true, false, true, "US")
	middle := makeRelay(2, 5, 5, true, false, true, "US")
	dest := makeRelay(3, 10, 10, true, false, true, "US")

	all := func() []*Relay { return []*Relay{middle} }
	hops, err := r.Resolve(origin, dest, all)
	if err != nil {
		t.Fatal(err)
	}
	if len(hops) != 3 {
		t.Errorf("got %d hops, want 3 (origin, middle, dest)", len(hops))
	}
	if hops[0].ID != origin.ID || hops[1].ID != middle.ID || hops[2].ID != dest.ID {
		t.Error("hops in wrong order")
	}
}

func TestResolveExcludesPaused(t *testing.T) {
	r := NewResolver(Config{MaxHops: 6, LocalRadiusKm: 25, ConeDeg: 60, MinProgress: 0.08})
	origin := makeRelay(1, 0, 0, true, false, true, "US")
	paused := makeRelay(2, 5, 5, true, true, true, "US") // paused
	dest := makeRelay(3, 10, 10, true, false, true, "US")

	all := func() []*Relay { return []*Relay{paused} }
	hops, err := r.Resolve(origin, dest, all)
	if err != nil {
		t.Fatal(err)
	}
	// Paused relay should be excluded, so we get direct route [origin, dest]
	if len(hops) != 2 {
		t.Errorf("paused relay excluded: got %d hops, want 2 (direct)", len(hops))
	}
	for _, h := range hops {
		if h.ID == paused.ID {
			t.Error("paused relay should not be in route")
		}
	}
}

func TestResolveExcludesInactive(t *testing.T) {
	r := NewResolver(Config{MaxHops: 6, LocalRadiusKm: 25, ConeDeg: 60, MinProgress: 0.08})
	origin := makeRelay(1, 0, 0, true, false, true, "US")
	inactive := makeRelay(2, 5, 5, false, false, true, "US") // inactive
	dest := makeRelay(3, 10, 10, true, false, true, "US")

	all := func() []*Relay { return []*Relay{inactive} }
	hops, err := r.Resolve(origin, dest, all)
	if err != nil {
		t.Fatal(err)
	}
	// Inactive relay should be excluded, so we get direct route [origin, dest]
	if len(hops) != 2 {
		t.Errorf("inactive relay excluded: got %d hops, want 2 (direct)", len(hops))
	}
	for _, h := range hops {
		if h.ID == inactive.ID {
			t.Error("inactive relay should not be in route")
		}
	}
}

func TestResolveExcludesNoPlate(t *testing.T) {
	r := NewResolver(Config{MaxHops: 6, LocalRadiusKm: 25, ConeDeg: 60, MinProgress: 0.08})
	origin := makeRelay(1, 0, 0, true, false, true, "US")
	noPlate := makeRelay(2, 5, 5, true, false, false, "US") // no plate
	dest := makeRelay(3, 10, 10, true, false, true, "US")

	all := func() []*Relay { return []*Relay{noPlate} }
	hops, err := r.Resolve(origin, dest, all)
	if err != nil {
		t.Fatal(err)
	}
	// Relay without plate should be excluded, so we get direct route [origin, dest]
	if len(hops) != 2 {
		t.Errorf("relay without plate excluded: got %d hops, want 2 (direct)", len(hops))
	}
	for _, h := range hops {
		if h.ID == noPlate.ID {
			t.Error("relay without plate should not be in route")
		}
	}
}

func TestResolveMaxHops(t *testing.T) {
	r := NewResolver(Config{MaxHops: 3, LocalRadiusKm: 25, ConeDeg: 60, MinProgress: 0.08})
	origin := makeRelay(1, 0, 0, true, false, true, "US")
	r2 := makeRelay(2, 2, 2, true, false, true, "US")
	r3 := makeRelay(3, 4, 4, true, false, true, "US")
	r4 := makeRelay(4, 6, 6, true, false, true, "US")
	r5 := makeRelay(5, 8, 8, true, false, true, "US")
	dest := makeRelay(6, 20, 20, true, false, true, "US")

	all := func() []*Relay { return []*Relay{r2, r3, r4, r5} }
	hops, err := r.Resolve(origin, dest, all)
	if err != nil {
		t.Fatal(err)
	}
	// MaxHops=3 means at most 3 hops (legs), so at most 4 relays
	// But we might get fewer if no more progress can be made
	if len(hops) > 4 {
		t.Errorf("exceeded max hops: got %d relays, want <= 4 (3 hops + origin)", len(hops))
	}
}

func TestResolveNoBacktracking(t *testing.T) {
	r := NewResolver(Config{MaxHops: 6, LocalRadiusKm: 25, ConeDeg: 60, MinProgress: 0.08})
	origin := makeRelay(1, 0, 0, true, false, true, "US")
	m1 := makeRelay(2, 5, 0, true, false, true, "US")
	m2 := makeRelay(3, 5, 5, true, false, true, "US")
	dest := makeRelay(4, 10, 10, true, false, true, "US")

	all := func() []*Relay { return []*Relay{m1, m2} }
	hops, err := r.Resolve(origin, dest, all)
	if err != nil {
		t.Fatal(err)
	}

	// Verify each hop gets closer to destination
	for i := 1; i < len(hops)-1; i++ {
		dPrev := Haversine(hops[i-1].Lat, hops[i-1].Lon, dest.Lat, dest.Lon)
		dCurr := Haversine(hops[i].Lat, hops[i].Lon, dest.Lat, dest.Lon)
		if dCurr >= dPrev {
			t.Errorf("hop %d backtracks: dist %.1f >= prev %.1f", i, dCurr, dPrev)
		}
	}
}
