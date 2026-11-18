package schedule

import (
	"testing"
	"time"

	"snailmail/internal/clock"
	"snailmail/internal/config"
)

func newTestConfig() *config.Config {
	return &config.Config{
		SpeedKmPerDay: config.SpeedKmPerDay{
			Surface: 250,
			First:   800,
			Air:     2500,
		},
		Handling: config.Handling{
			BaseHours:       4,
			Per100QueuedH:   2,
			GapThresholdKm:  600,
			GapHoursPer600:  6,
		},
		Route: config.Route{
			MaxHops:       6,
			LocalRadiusKm: 25,
			ConeDeg:       60,
			MinProgress:   0.08,
		},
		Calendar: config.Calendar{
			NoMovementWeekdays:  []int{0},
			SaturdaySpeedFactor: 0.5,
		},
		Customs: config.Customs{
			HoldMinH: 12,
			HoldMaxH: 72,
		},
		Limits: config.Limits{
			MaxSurfaceDays: 45,
			MaxAirDays:     21,
		},
	}
}

func newTestCalendar(t *testing.T) *clock.PostalCalendar {
	t.Helper()
	cfg := &config.Calendar{
		NoMovementWeekdays:  []int{0},
		SaturdaySpeedFactor: 0.5,
	}
	cal, err := clock.New(cfg, "")
	if err != nil {
		t.Fatal(err)
	}
	return cal
}

func makeHopRelay(id byte, cc string, lat, lon float64) HopRelay {
	var rid [16]byte
	rid[15] = id
	return HopRelay{ID: rid, CC: cc, Lat: lat, Lon: lon}
}

func TestScheduleBasic(t *testing.T) {
	cfg := newTestConfig()
	cal := newTestCalendar(t)

	relays := []HopRelay{
		makeHopRelay(1, "US", 40.7128, -74.0060),   // New York
		makeHopRelay(2, "US", 41.8781, -87.6298),   // Chicago
	}

	queuedAt := time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC) // Monday
	hops, deliverAt, err := Schedule(cfg, cal, queuedAt, relays, First, 0)
	if err != nil {
		t.Fatal(err)
	}

	if len(hops) != 2 {
		t.Errorf("got %d hops, want 2", len(hops))
	}

	if deliverAt.Before(queuedAt) {
		t.Errorf("deliverAt %v is before queuedAt %v", deliverAt, queuedAt)
	}

	// Verify no Sunday stamps
	for _, h := range hops {
		if h.StampAt.Weekday() == time.Sunday {
			t.Errorf("hop %d stamped on Sunday: %v", h.Idx, h.StampAt)
		}
	}
}

func TestScheduleCustomsHold(t *testing.T) {
	cfg := newTestConfig()
	cal := newTestCalendar(t)

	relays := []HopRelay{
		makeHopRelay(1, "US", 40.7128, -74.0060),   // New York
		makeHopRelay(2, "CA", 45.5017, -73.5673),   // Montreal
	}

	queuedAt := time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC)
	hops, _, err := Schedule(cfg, cal, queuedAt, relays, First, 0)
	if err != nil {
		t.Fatal(err)
	}

	// Cross-border should add customs hold (12-72h)
	// Base handling is 4h, plus customs hold
	expectedMinHours := 4.0 + 12.0 // base + min customs
	actualHours := hops[0].StampAt.Sub(queuedAt).Hours()
	if actualHours < expectedMinHours {
		t.Errorf("cross-border hold too short: %.1fh, want >= %.1fh", actualHours, expectedMinHours)
	}
}

func TestScheduleGapHold(t *testing.T) {
	cfg := newTestConfig()
	cal := newTestCalendar(t)

	// Long distance: New York to London (~5570 km)
	relays := []HopRelay{
		makeHopRelay(1, "US", 40.7128, -74.0060),
		makeHopRelay(2, "GB", 51.5074, -0.1278),
	}

	queuedAt := time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC)
	hops, _, err := Schedule(cfg, cal, queuedAt, relays, Air, 0)
	if err != nil {
		t.Fatal(err)
	}

	// Gap hold for >600km should add extra time
	// 5570km - 600km = 4970km, gap_hours = 4970/600 * 6 = 49.7h
	// Plus base handling 4h
	expectedMinHours := 4.0 + 40.0 // conservative check
	actualHours := hops[0].StampAt.Sub(queuedAt).Hours()
	if actualHours < expectedMinHours {
		t.Errorf("gap hold too short: %.1fh, want >= %.1fh", actualHours, expectedMinHours)
	}
}

