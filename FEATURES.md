# Snaelmail - Email Delivery System

A modern email delivery system with virtual post offices, tracking, and delivery notifications.

## Features Implemented

### 1. Tracking System
- **Tracking Numbers**: Each envelope gets a unique tracking ID
- **Tracking Events**: Track envelope journey through post offices
- **Tracking API**: View tracking information via `/api/tracking/:envelope_id`
- **Share Tracking**: Senders can share tracking numbers with recipients

### 2. Delivery Notifications
- **Scheduled Notifications**: Automatic notifications 1 day before delivery
- **Out for Delivery**: Notifications when envelope is out for delivery
- **Delivery Confirmation**: Notifications when envelope is delivered
- **Notification API**: View notifications via `/api/notifications`

### 3. User Settings
- **Auto-tracking**: Option to automatically add tracking to all envelopes
- **Delivery Confirmation**: Option to require delivery confirmation
- **IMAP Settings**: Configure IMAP access for email clients
- **Settings API**: Manage settings via `/api/settings`

### 4. Admin Panel
- **Admin Dashboard**: View system statistics (users, relays, envelopes)
- **User Management**: View and manage users
- **Relay Management**: View and manage post office relays
- **Envelope Management**: View all envelopes in the system
- **Admin API**: Access via `/api/admin/*` endpoints

### 5. Email Validation
- **Domain Validation**: Only allow signups from approved email domains
- **Supported Domains**: gmail.com, outlook.com, hotmail.com, live.com, yahoo.com, aol.com, icloud.com, protonmail.com, protonmail.ch
- **Validation API**: Validate email via `auth.ValidateEmail()`

## Database Schema

### New Tables

#### `user_settings`
- User preferences for tracking, delivery confirmation, and IMAP
- Fields: `user_id`, `auto_add_tracking`, `require_delivery_confirmation`, `imap_host`, `imap_port`, `imap_password_hash`

#### `tracking_events`
- Track envelope journey through post offices
- Fields: `id`, `envelope_id`, `event_type`, `location`, `notes`, `timestamp`, `notify_sender`, `notify_recipient`

#### `notifications`
- User notifications for tracking and delivery events
- Fields: `id`, `user_id`, `envelope_id`, `type`, `message`, `read`, `created_at`

### Modified Tables

#### `envelopes`
- Added: `tracking_id` (unique tracking number)
- Added: `tracking_shared` (whether tracking is shared with recipient)
- Added: `delivery_window_start` (delivery window start time)
- Added: `delivery_window_end` (delivery window end time)
- Added: `delivery_notification_sent` (whether notification was sent)

#### `users`
- Added: `email` (external email for notifications)
- Added: `is_admin` (admin flag)

## API Endpoints

### Tracking
- `GET /api/tracking/:envelope_id` - Get tracking information for an envelope
- `POST /api/tracking/:envelope_id/share` - Share tracking with recipient

### Notifications
- `GET /api/notifications` - Get user notifications
- `POST /api/notifications/read` - Mark notification as read

### Settings
- `GET /api/settings` - Get user settings
- `POST /api/settings` - Update user settings

### Admin
- `GET /api/admin/stats` - Get system statistics
- `GET /api/admin/users` - Get all users
- `GET /api/admin/relays` - Get all relays
- `GET /api/admin/letters` - Get all letters
- `GET /api/users/me` - Get current user info

## Frontend Pages

### `dashboard.html`
- Updated navigation bar with email display
- Added settings and admin panel buttons
- Settings button (visible to all users)
- Admin panel button (visible only to admins)

### `settings.html`
- Auto-tracking toggle
- Delivery confirmation toggle
- IMAP configuration (host, port, password)
- Save settings functionality

### `admin.html`
- System statistics dashboard
- User management table
- Relay management table
- Letter management table
- Tab-based navigation

## Email Validation

Users can only sign up with email addresses from approved domains:
- gmail.com
- outlook.com
- hotmail.com
- live.com
- yahoo.com
- aol.com
- icloud.com
- protonmail.com
- protonmail.ch

## Deployment

### Database Migration
```bash
# Run migration on the server
sudo -u postgres psql -d snailmail -f migrations/006_add_tracking_notifications.sql

# Grant permissions to snailmail user
sudo -u postgres psql -d snailmail -c "GRANT ALL PRIVILEGES ON TABLE user_settings TO snailmail;"
sudo -u postgres psql -d snailmail -c "GRANT ALL PRIVILEGES ON TABLE tracking_events TO snailmail;"
sudo -u postgres psql -d snailmail -c "GRANT ALL PRIVILEGES ON TABLE notifications TO snailmail;"
sudo -u postgres psql -d snailmail -c "GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO snailmail;"
```

### Build and Deploy
```bash
# Build the application
cd /home/binarygeek119/Projects/snailmail
go build -o snailmail ./cmd/site

# Copy to server
gcloud compute scp snailmail snailmail-server:/tmp/ --zone=us-central1-a
gcloud compute scp web/site/admin.html web/site/settings.html snailmail-server:/tmp/ --zone=us-central1-a

# SSH to server and deploy
gcloud compute ssh snailmail-server --zone=us-central1-a
sudo mv /tmp/snailmail /opt/snailmail/app/snailmail
sudo chmod +x /opt/snailmail/app/snailmail
sudo mv /tmp/admin.html /tmp/settings.html /opt/snailmail/app/web/site/
sudo chown snailmail:snailmail /opt/snailmail/app/web/site/*.html
sudo systemctl restart snailmail
```

## Testing

### Test Settings API
```bash
curl -s http://35.202.175.26/api/settings \
  -H "Authorization: Bearer <user_id>" | python3 -m json.tool
```

### Test Admin API
```bash
curl -s http://35.202.175.26/api/admin/stats \
  -H "Authorization: Bearer <admin_user_id>" | python3 -m json.tool
```

### Test Tracking API
```bash
curl -s http://35.202.175.26/api/tracking/<envelope_id> \
  -H "Authorization: Bearer <user_id>" | python3 -m json.tool
```

## Next Steps

1. **Delivery Worker**: Implement background worker to process delivery notifications
2. **Tracking Updates**: Implement automatic tracking event creation when envelopes move through relays
3. **Email Notifications**: Send email notifications for tracking updates
4. **IMAP Server**: Implement actual IMAP server for email client access
5. **Admin UI Improvements**: Add more admin features (user suspension, relay management, etc.)

## Security Notes

- All API endpoints require authentication via Bearer token
- Admin endpoints check for admin privileges
- Tracking information is only accessible to sender and recipient
- User settings are private to each user
- IMAP passwords are hashed before storage

## Architecture

```
Frontend (HTML/JS)
    ↓
API Endpoints (Go)
    ↓
Database (PostgreSQL)
    ↓
Background Workers (Go)
    ↓
External Services (Email, etc.)
```

## License

MIT License
