package notification

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"net/smtp"
	"time"

	"snailmail/internal/outbound"
	"snailmail/internal/store"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// EmailConfig holds SMTP configuration
type EmailConfig struct {
	SMTPHost     string
	SMTPPort     string
	SMTPUsername string
	SMTPPassword string
	FromEmail    string
	FromName     string
}

// Notifier handles sending notifications
type Notifier struct {
	config EmailConfig
	store  *store.Store
}

// NewNotifier creates a new notification service
func NewNotifier(config EmailConfig, store *store.Store) *Notifier {
	return &Notifier{
		config: config,
		store:  store,
	}
}

// TrackingUpdateEmail sends a tracking update notification
func (n *Notifier) TrackingUpdateEmail(ctx context.Context, userID uuid.UUID, envelopeID uuid.UUID, eventType string, location string) error {
	// Get user details
	user, err := n.store.GetUserByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	if user.Email == nil || *user.Email == "" {
		return fmt.Errorf("user has no email address configured")
	}

	// Get envelope details
	envelope, err := n.store.GetEnvelopeByID(ctx, envelopeID)
	if err != nil {
		return fmt.Errorf("failed to get envelope: %w", err)
	}

	// Prepare email content
	subject := fmt.Sprintf("Tracking Update: %s", envelope.Subject)
	body := n.generateTrackingEmailBody(eventType, location, envelope)

	// Send email
	return n.sendEmail(*user.Email, subject, body)
}

// DeliveryConfirmationEmail sends a delivery confirmation notification
func (n *Notifier) DeliveryConfirmationEmail(ctx context.Context, userID uuid.UUID, envelopeID uuid.UUID) error {
	// Get user details
	user, err := n.store.GetUserByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	if user.Email == nil || *user.Email == "" {
		return fmt.Errorf("user has no email address configured")
	}

	// Get envelope details
	envelope, err := n.store.GetEnvelopeByID(ctx, envelopeID)
	if err != nil {
		return fmt.Errorf("failed to get envelope: %w", err)
	}

	// Prepare email content
	subject := fmt.Sprintf("Delivery Confirmation: %s", envelope.Subject)
	body := n.generateDeliveryEmailBody(envelope)

	// Send email
	return n.sendEmail(*user.Email, subject, body)
}

// OutForDeliveryEmail sends an "out for delivery" notification
func (n *Notifier) OutForDeliveryEmail(ctx context.Context, userID uuid.UUID, envelopeID uuid.UUID, deliveryWindow time.Time) error {
	// Get user details
	user, err := n.store.GetUserByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	if user.Email == nil || *user.Email == "" {
		return fmt.Errorf("user has no email address configured")
	}

	// Get envelope details
	envelope, err := n.store.GetEnvelopeByID(ctx, envelopeID)
	if err != nil {
		return fmt.Errorf("failed to get envelope: %w", err)
	}

	// Prepare email content
	subject := fmt.Sprintf("Out for Delivery: %s", envelope.Subject)
	body := n.generateOutForDeliveryEmailBody(envelope, deliveryWindow)

	// Send email
	return n.sendEmail(*user.Email, subject, body)
}

// sendEmail sends an email using SMTP
func (n *Notifier) sendEmail(to, subject, body string) error {
	// If SMTP credentials are configured, use SMTP relay
	if n.config.SMTPUsername != "" && n.config.SMTPPassword != "" {
		return n.sendViaSMTP(to, subject, body)
	}

	// Otherwise, use direct outbound delivery
	result := outbound.DeliverEmail(context.Background(), n.config.FromEmail, to, subject, body, "")
	if !result.Success {
		return fmt.Errorf("direct delivery failed: %w", result.Error)
	}
	return nil
}

