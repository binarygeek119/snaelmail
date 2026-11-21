-- Add user settings table for email defaults
CREATE TABLE IF NOT EXISTS user_settings (
  user_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  
  -- Email defaults
  auto_add_tracking boolean NOT NULL DEFAULT false,
  request_delivery_confirmation boolean NOT NULL DEFAULT false,
  
  -- IMAP settings
  imap_enabled boolean NOT NULL DEFAULT false,
  imap_password_hash text,
  imap_password_salt text,
  imap_host text NOT NULL DEFAULT 'imap.snaelmail.org',
  imap_port integer NOT NULL DEFAULT 993,
  
  -- Display preferences
  theme text NOT NULL DEFAULT 'light' CHECK (theme IN ('light', 'dark', 'auto')),
  default_view text NOT NULL DEFAULT 'inbox' CHECK (default_view IN ('inbox', 'outbox')),
  
  -- Notification preferences
  notify_on_delivery boolean NOT NULL DEFAULT true,
  notify_on_tracking_update boolean NOT NULL DEFAULT true,
  
  -- Timestamps
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

-- Create index for faster lookups
CREATE INDEX IF NOT EXISTS idx_user_settings_user_id ON user_settings(user_id);

-- Insert default settings for existing users
INSERT INTO user_settings (user_id)
SELECT id FROM users
ON CONFLICT (user_id) DO NOTHING;
