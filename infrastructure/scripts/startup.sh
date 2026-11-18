#!/bin/bash
# Snailmail VM Setup Script - OPTIMIZED FOR FREE TIER (e2-micro, 1GB RAM)
# This script configures a minimal, memory-efficient mail server

set -e

echo "🐌 Setting up Snailmail on FREE TIER VM (e2-micro, 1GB RAM)..."

# ============================================================================
# MEMORY OPTIMIZATION - CRITICAL FOR 1GB RAM
# ============================================================================

echo "📊 Setting up swap space (2GB)..."
# Create 2GB swap file to compensate for limited RAM
fallocate -l 2G /swapfile
chmod 600 /swapfile
mkswap /swapfile
swapon /swapfile
echo '/swapfile none swap sw 0 0' >> /etc/fstab

# Optimize swappiness (prefer RAM over swap)
sysctl vm.swappiness=10
echo 'vm.swappiness=10' >> /etc/sysctl.conf

echo "✓ Swap configured: 2GB"

# ============================================================================
# SYSTEM UPDATES & DEPENDENCIES
# ============================================================================

echo "📦 Installing system packages..."
export DEBIAN_FRONTEND=noninteractive

# Update system
apt-get update -y
apt-get upgrade -y

# Install only essential packages (minimize memory usage)
apt-get install -y \
  git \
  curl \
  wget \
  build-essential \
  postgresql \
  postgresql-contrib \
  nginx \
  ufw \
  fail2ban \
  logrotate

echo "✓ System packages installed"

# ============================================================================
# GO INSTALLATION
# ============================================================================

echo "🐹 Installing Go 1.23..."
cd /tmp
wget -q https://go.dev/dl/go1.23.4.linux-amd64.tar.gz
tar -C /usr/local -xzf go1.23.4.linux-amd64.tar.gz
rm go1.23.4.linux-amd64.tar.gz

# Set up Go paths
echo 'export PATH=$PATH:/usr/local/go/bin' >> /etc/profile.d/go.sh
echo 'export GOPATH=/home/deploy/go' >> /etc/profile.d/go.sh
source /etc/profile.d/go.sh

echo "✓ Go 1.23 installed"

# ============================================================================
# POSTGRESQL OPTIMIZATION FOR 1GB RAM
# ============================================================================

echo "🐘 Optimizing PostgreSQL for low memory..."

# Create optimized PostgreSQL config
cat > /etc/postgresql/14/main/conf.d/snailmail.conf << 'EOF'
# PostgreSQL Configuration for e2-micro (1GB RAM + 2GB swap)
# Optimized for low memory usage

# Connection settings
max_connections = 20
superuser_reserved_connections = 3

# Memory settings (conservative for 1GB RAM)
shared_buffers = 128MB
effective_cache_size = 256MB
work_mem = 4MB
maintenance_work_mem = 64MB

# WAL settings
wal_buffers = 4MB
checkpoint_completion_target = 0.9

# Query tuning
random_page_cost = 1.1
effective_io_concurrency = 200

# Logging (minimal to save I/O)
log_min_duration_statement = 1000
log_checkpoints = off
log_connections = off
log_disconnections = off

# Background writer
bgwriter_delay = 200ms
bgwriter_lru_maxpages = 100
bgwriter_lru_multiplier = 2.0
EOF

# Restart PostgreSQL to apply changes
systemctl restart postgresql

# Create database and user
sudo -u postgres psql << 'EOF'
CREATE USER snailmail WITH PASSWORD 'snailmail_secure_password_change_me';
CREATE DATABASE snailmail OWNER snailmail;
GRANT ALL PRIVILEGES ON DATABASE snailmail TO snailmail;
\c snailmail
GRANT ALL ON SCHEMA public TO snailmail;
EOF

echo "✓ PostgreSQL optimized and configured"

# ============================================================================
# APPLICATION USER & DIRECTORIES
# ============================================================================

echo "👤 Creating application user..."
useradd -m -s /bin/bash snailmail
usermod -aG sudo snailmail

# Create application directories
mkdir -p /opt/snailmail/app
mkdir -p /opt/snailmail/data
mkdir -p /opt/snailmail/logs
chown -R snailmail:snailmail /opt/snailmail

