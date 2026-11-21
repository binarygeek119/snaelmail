-- Snailmail P0: initial schema
-- Idempotent via IF NOT EXISTS where possible.

DO $$ BEGIN
  -- citext extension for case-insensitive text
  IF NOT EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'citext') THEN
    CREATE EXTENSION citext;
  END IF;
END $$;

-- §5 relays
CREATE TABLE IF NOT EXISTS relays (
  id uuid PRIMARY KEY,
  hostname citext UNIQUE NOT NULL,
  pubkey bytea NOT NULL,
  key_fpr bytea UNIQUE NOT NULL,
  city text NOT NULL,
  region text NOT NULL,
  cc char(2) NOT NULL,
  lat double precision NOT NULL,
  lon double precision NOT NULL,
  tz text NOT NULL,
  stamp_svg text NOT NULL,
  stamp_png bytea NOT NULL,
  stamp_sha256 bytea NOT NULL,
  plate_json jsonb,
  plate_sig bytea,
  status text NOT NULL DEFAULT 'pending'
    CHECK (status IN ('pending','active','unverified','throttled','retired','suspended')),
  official bool NOT NULL DEFAULT false,
  paused bool NOT NULL DEFAULT false,
  central bool NOT NULL DEFAULT false,
  opened_at timestamptz,
  retired_at timestamptz,
  last_txt_check timestamptz,
  letters_passed bigint NOT NULL DEFAULT 0,
  complaints_30d int NOT NULL DEFAULT 0
);

-- §5 users
CREATE TABLE IF NOT EXISTS users (
  id uuid PRIMARY KEY,
  handle citext UNIQUE NOT NULL,
  address citext UNIQUE NOT NULL,
  pass_hash text NOT NULL,
  display_name text,
  home_relay_id uuid REFERENCES relays(id),
  home_relay_changed_at timestamptz,
  status text NOT NULL DEFAULT 'unverified'
    CHECK (status IN ('unverified','verified','suspended','closed')),
  storage_bytes bigint NOT NULL DEFAULT 0,
  letters_today int NOT NULL DEFAULT 0,
  quota_day date NOT NULL DEFAULT current_date,
  created_at timestamptz NOT NULL DEFAULT now()
);

-- §6 verifications
CREATE TABLE IF NOT EXISTS verifications (
  id uuid PRIMARY KEY,
  user_id uuid REFERENCES users(id) ON DELETE CASCADE,
  code_hash bytea NOT NULL,
  kind text NOT NULL,
  expires_at timestamptz NOT NULL,
  attempts int NOT NULL DEFAULT 0,
  consumed_at timestamptz
);

-- §11 relay challenges
CREATE TABLE IF NOT EXISTS relay_challenges (
  relay_id uuid REFERENCES relays(id) ON DELETE CASCADE,
  token text NOT NULL,
  expires_at timestamptz NOT NULL,
  PRIMARY KEY (relay_id, token)
);

-- §5 envelopes
CREATE TABLE IF NOT EXISTS envelopes (
  id uuid PRIMARY KEY,
  sender_id uuid,
  recipient_id uuid,
  direction text NOT NULL CHECK (direction IN ('local','inbound','outbound')),
  class text NOT NULL CHECK (class IN ('surface','first','air','registered')),
  subject text NOT NULL,
  ciphertext bytea NOT NULL,
  size_bytes int NOT NULL,
  route jsonb NOT NULL,
  total_km double precision NOT NULL,
  queued_at timestamptz NOT NULL,
  deliver_at timestamptz NOT NULL,
  state text NOT NULL DEFAULT 'queued'
    CHECK (state IN ('queued','in_transit','delivered','returned')),
  external_to citext,
  bounce_token bytea,
  log_leaf bytea
);

-- §5 hops
CREATE TABLE IF NOT EXISTS hops (
  envelope_id uuid REFERENCES envelopes(id) ON DELETE CASCADE,
  idx int NOT NULL,
  relay_id uuid NOT NULL,
  stamp_at timestamptz NOT NULL,
  stamped_at timestamptz,
  prev_hash bytea,
  sig bytea,
  plate_sha256 bytea,
  PRIMARY KEY (envelope_id, idx)
);
CREATE INDEX IF NOT EXISTS hops_due ON hops (stamp_at) WHERE stamped_at IS NULL;

-- §5 attachments
CREATE TABLE IF NOT EXISTS attachments (
  id uuid PRIMARY KEY,
  envelope_id uuid REFERENCES envelopes(id) ON DELETE CASCADE,
  filename text NOT NULL,
  mime text NOT NULL,
  size_bytes int NOT NULL,
  ciphertext bytea NOT NULL
);

-- §5 relay_daily
CREATE TABLE IF NOT EXISTS relay_daily (
  relay_id uuid,
  day date,
  stamped int DEFAULT 0,
  first_hop int DEFAULT 0,
  middle_hop int DEFAULT 0,
  last_hop int DEFAULT 0,
  returned int DEFAULT 0,
  complaints int DEFAULT 0,
  distance_sum bigint DEFAULT 0,
  distance_n int DEFAULT 0,
  class_counts jsonb DEFAULT '{}',
  PRIMARY KEY (relay_id, day)
);

-- §5 postmark_events
CREATE TABLE IF NOT EXISTS postmark_events (
  id bigserial PRIMARY KEY,
  relay_id uuid,
  envelope_id uuid,
  hop_idx int,
  stamped_at timestamptz,
  origin_city text,
  dest_city text,
  class text
);

-- §14 pages
CREATE TABLE IF NOT EXISTS pages (
  relay_id uuid PRIMARY KEY,
  revision int NOT NULL,
  manifest jsonb NOT NULL,
  sig bytea NOT NULL,
  status text NOT NULL DEFAULT 'draft'
    CHECK (status IN ('draft','review','live','rejected')),
  published_at timestamptz,
  review_note text
);

-- §14 page_assets
CREATE TABLE IF NOT EXISTS page_assets (
  id uuid PRIMARY KEY,
  relay_id uuid,
  kind text,
  w int,
  h int,
  bytes bytea,
  sha256 bytea,
  ord int
);

-- §16 transparency log
CREATE TABLE IF NOT EXISTS tlog_leaves (
  idx bigint PRIMARY KEY,
  leaf bytea NOT NULL
);

CREATE TABLE IF NOT EXISTS tlog_roots (
  day date PRIMARY KEY,
  root bytea NOT NULL,
  sig bytea NOT NULL
);

-- §1 invariant: opened_at is set once on first approval and never moves
DO $$ BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_trigger WHERE tgname = 'relays_opened_immutable' AND tgrelid = 'relays'::regclass
  ) THEN
    CREATE FUNCTION relays_opened_immutable_fn() RETURNS trigger LANGUAGE plpgsql AS
    $fn$
    BEGIN
      IF OLD.opened_at IS NOT NULL AND NEW.opened_at IS DISTINCT FROM OLD.opened_at THEN
        RAISE EXCEPTION 'opened_at is immutable once set (relay %)', OLD.id;
      END IF;
      RETURN NEW;
    END;
    $fn$;
    CREATE TRIGGER relays_opened_immutable
      BEFORE UPDATE ON relays
      FOR EACH ROW EXECUTE FUNCTION relays_opened_immutable_fn();
  END IF;
END $$;
