package routes

import (
	"encoding/json"
	"os"
	"time"

	"snailmail/internal/clock"
	"snailmail/internal/config"
	"snailmail/internal/route"
	"snailmail/internal/schedule"
)

// GoldenRoute is a test fixture for route resolution and scheduling.
type GoldenRoute struct {
	Name     string          `json:"name"`
	Origin   RelayFixture    `json:"origin"`
	Dest     RelayFixture    `json:"dest"`
	Class    string          `json:"class"`
	QueuedAt time.Time       `json:"queued_at"`
	Hops     []HopFixture    `json:"hops"`
	DeliverAt time.Time      `json:"deliver_at"`
	TotalKm  float64         `json:"total_km"`
}

type RelayFixture struct {
	ID   byte    `json:"id"`
	CC   string  `json:"cc"`
	City string  `json:"city"`
	Lat  float64 `json:"lat"`
	Lon  float64 `json:"lon"`
}

type HopFixture struct {
	Idx     int       `json:"idx"`
	RelayID byte      `json:"relay_id"`
	FromCC  string    `json:"from_cc"`
	ToCC    string    `json:"to_cc"`
	LegKm   float64   `json:"leg_km"`
	StampAt time.Time `json:"stamp_at"`
}

// GenerateGoldenRoutes creates test fixtures for route and schedule validation.
func GenerateGoldenRoutes() ([]GoldenRoute, error) {
	cfg := &config.Config{
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

	cal, err := clock.New(&cfg.Calendar, "")
	if err != nil {
		return nil, err
	}

	resolver := route.NewResolver(route.Config{
		MaxHops:       cfg.Route.MaxHops,
		LocalRadiusKm: float64(cfg.Route.LocalRadiusKm),
		ConeDeg:       cfg.Route.ConeDeg,
		MinProgress:   cfg.Route.MinProgress,
	})

	// Define test relays
	relays := map[byte]*route.Relay{
		1: {ID: [16]byte{15: 1}, Active: true, Plate: true, CC: "US", Lat: 40.7128, Lon: -74.0060, City: "New York"},
		2: {ID: [16]byte{15: 2}, Active: true, Plate: true, CC: "US", Lat: 41.8781, Lon: -87.6298, City: "Chicago"},
		3: {ID: [16]byte{15: 3}, Active: true, Plate: true, CC: "US", Lat: 39.7392, Lon: -104.9903, City: "Denver"},
		4: {ID: [16]byte{15: 4}, Active: true, Plate: true, CC: "US", Lat: 34.0522, Lon: -118.2437, City: "Los Angeles"},
		5: {ID: [16]byte{15: 5}, Active: true, Plate: true, CC: "CA", Lat: 45.5017, Lon: -73.5673, City: "Montreal"},
		6: {ID: [16]byte{15: 6}, Active: true, Plate: true, CC: "GB", Lat: 51.5074, Lon: -0.1278, City: "London"},
		7: {ID: [16]byte{15: 7}, Active: true, Plate: true, CC: "DE", Lat: 52.5200, Lon: 13.4050, City: "Berlin"},
		8: {ID: [16]byte{15: 8}, Active: true, Plate: true, CC: "JP", Lat: 35.6762, Lon: 139.6503, City: "Tokyo"},
	}

	allRelays := func() []*route.Relay {
		result := make([]*route.Relay, 0, len(relays))
		for _, r := range relays {
			result = append(result, r)
		}
		return result
	}

	var fixtures []GoldenRoute

	// Test case 1: Local delivery (same city)
	origin := relays[1]
	dest := relays[1]
	hops, err := resolver.Resolve(origin, dest, allRelays)
	if err != nil {
		return nil, err
	}
	fixtures = append(fixtures, GoldenRoute{
		Name:     "local_same_city",
		Origin:   toRelayFixture(origin),
		Dest:     toRelayFixture(dest),
		Class:    "first",
		QueuedAt: time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC),
		Hops:     toHopFixtures(hops, schedule.First),
	})

	// Test case 2: Domestic short distance
	origin = relays[1]
	dest = relays[2]
	hops, err = resolver.Resolve(origin, dest, allRelays)
	if err != nil {
		return nil, err
	}
	schedHops, deliverAt, err := schedule.Schedule(cfg, cal, time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC), toHopRelays(hops), schedule.First, 0)
	if err != nil {
		return nil, err
	}
	fixtures = append(fixtures, GoldenRoute{
		Name:      "domestic_short_first",
		Origin:    toRelayFixture(origin),
		Dest:      toRelayFixture(dest),
		Class:     "first",
		QueuedAt:  time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC),
		Hops:      toHopFixturesFromSchedule(schedHops),
		DeliverAt: deliverAt,
		TotalKm:   route.Haversine(origin.Lat, origin.Lon, dest.Lat, dest.Lon),
	})

	// Test case 3: Cross-border
	origin = relays[1]
	dest = relays[5]
	hops, err = resolver.Resolve(origin, dest, allRelays)
	if err != nil {
		return nil, err
	}
	schedHops, deliverAt, err = schedule.Schedule(cfg, cal, time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC), toHopRelays(hops), schedule.Surface, 0)
	if err != nil {
		return nil, err
	}
	fixtures = append(fixtures, GoldenRoute{
		Name:      "cross_border_surface",
		Origin:    toRelayFixture(origin),
		Dest:      toRelayFixture(dest),
		Class:     "surface",
		QueuedAt:  time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC),
		Hops:      toHopFixturesFromSchedule(schedHops),
		DeliverAt: deliverAt,
		TotalKm:   route.Haversine(origin.Lat, origin.Lon, dest.Lat, dest.Lon),
	})

	// Test case 4: International airmail
	origin = relays[1]
	dest = relays[8]
	hops, err = resolver.Resolve(origin, dest, allRelays)
	if err != nil {
		return nil, err
	}
	schedHops, deliverAt, err = schedule.Schedule(cfg, cal, time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC), toHopRelays(hops), schedule.Air, 0)
	if err != nil {
		return nil, err
	}
	fixtures = append(fixtures, GoldenRoute{
		Name:      "international_air",
		Origin:    toRelayFixture(origin),
		Dest:      toRelayFixture(dest),
		Class:     "air",
		QueuedAt:  time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC),
		Hops:      toHopFixturesFromSchedule(schedHops),
		DeliverAt: deliverAt,
		TotalKm:   route.Haversine(origin.Lat, origin.Lon, dest.Lat, dest.Lon),
	})

	return fixtures, nil
}

