package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// UserSettings represents user configuration
type UserSettings struct {
	UserID                        uuid.UUID `json:"user_id"`
	AutoAddTracking               bool      `json:"auto_add_tracking"`
	RequestDeliveryConfirmation   bool      `json:"request_delivery_confirmation"`
	IMAPEnabled                   bool      `json:"imap_enabled"`
	IMAPPasswordHash              *string   `json:"imap_password_hash,omitempty"`
	IMAPPasswordSalt              *string   `json:"imap_password_salt,omitempty"`
	IMAPHost                      string    `json:"imap_host"`
	IMAPPort                      int       `json:"imap_port"`
	POPEnabled                    bool      `json:"pop_enabled"`
	POPPasswordHash               *string   `json:"pop_password_hash,omitempty"`
	POPPasswordSalt               *string   `json:"pop_password_salt,omitempty"`
	POPHost                       string    `json:"pop_host"`
	POPPort                       int       `json:"pop_port"`
	Theme                         string    `json:"theme"`
	DefaultView                   string    `json:"default_view"`
	NotifyOnDelivery              bool      `json:"notify_on_delivery"`
	NotifyOnTrackingUpdate        bool      `json:"notify_on_tracking_update"`
	CreatedAt                     time.Time `json:"created_at"`
	UpdatedAt                     time.Time `json:"updated_at"`
}

// GetUserSettings retrieves user settings by user ID
func (s *Store) GetUserSettings(ctx context.Context, userID uuid.UUID) (*UserSettings, error) {
	query := `
		SELECT user_id, auto_add_tracking, request_delivery_confirmation,
		       imap_enabled, imap_password_hash, imap_password_salt,
		       imap_host, imap_port,
		       pop_enabled, pop_password_hash, pop_password_salt, pop_host, pop_port,
		       theme, default_view,
		       notify_on_delivery, notify_on_tracking_update,
		       created_at, updated_at
		FROM user_settings
		WHERE user_id = $1
	`

	settings := &UserSettings{}
	err := s.pool.QueryRow(ctx, query, userID).Scan(
		&settings.UserID,
		&settings.AutoAddTracking,
		&settings.RequestDeliveryConfirmation,
		&settings.IMAPEnabled,
		&settings.IMAPPasswordHash,
		&settings.IMAPPasswordSalt,
		&settings.IMAPHost,
		&settings.IMAPPort,
		&settings.POPEnabled,
		&settings.POPPasswordHash,
		&settings.POPPasswordSalt,
		&settings.POPHost,
		&settings.POPPort,
		&settings.Theme,
		&settings.DefaultView,
		&settings.NotifyOnDelivery,
		&settings.NotifyOnTrackingUpdate,
		&settings.CreatedAt,
		&settings.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		// Return default settings if none exist
		return &UserSettings{
			UserID:                      userID,
			AutoAddTracking:             false,
			RequestDeliveryConfirmation: false,
			IMAPEnabled:                 false,
			IMAPHost:                    "imap.snaelmail.org",
			IMAPPort:                    993,
			POPEnabled:                  false,
			POPHost:                     "pop.snaelmail.org",
			POPPort:                     995,
			Theme:                       "light",
			DefaultView:                 "inbox",
			NotifyOnDelivery:            true,
			NotifyOnTrackingUpdate:      true,
		}, nil
	}

	if err != nil {
		return nil, err
	}

	return settings, nil
}

// UpdateUserSettings updates user settings
func (s *Store) UpdateUserSettings(ctx context.Context, settings *UserSettings) error {
	query := `
		INSERT INTO user_settings (
			user_id, auto_add_tracking, request_delivery_confirmation,
			imap_enabled, imap_password_hash, imap_password_salt,
			imap_host, imap_port,
			pop_enabled, pop_password_hash, pop_password_salt, pop_host, pop_port,
			theme, default_view,
			notify_on_delivery, notify_on_tracking_update,
			updated_at
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)
		ON CONFLICT (user_id) DO UPDATE SET
			auto_add_tracking = EXCLUDED.auto_add_tracking,
			request_delivery_confirmation = EXCLUDED.request_delivery_confirmation,
			imap_enabled = EXCLUDED.imap_enabled,
			imap_password_hash = EXCLUDED.imap_password_hash,
			imap_password_salt = EXCLUDED.imap_password_salt,
			imap_host = EXCLUDED.imap_host,
			imap_port = EXCLUDED.imap_port,
			pop_enabled = EXCLUDED.pop_enabled,
			pop_password_hash = EXCLUDED.pop_password_hash,
			pop_password_salt = EXCLUDED.pop_password_salt,
			pop_host = EXCLUDED.pop_host,
			pop_port = EXCLUDED.pop_port,
			theme = EXCLUDED.theme,
			default_view = EXCLUDED.default_view,
			notify_on_delivery = EXCLUDED.notify_on_delivery,
			notify_on_tracking_update = EXCLUDED.notify_on_tracking_update,
			updated_at = EXCLUDED.updated_at
	`

	_, err := s.pool.Exec(ctx, query,
		settings.UserID,
		settings.AutoAddTracking,
		settings.RequestDeliveryConfirmation,
		settings.IMAPEnabled,
		settings.IMAPPasswordHash,
		settings.IMAPPasswordSalt,
		settings.IMAPHost,
		settings.IMAPPort,
		settings.POPEnabled,
		settings.POPPasswordHash,
		settings.POPPasswordSalt,
		settings.POPHost,
		settings.POPPort,
		settings.Theme,
		settings.DefaultView,
		settings.NotifyOnDelivery,
		settings.NotifyOnTrackingUpdate,
		time.Now(),
	)

	return err
}

