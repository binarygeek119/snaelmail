-- Received emails (inbound mail storage)
CREATE TABLE IF NOT EXISTS received_emails (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  from_address text NOT NULL,
  to_address text NOT NULL,
  subject text,
  body_text text,
  body_html text,
  headers jsonb,
  spam_score double precision NOT NULL DEFAULT 0,
  is_spam boolean NOT NULL DEFAULT false,
  is_read boolean NOT NULL DEFAULT false,
  folder text NOT NULL DEFAULT 'inbox',
  received_at timestamptz NOT NULL DEFAULT now(),
  read_at timestamptz
);

CREATE INDEX IF NOT EXISTS received_emails_user_idx ON received_emails(user_id);
CREATE INDEX IF NOT EXISTS received_emails_folder_idx ON received_emails(user_id, folder);
CREATE INDEX IF NOT EXISTS received_emails_received_idx ON received_emails(received_at DESC);

-- Email attachments
CREATE TABLE IF NOT EXISTS email_attachments (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  email_id uuid NOT NULL REFERENCES received_emails(id) ON DELETE CASCADE,
  filename text NOT NULL,
  content_type text NOT NULL,
  size_bytes bigint NOT NULL,
  storage_path text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS email_attachments_email_idx ON email_attachments(email_id);

-- Outbound delivery queue
CREATE TABLE IF NOT EXISTS outbound_queue (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  from_address text NOT NULL,
  to_address text NOT NULL,
  subject text NOT NULL,
  body text NOT NULL,
  dkim_signature text,
  status text NOT NULL DEFAULT 'queued',
  attempts int NOT NULL DEFAULT 0,
  last_attempt timestamptz,
  next_retry timestamptz,
  error_message text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS outbound_queue_status_idx ON outbound_queue(status);
CREATE INDEX IF NOT EXISTS outbound_queue_next_retry_idx ON outbound_queue(next_retry);