// sendViaSMTP sends email via configured SMTP relay
func (n *Notifier) sendViaSMTP(to, subject, body string) error {
	// Create email message
	msg := fmt.Sprintf("From: %s <%s>\r\n"+
		"To: %s\r\n"+
		"Subject: %s\r\n"+
		"MIME-Version: 1.0\r\n"+
		"Content-Type: text/html; charset=\"UTF-8\"\r\n"+
		"\r\n"+
		"%s\r\n",
		n.config.FromName, n.config.FromEmail,
		to,
		subject,
		body,
	)

	// Connect to SMTP server
	addr := n.config.SMTPHost + ":" + n.config.SMTPPort
	auth := smtp.PlainAuth("", n.config.SMTPUsername, n.config.SMTPPassword, n.config.SMTPHost)

	// Send email
	err := smtp.SendMail(addr, auth, n.config.FromEmail, []string{to}, []byte(msg))
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}

// generateTrackingEmailBody generates the HTML body for tracking update emails
func (n *Notifier) generateTrackingEmailBody(eventType, location string, envelope *store.Envelope) string {
	tmpl := `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); color: white; padding: 30px; text-align: center; border-radius: 10px 10px 0 0; }
        .content { background: #f9f9f9; padding: 30px; border-radius: 0 0 10px 10px; }
        .event-type { font-size: 24px; font-weight: bold; color: #667eea; margin-bottom: 10px; }
        .location { color: #666; margin-bottom: 20px; }
        .details { background: white; padding: 20px; border-radius: 8px; margin: 20px 0; }
        .footer { text-align: center; margin-top: 30px; color: #999; font-size: 12px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>📬 Tracking Update</h1>
        </div>
        <div class="content">
            <div class="event-type">{{.EventType}}</div>
            <div class="location">📍 {{.Location}}</div>
            
            <div class="details">
                <p><strong>Subject:</strong> {{.Subject}}</p>
                <p><strong>Tracking ID:</strong> {{.TrackingID}}</p>
                <p><strong>From:</strong> {{.Sender}}</p>
                <p><strong>To:</strong> {{.Recipient}}</p>
            </div>
            
            <p>Your letter is on its way! You can track its progress in your Snaelmail dashboard.</p>
        </div>
        <div class="footer">
            <p>This is an automated message from Snaelmail</p>
        </div>
    </div>
</body>
</html>
`

	data := struct {
		EventType   string
		Location    string
		Subject     string
		TrackingID  string
		Sender      string
		Recipient   string
	}{
		EventType:  formatEventType(eventType),
		Location:   location,
		Subject:    envelope.Subject,
		TrackingID: *envelope.TrackingID,
		Sender:     n.getUserAddress(envelope.SenderID),
		Recipient:  n.getUserAddress(envelope.RecipientID),
	}

	t := template.Must(template.New("email").Parse(tmpl))
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return fmt.Sprintf("Tracking update for: %s", envelope.Subject)
	}

	return buf.String()
}

// generateDeliveryEmailBody generates the HTML body for delivery confirmation emails
func (n *Notifier) generateDeliveryEmailBody(envelope *store.Envelope) string {
	tmpl := `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: linear-gradient(135deg, #4CAF50 0%, #45a049 100%); color: white; padding: 30px; text-align: center; border-radius: 10px 10px 0 0; }
        .content { background: #f9f9f9; padding: 30px; border-radius: 0 0 10px 10px; }
        .success-icon { font-size: 48px; margin-bottom: 20px; }
        .details { background: white; padding: 20px; border-radius: 8px; margin: 20px 0; }
        .footer { text-align: center; margin-top: 30px; color: #999; font-size: 12px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <div class="success-icon">✓</div>
            <h1>Delivered Successfully!</h1>
        </div>
        <div class="content">
            <p>Your letter has been delivered successfully.</p>
            
            <div class="details">
                <p><strong>Subject:</strong> {{.Subject}}</p>
                <p><strong>Tracking ID:</strong> {{.TrackingID}}</p>
                <p><strong>From:</strong> {{.Sender}}</p>
                <p><strong>To:</strong> {{.Recipient}}</p>
                <p><strong>Delivered:</strong> {{.DeliveredAt}}</p>
            </div>
            
            <p>Thank you for using Snaelmail!</p>
        </div>
        <div class="footer">
            <p>This is an automated message from Snaelmail</p>
        </div>
    </div>
</body>
</html>
`

	data := struct {
		Subject     string
		TrackingID  string
		Sender      string
		Recipient   string
		DeliveredAt string
	}{
		Subject:     envelope.Subject,
		TrackingID:  *envelope.TrackingID,
		Sender:      n.getUserAddress(envelope.SenderID),
		Recipient:   n.getUserAddress(envelope.RecipientID),
		DeliveredAt: envelope.DeliverAt.Format("January 2, 2006 at 3:04 PM"),
	}

	t := template.Must(template.New("email").Parse(tmpl))
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return fmt.Sprintf("Delivery confirmation for: %s", envelope.Subject)
	}

	return buf.String()
}

