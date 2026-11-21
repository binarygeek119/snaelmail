# Netcup Migration Guide for Snaelmail

## Overview
Migrate Snaelmail from Google Cloud (GCP) to Netcup VPS for port 25 access.

## Step 1: Order Netcup VPS

### Recommended Plan: VPS 1000 G11s (or similar)
- **Price:** ~€3.00-4.00/month
- **RAM:** 2-4 GB
- **Storage:** 40-80 GB SSD
- **Location:** Choose Nuremberg or Vienna (EU) for good connectivity
- **OS:** Ubuntu 24.04 LTS

### Order Process
1. Go to [netcup.eu](https://www.netcup.eu)
2. Navigate to VPS → VPS 1000/2000 series
3. Select Ubuntu 24.04 LTS as OS
4. Complete order and payment
5. You'll receive server credentials via email

### Important: Identity Verification
Netcup may require identity verification for new accounts:
- Have your ID/passport ready
- They may ask for a selfie with ID
- Process usually takes 1-24 hours
- This is normal and prevents spam abuse

## Step 2: Initial Server Setup

### Connect to Server
```bash
ssh root@YOUR_NETCUP_IP
```

### Update System
```bash
apt update && apt upgrade -y
apt install -y curl wget git ufw fail2ban
```

### Create snailmail User
```bash
useradd -m -s /bin/bash snailmail
usermod -aG sudo snailmail
```

### Set Up Firewall
```bash
ufw allow 22/tcp    # SSH
ufw allow 80/tcp    # HTTP
ufw allow 443/tcp   # HTTPS
ufw allow 25/tcp    # SMTP inbound
ufw allow 587/tcp   # SMTP submission
ufw allow 465/tcp   # SMTPS
ufw allow 993/tcp   # IMAPS
ufw allow 995/tcp   # POP3S
ufw enable
```

### Verify Port 25 is Open
```bash
nc -vz gmail-smtp-in.l.google.com 25
# Expected: Connection succeeded
```

## Step 3: Install Dependencies

### PostgreSQL 16
```bash
apt install -y postgresql-16 postgresql-client-16
sudo -u postgres createuser snailmail
sudo -u postgres createdb snailmail -O snailmail
sudo -u postgres psql -c "ALTER USER snailmail WITH PASSWORD 'snailmail_secure_password_change_me';"
```

### Go 1.23
```bash
wget https://go.dev/dl/go1.23.0.linux-amd64.tar.gz
tar -C /usr/local -xzf go1.23.0.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> /etc/profile
source /etc/profile
go version
```

### Nginx
```bash
apt install -y nginx
```

### Let's Encrypt (Certbot)
```bash
apt install -y certbot python3-certbot-nginx
```

## Step 4: Set Up PTR (Reverse DNS)

### Via Netcup SCP (Server Control Panel)
1. Log in to [Netcup SCP](https://www.servercontrolpanel.de)
2. Select your server
3. Go to **Network** → **Reverse DNS (rDNS)**
4. Set PTR record: `YOUR_IP` → `mail.snaelmail.org`
5. Save

### Verify PTR
```bash
dig -x YOUR_NETCUP_IP +short
# Should return: mail.snaelmail.org
```

## Step 5: Configure DNS in Cloudflare

### A Records
```
snaelmail.org.        A     YOUR_NETCUP_IP
mail.snaelmail.org.   A     YOUR_NETCUP_IP
www.snaelmail.org.    A     YOUR_NETCUP_IP
```

### MX Record
```
snaelmail.org.        MX    10  mail.snaelmail.org.
```

### SPF Record
```
snaelmail.org.        TXT   "v=spf1 mx a:mail.snaelmail.org ~all"
```

### DKIM Record (generate keys first)
```bash
# Generate DKIM keys
openssl genrsa -out /etc/snailmail/dkim_private.pem 2048
openssl rsa -in /etc/snailmail/dkim_private.pem -pubout -out /etc/snailmail/dkim_public.pem

# Get public key (remove headers and newlines)
cat /etc/snailmail/dkim_public.pem | grep -v "PUBLIC KEY" | tr -d '\n'
```
Then add DNS TXT record:
```
default._domainkey.snaelmail.org.  TXT  "v=DKIM1; k=rsa; p=YOUR_PUBLIC_KEY"
```

### DMARC Record
```
_dmarc.snaelmail.org.  TXT  "v=DMARC1; p=quarantine; rua=mailto:dmarc@snaelmail.org"
```

## Step 6: Set Up TLS Certificates

### Obtain Certificate
```bash
certbot certonly --standalone -d snaelmail.org -d mail.snaelmail.org -d www.snaelmail.org
```

### Auto-Renewal
```bash
# Test renewal
certbot renew --dry-run

# Add cron job (certbot usually adds this automatically)
echo "0 0,12 * * * root certbot renew --quiet" | sudo tee -a /etc/crontab
```

### Certificate Paths
```
/etc/letsencrypt/live/snaelmail.org/fullchain.pem
/etc/letsencrypt/live/snaelmail.org/privkey.pem
```

## Step 7: Migrate Database

### Export from GCP
```bash
# On GCP VM
sudo -u postgres pg_dump snailmail > /tmp/snailmail_backup.sql
gcloud compute scp snailmail-server:/tmp/snailmail_backup.sql ./snailmail_backup.sql --zone=us-central1-a
```

### Import to Netcup
```bash
# On Netcup VM
scp snailmail_backup.sql root@YOUR_NETCUP_IP:/tmp/
sudo -u postgres psql snailmail < /tmp/snailmail_backup.sql
```

### Verify
```bash
sudo -u postgres psql snailmail -c "SELECT COUNT(*) FROM users;"
sudo -u postgres psql snailmail -c "SELECT hostname, central FROM relays;"
```

## Step 8: Deploy Application

### Clone Repository
```bash
mkdir -p /opt/snailmail
cd /opt/snailmail
git clone https://github.com/binarygeek119/snaelmail.git app
cd app
```

### Build
```bash
cd /opt/snailmail/app
go build -o /opt/snailmail/app/snaelmail ./cmd/site
```

### Create Config
```bash
cat > /opt/snailmail/app/config.toml << 'EOF'
mail_domain = "snaelmail.org"
site_key    = "/etc/snailmail/site.key"

[speed_km_per_day]
surface = 250
first   = 800
air     = 2500

[handling]
base_hours        = 4
per_100_queued_h  = 2
gap_threshold_km  = 600
gap_hours_per_600 = 6

[route]
max_hops        = 6
local_radius_km = 25
cone_deg        = 60
min_progress    = 0.08

[calendar]
no_movement_weekdays  = [0]
saturday_speed_factor = 0.5
holidays              = "holidays.toml"

[customs]
hold_min_h = 12
hold_max_h = 72

[stamp]
canvas    = [320, 400]
positions = [[18,16],[18,120],[18,224],[18,300],[120,16],[120,120]]

[limits]
max_surface_days = 45
max_air_days     = 21

# SMTP configuration (now using direct delivery)
smtp_username = ""
smtp_password = ""

# IMAP server configuration
imap_port = "993"
imap_cert_file = "/etc/letsencrypt/live/snaelmail.org/fullchain.pem"
imap_key_file = "/etc/letsencrypt/live/snaelmail.org/privkey.pem"

# TLS certificates
tls_cert_file = "/etc/letsencrypt/live/snaelmail.org/fullchain.pem"
tls_key_file = "/etc/letsencrypt/live/snaelmail.org/privkey.pem"
EOF
```

### Create Systemd Service
```bash
cat > /etc/systemd/system/snailmail.service << 'EOF'
[Unit]
Description=Snaelmail Application
After=network.target postgresql.service

[Service]
Type=simple
User=snailmail
WorkingDirectory=/opt/snailmail/app
ExecStart=/opt/snailmail/app/snaelmail -config /opt/snailmail/app/config.toml
Restart=always
RestartSec=10
StandardOutput=journal
StandardError=journal

Environment=SMAIL_DSN=postgres://snailmail:snailmail_secure_password_change_me@localhost:5432/snailmail?sslmode=disable

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable snailmail
systemctl start snailmail
```

### Configure Nginx
```bash
cat > /etc/nginx/sites-available/snailmail << 'EOF'
server {
    listen 80;
    server_name snaelmail.org www.snaelmail.org;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}

server {
    listen 443 ssl;
    server_name snaelmail.org www.snaelmail.org;

    ssl_certificate /etc/letsencrypt/live/snaelmail.org/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/snaelmail.org/privkey.pem;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
EOF

ln -s /etc/nginx/sites-available/snailmail /etc/nginx/sites-enabled/
nginx -t && systemctl restart nginx
```

## Step 9: Verify Everything

### Test Port 25
```bash
nc -vz gmail-smtp-in.l.google.com 25
# Should succeed
```

### Test Web Service
```bash
curl -s https://snaelmail.org | head -5
```

### Test SMTP Inbound
```bash
telnet YOUR_NETCUP_IP 25
# Should show: 220 snaelmail.org ESMTP
```

### Test Email Deliverability
1. Send a test email from your snaelmail.org address to a Gmail address
2. Check if it arrives (not in spam)
3. Use [mail-tester.com](https://www.mail-tester.com) to check spam score
4. Verify SPF, DKIM, DMARC all pass

## Step 10: Decommission GCP

### After 48 Hours of Stable Operation
1. Stop GCP VM
2. Delete GCP VM
3. Delete GCP firewall rules
4. Update any remaining DNS records
5. Cancel GCP billing

## Cost Comparison

| Item | GCP (current) | Netcup (new) |
|------|---------------|--------------|
| VM | $0 (free tier) | ~€3-4/mo |
| Port 25 | ❌ Blocked | ✅ Open |
| RAM | 1 GB | 2-4 GB |
| Storage | 10 GB | 40-80 GB |
| Bandwidth | Limited | Generous |

## Post-Migration Tasks

1. **Monitor email deliverability** for first 2 weeks
2. **Warm up IP** - Start with low volume, increase gradually
3. **Check blacklists** - Monitor Spamhaus, Barracuda, etc.
4. **Set up monitoring** - Uptime, queue depth, bounce rates
5. **Implement outbound SMTP** in the Go code (Phase 2 of email service plan)

## Rollback Plan

If migration fails:
1. Keep GCP VM running for 48 hours after switch
2. DNS TTL is set to 300s (5 min) for quick rollback
3. Revert DNS A records to GCP IP
4. Investigate and fix issues on Netcup
5. Re-attempt migration

## Timeline

| Day | Task |
|-----|------|
| 1 | Order Netcup VPS, wait for provisioning |
| 1-2 | Identity verification (if required) |
| 2 | Initial server setup, install dependencies |
| 2 | Set up PTR record, configure DNS |
| 3 | Migrate database, deploy application |
| 3 | Set up TLS certificates, Nginx |
| 4 | Test everything, verify email flow |
| 5 | Monitor, fix issues |
| 7 | Decommission GCP (if stable) |
EOF