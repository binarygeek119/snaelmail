-- Add user location for delivery timing
ALTER TABLE users ADD COLUMN IF NOT EXISTS city text;
ALTER TABLE users ADD COLUMN IF NOT EXISTS state text;
ALTER TABLE users ADD COLUMN IF NOT EXISTS country text;
