ALTER TABLE relays ADD COLUMN IF NOT EXISTS owner_user_id uuid REFERENCES users(id);
CREATE INDEX IF NOT EXISTS relays_owner_user_id_idx ON relays(owner_user_id);
