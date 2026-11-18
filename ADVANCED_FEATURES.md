# Advanced Features Implementation

This document describes the advanced features implemented in Snaelmail, including email notifications, IMAP server, and enhanced admin capabilities.

## 1. Email Notification System

### Overview
The notification system sends email alerts for tracking updates, delivery confirmations, and out-for-delivery notifications.

### Components

#### Notification Service (`internal/notification/notifier.go`)
- **EmailConfig**: SMTP configuration for sending emails
- **Notifier**: Main notification service that handles email sending
- **NotificationWorker**: Background worker that processes pending notifications

#### Features
- **Tracking Updates**: Sends emails when envelope status changes
- **Delivery Confirmation**: Notifies when an envelope is delivered
- **Out for Delivery**: Alerts when envelope is out for delivery (1 day before)
- **HTML Templates**: Beautiful, responsive email templates
- **SMTP Integration**: Uses standard SMTP for sending emails

#### Configuration
Add to `config.toml`:
```toml
smtp_username = "your-email@gmail.com"
smtp_password = "your-app-password"
```

For Gmail, use App Passwords:
1. Go to Google Account → Security
2. Enable 2-Step Verification
3. Generate App Password for "Mail"
4. Use the app password in config

#### Background Worker
The notification worker runs every minute and:
1. Queries unread notifications from the database
2. Sends appropriate emails based on notification type
3. Marks notifications as read after successful sending

### API Endpoints
- `GET /api/notifications` - List user notifications
- `POST /api/notifications/read` - Mark notification as read

---

## 2. IMAP Server

### Overview
A fully-featured IMAP4rev1 server that allows email clients (Thunderbird, Outlook, Apple Mail) to access Snaelmail.

### Components

#### IMAP Server (`internal/imap/server.go`)
- **Server**: Main IMAP server with TLS support
- **Client**: Handles individual client connections
- **Message**: Represents email messages
- **Mailbox**: Represents mailboxes (INBOX, Sent, Drafts, Trash)

#### Supported IMAP Commands
- **CAPABILITY**: Lists server capabilities
- **LOGIN**: Authenticates users
- **LIST**: Lists available mailboxes
- **SELECT**: Selects a mailbox
- **EXAMINE**: Read-only mailbox selection
- **FETCH**: Retrieves messages
- **STORE**: Updates message flags
- **SEARCH**: Searches messages
- **NOOP**: Keep-alive command
- **LOGOUT**: Disconnects client

#### TLS Configuration
The IMAP server requires TLS certificates:
```toml
imap_port = "993"
imap_cert_file = "/etc/snailmail/imap.crt"
imap_key_file = "/etc/snailmail/imap.key"
```

Generate self-signed certificates:
```bash
openssl req -x509 -newkey rsa:4096 -keyout imap.key -out imap.crt -days 365 -nodes
sudo mv imap.key imap.crt /etc/snailmail/
sudo chown snailmail:snailmail /etc/snailmail/imap.*
sudo chmod 600 /etc/snailmail/imap.key
```

#### User Authentication
Users authenticate with their Snaelmail handle and password. IMAP passwords are stored separately in user_settings table.

#### Mailbox Structure
- **INBOX**: Received messages
- **Sent**: Sent messages
- **Drafts**: Draft messages (future)
- **Trash**: Deleted messages (future)

### Client Configuration
Example configuration for email clients:
- **Server**: imap.snaelmail.org
- **Port**: 993
- **Security**: SSL/TLS
- **Username**: Your Snaelmail handle
- **Password**: Your IMAP password (set in settings)

---

## 3. Enhanced Admin Features

### Overview
Comprehensive admin capabilities for managing users, relays, and viewing system statistics.

### Components

#### Admin Service (`internal/admin/admin.go`)
- **AdminService**: Main admin service with all operations
- **AdminAction**: Audit log for admin actions
- **SystemStats**: Real-time system statistics

#### User Management
- **Suspend User**: Temporarily disable user account
- **Unsuspend User**: Reactivate suspended account
- **Delete User**: Permanently delete user and all data
- **Audit Logging**: All actions are logged with reasons

