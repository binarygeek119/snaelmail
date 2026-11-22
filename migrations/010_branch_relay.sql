-- Branch relay support: asset filenames and relay API key for self-hosted branch updates

ALTER TABLE page_assets ADD COLUMN IF NOT EXISTS filename text;

ALTER TABLE relays ADD COLUMN IF NOT EXISTS branch_api_key text;
CREATE INDEX IF NOT EXISTS relays_branch_api_key_idx ON relays (branch_api_key) WHERE branch_api_key IS NOT NULL;
