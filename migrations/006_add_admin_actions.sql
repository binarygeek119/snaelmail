-- Migration: Add admin actions table for audit logging

CREATE TABLE IF NOT EXISTS admin_actions (
    id BIGSERIAL PRIMARY KEY,
    admin_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    action TEXT NOT NULL,
    target_id UUID NOT NULL,
    reason TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_admin_actions_admin_id ON admin_actions(admin_id);
CREATE INDEX IF NOT EXISTS idx_admin_actions_created_at ON admin_actions(created_at);
CREATE INDEX IF NOT EXISTS idx_admin_actions_action ON admin_actions(action);
