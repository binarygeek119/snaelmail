# Snaelmail - Full Email Service Plan
## From "Deliberately Slow Email" to "Independent Gmail-like Service"

## Vision
Transform Snaelmail from a novelty "slow email" service into a fully independent email provider that can send and receive email directly without relying on external SMTP relays (Gmail, SendGrid, Mailgun). Users get `@snaelmail.org` addresses and can email anyone on the internet.

## Current Architecture
- **Stack**: Go 1.23, PostgreSQL 16, html/template, no ORM
- **Deployment**: Google Cloud e2-micro (1GB RAM, free tier)
- **Domain**: snaelmail.org (Cloudflare DNS)
- **Existing**: SMTP receiver, IMAP server, web UI, user management, admin panel

## Core Problem
The service currently relies on external SMTP (Gmail/SendGrid) for outbound email. To be independent, we need:
1. **Outbound SMTP** - Direct delivery to recipient MX servers
2. **Inbound SMTP** - Accept mail from other mail servers
3. **DNS records** - MX, SPF, DKIM, DMARC, PTR
4. **Mail storage** - Store received emails for IMAP/web access
5. **Security** - TLS, authentication, spam filtering

---

## Phase 1: DNS & Infrastructure (Week 1)
### Goal: Configure DNS so other mail servers can find and trust us

### 1.1 MX Records
```
snaelmail.org.  MX  10  mail.snaelmail.org.
```

### 1.2 SPF Record
```
snaelmail.org.  TXT  "v=spf1 mx a:mail.snaelmail.org ~all"
```

### 1.3 DKIM
- Generate 2048-bit RSA key pair
- Publish public key as DNS TXT record:
```
default._domainkey.snaelmail.org.  TXT  "v=DKIM1; k=rsa; p=<public_key>"
```
- Sign all outbound emails with private key

### 1.4 DMARC
```
_dmarc.snaelmail.org.  TXT  "v=DMARC1; p=quarantine; rua=mailto:dmarc@snaelmail.org"
```

### 1.5 PTR (Reverse DNS)
- Request PTR record from GCP for the VM IP
- Format: `mail.snaelmail.org` → VM IP

### 1.6 TLS Certificates
- Use Let's Encrypt (certbot) for `mail.snaelmail.org`
- Auto-renewal via cron
- Certificate for: SMTP (25/587/465), IMAP (993), POP3 (995), HTTPS (443)

### Deliverables
- [ ] Cloudflare DNS records configured
- [ ] DKIM keys generated and stored
- [ ] Let's Encrypt cert obtained
- [ ] PTR record requested from GCP

---

## Phase 2: Outbound SMTP (Week 2-3)
### Goal: Send email directly to recipient MX servers

### 2.1 Outbound SMTP Server (`internal/smtp/outbound.go`)
- Connect to recipient's MX server on port 25
- TLS via STARTTLS (opportunistic)
- Handle multiple MX records (try in priority order)
- Connection pooling and rate limiting
- Retry logic with exponential backoff

### 2.2 Delivery Queue (`internal/queue/`)
- Database-backed queue for outbound emails
- States: queued, sending, sent, failed, bounced
- Retry schedule: 5min, 15min, 1hr, 4hr, 12hr, 24hr, 48hr
- Max retries: 7 (then mark as bounced)
- Priority: user-initiated > system notifications

### 2.3 DKIM Signing (`internal/dkim/`)
- Sign outbound emails with RSA-SHA256
- Add DKIM-Signature header
- Validate signature before sending

### 2.4 Bounce Handling
- Parse DSN (Delivery Status Notification) messages
- Update delivery status in database
- Notify sender of bounce
- Track bounce rates for IP reputation

### 2.5 Rate Limiting
- Per-domain rate limits (e.g., 10 emails/hour to gmail.com)
- Per-user rate limits (daily sending quota)
- Global rate limits to protect IP reputation
- Queue prioritization when rate-limited

