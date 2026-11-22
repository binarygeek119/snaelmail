ALTER TABLE users ADD COLUMN IF NOT EXISTS is_mod boolean NOT NULL DEFAULT false;
ALTER TABLE relays ADD COLUMN IF NOT EXISTS suspended_until timestamptz;
CREATE INDEX IF NOT EXISTS relays_suspended_until_idx ON relays(suspended_until);
