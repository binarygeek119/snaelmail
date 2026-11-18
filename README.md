# 🐌 Snaelmail - The Slow Email Service

**Email that takes its time. Deliberately slow communication through a network of virtual post offices.**

## What is Snaelmail?

Snaelmail is an experimental email service that brings back the anticipation and thoughtfulness of snail mail. Instead of instant delivery, your messages travel through a network of virtual post offices, collecting digital postmarks along the way. The distance between sender and recipient determines delivery time.

- **Distance-based delivery**: Messages take time proportional to geographic distance
- **Digital postmarks**: Each post office stamps your message as it passes through
- **Privacy-first**: End-to-end encrypted messages
- **Post office pages**: Each relay has a public page showcasing its community
- **Simple interface**: Clean, modern web UI

## Features

### For Users
- Send emails to any address (internal or external)
- Beautiful web dashboard for composing and reading mail
- Track your letters as they journey through the network
- View delivery status and postmark trail
- Simple signup and verification process

### For Post Office Operators
- Run your own virtual post office
- Customize your post office page
- Monitor mail traffic and statistics
- Manage postmark designs
- Admin dashboard for oversight

## Tech Stack

- **Backend**: Go 1.23
- **Database**: PostgreSQL 16
- **Frontend**: Vanilla HTML/CSS/JavaScript
- **Cryptography**: Ed25519 signatures, X25519 key exchange
- **Infrastructure**: Google Cloud Platform (free tier compatible)
- **DNS**: Cloudflare

## Quick Start

### Prerequisites
- Go 1.23+
- PostgreSQL 16+
- Google Cloud account (for deployment)

### Local Development

```bash
# Clone the repository
git clone https://github.com/yourusername/snaelmail.git
cd snaelmail

# Set up database
psql -U postgres -c "CREATE DATABASE snailmail;"
PGPASSWORD=snailmail_secure_password_change_me psql -h localhost -U snailmail -d snailmail -f migrations/001_initial.sql
PGPASSWORD=snailmail_secure_password_change_me psql -h localhost -U snailmail -d snailmail -f migrations/002_p7.sql

# Run migrations
go run ./cmd/migrate

# Start the server
go run ./cmd/site
```

The server will start on `http://localhost:8080`

### Deployment to Google Cloud

See [FREE_TIER_GUIDE.md](FREE_TIER_GUIDE.md) for complete instructions on deploying to Google Cloud's free tier.

```bash
# Quick deployment
./setup-free-tier.sh
```

## Project Structure

```
snaelmail/
├── cmd/
│   ├── site/          # Main web server
│   ├── relay/         # Relay daemon
│   └── migrate/       # Database migrations
├── internal/
│   ├── auth/          # Authentication
│   ├── clock/         # Postal calendar
│   ├── config/        # Configuration
│   ├── crypto/        # Encryption
│   ├── lookup/        # Search functionality
│   ├── page/          # Post office pages
│   ├── plate/         # Postmark generation
│   ├── relayreg/      # Relay registration
│   ├── render/        # Page rendering
│   ├── route/         # Message routing
│   ├── schedule/      # Delivery scheduling
│   ├── stamp/         # Postmark chain
│   ├── store/         # Database layer
│   └── tlog/          # Transparency log
├── migrations/        # SQL migrations
├── web/site/          # Frontend HTML/CSS/JS
└── testdata/          # Test fixtures
```

## Database Schema

The system uses 13 core tables:
- `users` - User accounts
- `relays` - Post office relays
- `envelopes` - Message envelopes
- `hops` - Message route hops
- `stamps` - Postmarks at each relay
- `pages` - Public post office pages
- `tlog_leaves` / `tlog_roots` - Transparency log

See `migrations/001_initial.sql` for the complete schema.

## API Endpoints

### User API
- `POST /api/signup` - Create account
- `POST /api/session` - Login
- `POST /api/verify` - Verify email
- `POST /api/letters` - Send email
- `GET /api/inbox` - Get inbox
- `GET /api/outbox` - Get sent mail

### Relay API
- `POST /api/relays` - Register relay
- `POST /api/relay/:id/verify` - Verify relay
- `GET /open-post-offices` - List all post offices
- `GET /open-post-offices/:id` - View post office page

### Admin API
- `GET /api/admin/stats` - System statistics
- `GET /api/admin/users` - Manage users
- `GET /api/admin/relays` - Manage relays

## Configuration

Configuration is via `config.toml`:

```toml
[server]
port = 8080

[database]
host = "localhost"
port = 5432
name = "snailmail"
user = "snailmail"

[email]
domain = "snaelmail.org"

[routing]
base_speed = 100  # km/hour
```

## Development

### Running Tests

```bash
go test ./...
```

### Building

```bash
go build -o snailmail ./cmd/site
```

## Deployment

### Google Cloud (Free Tier)

The project is designed to run on Google Cloud's free tier:
- e2-micro VM (2 vCPU, 1GB RAM)
- 30GB persistent disk
- Cloud SQL or local PostgreSQL

See [FREE_TIER_GUIDE.md](FREE_TIER_GUIDE.md) for detailed setup instructions.

### Domain Setup

The project uses Cloudflare for DNS and SSL:
- Automatic SSL certificates
- DNS management
- Email routing

See [cloudflare/SETUP_GUIDE.md](cloudflare/SETUP_GUIDE.md) for setup instructions.

## Contributing

Contributions are welcome! Please:
1. Fork the repository
2. Create a feature branch
3. Write tests for new functionality
4. Ensure all tests pass
5. Submit a pull request

## License

This project is open source. See LICENSE file for details.

## Roadmap

- [ ] Mobile app support
- [ ] Custom postmark designs
- [ ] Post office themes
- [ ] Message attachments
- [ ] Group mailing lists
- [ ] API for third-party integrations

## Acknowledgments

Inspired by the beauty of handwritten letters and the anticipation of waiting for mail.

---

**Note**: This is an experimental project exploring slow communication in a fast-paced digital world.
