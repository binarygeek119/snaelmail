// Package schedule computes stamp_at and deliver_at for each hop in a route,
// respecting handling delays, gap holds, customs holds, and calendar rules.
package schedule

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math"
	"math/rand"
	"time"

	"snailmail/internal/clock"
	"snailmail/internal/config"
)

// Class is the mail class.
type Class string

const (
	Surface    Class = "surface"
	First      Class = "first"
	Air        Class = "air"
	Registered Class = "registered"
)

// Speed returns the km/day for this class from config.
func (c Class) Speed(cfg *config.SpeedKmPerDay) (int, error) {
	switch c {
	case Surface:
		return cfg.Surface, nil
	case First:
		return cfg.First, nil
	case Air:
		return cfg.Air, nil
	case Registered:
		return cfg.First, nil // registered uses first-class speed
	default:
		return 0, fmt.Errorf("unknown class %q", c)
	}
}

// Hop is one leg in a route with its scheduled stamp time.
type Hop struct {
	Idx      int
	RelayID  [16]byte
	FromCC   string
	ToCC     string
	LegKm    float64
	StampAt  time.Time
}

// Schedule computes stamp_at for each hop and deliver_at for the final delivery.
// queuedAt is when the letter enters the system.
func Schedule(
	cfg *config.Config,
	cal *clock.PostalCalendar,
	queuedAt time.Time,
	relays []HopRelay,
	class Class,
	queued int, // number already queued at each relay (for handling calc)
) ([]Hop, time.Time, error) {
	if len(relays) < 2 {
		return nil, time.Time{}, fmt.Errorf("schedule: need at least 2 relays, got %d", len(relays))
	}

	speed, err := class.Speed(&cfg.SpeedKmPerDay)
	if err != nil {
		return nil, time.Time{}, err
	}

	// Pickup delay: 2h from queued_at, floored to next dispatch window
	pickup, err := cal.Hold(queuedAt, 2)
	if err != nil {
		return nil, time.Time{}, err
	}
	pickup = cal.FloorToDispatch(pickup)

	hops := make([]Hop, 0, len(relays))
	cur := pickup

	for i := 0; i < len(relays)-1; i++ {
		from := relays[i]
		to := relays[i+1]

		// Holding time at this relay
		holdH := handlingHours(cfg, queued)
		cur, err = cal.Hold(cur, holdH)
		if err != nil {
			return nil, time.Time{}, err
		}

		legKm := haversine(from.Lat, from.Lon, to.Lat, to.Lon)

		// Gap hold if leg exceeds threshold
		if legKm > float64(cfg.Handling.GapThresholdKm) {
			gapH := (legKm - float64(cfg.Handling.GapThresholdKm)) / 600.0 * float64(cfg.Handling.GapHoursPer600)
			cur, err = cal.Hold(cur, gapH)
			if err != nil {
				return nil, time.Time{}, err
			}
		}

		// Customs hold on first cross-border hop
		if i == 0 && from.CC != to.CC {
			customsH := customsHold(from.ID, to.ID, cfg.Customs.HoldMinH, cfg.Customs.HoldMaxH)
			cur, err = cal.Hold(cur, customsH)
			if err != nil {
				return nil, time.Time{}, err
			}
		}

		// Advance across the leg
		cur, err = cal.Advance(cur, legKm, float64(speed))
		if err != nil {
			return nil, time.Time{}, err
		}

		hops = append(hops, Hop{
			Idx:     i,
			RelayID: from.ID,
			FromCC:  from.CC,
			ToCC:    to.CC,
			LegKm:   legKm,
			StampAt: cur,
		})
	}

	// Final hop: stamp the destination relay
	final := relays[len(relays)-1]
	prevHop := hops[len(hops)-1]
	hops = append(hops, Hop{
		Idx:     len(relays) - 1,
		RelayID: final.ID,
		FromCC:  prevHop.ToCC,
		ToCC:    final.CC,
		LegKm:   0,
		StampAt: cur,
	})

	// deliver_at = final stamp + 1h
	deliverAt, err := cal.Hold(cur, 1)
	if err != nil {
		return nil, time.Time{}, err
	}

	// Check class day cap
	totalDays := deliverAt.Sub(queuedAt).Hours() / 24
	if class == Surface && totalDays > float64(cfg.Limits.MaxSurfaceDays) {
		return nil, time.Time{}, fmt.Errorf("schedule: surface delivery %.1f days exceeds cap %d", totalDays, cfg.Limits.MaxSurfaceDays)
	}
	if class == Air && totalDays > float64(cfg.Limits.MaxAirDays) {
		return nil, time.Time{}, fmt.Errorf("schedule: air delivery %.1f days exceeds cap %d", totalDays, cfg.Limits.MaxAirDays)
	}

	return hops, deliverAt, nil
}

// HopRelay is the relay info needed for scheduling.
type HopRelay struct {
	ID   [16]byte
	CC   string
	Lat  float64
	Lon  float64
}

// handlingHours computes the base handling time plus queue-dependent adder.
func handlingHours(cfg *config.Config, queued int) float64 {
	base := float64(cfg.Handling.BaseHours)
	per100 := float64(cfg.Handling.Per100QueuedH)
	adder := per100 * float64(queued) / 100.0
	return base + adder
}

// customsHold returns a deterministic hold hours seeded by the envelope ID.
func customsHold(from, to [16]byte, minH, maxH int) float64 {
	h := sha256.New()
	h.Write(from[:])
	h.Write(to[:])
	sum := h.Sum(nil)
	seed := int64(binary.BigEndian.Uint64(sum[:8]))
	rng := rand.New(rand.NewSource(seed))
	span := maxH - minH
	return float64(minH) + rng.Float64()*float64(span)
}

// haversine is duplicated here to avoid circular deps; see route.Haversine.
func haversine(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371.0
	dLat := (lat2 - lat1) * math.Pi / 180
	dLon := (lon2 - lon1) * math.Pi / 180
	lat1r := lat1 * math.Pi / 180
	lat2r := lat2 * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(lat1r)*math.Cos(lat2r)*math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return R * c
}
