# 🐌 Snaelmail

**Slow, deliberate email through a network of virtual post offices.**

Snaelmail is an independent email service where messages travel through post office relays, collect postmarks, and arrive based on geographic distance - bringing back the anticipation of snail mail.

## Highlights

- **`@snaelmail.org` mailboxes** with web inbox/outbox and SMTP/IMAP/POP3 support
- **Distance-influenced delivery** with relay/postmark-style routing metadata
- **Public post office pages** for central and self-hosted relays (photos, stamp, about, map)
- **Self-hosted relays** via Docker image on GHCR with a setup web UI
- **Admin panel** for users, relays, central post office page, and moderation

## Stack

- **Backend**: Go 1.23, `net/http`, `html/template`
- **Database**: PostgreSQL 16 (no ORM, SQL migrations in `migrations/`)
- **Frontend**: static HTML/CSS/JS in `web/site/`
- **Deploy**: Netcup VPS (Debian 13), Nginx reverse proxy, systemd
- **DNS/TLS**: Cloudflare DNS, Let's Encrypt certs

## Repository Layout

```text
cmd/
  site/                  # Main Snaelmail web/API/mail server
  relay/                 # Self-hosted relay daemon + setup UI
internal/
  auth/                  # Auth, sessions, passwords
  config/                # Config loading
  mail/                  # SMTP/IMAP/POP3/delivery
  page/                  # Public page manifest/validation
  render/                # Post office page/stamp rendering
  relayversion/          # Relay semver enforcement
  store/                 # Postgres data access
  ...
migrations/              # SQL migrations (001+)
web/site/                # Web UI pages
.github/workflows/       # CI / relay Docker build
```

## Local Development

### Prerequisites

- Go 1.23+
- PostgreSQL 16+

### Run the site

```bash
# Create DB
createdb snailmail

# Apply migrations (in order)
for f in migrations/*.sql; do psql "$SMAIL_DSN" -f "$f"; done

# Run
export SMAIL_DSN="postgres://user:pass@localhost:5432/snailmail?sslmode=disable"
go run ./cmd/site -config config.toml
```

The site serves on the configured HTTP addr (default `:8080` in config).

## Main Site (Production)

Current production runs on a Netcup VPS behind Nginx/systemd.

Typical deploy flow:

```bash
ssh root@62.83.135.138
cd /opt/snailmail/app
git pull origin main
/usr/local/go/bin/go build -buildvcs=false -o /opt/snailmail/app/snaelmail ./cmd/site
setcap 'cap_net_bind_service=+ep' /opt/snailmail/app/snaelmail
chown snailmail:snailmail /opt/snailmail/app/snaelmail
systemctl restart snailmail
```

## Self-Hosted Relay

Run a relay with Docker:

```bash
docker run -d \
  --name snaelmail-relay \
  -p 8477:8477 \
  -v snaelmail-relay-config:/config \
  ghcr.io/binarygeek119/snaelmail-relay:latest
```

Open `http://localhost:8477/` for the setup wizard. See [relay.md](relay.md) for full relay docs.

## Key Features

### Accounts & Settings
- Signup/login/verification
- Account location for delivery timing
- IMAP/POP3 settings and passwords
- Relay API key generation for self-hosted relay registration

### Admin Panel
- Dashboard stats, users, letters, post offices
- Accept/suspend/remove relays
- Admin preview for unpublished relay pages
- Central post office editor: location, stamp, city images, hero image

### Relay Setup Wizard
1. Identity (keypair + main site username/API key)
2. Post Office Info (city, region, country, timezone, coordinates)
3. About (local/personal post office description)
4. Images (city images + city map)
5. Stamp (default vintage stamp or custom upload)
6. Register with main site

After registration, relay switches to a dashboard with notifications and mail activity.

## Relay Docker Images

Published to GHCR by `.github/workflows/relay-docker.yml`:

- `ghcr.io/binarygeek119/snaelmail-relay:latest`
- `ghcr.io/binarygeek119/snaelmail-relay:sha-...`
- semver tags on `v*` releases

## Issues / Feature Requests

- [Bug report](https://github.com/binarygeek119/snaelmail/issues/new?template=bug_report.yml)
- [Feature request](https://github.com/binarygeek119/snaelmail/issues/new?template=feature_request.yml)

## License

Open source - see repository for license details.
