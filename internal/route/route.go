// Package route resolves multi-hop paths between relays using geographic
// heuristics. The algorithm is a greedy search that minimizes detour while
// enforcing forward progress and directional cone constraints.
package route

import (
	"errors"
	"math"
)

// ErrNoRoute is returned when no valid path exists between two relays.
var ErrNoRoute = errors.New("no route")

// Relay is the subset of relay metadata needed for routing.
type Relay struct {
	ID     [16]byte // uuid
	Active bool
	Paused bool
	Plate  bool // plate is present and valid
	City   string
	CC     string
	Lat    float64
	Lon    float64
}

// Config is the routing configuration from config.toml §4.
type Config struct {
	MaxHops       int
	LocalRadiusKm float64
	ConeDeg       int
	MinProgress   float64
}

// Resolver builds routes between relays.
type Resolver struct {
	cfg Config
}

// NewResolver returns a resolver for the given config.
func NewResolver(cfg Config) *Resolver {
	return &Resolver{cfg: cfg}
}

// Resolve returns the ordered list of relays from origin to dest.
// Returns ErrNoRoute if origin and dest are the same relay, or if fewer than
// two hops would result.
func (r *Resolver) Resolve(origin, dest *Relay, all func() []*Relay) ([]*Relay, error) {
	if origin.ID == dest.ID {
		return []*Relay{origin}, nil
	}

	hops := []*Relay{origin}
	cur := origin

	for {
		remaining := Haversine(cur.Lat, cur.Lon, dest.Lat, dest.Lon)
		if remaining <= r.cfg.LocalRadiusKm || len(hops) >= r.cfg.MaxHops {
			break
		}
		bearing := Bearing(cur.Lat, cur.Lon, dest.Lat, dest.Lon)

		var best *Relay
		var bestScore float64
		for _, cand := range all() {
			if cand.ID == cur.ID || containsID(hops, cand.ID) {
				continue
			}
			if !cand.Active || cand.Paused || !cand.Plate {
				continue
			}
			distToDest := Haversine(cand.Lat, cand.Lon, dest.Lat, dest.Lon)
			if distToDest >= remaining*(1-r.cfg.MinProgress) {
				continue
			}
			if !WithinCone(cur.Lat, cur.Lon, cand.Lat, cand.Lon, bearing, r.cfg.ConeDeg) {
				continue
			}
			score := Haversine(cur.Lat, cur.Lon, cand.Lat, cand.Lon) + distToDest
			if best == nil || score < bestScore {
				best, bestScore = cand, score
			}
		}
		if best == nil {
			break
		}
		hops = append(hops, best)
		cur = best
	}

	if cur.ID != dest.ID {
		hops = append(hops, dest)
	}
	if len(hops) < 2 {
		return nil, ErrNoRoute
	}
	return hops, nil
}

func containsID(hops []*Relay, id [16]byte) bool {
	for _, h := range hops {
		if h.ID == id {
			return true
		}
	}
	return false
}

// Haversine returns the great-circle distance in km between two points.
func Haversine(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371.0 // Earth radius in km
	dLat := radians(lat2 - lat1)
	dLon := radians(lon2 - lon1)
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(radians(lat1))*math.Cos(radians(lat2))*
			math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return R * c
}

// Bearing returns the initial bearing in degrees from point 1 to point 2.
func Bearing(lat1, lon1, lat2, lon2 float64) float64 {
	dLon := radians(lon2 - lon1)
	lat1r := radians(lat1)
	lat2r := radians(lat2)
	y := math.Sin(dLon) * math.Cos(lat2r)
	x := math.Cos(lat1r)*math.Sin(lat2r) - math.Sin(lat1r)*math.Cos(lat2r)*math.Cos(dLon)
	b := degrees(math.Atan2(y, x))
	return math.Mod(b+360, 360)
}

// WithinCone returns true if the candidate lies within ±halfAngle degrees of
// the bearing from origin.
func WithinCone(oLat, oLon, cLat, cLon, bearing float64, halfAngle int) bool {
	cb := Bearing(oLat, oLon, cLat, cLon)
	diff := math.Abs(cb - bearing)
	if diff > 180 {
		diff = 360 - diff
	}
	return diff <= float64(halfAngle)
}

func radians(d float64) float64 { return d * math.Pi / 180 }
func degrees(r float64) float64 { return r * 180 / math.Pi }
