CREATE TABLE IF NOT EXISTS stamp_versions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    relay_id uuid NOT NULL REFERENCES relays(id) ON DELETE CASCADE,
    version_num integer NOT NULL,
    sha256 bytea NOT NULL,
    format text NOT NULL CHECK (format IN ('svg','png')),
    svg_content text,
    png_content bytea,
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','approved','rejected','retired')),
    is_current boolean NOT NULL DEFAULT false,
    stamp_date date NOT NULL DEFAULT CURRENT_DATE,
    event text,
    honor text,
    info text,
    created_at timestamptz NOT NULL DEFAULT now(),
    reviewed_at timestamptz,
    reviewed_by uuid REFERENCES users(id),
    UNIQUE (relay_id, version_num)
);
CREATE INDEX IF NOT EXISTS stamp_versions_relay_id_idx ON stamp_versions(relay_id);
CREATE INDEX IF NOT EXISTS stamp_versions_status_idx ON stamp_versions(status);
CREATE INDEX IF NOT EXISTS stamp_versions_relay_current_idx ON stamp_versions(relay_id, is_current) WHERE is_current = true;
