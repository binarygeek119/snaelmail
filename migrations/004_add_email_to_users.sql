-- Add email column to users table
ALTER TABLE users ADD COLUMN IF NOT EXISTS email citext UNIQUE;

-- Create index for faster email lookups
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
