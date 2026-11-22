# Snaelmail Relay

A Snaelmail relay is a self-hosted post office node. It registers with the main Snaelmail site, hosts its own post office identity/location, and can publish a public post office page.

## Quick Start (Docker)

```bash
docker run -d \
  --name snaelmail-relay \
  -p 8477:8477 \
  -v snaelmail-relay-config:/config \
  ghcr.io/binarygeek119/snaelmail-relay:latest
```

Then open the setup UI:

- http://localhost:8477/

## Setup Web UI

The relay setup UI runs on **port 8477** and lets you:

1. **Generate Relay Identity** - creates an Ed25519 keypair; export identity as TXT.
2. **Register with Main Site** - registers the relay with `https://snaelmail.org`.
   - Main site URL is hardcoded.
   - Hostname is auto-derived from City + Region/State + Country Code (not editable).
   - Country Code, Region/State, and Timezone use dropdowns.
   - Latitude/Longitude can be found via [latlong.net](https://www.latlong.net/).
3. **City Images** - upload images, save locally, sync to main site, copy from Central Post Office, preview local public site.
4. **Post Office Stamp** - upload SVG or PNG stamp.

## Config / Data

Relay data is stored in `SMAIL_RELAY_DIR` (default: `/config` in Docker).

- **SQLite DB**: `/config/relay.db` - stores relay location and relay info.
- **Images**: `/config/images/` - post office city images.
- **Stamp**: `/config/stamp.svg` or `/config/stamp.png`.
- **Keys**: `/config/relay.key`, `/config/relay.pub`.
- **State**: `/config/setup.json` (legacy sync).

Mount `/config` as a volume to persist data.

## Ports

| Port | Direction | Purpose |
|------|-----------|---------|
| 8477/tcp | Inbound | Relay setup/dashboard web UI |
| 443/tcp | Outbound | Connect to main Snaelmail site/API |
| 53/tcp+udp | Outbound | DNS resolution |

## Versioning

Relay Docker images are published to:

- `ghcr.io/binarygeek119/snaelmail-relay`

Tags:

- `latest` - latest main build
- `sha-...` - commit build
- `vX.Y.Z`, `vX.Y`, `vX` - semver tags when pushing `v*` git tags

The relay reports `X-Relay-Version` when connecting to the main site.

## Minimum Version Enforcement

The main site can enforce a minimum relay version via:

```bash
SMAIL_MIN_RELAY_VERSION=1.0.0
```

If set, relay connect/register/publish actions require `X-Relay-Version` >= minimum. Old or missing versions receive `426` with `minimum_version` and `your_version`.

## Public Site Sync

- Images uploaded in relay UI are saved to `/config/images`.
- **Sync to Main Site** publishes page info and images to Snaelmail for the public post office page.
- When registered, relay auto-syncs images.
- If local public site info/images change, sync again to update the main site page.
- **Preview Public Site** shows a local preview (not hosted by snaelmail.org).

## Development

```bash
go build ./cmd/relay
./relay -addr :8477
```

Optional env vars:

- `SMAIL_RELAY_DIR` - config/data directory (default `~/.snailmail-relay`)
- `SMAIL_DSN` - Postgres DSN for full relay DB-backed services (optional; setup UI works without it)

## Issues & Feature Requests

- [Report a bug](https://github.com/binarygeek119/snaelmail/issues/new?template=bug_report.yml)
- [Request a feature](https://github.com/binarygeek119/snaelmail/issues/new?template=feature_request.yml)
