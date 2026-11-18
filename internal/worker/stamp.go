package worker

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// StampWorker processes due hops every 15 seconds.
type StampWorker struct {
	pool     *pgxpool.Pool
	interval time.Duration
}

// NewStampWorker creates a stamp worker with the given poll interval.
func NewStampWorker(pool *pgxpool.Pool, interval time.Duration) *StampWorker {
	return &StampWorker{
		pool:     pool,
		interval: interval,
	}
}

// Run starts the stamp worker loop. It blocks until ctx is cancelled.
func (w *StampWorker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	log.Printf("stamp worker: starting (interval=%s)", w.interval)

	for {
		select {
		case <-ctx.Done():
			log.Println("stamp worker: shutting down")
			return
		case <-ticker.C:
			if err := w.processDueHops(ctx); err != nil {
				log.Printf("stamp worker: error processing hops: %v", err)
			}
		}
	}
}

// processDueHops fetches and stamps up to 200 due hops.
func (w *StampWorker) processDueHops(ctx context.Context) error {
	rows, err := w.pool.Query(ctx, `
		SELECT h.envelope_id, h.idx, h.relay_id, h.stamp_at
		FROM hops h
		WHERE h.stamp_at <= now() AND h.stamped_at IS NULL
		ORDER BY h.stamp_at
		LIMIT 200
		FOR UPDATE SKIP LOCKED
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	var hopCount int
	for rows.Next() {
		var envelopeID uuid.UUID
		var idx int
		var relayID uuid.UUID
		var stampAt time.Time

		if err := rows.Scan(&envelopeID, &idx, &relayID, &stampAt); err != nil {
			return err
		}

		if err := w.stampHop(ctx, envelopeID, idx, relayID, stampAt); err != nil {
			log.Printf("stamp worker: failed to stamp hop %d for envelope %s: %v", idx, envelopeID, err)
			continue
		}
		hopCount++
	}

	if hopCount > 0 {
		log.Printf("stamp worker: stamped %d hops", hopCount)
	}

	return nil
}

// stampHop marks a hop as stamped and updates related counters.
func (w *StampWorker) stampHop(ctx context.Context, envelopeID uuid.UUID, idx int, relayID uuid.UUID, stampAt time.Time) error {
	tx, err := w.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Mark hop as stamped
	_, err = tx.Exec(ctx, `
		UPDATE hops SET stamped_at = now()
		WHERE envelope_id = $1 AND idx = $2 AND stamped_at IS NULL
	`, envelopeID, idx)
	if err != nil {
		return err
	}

	// Get envelope details
	var direction string
	var class string
	var state string
	var totalHops int
	err = tx.QueryRow(ctx, `
		SELECT direction, class, state, (SELECT COUNT(*) FROM hops WHERE envelope_id = $1)
		FROM envelopes WHERE id = $1
	`, envelopeID).Scan(&direction, &class, &state, &totalHops)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil // envelope deleted
		}
		return err
	}

	// Determine hop type
	var hopType string
	if idx == 0 {
		hopType = "first_hop"
	} else if idx == totalHops-1 {
		hopType = "last_hop"
	} else {
		hopType = "middle_hop"
	}

	// Update relay_daily counter
	today := time.Now().Truncate(24 * time.Hour)
	_, err = tx.Exec(ctx, `
		INSERT INTO relay_daily (relay_id, day, stamped, `+hopType+`)
		VALUES ($1, $2, 1, 1)
		ON CONFLICT (relay_id, day) DO UPDATE
		SET stamped = relay_daily.stamped + 1,
		    `+hopType+` = relay_daily.`+hopType+` + 1
	`, relayID, today)
	if err != nil {
		return err
	}

	// Increment relay letters_passed
	_, err = tx.Exec(ctx, `
		UPDATE relays SET letters_passed = letters_passed + 1 WHERE id = $1
	`, relayID)
	if err != nil {
		return err
	}

	// Insert postmark event
	_, err = tx.Exec(ctx, `
		INSERT INTO postmark_events (relay_id, envelope_id, hop_idx, stamped_at, class)
		VALUES ($1, $2, $3, now(), $4)
	`, relayID, envelopeID, idx, class)
	if err != nil {
		return err
	}

	// If this is the last hop and envelope is still in_transit, mark as delivered
	if hopType == "last_hop" && state == "in_transit" {
		_, err = tx.Exec(ctx, `
			UPDATE envelopes SET state = 'delivered' WHERE id = $1
		`, envelopeID)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}
