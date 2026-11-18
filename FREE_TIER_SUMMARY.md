# 🎉 FREE TIER SETUP COMPLETE!

## Your Snailmail Server Runs for $0/month on Google Cloud!

All configuration files have been created and optimized for Google Cloud's **Always Free** tier.

## 📁 Files Created/Updated

### 🚀 Main Setup
- **`setup-free-tier.sh`** - Automated setup script (run this!)
- **`README_FREE_TIER.md`** - Quick start guide
- **`FREE_TIER_GUIDE.md`** - Complete documentation

### 🔧 Infrastructure (Terraform)
- **`infrastructure/terraform/main.tf`** - Free tier config (e2-micro)
- **`infrastructure/terraform/variables.tf`** - Free tier variables
- **`infrastructure/terraform/terraform.tfvars`** - Your configuration
- **`infrastructure/scripts/startup.sh`** - Optimized VM init (1GB RAM)

### 🌐 Dynamic DNS (For Ephemeral IP)
- **`scripts/update-dynamic-dns.sh`** - Auto-update DNS on IP change
- **`scripts/snailmail-dns-updater.service`** - Systemd service

### 📚 Documentation
- **`DNS_CONFIG.md`** - DNS setup instructions
- **`DNS_RECORDS.txt`** - Copy-paste DNS records
- **`QUICKSTART.md`** - Step-by-step guide
- **`GCP_SETUP.md`** - GCP-specific instructions

## 🎯 What You Get (FREE!)

| Feature | Free Tier | Details |
|---------|-----------|---------|
| **VM** | e2-micro | 2 vCPUs, 1GB RAM |
| **Storage** | 30 GB | Standard persistent disk |
| **Network** | 1 GB/month | Egress outside North America |
| **Swap** | 2 GB | Auto-configured for safety |
| **PostgreSQL** | Optimized | Tuned for low memory |
| **Nginx** | Reverse proxy | Configured and ready |
| **Go App** | Memory limited | 512MB max via systemd |
| **Firewall** | Secure | Only necessary ports open |
| **Auto-deploy** | GitHub Actions | Push to main = auto-deploy |

## 🚀 Quick Start (3 Steps)

### 1. Run Setup Script
```bash
cd /home/binarygeek119/Projects/snailmail
./setup-free-tier.sh
```

This will:
- Create GCP VM (e2-micro, FREE)
- Install all dependencies
- Configure PostgreSQL (optimized for 1GB RAM)
- Set up firewall
- Deploy application
- Create systemd services

### 2. Configure DNS
After setup, get your VM IP:
```bash
gcloud compute instances describe snailmail-server \
  --zone=us-central1-a \
  --format='value(networkInterfaces[0].accessConfigs[0].natIP)'
```

Then:
- **Option A**: Use DuckDNS (free, handles IP changes)
- **Option B**: Buy static IP ($3/month)

### 3. Set Up SMTP Relay
GCP blocks port 25, so use SendGrid (free tier: 100 emails/day):
```bash
# SSH into VM
gcloud compute ssh deploy@snailmail-server --zone=us-central1-a

# Edit config
sudo nano /opt/snailmail/app/config.yaml

# Add SendGrid credentials
smtp:
  host: "smtp.sendgrid.net"
  port: 587
  username: "apikey"
  password: "YOUR_SENDGRID_API_KEY"
```

## 💰 Cost Breakdown

### FREE TIER ($0/month) ✅
- e2-micro VM: **$0**
- 30 GB disk: **$0**
- Ephemeral IP: **$0**
- 1 GB egress: **$0**
- **Total: $0/month**

### Optional Upgrades
- Static IP: +$3/month (avoids DNS updates on restart)
- Additional storage: +$0.04/GB/month
- More egress: +$0.12/GB after first 1 GB

## ⚙️ Optimizations for 1GB RAM

We've tuned everything for the e2-micro's limited resources:

### Memory
- ✅ 2GB swap space (safety net)
- ✅ PostgreSQL: 128MB shared_buffers
- ✅ App: Limited to 512MB
- ✅ Swappiness: 10 (prefer RAM over swap)

### Database
- ✅ Max connections: 20 (reduced from default)
- ✅ Work memory: 4MB
- ✅ Minimal logging

### Application
- ✅ Memory limits via systemd
- ✅ Reduced cache size (50MB)
- ✅ Minimal logging (warn level)

### System
- ✅ Log rotation (7 days)
- ✅ Fail2ban (security)
- ✅ Auto-updates (security patches)

## 🔍 Useful Commands

### Check Status
```bash
# SSH into VM
gcloud compute ssh deploy@snailmail-server --zone=us-central1-a

# Check services
sudo systemctl status snailmail
sudo systemctl status postgresql
sudo systemctl status nginx

# Check memory
/opt/snailmail/check-memory.sh
```

### View Logs
```bash
# App logs
sudo journalctl -u snailmail -f

# Nginx logs
sudo tail -f /var/log/nginx/error.log

# PostgreSQL logs
sudo tail -f /var/log/postgresql/postgresql-14-main.log
```

### Deploy Updates
```bash
# Manual deploy
/opt/snailmail/deploy.sh

# Or push to GitHub (auto-deploys)
git push origin main
```