### Database Schema
```sql
CREATE TABLE outbound_queue (
    id UUID PRIMARY KEY,
    envelope_id UUID REFERENCES envelopes(id),
    from_address TEXT NOT NULL,
    to_address TEXT NOT NULL,
    mx_host TEXT,
    mx_priority INT,
    status TEXT NOT NULL DEFAULT 'queued',
    attempts INT NOT NULL DEFAULT 0,
    last_attempt TIMESTAMPTZ,
    next_retry TIMESTAMPTZ,
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### Deliverables
- [ ] Outbound SMTP client with MX lookup
- [ ] Delivery queue with retry logic
- [ ] DKIM signing
- [ ] Bounce handling
- [ ] Rate limiting

---

## Phase 3: Inbound SMTP (Week 3-4)
### Goal: Accept email from other mail servers

### 3.1 Inbound SMTP Server (`internal/smtp/inbound.go`)
- Listen on port 25 (requires root or capability)
- Handle EHLO, MAIL FROM, RCPT TO, DATA, QUIT
- TLS via STARTTLS
- Authenticate sender via SPF/DKIM/DMARC
- Accept mail for local domains only
- Reject relay attempts (open relay protection)

### 3.2 Mail Processing Pipeline
1. Receive raw email
2. Parse MIME (multipart, attachments)
3. Verify SPF/DKIM/DMARC
4. Spam scoring (basic heuristic + DNSBL)
5. Virus scanning (ClamAV integration, optional)
6. Store in user's mailbox
7. Trigger notification (if configured)

### 3.3 Spam Filtering (`internal/spam/`)
- DNSBL checks (Spamhaus, Barracuda, etc.)
- SPF verification
- DKIM verification
- DMARC policy enforcement
- Content analysis (keyword scoring)
- Greylisting (optional, for unknown senders)
- User-level spam rules

### 3.4 Mail Storage
- Store emails in PostgreSQL (metadata) + filesystem (attachments)
- Maildir format for IMAP compatibility
- Full-text search indexing
- Attachment handling with size limits

### Database Schema
```sql
CREATE TABLE received_emails (
    id UUID PRIMARY KEY,
    user_id UUID REFERENCES users(id),
    from_address TEXT NOT NULL,
    to_address TEXT NOT NULL,
    subject TEXT,
    body_text TEXT,
    body_html TEXT,
    headers JSONB,
    spam_score FLOAT DEFAULT 0,
    is_spam BOOLEAN DEFAULT false,
    received_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    read_at TIMESTAMPTZ,
    folder TEXT NOT NULL DEFAULT 'inbox'
);