// generateOutForDeliveryEmailBody generates the HTML body for "out for delivery" emails
func (n *Notifier) generateOutForDeliveryEmailBody(envelope *store.Envelope, deliveryWindow time.Time) string {
	tmpl := `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: linear-gradient(135deg, #FF9800 0%, #F57C00 100%); color: white; padding: 30px; text-align: center; border-radius: 10px 10px 0 0; }
        .content { background: #f9f9f9; padding: 30px; border-radius: 0 0 10px 10px; }
        .icon { font-size: 48px; margin-bottom: 20px; }
        .details { background: white; padding: 20px; border-radius: 8px; margin: 20px 0; }
        .delivery-window { background: #FFF3E0; padding: 15px; border-radius: 8px; margin: 20px 0; border-left: 4px solid #FF9800; }
        .footer { text-align: center; margin-top: 30px; color: #999; font-size: 12px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <div class="icon">🚚</div>
            <h1>Out for Delivery!</h1>
        </div>
        <div class="content">
            <p>Your letter is out for delivery and will arrive soon!</p>
            
            <div class="delivery-window">
                <p><strong>Expected Delivery Window:</strong></p>
                <p style="font-size: 18px; margin: 0;">{{.DeliveryWindow}}</p>
            </div>
            
            <div class="details">
                <p><strong>Subject:</strong> {{.Subject}}</p>
                <p><strong>Tracking ID:</strong> {{.TrackingID}}</p>
                <p><strong>From:</strong> {{.Sender}}</p>
                <p><strong>To:</strong> {{.Recipient}}</p>
            </div>
            
            <p>Make sure someone is available to receive the delivery!</p>
        </div>
        <div class="footer">
            <p>This is an automated message from Snaelmail</p>
        </div>
    </div>
</body>
</html>
`

	data := struct {
		Subject        string
		TrackingID     string
		Sender         string
		Recipient      string
		DeliveryWindow string
	}{
		Subject:        envelope.Subject,
		TrackingID:     *envelope.TrackingID,
		Sender:         n.getUserAddress(envelope.SenderID),
		Recipient:      n.getUserAddress(envelope.RecipientID),
		DeliveryWindow: deliveryWindow.Format("January 2, 2006 at 3:04 PM"),
	}

	t := template.Must(template.New("email").Parse(tmpl))
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return fmt.Sprintf("Out for delivery: %s", envelope.Subject)
	}

	return buf.String()
}

// getUserAddress gets the user's email address
func (n *Notifier) getUserAddress(userID *uuid.UUID) string {
	if userID == nil {
		return "Unknown"
	}
	
	ctx := context.Background()
	user, err := n.store.GetUserByID(ctx, *userID)
	if err != nil {
		return "Unknown"
	}
	
	// Prefer external email if available, otherwise use internal address
	if user.Email != nil && *user.Email != "" {
		return *user.Email
	}
	return user.Address
}
// formatEventType formats the event type for display
func formatEventType(eventType string) string {
	switch eventType {
	case "queued":
		return "📦 Letter Queued"
	case "in_transit":
		return "🚚 In Transit"
	case "out_for_delivery":
		return "📬 Out for Delivery"
	case "delivered":
		return "✓ Delivered"
	case "failed":
		return "✗ Delivery Failed"
	default:
		return eventType
	}
}

