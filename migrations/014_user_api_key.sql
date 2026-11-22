ALTER TABLE users ADD COLUMN IF NOT EXISTS api_key text;
CREATE UNIQUE INDEX IF NOT EXISTS users_api_key_idx ON users(api_key) WHERE api_key IS NOT NULL;