CREATE TABLE email_attachments (
    id UUID PRIMARY KEY,
    email_id UUID REFERENCES received_emails(id) ON DELETE CASCADE,
    filename TEXT NOT NULL,
    content_type TEXT NOT NULL,
    size_bytes BIGINT NOT NULL,
    storage_path TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### Deliverables
- [ ] Inbound SMTP server with TLS
- [ ] Email parsing (MIME, attachments)
- [ ] SPF/DKIM/DMARC verification
- [ ] Basic spam filtering
- [ ] Mail storage in database

---

## Phase 4: IMAP/POP3 Enhancement (Week 4-5)
### Goal: Full email client support

### 4.1 IMAP Server Improvements
- TLS on port 993 (currently stub)
- Full IMAP4rev1 compliance
- Folder management (create, delete, rename)
- Search (IMAP SEARCH command)
- Idle/push notifications (IMAP IDLE)
- Quota reporting (IMAP QUOTA)
- Multi-folder support (Sent, Drafts, Trash, Spam, Archive)

### 4.2 POP3 Server (`internal/pop3/`)
- Basic POP3 (RFC 1939)
- TLS on port 995
- UIDL support
- Leave mail on server option

### 4.3 SMTP AUTH
- SMTP authentication for sending (port 587)
- SASL PLAIN and LOGIN mechanisms
- Rate limiting per authenticated user
- Allow authenticated users to relay through our server

### Deliverables
- [ ] IMAP TLS + full compliance
- [ ] POP3 server
- [ ] SMTP AUTH on port 587
- [ ] Folder management

---

## Phase 5: Webmail Interface (Week 5-7)
### Goal: Gmail-like web experience

### 5.1 Core Webmail Features
- Inbox view with threading
- Compose (new, reply, reply-all, forward)
- Rich text editor (HTML compose)
- Attachment upload/download
- Search (full-text across all emails)
- Labels/folders
- Star/flag
- Archive, delete, mark as read
- Drag-and-drop

### 5.2 Advanced Features
- Contacts management (address book)
- Filters/rules (auto-sort incoming mail)
- Signatures
- Vacation auto-reply
- Aliases (user+tag@snaelmail.org)
- Email forwarding

### 5.3 UI/UX
- Responsive design (mobile-friendly)
- Keyboard shortcuts
- Dark mode
- Conversation view (threading)
- Infinite scroll / pagination
- Real-time notifications (WebSocket/SSE)

### Deliverables
- [ ] Full webmail interface
- [ ] Compose with attachments
- [ ] Search
- [ ] Labels/folders
- [ ] Contacts
- [ ] Filters

---

## Phase 6: Security & Anti-Abuse (Week 7-8)
### Goal: Protect the service and its users

### 6.1 TLS Everywhere
- SMTP: STARTTLS on 25/587, implicit TLS on 465
- IMAP: TLS on 993
- POP3: TLS on 995
- HTTPS: Let's Encrypt auto-renewal
- HSTS headers
- TLS 1.2+ only

### 6.2 Authentication Security
- Brute force protection (fail2ban-like)
- Account lockout after N failed attempts
- 2FA (TOTP) support
- Session management with secure cookies
- Password strength requirements

### 6.3 Anti-Abuse
- Rate limiting (per IP, per user, per domain)
- Connection throttling
- Greylisting for unknown senders
- DNSBL integration
- Content filtering
- Account verification (email/phone)
- Captcha for signup

### 6.4 Monitoring
- Mail queue depth monitoring
- Bounce rate tracking
- Spam rate tracking
- IP reputation monitoring
- Alert on delivery failures

### Deliverables
- [ ] TLS on all protocols
- [ ] Brute force protection
- [ ] 2FA support
- [ ] Rate limiting
- [ ] Monitoring dashboard

---

## Phase 7: Performance & Scaling (Week 8-9)
### Goal: Run efficiently on e2-micro (1GB RAM)

### 7.1 Memory Optimization
- Connection pooling (limit concurrent connections)
- Streaming email parsing (don't load entire email in memory)
- Lazy loading of attachments
- Database connection pooling (pgx)
- Go runtime tuning (GOGC, GOMAXPROCS)

### 7.2 Storage Optimization
- Compress email bodies
- Deduplicate attachments (content-addressable storage)
- Archive old emails
- Quota enforcement
- Cleanup of deleted emails (purge after 30 days)

### 7.3 Caching
- In-memory cache for frequently accessed data
- DNS cache for MX lookups
- Template caching
- Static asset caching (CDN via Cloudflare)

### 7.4 Background Workers
- Delivery queue worker (1 goroutine)
- Inbound processing worker (1 goroutine)
- Spam filter worker (1 goroutine)
- Cleanup worker (1 goroutine)
- Notification worker (1 goroutine)

### Memory Budget (1GB RAM)
```
PostgreSQL:     256 MB
Go application: 256 MB
OS + other:     256 MB
Buffer:         256 MB
```

### Deliverables
- [ ] Memory profiling and optimization
- [ ] Connection pooling
- [ ] Storage optimization
- [ ] Background worker tuning

---

## Phase 8: Testing & Launch (Week 9-10)
### Goal: Production-ready service

### 8.1 Testing
- Unit tests for all new components
- Integration tests (send/receive cycle)
- Load testing (concurrent connections)
- Spam filter accuracy testing
- Security audit

### 8.2 Monitoring
- Health check endpoints
- Prometheus metrics
- Log aggregation
- Alerting (email/SMS on critical failures)

### 8.3 Documentation
- User guide (how to use webmail, IMAP, SMTP)
- Admin guide (managing users, domains, queues)
- API documentation
- Troubleshooting guide

### 8.4 Launch Checklist
- [ ] All DNS records propagated
- [ ] TLS certificates valid
- [ ] SPF/DKIM/DMARC passing (test with mail-tester.com)
- [ ] Inbound/outbound email working
- [ ] IMAP/POP3 working with major clients
- [ ] Webmail fully functional
- [ ] Spam filtering active
- [ ] Monitoring in place
- [ ] Backup strategy implemented

---

## Technical Decisions

### Why Not Use Existing Mail Servers (Postfix/Dovecot)?
- We want a single Go binary, not multiple services
- Custom "slow email" features need integration
- Learning and control over the full stack
- But: Consider using Postfix for SMTP and Dovecot for IMAP if Go implementations prove too complex

### Database vs Maildir
- Use PostgreSQL for metadata and small emails
- Use filesystem (Maildir format) for large emails and attachments
- Hybrid approach for best performance

### Queue Implementation
- Database-backed queue (not in-memory) for persistence
- Survives restarts
- Easy to monitor and debug

### Spam Filtering Approach
- Start with basic heuristics + DNSBL
- Add Bayesian filtering later
- Consider integrating rspamd via API

---

## Risk Assessment

### High Risk
1. **IP Reputation**: New IP addresses have no reputation. Major providers may block or spam-folder our emails initially.
   - Mitigation: Warm up IP gradually, monitor blacklists, implement proper authentication

2. **Port 25 Blocking**: GCP blocks outbound port 25 by default.
   - Mitigation: Request unblock from GCP, or use a relay for outbound (defeats purpose)

3. **Resource Constraints**: e2-micro has 1GB RAM. Full email service is memory-hungry.
   - Mitigation: Optimize memory usage, consider upgrading to e2-small if needed

### Medium Risk
4. **Spam Abuse**: Service could be used for spam, damaging IP reputation.
   - Mitigation: Strict signup verification, rate limiting, content filtering

5. **TLS Certificate Management**: Let's Encrypt certs expire every 90 days.
   - Mitigation: Automated renewal via cron

### Low Risk
6. **DNS Propagation**: DNS changes can take time.
   - Mitigation: Use Cloudflare for fast propagation

---

## Estimated Timeline
- Phase 1 (DNS): 1 week
- Phase 2 (Outbound): 2 weeks
- Phase 3 (Inbound): 2 weeks
- Phase 4 (IMAP/POP3): 1 week
- Phase 5 (Webmail): 2 weeks
- Phase 6 (Security): 1 week
- Phase 7 (Performance): 1 week
- Phase 8 (Testing): 1 week

**Total: ~11 weeks**

## Success Criteria
1. User can send email to any address on the internet
2. User can receive email from any address on the internet
3. Email passes SPF, DKIM, DMARC checks
4. Works with standard email clients (Thunderbird, Outlook, Apple Mail)
5. Webmail interface is functional and responsive
6. IP is not blacklisted
7. Runs within 1GB RAM on e2-micro
8. 99%+ uptime