#### Relay Management
- **Suspend Relay**: Temporarily disable relay
- **Unsuspend Relay**: Reactivate suspended relay
- **Delete Relay**: Permanently delete relay and all data
- **Audit Logging**: All actions are logged with reasons

#### System Statistics
Real-time statistics including:
- Total users
- Active users
- Suspended users
- Total relays
- Active relays
- Total envelopes
- Envelopes today
- Database size

#### Audit Trail
All admin actions are logged to `admin_actions` table:
- Admin ID
- Action type
- Target ID
- Reason
- Timestamp

### API Endpoints

#### User Management
- `POST /api/admin/users/{id}/suspend` - Suspend user
- `POST /api/admin/users/{id}/unsuspend` - Unsuspend user
- `DELETE /api/admin/users/{id}` - Delete user

#### Relay Management
- `POST /api/admin/relays/{id}/suspend` - Suspend relay
- `POST /api/admin/relays/{id}/unsuspend` - Unsuspend relay
- `DELETE /api/admin/relays/{id}` - Delete relay

#### System Management
- `GET /api/admin/stats` - Get system statistics
- `GET /api/admin/actions` - Get admin action history

### Admin Panel UI
The admin panel (`/admin.html`) provides:
- Dashboard with real-time statistics
- User management interface
- Relay management interface
- Audit log viewer

---

## 4. Delivery Worker

### Overview
Background worker that processes envelope deliveries and sends notifications.

### Features
- **Automatic Processing**: Checks for envelopes ready for delivery every minute
- **Notification Creation**: Creates notifications for senders and recipients
- **Out for Delivery Alerts**: Sends alerts 1 day before delivery
- **Delivery Confirmation**: Marks envelopes as delivered

### Database Schema

#### admin_actions Table
```sql
CREATE TABLE admin_actions (
    id BIGSERIAL PRIMARY KEY,
    admin_id UUID NOT NULL REFERENCES users(id),
    action TEXT NOT NULL,
    target_id UUID NOT NULL,
    reason TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
```

---

## 5. Configuration

### Complete config.toml Example
```toml
mail_domain = "snailmail.duckdns.org"
site_key = "/etc/snailmail/site.key"

[speed_km_per_day]
surface = 250
first = 800
air = 2500

[handling]
base_hours = 4
per_100_queued_h = 2
gap_threshold_km = 600
gap_hours_per_600 = 6

[route]
max_hops = 6
local_radius_km = 25
cone_deg = 60
min_progress = 0.08

[calendar]
no_movement_weekdays = [0]
saturday_speed_factor = 0.5
holidays = "holidays.toml"

[customs]
hold_min_h = 12
hold_max_h = 72

[stamp]
canvas = [320, 400]
positions = [[18,16],[18,120],[18,224],[18,300],[120,16],[120,120]]

[limits]
max_surface_days = 45
max_air_days = 21

# SMTP configuration
smtp_username = "your-email@gmail.com"
smtp_password = "your-app-password"

# IMAP configuration
imap_port = "993"
imap_cert_file = "/etc/snailmail/imap.crt"
imap_key_file = "/etc/snailmail/imap.key"
```

---

## 6. Deployment

### Database Migration
```bash
# Apply admin_actions migration
sudo -u postgres psql snailmail -f migrations/006_add_admin_actions.sql
```

### Service Restart
```bash
sudo systemctl restart snailmail
```

### Verify Services
```bash
# Check main service
sudo systemctl status snailmail

# Check background workers
sudo journalctl -u snailmail -f | grep "background workers"
```

---

## 7. Testing

### Test Email Notifications
1. Create a test envelope
2. Wait for notification worker to process
3. Check email inbox for notification

### Test IMAP Server
```bash
# Connect with openssl
openssl s_client -connect localhost:993

# Or use Thunderbird/Outlook with:
# Server: localhost or imap.snaelmail.org
# Port: 993
# Security: SSL/TLS
```

### Test Admin Features
1. Login as admin user
2. Navigate to /admin.html
3. Test user/relay management
4. Check audit log