## 📊 Free Tier Limits & Workarounds

| Limit | Free Tier | Our Solution |
|-------|-----------|--------------|
| **RAM** | 1 GB | 2GB swap + optimized configs |
| **CPU** | 2 vCPUs | Sufficient for mail server |
| **Disk** | 30 GB | Adequate for mail storage |
| **Egress** | 1 GB/month | Monitor usage, compress images |
| **Port 25** | Blocked | Use SMTP relay (SendGrid free) |
| **Static IP** | $3/month | Use DuckDNS (free) |

## 🌐 DNS Options

### Option A: DuckDNS (FREE) ✅ Recommended
```bash
# 1. Sign up at https://www.duckdns.org/
# 2. Create subdomain: snailmail.duckdns.org
# 3. Edit script with your token
nano scripts/update-dynamic-dns.sh

# 4. Install as service
sudo cp scripts/snailmail-dns-updater.service /etc/systemd/system/
sudo systemctl enable --now snailmail-dns-updater
```

### Option B: Static IP ($3/month)
```bash
# Reserve static IP
gcloud compute addresses create snailmail-ip --region=us-central1

# Update Terraform to use it (removes from free tier)
# Edit infrastructure/terraform/main.tf
```

## 📧 SMTP Relay (Required)

GCP blocks outbound port 25. Use one of these free options:

### SendGrid (Recommended)
- **Free tier**: 100 emails/day
- **Sign up**: https://sendgrid.com/
- **Setup**: Add API key to config.yaml

### Mailgun
- **Free tier**: 100 emails/day (first 3 months)
- **Sign up**: https://www.mailgun.com/

### AWS SES
- **Free tier**: 62,000 emails/month (if sent from EC2)
- **Sign up**: https://aws.amazon.com/ses/

## 🔐 Security Features

✅ **Firewall**: Only ports 22, 80, 443, 1143, 25 open
✅ **Fail2ban**: Blocks brute force attacks
✅ **SSH keys**: No password authentication
✅ **SSL/TLS**: Let's Encrypt (free)
✅ **PostgreSQL**: Local only, not exposed
✅ **App isolation**: systemd security features
✅ **Auto-updates**: Security patches

## 📈 Monitoring

### Check Free Tier Usage
```bash
# View billing
gcloud billing accounts list

# Check VM metrics
gcloud compute instances describe snailmail-server --zone=us-central1-a

# Check memory
free -h
```

### Set Up Alerts
1. Go to GCP Console → Billing → Budgets
2. Create budget alert for $1
3. Get email if you exceed free tier

## 🆘 Troubleshooting

### VM Won't Start
```bash
gcloud compute instances reset snailmail-server --zone=us-central1-a
```

### Out of Memory
```bash
# Check what's using memory
ps aux --sort=-%mem | head -20

# Increase swap temporarily
sudo fallocate -l 1G /swapfile2
sudo mkswap /swapfile2
sudo swapon /swapfile2
```

### High CPU
```bash
# Check processes
top -bn1 | head -20

# Restart services
sudo systemctl restart postgresql
sudo systemctl restart snailmail
```

### SMTP Not Working
- Check SendGrid API key in config.yaml
- Verify port 587 is accessible
- Check logs: `sudo journalctl -u snailmail -f`

## 📚 Documentation

- **[README_FREE_TIER.md](README_FREE_TIER.md)** - Quick start
- **[FREE_TIER_GUIDE.md](FREE_TIER_GUIDE.md)** - Complete guide
- **[DNS_CONFIG.md](DNS_CONFIG.md)** - DNS setup
- **[QUICKSTART.md](QUICKSTART.md)** - Step-by-step
- **[GCP_SETUP.md](GCP_SETUP.md)** - GCP instructions

## 🎯 Next Steps

1. ✅ Review files created
2. 🚀 Run `./setup-free-tier.sh`
3. 🌐 Configure DNS (DuckDNS recommended)
4. 📧 Set up SMTP relay (SendGrid)
5. 🔒 Install SSL certificate
6. ✉️ Start sending mail!

## 💡 Pro Tips

1. **Monitor daily** for first week
2. **Use dynamic DNS** to avoid $3/month static IP
3. **Compress attachments** to save egress
4. **Enable caching** to reduce database load
5. **Use Cloudflare** (free) for CDN and DDoS protection
6. **Set up backups** to Google Cloud Storage (free tier)

## 🎉 Success Metrics

After setup, you'll have:
- ✅ Fully functional mail server
- ✅ Web interface accessible via HTTPS
- ✅ IMAP support for email clients
- ✅ Auto-deployment from GitHub
- ✅ $0 monthly cost
- ✅ Production-ready configuration

---

## 📞 Need Help?

1. Check [FREE_TIER_GUIDE.md](FREE_TIER_GUIDE.md) for detailed instructions
2. Review troubleshooting section above
3. Check logs: `sudo journalctl -u snailmail -f`
4. Open GitHub issue if stuck

---

**Ready to launch?** Run the setup script:

```bash
./setup-free-tier.sh
```

🐌✉️ Happy mailing on the free tier!
