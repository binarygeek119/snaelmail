# Snaelmail Relay

A Snaelmail relay is a self-hosted post office node. It registers with the main Snaelmail site, publishes a public post office page, and can participate in the Snaelmail network.

## Quick Start (Docker)

```bash
docker run -d \
  --name snaelmail-relay \
  -p 8477:8477 \
  -v snaelmail-relay-config:/config \
  ghcr.io/binarygeek119/snaelmail-relay:latest
```

Open the setup UI:

- http://localhost:8477/

## Setup Wizard (first run)

The relay UI runs on **port 8477** and guides setup:

1. **Relay Identity** - generate Ed25519 keypair, export identity TXT, enter main site username + API key (from snaelmail.org Account Settings).
2. **Post Office Info (Part 1)** - city, country, region/state, timezone, latitude/longitude. Hostname is auto-derived from city/region/country.
3. **About (Part 2)** - local/personal description shown on the public post office page.
4. **City Images** - upload gallery images, set hero image, upload optional city map, copy from Central Post Office, preview local page.
5. **Stamp** - use default vintage stamp (includes city/region/country) or upload custom SVG/PNG.
6. **Register** - register with `https://snaelmail.org`.

After registration, the UI switches to a dashboard.

## Dashboard

- **Notifications** - relay status/need-to-know info (pending, active, offline, suspended, blocked, update needed).
- **Mail Activity** - letters through, delivered, sent, in route (from local relay stats when DB-backed).
- **Sync to Main Site** - publish page info/images/map/stamp.
- **Preview Public Site** - local preview.

## Config / Data

Stored in `SMAIL_RELAY_DIR` (default `/config` in Docker):

- `/config/relay.db` - SQLite local relay config/state
- `/config/images/` - city images and optional `map.*`
- `/config/stamp.svg` or `/config/stamp.png` - custom stamp (optional)
- `/config/relay.key`, `/config/relay.pub` - relay identity keys

Mount `/config` as a volume to persist data.

## Ports

| Port | Direction | Purpose |
|------|-----------|---------|
| 8477/tcp | Inbound | Relay setup/dashboard web UI |
| 443/tcp | Outbound | Main Snaelmail site/API |
| 53/tcp+udp | Outbound | DNS |

## Versioning

Images published to `ghcr.io/binarygeek119/snaelmail-relay`:

- `latest` - latest main build
- `sha-...` - commit build
- `vX.Y.Z` / `vX.Y` / `vX` - release tags

Relay sends `X-Relay-Version` to main site. Main site may enforce `SMAIL_MIN_RELAY_VERSION`; below-minimum relays get `426`.

## Public Site Sync

- Uploaded images/map/stamp are saved locally in `/config`.
- **Sync to Main Site** publishes manifest + assets to snaelmail.org.
- Sync is allowed for pending/unverified relays so admins/mods can review the full page before acceptance.
- Public users only see accepted/live relay pages; admins/mods can preview unpublished pages from Admin Panel.

## Development

```bash
go build ./cmd/relay
./relay -addr :8477
```

Optional env vars:

- `SMAIL_RELAY_DIR` - config/data dir (default `~/.snailmail-relay`)
- `SMAIL_DSN` - Postgres DSN for full relay DB-backed services (setup UI works without it)

## Issues & Feature Requests

- [Report a bug](https://github.com/binarygeek119/snaelmail/issues/new?template=bug_report.yml)
- [Request a feature](https://github.com/binarygeek119/snaelmail/issues/new?template=feature_request.yml)
