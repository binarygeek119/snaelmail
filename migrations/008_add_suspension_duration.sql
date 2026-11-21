-- Add suspension duration support
ALTER TABLE users ADD COLUMN IF NOT EXISTS suspended_until TIMESTAMPTZ;
ALTER TABLE users ADD COLUMN IF NOT EXISTS suspension_reason TEXT;

-- Create index for finding suspended users whose suspension has expired
CREATE INDEX IF NOT EXISTS idx_users_suspended_until ON users(suspended_until) WHERE suspended_until IS NOT NULL;

-- Add to admin_actions for tracking
CREATE INDEX IF NOT EXISTS idx_admin_actions_target ON admin_actions(target_id);
