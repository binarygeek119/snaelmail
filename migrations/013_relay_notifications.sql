ALTER TABLE page_assets ADD COLUMN IF NOT EXISTS status text NOT NULL DEFAULT 'approved';
ALTER TABLE page_assets ADD COLUMN IF NOT EXISTS review_note text;

CREATE TABLE IF NOT EXISTS relay_notifications (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  relay_id uuid NOT NULL REFERENCES relays(id) ON DELETE CASCADE,
  type text NOT NULL,
  message text NOT NULL,
  severity text NOT NULL DEFAULT 'info',
  read_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS relay_notifications_relay_idx ON relay_notifications(relay_id, created_at DESC);