func TestScheduleNoSundayStamps(t *testing.T) {
	cfg := newTestConfig()
	cal := newTestCalendar(t)

	// Start on Saturday to force Sunday skip
	relays := []HopRelay{
		makeHopRelay(1, "US", 40.7128, -74.0060),
		makeHopRelay(2, "US", 41.8781, -87.6298),
	}

	queuedAt := time.Date(2026, 11, 21, 10, 0, 0, 0, time.UTC) // Saturday
	hops, _, err := Schedule(cfg, cal, queuedAt, relays, Surface, 0)
	if err != nil {
		t.Fatal(err)
	}

	for _, h := range hops {
		if h.StampAt.Weekday() == time.Sunday {
			t.Errorf("hop %d stamped on Sunday: %v", h.Idx, h.StampAt)
		}
	}
}

func TestScheduleSaturdayHalfSpeed(t *testing.T) {
	cfg := newTestConfig()
	cal := newTestCalendar(t)

	relays := []HopRelay{
		makeHopRelay(1, "US", 40.7128, -74.0060),
		makeHopRelay(2, "US", 41.0, -74.5), // Short distance ~30km
	}

	// Same distance, but start on Saturday vs Monday
	sat := time.Date(2026, 11, 21, 10, 0, 0, 0, time.UTC)
	mon := time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC)

	hopsSat, _, err := Schedule(cfg, cal, sat, relays, Surface, 0)
	if err != nil {
		t.Fatal(err)
	}
	hopsMon, _, err := Schedule(cfg, cal, mon, relays, Surface, 0)
	if err != nil {
		t.Fatal(err)
	}

	// Saturday should take longer (half speed)
	satDuration := hopsSat[0].StampAt.Sub(sat).Hours()
	monDuration := hopsMon[0].StampAt.Sub(mon).Hours()

	// Saturday duration should be longer due to half speed
	// The difference might be small for short distances, but should be noticeable
	if satDuration <= monDuration {
		t.Errorf("Saturday should be slower: sat=%.1fh, mon=%.1fh", satDuration, monDuration)
	}
}

func TestScheduleMaxHops(t *testing.T) {
	cfg := newTestConfig()
	cal := newTestCalendar(t)

	// Create a 7-hop route (exceeds max_hops=6)
	relays := make([]HopRelay, 7)
	for i := 0; i < 7; i++ {
		relays[i] = makeHopRelay(byte(i+1), "US", 40+float64(i), -74+float64(i))
	}

	queuedAt := time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC)
	_, _, err := Schedule(cfg, cal, queuedAt, relays, First, 0)
	if err != nil {
		t.Fatal(err)
	}
	// Schedule should still work, route resolution enforces max_hops
}

func TestClassSpeed(t *testing.T) {
	cfg := &config.SpeedKmPerDay{
		Surface: 250,
		First:   800,
		Air:     2500,
	}

	tests := []struct {
		class Class
		want  int
	}{
		{Surface, 250},
		{First, 800},
		{Air, 2500},
		{Registered, 800}, // registered uses first-class speed
	}

	for _, tt := range tests {
		got, err := tt.class.Speed(cfg)
		if err != nil {
			t.Errorf("Speed(%s): %v", tt.class, err)
		}
		if got != tt.want {
			t.Errorf("Speed(%s) = %d, want %d", tt.class, got, tt.want)
		}
	}
}

func TestCustomsHoldDeterministic(t *testing.T) {
	from := [16]byte{1, 2, 3}
	to := [16]byte{4, 5, 6}

	h1 := customsHold(from, to, 12, 72)
	h2 := customsHold(from, to, 12, 72)

	if h1 != h2 {
		t.Errorf("customsHold not deterministic: %.2f != %.2f", h1, h2)
	}

	if h1 < 12 || h1 > 72 {
		t.Errorf("customsHold %.2f outside [12, 72]", h1)
	}
}
