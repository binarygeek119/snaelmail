-- Add tracking support to envelopes table
-- Migration: P3 - Add tracking to envelopes

-- Add tracking fields to envelopes
ALTER TABLE envelopes ADD COLUMN IF NOT EXISTS tracking_number citext;
ALTER TABLE envelopes ADD COLUMN IF NOT EXISTS tracking_data jsonb DEFAULT '[]';

-- Create unique index on tracking_number for fast lookups
CREATE UNIQUE INDEX IF NOT EXISTS idx_envelopes_tracking_number ON envelopes(tracking_number) WHERE tracking_number IS NOT NULL;

-- Add index for querying by sender and tracking
CREATE INDEX IF NOT EXISTS idx_envelopes_sender_tracking ON envelopes(sender_id, tracking_number) WHERE tracking_number IS NOT NULL;
