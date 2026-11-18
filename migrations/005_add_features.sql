-- Add tracking support
ALTER TABLE envelopes ADD COLUMN IF NOT EXISTS tracking_id TEXT UNIQUE;
ALTER TABLE envelopes ADD COLUMN IF NOT EXISTS tracking_shared BOOLEAN DEFAULT false;

-- Add admin flag to users
ALTER TABLE users ADD COLUMN IF NOT EXISTS is_admin BOOLEAN DEFAULT false;

-- Create user settings table
CREATE TABLE IF NOT EXISTS user_settings (
  user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  auto_add_tracking BOOLEAN DEFAULT false,
  require_delivery_confirmation BOOLEAN DEFAULT false,
  imap_password_hash TEXT,
  imap_host TEXT DEFAULT 'imap.snaelmail.org',
  imap_port INTEGER DEFAULT 993,
  created_at TIMESTAMPTZ DEFAULT now(),
  updated_at TIMESTAMPTZ DEFAULT now()
);

-- Create tracking events table
CREATE TABLE IF NOT EXISTS tracking_events (
  id BIGSERIAL PRIMARY KEY,
  envelope_id UUID REFERENCES envelopes(id) ON DELETE CASCADE,
  event_type TEXT NOT NULL CHECK (event_type IN ('queued', 'in_transit', 'out_for_delivery', 'delivered', 'failed')),
  location TEXT,
  notes TEXT,
  timestamp TIMESTAMPTZ DEFAULT now(),
  notify_sender BOOLEAN DEFAULT false,
  notify_recipient BOOLEAN DEFAULT false
);

CREATE INDEX IF NOT EXISTS idx_tracking_events_envelope ON tracking_events(envelope_id);
CREATE INDEX IF NOT EXISTS idx_tracking_events_timestamp ON tracking_events(timestamp);

-- Create notifications table
CREATE TABLE IF NOT EXISTS notifications (
  id BIGSERIAL PRIMARY KEY,
  user_id UUID REFERENCES users(id) ON DELETE CASCADE,
  envelope_id UUID REFERENCES envelopes(id) ON DELETE CASCADE,
  type TEXT NOT NULL CHECK (type IN ('delivery_scheduled', 'out_for_delivery', 'delivered', 'tracking_update')),
  message TEXT NOT NULL,
  read BOOLEAN DEFAULT false,
  created_at TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_notifications_user ON notifications(user_id, read);
CREATE INDEX IF NOT EXISTS idx_notifications_created ON notifications(created_at);

-- Add delivery window to envelopes
ALTER TABLE envelopes ADD COLUMN IF NOT EXISTS delivery_window_start TIMESTAMPTZ;
ALTER TABLE envelopes ADD COLUMN IF NOT EXISTS delivery_window_end TIMESTAMPTZ;
ALTER TABLE envelopes ADD COLUMN IF NOT EXISTS delivery_notification_sent BOOLEAN DEFAULT false;