// SendVerificationEmail sends a 6-digit verification code to the user's email
func (n *Notifier) SendVerificationEmail(ctx context.Context, email, code string) error {
	subject := "Verify Your Snaelmail Account"
	body := n.generateVerificationEmailBody(code)
	return n.sendEmail(email, subject, body)
}

// generateVerificationEmailBody generates the HTML body for verification emails
func (n *Notifier) generateVerificationEmailBody(code string) string {
	tmpl := `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); color: white; padding: 30px; text-align: center; border-radius: 10px 10px 0 0; }
        .content { background: #f9f9f9; padding: 30px; border-radius: 0 0 10px 10px; }
        .code { font-size: 32px; font-weight: bold; color: #667eea; letter-spacing: 8px; text-align: center; margin: 30px 0; padding: 20px; background: white; border-radius: 8px; border: 2px dashed #667eea; }
        .footer { text-align: center; margin-top: 30px; color: #999; font-size: 12px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>🐌 Welcome to Snaelmail!</h1>
        </div>
        <div class="content">
            <p>Thank you for creating a Snaelmail account. To verify your email address, please use the verification code below:</p>
            
            <div class="code">{{.Code}}</div>
            
            <p>This code will expire in 24 hours. If you did not request this verification, please ignore this email.</p>
            
            <p>Once verified, you can start sending letters through our virtual post office network!</p>
        </div>
        <div class="footer">
            <p>This is an automated message from Snaelmail</p>
        </div>
    </div>
</body>
</html>
`

	data := struct {
		Code string
	}{
		Code: code,
	}

	t := template.Must(template.New("email").Parse(tmpl))
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return fmt.Sprintf("Verification code: %s", code)
	}

	return buf.String()
}

// NotificationWorker runs as a background worker to process pending notifications
type NotificationWorker struct {
	notifier *Notifier
	pool     *pgxpool.Pool
	interval time.Duration
}

// NewNotificationWorker creates a new notification worker
func NewNotificationWorker(notifier *Notifier, pool *pgxpool.Pool, interval time.Duration) *NotificationWorker {
	return &NotificationWorker{
		notifier: notifier,
		pool:     pool,
		interval: interval,
	}
}

// Start starts the notification worker
func (w *NotificationWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.processPendingNotifications(ctx)
		}
	}
}

// processPendingNotifications processes all pending notifications
func (w *NotificationWorker) processPendingNotifications(ctx context.Context) {
	// Get pending notifications
	rows, err := w.pool.Query(ctx, `
		SELECT id, user_id, envelope_id, type, message
		FROM notifications
		WHERE read = false
		ORDER BY created_at ASC
		LIMIT 10
	`)
	if err != nil {
		fmt.Printf("Error querying notifications: %v\n", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var id int64
		var userID, envelopeID uuid.UUID
		var notificationType, message string

		if err := rows.Scan(&id, &userID, &envelopeID, &notificationType, &message); err != nil {
			continue
		}

		// Send notification based on type
		var sendErr error
		switch notificationType {
		case "tracking_update":
			// Get location from tracking event
			location := "Unknown location"
			sendErr = w.notifier.TrackingUpdateEmail(ctx, userID, envelopeID, notificationType, location)
		case "delivery_confirmation":
			sendErr = w.notifier.DeliveryConfirmationEmail(ctx, userID, envelopeID)
		case "out_for_delivery":
			// Get delivery window from envelope
			deliveryWindow := time.Now().Add(24 * time.Hour)
			sendErr = w.notifier.OutForDeliveryEmail(ctx, userID, envelopeID, deliveryWindow)
		}

		if sendErr != nil {
			fmt.Printf("Error sending notification %d: %v\n", id, sendErr)
			continue
		}

		// Mark notification as read
		_, err = w.pool.Exec(ctx, `
			UPDATE notifications
			SET read = true
			WHERE id = $1
		`, id)
		if err != nil {
			fmt.Printf("Error marking notification %d as read: %v\n", id, err)
		}
	}
}
