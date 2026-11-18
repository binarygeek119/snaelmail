// Package stats computes time-windowed aggregates for relay and network stats.
package stats

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Summary is the aggregate stats for a relay over a window.
type Summary struct {
	Stamped       int   `json:"stamped"`
	FirstHop      int   `json:"first_hop"`
	MiddleHop     int   `json:"middle_hop"`
	LastHop       int   `json:"last_hop"`
	Returned      int   `json:"returned"`
	Complaints    int   `json:"complaints"`
	DistanceSum   int64 `json:"distance_sum_km"`
	DistanceN     int   `json:"distance_n"`
	AvgDistanceKm float64 `json:"avg_distance_km"`
}

// DailyRow is a relay_daily row for a single day.
type DailyRow struct {
	Day        time.Time `json:"day"`
	Stamped    int       `json:"stamped"`
	FirstHop   int       `json:"first_hop"`
	MiddleHop  int       `json:"middle_hop"`
	LastHop    int       `json:"last_hop"`
	Returned   int       `json:"returned"`
	Complaints int       `json:"complaints"`
}

// Window returns the aggregate stats for a relay over a time window.
func Window(ctx context.Context, pool *pgxpool.Pool, relayID uuid.UUID, from, to time.Time) (*Summary, error) {
	var s Summary
	err := pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(stamped),0),
		       COALESCE(SUM(first_hop),0),
		       COALESCE(SUM(middle_hop),0),
		       COALESCE(SUM(last_hop),0),
		       COALESCE(SUM(returned),0),
		       COALESCE(SUM(complaints),0),
		       COALESCE(SUM(distance_sum),0),
		       COALESCE(SUM(distance_n),0)
		FROM relay_daily
		WHERE relay_id = $1 AND day >= $2 AND day < $3
	`, relayID, from, to).Scan(
		&s.Stamped, &s.FirstHop, &s.MiddleHop, &s.LastHop,
		&s.Returned, &s.Complaints, &s.DistanceSum, &s.DistanceN,
	)
	if err != nil {
		return nil, fmt.Errorf("stats window: %w", err)
	}
	if s.DistanceN > 0 {
		s.AvgDistanceKm = float64(s.DistanceSum) / float64(s.DistanceN)
	}
	return &s, nil
}

// Daily returns per-day rows for a relay over a window (for sparkline).
func Daily(ctx context.Context, pool *pgxpool.Pool, relayID uuid.UUID, from, to time.Time) ([]DailyRow, error) {
	rows, err := pool.Query(ctx, `
		SELECT day, stamped, first_hop, middle_hop, last_hop, returned, complaints
		FROM relay_daily
		WHERE relay_id = $1 AND day >= $2 AND day < $3
		ORDER BY day
	`, relayID, from, to)
	if err != nil {
		return nil, fmt.Errorf("stats daily: %w", err)
	}
	defer rows.Close()

	var out []DailyRow
	for rows.Next() {
		var r DailyRow
		if err := rows.Scan(&r.Day, &r.Stamped, &r.FirstHop, &r.MiddleHop, &r.LastHop, &r.Returned, &r.Complaints); err != nil {
			return nil, fmt.Errorf("stats daily scan: %w", err)
		}
		out = append(out, r)
	}
	return out, nil
}

// NetworkWide returns aggregate stats across all relays.
func NetworkWide(ctx context.Context, pool *pgxpool.Pool, from, to time.Time) (*Summary, error) {
	var s Summary
	err := pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(stamped),0),
		       COALESCE(SUM(first_hop),0),
		       COALESCE(SUM(middle_hop),0),
		       COALESCE(SUM(last_hop),0),
		       COALESCE(SUM(returned),0),
		       COALESCE(SUM(complaints),0),
		       COALESCE(SUM(distance_sum),0),
		       COALESCE(SUM(distance_n),0)
		FROM relay_daily
		WHERE day >= $1 AND day < $2
	`, from, to).Scan(
		&s.Stamped, &s.FirstHop, &s.MiddleHop, &s.LastHop,
		&s.Returned, &s.Complaints, &s.DistanceSum, &s.DistanceN,
	)
	if err != nil {
		return nil, fmt.Errorf("stats network: %w", err)
	}
	if s.DistanceN > 0 {
		s.AvgDistanceKm = float64(s.DistanceSum) / float64(s.DistanceN)
	}
	return &s, nil
}

// QueueDepth returns the number of hops currently due for stamping.
func QueueDepth(ctx context.Context, pool *pgxpool.Pool) (int, error) {
	var n int
	err := pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM hops WHERE stamp_at <= now() AND stamped_at IS NULL
	`).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("stats queue depth: %w", err)
	}
	return n, nil
}

// DueLagSeconds returns the age of the oldest due hop in seconds.
func DueLagSeconds(ctx context.Context, pool *pgxpool.Pool) (float64, error) {
	var lag float64
	err := pool.QueryRow(ctx, `
		SELECT COALESCE(EXTRACT(EPOCH FROM (now() - MIN(stamp_at))), 0)
		FROM hops WHERE stamp_at <= now() AND stamped_at IS NULL
	`).Scan(&lag)
	if err != nil {
		return 0, fmt.Errorf("stats due lag: %w", err)
	}
	return lag, nil
}

// RelayStatusCounts returns counts of relays by status.
func RelayStatusCounts(ctx context.Context, pool *pgxpool.Pool) (map[string]int, error) {
	rows, err := pool.Query(ctx, `SELECT status, COUNT(*) FROM relays GROUP BY status`)
	if err != nil {
		return nil, fmt.Errorf("stats relay status: %w", err)
	}
	defer rows.Close()

	out := make(map[string]int)
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		out[status] = count
	}
	return out, nil
}
