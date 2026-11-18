# Google Cloud FREE TIER Setup Guide

## 🎉 Your Snailmail Server Runs COMPLETELY FREE!

This guide explains how to run Snailmail on Google Cloud's **Always Free** tier with zero monthly cost.

## 📊 What's Included in Free Tier

### Compute Engine (Always Free)
- **Machine type**: e2-micro (2 vCPUs, 1 GB RAM)
- **Regions**: us-west1, us-central1, us-east1
- **Disk**: 30 GB standard persistent disk
- **Network**: 1 GB egress per month (outside North America)

### What's NOT Free
- **Static IP**: $3/month (we use ephemeral IP instead)
- **Additional storage**: $0.04/GB/month
- **Additional egress**: $0.12/GB after first 1 GB

## 🚀 Quick Setup (15 minutes)

### Step 1: Create GCP Account

1. Go to https://cloud.google.com/
2. Sign up (requires credit card, but won't charge for free tier)
3. Create a new project: `snailmail-free`

### Step 2: Enable APIs

```bash
# Install gcloud CLI
curl https://sdk.cloud.google.com | bash

# Login
gcloud auth login

# Set project
gcloud config set project snailmail-free

# Enable required APIs
gcloud services enable compute.googleapis.com
```

### Step 3: Configure Terraform

```bash
cd infrastructure/terraform

# Edit terraform.tfvars with your project ID
nano terraform.tfvars
# Change: project_id = "snailmail-free"
```

### Step 4: Deploy Infrastructure

```bash
# Initialize Terraform
terraform init

# Preview changes
terraform plan

# Apply (creates VM)
terraform apply
```

**Wait 5-10 minutes for the VM to boot and run the startup script.**

### Step 5: Get Your IP Address

```bash
# Get the ephemeral IP
gcloud compute instances describe snailmail-server \
  --zone=us-central1-a \
  --format='get(networkInterfaces[0].accessConfigs[0].natIP)'
```

**Save this IP!** You'll need it for DNS.

## ⚠️ FREE TIER LIMITATIONS

### 1. Ephemeral IP Address
**Problem**: IP changes when VM restarts
**Solution**: 
- Option A: Use dynamic DNS (DuckDNS, No-IP)
- Option B: Upgrade to static IP ($3/month)
- Option C: Script to update DNS on IP change

### 2. Limited RAM (1 GB)
**Problem**: PostgreSQL + Go app + Nginx can exceed 1GB
**Solution**: We've optimized everything:
- 2GB swap space configured
- PostgreSQL tuned for low memory (128MB shared_buffers)
- App limited to 512MB via systemd
- Conservative connection limits

### 3. SMTP Port 25 Blocked
**Problem**: GCP blocks outbound port 25 to prevent spam
**Solution**: Use SMTP relay service:
- **SendGrid**: 100 emails/day free
- **Mailgun**: 100 emails/day free (first 3 months)
- **AWS SES**: 62,000 emails/month free (if sent from EC2)

### 4. Limited Network Egress
**Problem**: Only 1 GB free egress per month
**Solution**: 
- Most traffic is within North America (free)
- Monitor usage: `gcloud compute instances describe snailmail-server`
- Upgrade if needed ($0.12/GB after first 1 GB)

## 🔧 SMTP Relay Setup (Required)

Since port 25 is blocked, use SendGrid:

### Step 1: Create SendGrid Account
1. Go to https://sendgrid.com/
2. Sign up for free tier (100 emails/day)
3. Verify your domain

### Step 2: Configure SMTP Relay

```bash
# SSH into VM
gcloud compute ssh deploy@snailmail-server --zone=us-central1-a

# Edit config.yaml
sudo nano /opt/snailmail/app/config.yaml
```

Add SMTP relay configuration:
```yaml
smtp:
  host: "smtp.sendgrid.net"
  port: 587
  username: "apikey"
  password: "YOUR_SENDGRID_API_KEY"
  from_address: "noreply@snailmail.org"
  from_name: "Snailmail"
  tls: true
```

### Step 3: Restart Application

```bash
sudo systemctl restart snailmail
```

## 🌐 DNS Setup with Ephemeral IP

### Option A: Dynamic DNS (Recommended for Free Tier)

Use DuckDNS (free) to handle IP changes:

1. Go to https://www.duckdns.org/
2. Sign in with Google
3. Create subdomain: `snailmail.duckdns.org`
4. Set IP to your VM's current IP

Install auto-update script:
```bash
# SSH into VM
gcloud compute ssh deploy@snailmail-server --zone=us-central1-a

# Create IP update script
cat > /opt/snailmail/update-ip.sh << 'EOF'
#!/bin/bash
# Update DuckDNS with current IP

DUCKDNS_TOKEN="YOUR_DUCKDNS_TOKEN"
DOMAIN="snailmail"

IP=$(curl -s http://metadata.google.internal/computeMetadata/v1/instance/network-interfaces/0/access-configs/0/external-ip -H "Metadata-Flavor: Google")

curl -s "https://www.duckdns.org/update?domains=$DOMAIN&token=$DUCKDNS_TOKEN&ip=$IP"
EOF

chmod +x /opt/snailmail/update-ip.sh

# Run on boot
cat > /etc/systemd/system/snailmail-ip-update.service << 'EOF'
[Unit]
Description=Update DuckDNS IP on boot
After=network.target

[Service]
Type=oneshot
ExecStart=/opt/snailmail/update-ip.sh
RemainAfterExit=yes

[Install]
WantedBy=multi-user.target
EOF

systemctl enable snailmail-ip-update
```

### Option B: Upgrade to Static IP ($3/month)

```bash
# Reserve static IP
gcloud compute addresses create snailmail-ip --region=us-central1

# Get the IP
gcloud compute addresses describe snailmail-ip --region=us-central1 --format='value(address)'

# Attach to VM (requires stopping VM)
gcloud compute instances stop snailmail-server --zone=us-central1-a
gcloud compute instances delete-access-config snailmail-server \
  --zone=us-central1-a \
  --access-config-name="external-nat"
gcloud compute instances add-access-config snailmail-server \
  --zone=us-central1-a \
  --address=YOUR_STATIC_IP
gcloud compute instances start snailmail-server --zone=us-central1-a
```

Then update Terraform to use static IP (removes from free tier).

## 📊 Memory Monitoring

Check memory usage:
```bash
# SSH into VM
gcloud compute ssh deploy@snailmail-server --zone=us-central1-a

# Check memory
/opt/snailmail/check-memory.sh

# Or detailed view
free -h
ps aux --sort=-%mem | head -10
```

### Memory Optimization Tips

1. **Reduce PostgreSQL connections**:
```bash
sudo nano /etc/postgresql/14/main/conf.d/snailmail.conf
# Change: max_connections = 10
```

2. **Limit app memory further**:
```bash
sudo systemctl edit snailmail
# Add:
[Service]
MemoryMax=384M
MemoryHigh=256M
```

3. **Disable caching if needed**:
```yaml
# In config.yaml
cache:
  enabled: false
```

## 🔍 Monitoring & Troubleshooting

### Check Service Status
```bash
sudo systemctl status snailmail
sudo systemctl status postgresql
sudo systemctl status nginx
```

### View Logs
```bash
# App logs
sudo journalctl -u snailmail -f

# Nginx logs
sudo tail -f /var/log/nginx/access.log
sudo tail -f /var/log/nginx/error.log

# PostgreSQL logs
sudo tail -f /var/log/postgresql/postgresql-14-main.log
```

### High Memory Usage?
```bash
# Check what's using memory
ps aux --sort=-%mem | head -20

# Check swap usage
swapon --show

# Restart services to free memory
sudo systemctl restart postgresql
sudo systemctl restart snailmail
```

### VM Not Responding?
```bash
# Check from GCP Console
# https://console.cloud.google.com/compute/instances

# Serial console (if SSH fails)
gcloud compute connect-to-serial-port snailmail-server --zone=us-central1-a

# Hard restart
gcloud compute instances reset snailmail-server --zone=us-central1-a
```

## 📈 When to Upgrade from Free Tier

Consider upgrading if:
- **Memory**: Consistently using >80% RAM + swap
- **CPU**: Sustained >70% CPU usage
- **Storage**: Using >25 GB disk
- **Traffic**: >1 GB egress per month
- **Users**: >100 active users

### Upgrade Path

```bash
# Edit Terraform config
nano infrastructure/terraform/main.tf

# Change machine type
machine_type = "e2-small"  # 2 vCPUs, 2 GB RAM ($6/month)

# Add static IP
resource "google_compute_address" "static" {
  name   = "snailmail-ip"
  region = var.region
}

# Apply changes
terraform apply
```

**Cost after upgrade**: ~$9/month (e2-small + static IP)

## 💰 Cost Breakdown

### FREE TIER ($0/month)
- e2-micro VM: $0
- 30 GB disk: $0
- Ephemeral IP: $0
- 1 GB egress: $0
- **Total: $0/month**

### UPGRADED (~$9/month)
- e2-small VM: $6
- 30 GB disk: $0
- Static IP: $3
- 1 GB egress: $0
- **Total: ~$9/month**

## 🎯 Checklist

After setup, verify:

- [ ] VM is running (`gcloud compute instances list`)
- [ ] Can SSH into VM (`gcloud compute ssh deploy@snailmail-server`)
- [ ] Application is responding (`curl http://YOUR_IP/health`)
- [ ] PostgreSQL is running (`sudo systemctl status postgresql`)
- [ ] Swap is configured (`swapon --show`)
- [ ] Memory usage is reasonable (`free -h`)
- [ ] SMTP relay is configured (test email sending)
- [ ] DNS is pointed to VM IP
- [ ] SSL certificate is installed (Let's Encrypt)

## 🆘 Support

### Out of Memory (OOM)
```bash
# Check OOM logs
sudo dmesg | grep -i "out of memory"

# Increase swap (temporary)
sudo fallocate -l 1G /swapfile2
sudo mkswap /swapfile2
sudo swapon /swapfile2

# Make permanent
echo '/swapfile2 none swap sw 0 0' | sudo tee -a /etc/fstab
```

### High CPU Usage
```bash
# Check what's using CPU
top -bn1 | head -20

# Restart heavy services
sudo systemctl restart postgresql
sudo systemctl restart snailmail
```

### Disk Full
```bash
# Check disk usage
df -h

# Clean up old logs
sudo journalctl --vacuum-time=7d
sudo apt-get clean

# Remove old packages
sudo apt-get autoremove -y
```

## 📚 Additional Resources

- [GCP Free Tier Documentation](https://cloud.google.com/free/docs/free-tier-usage-limits)
- [PostgreSQL Performance Tuning](https://www.postgresql.org/docs/current/runtime-config.html)
- [Go Memory Profiling](https://go.dev/blog/profiling-go-programs)

## 🎉 Success!

You now have a fully functional mail server running on Google Cloud's free tier!

**Key accomplishments:**
✅ Zero monthly cost
✅ Full mail server functionality
✅ Automatic memory optimization
✅ Swap space for safety
✅ Production-ready configuration

**Remember:**
- Monitor memory usage regularly
- Use SMTP relay for outbound email
- Consider dynamic DNS for ephemeral IP
- Upgrade when you outgrow free tier

Happy mailing! 🐌✉️
