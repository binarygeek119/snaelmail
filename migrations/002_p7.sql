-- P7: Add transparency log and moderation tables

-- Moderation log table
CREATE TABLE IF NOT EXISTS moderation_log (
    id UUID PRIMARY KEY,
    action TEXT NOT NULL,
    target_id UUID NOT NULL,
    target_type TEXT NOT NULL,
    admin_id UUID NOT NULL,
    reason TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_moderation_log_target ON moderation_log(target_id);
CREATE INDEX IF NOT EXISTS idx_moderation_log_created ON moderation_log(created_at DESC);

-- Transparency log tables
CREATE TABLE IF NOT EXISTS tlog_leaves (
    idx BIGINT PRIMARY KEY,
    leaf BYTEA NOT NULL
);

CREATE TABLE IF NOT EXISTS tlog_roots (
    day DATE PRIMARY KEY,
    root BYTEA NOT NULL,
    sig BYTEA NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_tlog_roots_day ON tlog_roots(day DESC);