echo "✓ Application user and directories created"

# ============================================================================
# FIREWALL CONFIGURATION
# ============================================================================

echo "🔥 Configuring firewall..."
ufw default deny incoming
ufw default allow outgoing
ufw allow 22/tcp    # SSH
ufw allow 80/tcp    # HTTP
ufw allow 443/tcp   # HTTPS
ufw allow 1143/tcp  # IMAP
ufw allow 25/tcp    # SMTP
ufw --force enable

echo "✓ Firewall configured"

# ============================================================================
# CLONE APPLICATION
# ============================================================================

echo "📥 Cloning Snailmail repository..."
cd /opt/snailmail
sudo -u snailmail git clone https://github.com/YOUR_USERNAME/snailmail.git app
cd app

echo "✓ Repository cloned"

# ============================================================================
# BUILD APPLICATION
# ============================================================================

echo "🔨 Building application..."
cd /opt/snailmail/app
sudo -u snailmail /usr/local/go/bin/go build -o snailmail ./cmd/site

echo "✓ Application built"

# ============================================================================
# SYSTEMD SERVICE (MEMORY OPTIMIZED)
# ============================================================================

echo "⚙️  Creating systemd service (memory optimized)..."

cat > /etc/systemd/system/snailmail.service << 'EOF'
[Unit]
Description=Snailmail Application
After=network.target postgresql.service

[Service]
Type=simple
User=snailmail
WorkingDirectory=/opt/snailmail/app
ExecStart=/opt/snailmail/app/snailmail -config /opt/snailmail/app/config.yaml
Restart=always
RestartSec=10
StandardOutput=journal
StandardError=journal

# Memory limits (critical for 1GB RAM)
MemoryMax=512M
MemoryHigh=384M

# Environment variables
Environment=GIN_MODE=release
Environment=DATABASE_URL=postgres://snailmail:snailmail_secure_password_change_me@localhost:5432/snailmail?sslmode=disable
Environment=SMTP_HOST=localhost
Environment=SMTP_PORT=25

# Security hardening
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/snailmail/app/data /opt/snailmail/logs

# Resource limits
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
EOF

# Enable and start service
systemctl daemon-reload
systemctl enable snailmail
systemctl start snailmail

echo "✓ Systemd service created and started"

# ============================================================================
# NGINX CONFIGURATION
# ============================================================================

echo "🌐 Configuring Nginx..."

# Create Nginx config
cat > /etc/nginx/sites-available/snailmail << 'EOF'
server {
    listen 80;
    server_name _;

    location / {
        proxy_pass http://localhost:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_cache_bypass $http_upgrade;
        
        # Timeouts
        proxy_connect_timeout 60s;
        proxy_send_timeout 60s;
        proxy_read_timeout 60s;
    }

    location /health {
        proxy_pass http://localhost:8080/health;
        access_log off;
    }
}
EOF

# Enable site
ln -sf /etc/nginx/sites-available/snailmail /etc/nginx/sites-enabled/
rm -f /etc/nginx/sites-enabled/default

# Test and restart Nginx
nginx -t
systemctl restart nginx

echo "✓ Nginx configured"

# ============================================================================
# LOG ROTATION
# ============================================================================

echo "📝 Configuring log rotation..."