// WriteGoldenRoutes writes fixtures to a JSON file.
func WriteGoldenRoutes(path string) error {
	fixtures, err := GenerateGoldenRoutes()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(fixtures, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func toRelayFixture(r *route.Relay) RelayFixture {
	return RelayFixture{
		ID:   r.ID[15],
		CC:   r.CC,
		City: r.City,
		Lat:  r.Lat,
		Lon:  r.Lon,
	}
}

func toHopRelays(hops []*route.Relay) []schedule.HopRelay {
	result := make([]schedule.HopRelay, len(hops))
	for i, h := range hops {
		result[i] = schedule.HopRelay{
			ID:  h.ID,
			CC:  h.CC,
			Lat: h.Lat,
			Lon: h.Lon,
		}
	}
	return result
}

func toHopFixtures(hops []*route.Relay, class schedule.Class) []HopFixture {
	result := make([]HopFixture, len(hops))
	for i, h := range hops {
		result[i] = HopFixture{
			Idx:     i,
			RelayID: h.ID[15],
			FromCC:  "",
			ToCC:    h.CC,
		}
	}
	return result
}

func toHopFixturesFromSchedule(hops []schedule.Hop) []HopFixture {
	result := make([]HopFixture, len(hops))
	for i, h := range hops {
		result[i] = HopFixture{
			Idx:     h.Idx,
			RelayID: h.RelayID[15],
			FromCC:  h.FromCC,
			ToCC:    h.ToCC,
			LegKm:   h.LegKm,
			StampAt: h.StampAt,
		}
	}
	return result
}
