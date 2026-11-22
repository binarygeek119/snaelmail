ALTER TABLE user_settings ADD COLUMN IF NOT EXISTS pop_enabled boolean NOT NULL DEFAULT false;
ALTER TABLE user_settings ADD COLUMN IF NOT EXISTS pop_password_hash text;
ALTER TABLE user_settings ADD COLUMN IF NOT EXISTS pop_password_salt text;
ALTER TABLE user_settings ADD COLUMN IF NOT EXISTS pop_host text NOT NULL DEFAULT 'pop.snaelmail.org';
ALTER TABLE user_settings ADD COLUMN IF NOT EXISTS pop_port integer NOT NULL DEFAULT 995;
