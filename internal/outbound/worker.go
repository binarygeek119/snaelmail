package outbound

import (
	"context"
	"fmt"
	"log"
	"time"

	"snailmail/internal/store"
)

// DeliveryWorker processes the outbound delivery queue
type DeliveryWorker struct {
	store    *store.Store
	interval time.Duration
	done     chan struct{}
}

// NewDeliveryWorker creates a new delivery worker
func NewDeliveryWorker(store *store.Store, interval time.Duration) *DeliveryWorker {
	return &DeliveryWorker{
		store:    store,
		interval: interval,
		done:     make(chan struct{}),
	}
}

// Start starts the delivery worker
func (w *DeliveryWorker) Start() {
	log.Printf("Outbound delivery worker started (interval: %v)", w.interval)
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			w.processQueue()
		case <-w.done:
			return
		}
	}
}

// Stop stops the delivery worker
func (w *DeliveryWorker) Stop() {
	close(w.done)
}

// processQueue processes pending emails in the queue
func (w *DeliveryWorker) processQueue() {
	ctx := context.Background()

	rows, err := w.store.Pool().Query(ctx, `
		SELECT id, from_address, to_address, subject, body, dkim_signature
		FROM outbound_queue
		WHERE status = 'queued' AND (next_retry IS NULL OR next_retry <= now())
		ORDER BY created_at ASC
		LIMIT 10
	`)
	if err != nil {
		log.Printf("Delivery worker: query error: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var id, from, to, subject, body string
		var dkimSig *string
		if err := rows.Scan(&id, &from, &to, &subject, &body, &dkimSig); err != nil {
			log.Printf("Delivery worker: scan error: %v", err)
			continue
		}

		sig := ""
		if dkimSig != nil {
			sig = *dkimSig
		}

		// Mark as sending
		w.store.Pool().Exec(ctx, `
			UPDATE outbound_queue SET status = 'sending', last_attempt = now(), attempts = attempts + 1
			WHERE id = $1
		`, id)

		// Attempt delivery
		result := DeliverEmail(ctx, from, to, subject, body, sig)

		if result.Success {
			w.store.Pool().Exec(ctx, `
				UPDATE outbound_queue SET status = 'sent', updated_at = now()
				WHERE id = $1
			`, id)
			log.Printf("Delivered email %s to %s via %s", id, to, result.Host)
		} else {
			// Schedule retry
			nextRetry := time.Now().Add(5 * time.Minute)
			w.store.Pool().Exec(ctx, `
				UPDATE outbound_queue SET status = 'queued', error_message = $2, next_retry = $3, updated_at = now()
				WHERE id = $1
			`, id, result.Error.Error(), nextRetry)
			log.Printf("Delivery failed for %s: %v, retrying at %v", id, result.Error, nextRetry)
		}
	}
}

// Enqueue adds an email to the delivery queue
func Enqueue(ctx context.Context, store *store.Store, from, to, subject, body, dkimSig string) error {
	_, err := store.Pool().Exec(ctx, `
		INSERT INTO outbound_queue (from_address, to_address, subject, body, dkim_signature, status, next_retry)
		VALUES ($1, $2, $3, $4, $5, 'queued', now())
	`, from, to, subject, body, dkimSig)
	if err != nil {
		return fmt.Errorf("enqueue: %w", err)
	}
	return nil
}