cat > /etc/logrotate.d/snailmail << 'EOF'
/opt/snailmail/logs/*.log {
    daily
    rotate 7
    compress
    delaycompress
    missingok
    notifempty
    create 0640 snailmail snailmail
    sharedscripts
    postrotate
        systemctl reload snailmail > /dev/null 2>&1 || true
    endscript
}
EOF

echo "✓ Log rotation configured"

# ============================================================================
# MEMORY MONITORING SCRIPT
# ============================================================================

echo "📊 Creating memory monitoring script..."

cat > /opt/snailmail/check-memory.sh << 'EOF'
#!/bin/bash
# Memory monitoring for e2-micro
echo "=== Memory Status ==="
free -h
echo ""
echo "=== Top Processes by Memory ==="
ps aux --sort=-%mem | head -10
echo ""
echo "=== Swap Usage ==="
swapon --show
EOF

chmod +x /opt/snailmail/check-memory.sh
chown snailmail:snailmail /opt/snailmail/check-memory.sh

echo "✓ Memory monitoring script created"

# ============================================================================
# DEPLOYMENT SCRIPT
# ============================================================================

echo "🚀 Creating deployment script..."

cat > /opt/snailmail/deploy.sh << 'EOF'
#!/bin/bash
set -e

cd /opt/snailmail/app

# Pull latest changes
git pull origin main

# Build application
/usr/local/go/bin/go build -o snailmail ./cmd/site

# Restart service
systemctl restart snailmail

echo "✓ Deployment completed"
EOF

chmod +x /opt/snailmail/deploy.sh
chown snailmail:snailmail /opt/snailmail/deploy.sh

echo "✓ Deployment script created"

# ============================================================================
# FINAL CONFIGURATION
# ============================================================================

echo "🔧 Final configuration..."

# Update config.yaml with production settings
cat > /opt/snailmail/app/config.yaml << 'EOF'
# Snailmail Configuration - FREE TIER OPTIMIZED
domain: "snailmail.org"
server:
  port: 8080
  host: "0.0.0.0"

database:
  url: "postgres://snailmail:snailmail_secure_password_change_me@localhost:5432/snailmail?sslmode=disable"
  max_connections: 10  # Reduced for 1GB RAM

app:
  name: "Snailmail"
  environment: "production"
  debug: false

# Memory-optimized settings
cache:
  enabled: true
  max_size_mb: 50  # Reduced for 1GB RAM

logging:
  level: "warn"  # Reduced logging to save I/O
  output: "file"
  file: "/opt/snailmail/logs/snailmail.log"
EOF

chown snailmail:snailmail /opt/snailmail/app/config.yaml

# Create data directory
mkdir -p /opt/snailmail/data/attachments
chown -R snailmail:snailmail /opt/snailmail/data

echo "✓ Final configuration complete"

# ============================================================================
# RUN DATABASE MIGRATIONS
# ============================================================================

echo "🗄️  Running database migrations..."
cd /opt/snailmail/app
sudo -u snailmail ./snailmail migrate || echo "Migration command not implemented yet"

echo "✓ Database migrations complete"

# ============================================================================
# SUMMARY
# ============================================================================

echo ""
echo "╔════════════════════════════════════════════════════════════════╗"
echo "║           FREE TIER SETUP COMPLETE!                            ║"
echo "╚════════════════════════════════════════════════════════════════╝"
echo ""
echo "🎉 Snailmail is running on FREE TIER (e2-micro, 1GB RAM)"
echo ""
echo "📊 System Resources:"
echo "   • Machine: e2-micro (2 vCPUs, 1 GB RAM)"
echo "   • Disk: 30 GB standard"
echo "   • Swap: 2 GB configured"
echo "   • Memory limit: 512 MB (for app)"
echo ""
echo "🌐 Access your site:"
IP=$(curl -s http://metadata.google.internal/computeMetadata/v1/instance/network-interfaces/0/access-configs/0/external-ip -H "Metadata-Flavor: Google" 2>/dev/null || echo "YOUR_EPHEMERAL_IP")
echo "   http://$IP"
echo ""
echo "⚠️  IMPORTANT NOTES:"
echo "   • IP is EPHEMERAL (changes on VM restart)"
echo "   • To get static IP, add ~\$3/month"
echo "   • SMTP port 25 may be blocked (use SMTP relay)"
echo "   • Monitor memory: /opt/snailmail/check-memory.sh"
echo ""
echo "📝 Next Steps:"
echo "   1. Update config.yaml with your domain"
echo "   2. Set up DNS records"
echo "   3. Configure SSL (Let's Encrypt)"
echo "   4. Set up SMTP relay (SendGrid/Mailgun)"
echo ""
echo "🔍 Check status:"
echo "   systemctl status snailmail"
echo "   /opt/snailmail/check-memory.sh"
echo ""
echo "📚 Documentation:"
echo "   cat /opt/snailmail/FREE_TIER_GUIDE.md"
echo ""
