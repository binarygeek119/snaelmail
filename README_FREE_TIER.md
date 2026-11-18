# 🎉 Snailmail on Google Cloud FREE TIER

## Run Your Mail Server for $0/month!

This project is fully configured to run on Google Cloud's **Always Free** tier with zero monthly cost.

## 🚀 Quick Start (5 minutes)

```bash
# 1. Clone and setup
git clone https://github.com/YOUR_USERNAME/snailmail.git
cd snailmail

# 2. Run the automated setup
./setup-free-tier.sh
```

That's it! The script will:
- ✅ Create a free e2-micro VM (2 vCPUs, 1GB RAM)
- ✅ Install Go, PostgreSQL, Nginx
- ✅ Configure and optimize everything
- ✅ Set up firewall and security
- ✅ Deploy the application

## 💰 What's Free?

| Resource | Free Tier | Our Usage |
|----------|-----------|-----------|
| VM (e2-micro) | Always free | ✓ 2 vCPUs, 1GB RAM |
| Disk (30GB) | Always free | ✓ Standard persistent disk |
| Network (1GB) | Always free | ✓ Egress to North America |
| **Total Cost** | | **$0/month** |

### What's NOT Free (Optional)
- Static IP: $3/month (we use ephemeral IP + dynamic DNS)
- Additional storage: $0.04/GB/month
- SMTP relay: Free tiers available (SendGrid: 100/day)

## 📋 Prerequisites

- Google Cloud account ([sign up free](https://cloud.google.com/))
- gcloud CLI ([install guide](https://cloud.google.com/sdk/docs/install))
- Terraform ([install guide](https://developer.hashicorp.com/terraform/downloads))
- Domain name (~$10/year from Namecheap/Porkbun)

## 🔧 After Setup

### 1. Get Your VM IP
```bash
gcloud compute instances describe snailmail-server \
  --zone=us-central1-a \
  --format='value(networkInterfaces[0].accessConfigs[0].natIP)'
```

### 2. Configure DNS
Point your domain to the VM IP. Since the IP is ephemeral (changes on restart), use:

**Option A: DuckDNS (Free)**
```bash
# Edit the script with your DuckDNS token
nano scripts/update-dynamic-dns.sh

# Install as systemd service
sudo cp scripts/snailmail-dns-updater.service /etc/systemd/system/
sudo systemctl enable --now snailmail-dns-updater
```

**Option B: Static IP ($3/month)**
```bash
# Reserve static IP
gcloud compute addresses create snailmail-ip --region=us-central1
```

### 3. Set Up SMTP Relay (Required)
GCP blocks port 25. Use a free SMTP relay:

```bash
# SSH into VM
gcloud compute ssh deploy@snailmail-server --zone=us-central1-a

# Edit config
sudo nano /opt/snailmail/app/config.yaml

# Add SMTP relay (example: SendGrid)
smtp:
  host: "smtp.sendgrid.net"
  port: 587
  username: "apikey"
  password: "YOUR_SENDGRID_API_KEY"
```

### 4. Set Up SSL
```bash
# SSH into VM
gcloud compute ssh deploy@snailmail-server --zone=us-central1-a

# Install Let's Encrypt
sudo apt update
sudo apt install certbot python3-certbot-nginx
sudo certbot --nginx -d snailmail.org -d www.snailmail.org
```

## 📊 Memory Optimization

The e2-micro has only 1GB RAM. We've optimized everything:

- ✅ 2GB swap space configured
- ✅ PostgreSQL tuned (128MB shared_buffers)
- ✅ App limited to 512MB via systemd
- ✅ Minimal logging to save I/O

Monitor memory:
```bash
/opt/snailmail/check-memory.sh
```

## 📁 File Structure

```
snailmail/
├── setup-free-tier.sh           # 🚀 Main setup script
├── FREE_TIER_GUIDE.md           # 📖 Complete guide
├── scripts/
│   ├── update-dynamic-dns.sh    # 🌐 Dynamic DNS updater
│   └── snailmail-dns-updater.service
├── infrastructure/
│   ├── terraform/
│   │   ├── main.tf              # Free tier Terraform config
│   │   └── variables.tf
│   └── scripts/
│       └── startup.sh           # VM initialization (optimized)
└── ... (application code)
```

## 🔍 Useful Commands

### SSH into VM
```bash
gcloud compute ssh deploy@snailmail-server --zone=us-central1-a
```

### Check Status
```bash
sudo systemctl status snailmail
sudo systemctl status postgresql
sudo systemctl status nginx
```

### View Logs
```bash
sudo journalctl -u snailmail -f
```

### Check Memory
```bash
/opt/snailmail/check-memory.sh
```

### Deploy Updates
```bash
/opt/snailmail/deploy.sh
```

## ⚠️ Free Tier Limitations

| Limitation | Workaround |
|------------|------------|
| Ephemeral IP (changes on restart) | Use DuckDNS (free) or static IP ($3/mo) |
| 1GB RAM | 2GB swap + optimized configs |
| Port 25 blocked | Use SMTP relay (SendGrid free tier) |
| 1GB egress/month | Monitor usage, upgrade if needed |

## 📈 Monitoring

### Check Free Tier Usage
```bash
# View billing
gcloud billing accounts list

# Check VM metrics
gcloud compute instances describe snailmail-server --zone=us-central1-a
```

### Set Up Alerts
```bash
# Create budget alert in GCP Console
# https://console.cloud.google.com/billing/budgets
```

## 🆘 Troubleshooting

### VM Won't Start
```bash
gcloud compute instances reset snailmail-server --zone=us-central1-a
```

### Out of Memory
```bash
# Check memory
free -h

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

## 💡 Tips

1. **Monitor daily**: Check memory and disk usage
2. **Use dynamic DNS**: Avoid $3/month static IP cost
3. **Optimize images**: Compress attachments to save egress
4. **Enable caching**: Reduce database load
5. **Use CDN**: Serve static assets from Cloudflare (free)

## 📚 Documentation

- **[FREE_TIER_GUIDE.md](FREE_TIER_GUIDE.md)** - Complete setup guide
- **[DNS_CONFIG.md](DNS_CONFIG.md)** - DNS setup instructions
- **[QUICKSTART.md](QUICKSTART.md)** - Quick start guide
- **[GCP_SETUP.md](GCP_SETUP.md)** - GCP-specific instructions

## 🎯 Cost Comparison

### FREE TIER ($0/month)
- e2-micro VM: $0
- 30 GB disk: $0
- Ephemeral IP: $0
- 1 GB egress: $0
- **Total: $0/month** ✅

### STANDARD ($9/month)
- e2-small VM: $6
- 30 GB disk: $0
- Static IP: $3
- **Total: ~$9/month**

### PAID ($25/month)
- e2-medium VM: $25
- 30 GB disk: $0
- Static IP: $3
- **Total: ~$28/month**

## 🎉 Success!

You now have a fully functional mail server running on Google Cloud's free tier!

**What you get:**
✅ Full mail server (SMTP, IMAP)
✅ Web interface
✅ Zero monthly cost
✅ Automatic updates via GitHub Actions
✅ Production-ready configuration

**Next steps:**
1. Run `./setup-free-tier.sh`
2. Configure DNS (DuckDNS recommended)
3. Set up SMTP relay (SendGrid free)
4. Install SSL certificate (Let's Encrypt)
5. Start sending mail! 🐌✉️

---

**Questions?** Check [FREE_TIER_GUIDE.md](FREE_TIER_GUIDE.md) for detailed instructions.

**Issues?** Open a GitHub issue or check the troubleshooting section.

**Love it?** Star the repo and share with friends! ⭐