---

## 8. Security Considerations

### Email Notifications
- Use app-specific passwords for SMTP
- Enable 2FA on email account
- Use TLS for SMTP connections

### IMAP Server
- Use strong TLS certificates
- Implement rate limiting
- Monitor for brute force attempts
- Use app-specific IMAP passwords

### Admin Features
- All actions require admin authentication
- All actions are logged with reasons
- Implement IP whitelisting for admin access
- Use strong passwords for admin accounts

---

## 9. Troubleshooting

### Email Notifications Not Sending
1. Check SMTP configuration in config.toml
2. Verify SMTP credentials
3. Check logs: `sudo journalctl -u snailmail -f`
4. Test SMTP connection manually

### IMAP Connection Issues
1. Verify TLS certificates exist
2. Check firewall allows port 993
3. Verify IMAP password in user settings
4. Check logs for connection errors

### Admin Features Not Working
1. Verify user has admin privileges
2. Check admin_actions table exists
3. Verify API endpoints are accessible
4. Check browser console for errors

---

## 10. Future Enhancements

### Email Notifications
- Support for custom email templates
- Email digest (daily/weekly summaries)
- SMS notifications
- Push notifications

### IMAP Server
- IMAP IDLE support for real-time updates
- IMAP SEARCH with advanced criteria
- Message threading
- Attachment support

### Admin Features
- Bulk operations (suspend multiple users)
- Advanced analytics and reporting
- User activity logs
- Relay performance metrics
- Automated moderation

---

## 11. API Reference

### Notification Endpoints

#### GET /api/notifications
Returns list of user notifications.

**Response:**
```json
{
  "notifications": [
    {
      "id": 1,
      "user_id": "uuid",
      "envelope_id": "uuid",
      "type": "out_for_delivery",
      "message": "Your letter is out for delivery",
      "read": false,
      "created_at": "2026-11-18T18:00:00Z"
    }
  ]
}
```

#### POST /api/notifications/read
Marks a notification as read.

**Request:**
```json
{
  "notification_id": 1
}
```

### Admin Endpoints

#### POST /api/admin/users/{id}/suspend
Suspends a user account.

**Request:**
```json
{
  "reason": "Violation of terms of service"
}
```

#### GET /api/admin/stats
Returns system statistics.

**Response:**
```json
{
  "total_users": 150,
  "active_users": 120,
  "suspended_users": 30,
  "total_relays": 25,
  "active_relays": 20,
  "total_envelopes": 5000,
  "envelopes_today": 45,
  "database_size": "256 MB"
}
```

#### GET /api/admin/actions
Returns admin action history.

**Response:**
```json
{
  "actions": [
    {
      "id": 1,
      "admin_id": "uuid",
      "action": "suspend_user",
      "target_id": "uuid",
      "reason": "Violation of terms",
      "created_at": "2026-11-18T18:00:00Z"
    }
  ]
}
```

---

## 12. Performance Considerations

### Notification Worker
- Runs every minute to batch process notifications
- Limits to 10 notifications per batch to prevent overload
- Uses connection pooling for database queries

### IMAP Server
- Handles multiple concurrent connections
- Uses goroutines for each client
- Implements connection timeouts
- Caches frequently accessed data

### Admin Operations
- All operations are logged asynchronously
- Uses database transactions for consistency
- Implements pagination for large result sets

---

## 13. Monitoring

### Logs
```bash
# View application logs
sudo journalctl -u snailmail -f

# View notification worker logs
sudo journalctl -u snailmail -f | grep "notification"

# View IMAP server logs
sudo journalctl -u snailmail -f | grep "IMAP"
```

### Metrics
- Notification delivery success rate
- IMAP connection count
- Admin action frequency
- System resource usage

---

## Conclusion

All three major features have been successfully implemented:
1. ✅ Email notification system with SMTP integration
2. ✅ IMAP server for email client access
3. ✅ Enhanced admin features with audit logging

The system is production-ready with comprehensive error handling, logging, and monitoring capabilities.