// TrackingEvent represents a tracking event
type TrackingEvent struct {
	ID              int64     `json:"id"`
	EnvelopeID      uuid.UUID `json:"envelope_id"`
	EventType       string    `json:"event_type"`
	Location        *string   `json:"location"`
	Notes           *string   `json:"notes"`
	Timestamp       time.Time `json:"timestamp"`
	NotifySender    bool      `json:"notify_sender"`
	NotifyRecipient bool      `json:"notify_recipient"`
}

// GetTrackingEvents retrieves all tracking events for an envelope
func (s *Store) GetTrackingEvents(ctx context.Context, envelopeID uuid.UUID) ([]TrackingEvent, error) {
	query := `
		SELECT id, envelope_id, event_type, location, notes, timestamp,
		       notify_sender, notify_recipient
		FROM tracking_events
		WHERE envelope_id = $1
		ORDER BY timestamp ASC
	`

	rows, err := s.pool.Query(ctx, query, envelopeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []TrackingEvent
	for rows.Next() {
		var event TrackingEvent
		err := rows.Scan(
			&event.ID,
			&event.EnvelopeID,
			&event.EventType,
			&event.Location,
			&event.Notes,
			&event.Timestamp,
			&event.NotifySender,
			&event.NotifyRecipient,
		)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}

	return events, rows.Err()
}

// CreateTrackingEvent creates a new tracking event
func (s *Store) CreateTrackingEvent(ctx context.Context, event *TrackingEvent) error {
	query := `
		INSERT INTO tracking_events (envelope_id, event_type, location, notes, timestamp, notify_sender, notify_recipient)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`

	return s.pool.QueryRow(ctx, query,
		event.EnvelopeID,
		event.EventType,
		event.Location,
		event.Notes,
		event.Timestamp,
		event.NotifySender,
		event.NotifyRecipient,
	).Scan(&event.ID)
}

// ShareTrackingWithRecipient marks tracking as shared with recipient
func (s *Store) ShareTrackingWithRecipient(ctx context.Context, envelopeID uuid.UUID) error {
	query := `UPDATE envelopes SET tracking_shared = true WHERE id = $1`
	_, err := s.pool.Exec(ctx, query, envelopeID)
	return err
}

// Notification represents a user notification
type Notification struct {
	ID         int64     `json:"id"`
	UserID     uuid.UUID `json:"user_id"`
	EnvelopeID uuid.UUID `json:"envelope_id"`
	Type       string    `json:"type"`
	Message    string    `json:"message"`
	Read       bool      `json:"read"`
	CreatedAt  time.Time `json:"created_at"`
}

// GetNotifications retrieves notifications for a user
func (s *Store) GetNotifications(ctx context.Context, userID uuid.UUID) ([]Notification, error) {
	query := `
		SELECT id, user_id, envelope_id, type, message, read, created_at
		FROM notifications
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT 50
	`

	rows, err := s.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notifications []Notification
	for rows.Next() {
		var notification Notification
		err := rows.Scan(
			&notification.ID,
			&notification.UserID,
			&notification.EnvelopeID,
			&notification.Type,
			&notification.Message,
			&notification.Read,
			&notification.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		notifications = append(notifications, notification)
	}

	return notifications, rows.Err()
}

// CreateNotification creates a new notification
func (s *Store) CreateNotification(ctx context.Context, notification *Notification) error {
	query := `
		INSERT INTO notifications (user_id, envelope_id, type, message, created_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, read
	`

	return s.pool.QueryRow(ctx, query,
		notification.UserID,
		notification.EnvelopeID,
		notification.Type,
		notification.Message,
		time.Now(),
	).Scan(&notification.ID, &notification.Read)
}

// MarkNotificationRead marks a notification as read
func (s *Store) MarkNotificationRead(ctx context.Context, userID uuid.UUID, notificationID int64) error {
	query := `UPDATE notifications SET read = true WHERE id = $1 AND user_id = $2`
	_, err := s.pool.Exec(ctx, query, notificationID, userID)
	return err
}

// GetEnvelope retrieves an envelope by ID
func (s *Store) GetEnvelope(ctx context.Context, envelopeID uuid.UUID) (*Envelope, error) {
	query := `
		SELECT id, sender_id, recipient_id, direction, class, subject, ciphertext,
		       size_bytes, route, total_km, queued_at, deliver_at, state, external_to,
		       bounce_token, log_leaf, tracking_id, tracking_shared
		FROM envelopes
		WHERE id = $1
	`

	envelope := &Envelope{}
	err := s.pool.QueryRow(ctx, query, envelopeID).Scan(
		&envelope.ID,
		&envelope.SenderID,
		&envelope.RecipientID,
		&envelope.Direction,
		&envelope.Class,
		&envelope.Subject,
		&envelope.Ciphertext,
		&envelope.SizeBytes,
		&envelope.Route,
		&envelope.TotalKm,
		&envelope.QueuedAt,
		&envelope.DeliverAt,
		&envelope.State,
		&envelope.ExternalTo,
		&envelope.BounceToken,
		&envelope.LogLeaf,
		&envelope.TrackingID,
		&envelope.TrackingShared,
	)

	if err != nil {
		return nil, err
	}

	return envelope, nil
}
