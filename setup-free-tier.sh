#!/bin/bash
# Snailmail Free Tier Setup Script
# This script automates the entire FREE TIER setup process

set -e

echo "╔════════════════════════════════════════════════════════════════╗"
echo "║     Snailmail FREE TIER Setup (Google Cloud e2-micro)         ║"
echo "╚════════════════════════════════════════════════════════════════╝"
echo ""

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# ============================================================================
# PREREQUISITES CHECK
# ============================================================================

echo "🔍 Checking prerequisites..."

# Check gcloud
if ! command -v gcloud &> /dev/null; then
    echo -e "${RED}❌ gcloud CLI not found${NC}"
    echo "Install: https://cloud.google.com/sdk/docs/install"
    exit 1
fi

# Check terraform
if ! command -v terraform &> /dev/null; then
    echo -e "${RED}❌ Terraform not found${NC}"
    echo "Install: https://developer.hashicorp.com/terraform/downloads"
    exit 1
fi

# Check authentication
if ! gcloud auth list --filter=status:ACTIVE --format="value(account)" | grep -q "@"; then
    echo -e "${RED}❌ Not authenticated with gcloud${NC}"
    echo "Run: gcloud auth login"
    exit 1
fi

echo -e "${GREEN}✓ All prerequisites met${NC}"
echo ""

# ============================================================================
# PROJECT SETUP
# ============================================================================

echo "📋 Project Configuration"
echo ""

# Get current project
CURRENT_PROJECT=$(gcloud config get-value project 2>/dev/null)
echo "Current project: $CURRENT_PROJECT"
echo ""

read -p "Use this project? (Y/n): " USE_CURRENT
if [[ $USE_CURRENT =~ ^[Nn] ]]; then
    read -p "Enter project ID: " PROJECT_ID
    gcloud config set project $PROJECT_ID
else
    PROJECT_ID=$CURRENT_PROJECT
fi

echo ""
echo "Using project: $PROJECT_ID"
echo ""

# ============================================================================
# REGION SELECTION (FREE TIER)
# ============================================================================

echo "🌍 Select FREE TIER region:"
echo "  1) us-central1 (Iowa) - Recommended"
echo "  2) us-west1 (Oregon)"
echo "  3) us-east1 (South Carolina)"
echo ""
read -p "Choose region (1-3) [1]: " REGION_CHOICE

case $REGION_CHOICE in
    2) REGION="us-west1"; ZONE="us-west1-a" ;;
    3) REGION="us-east1"; ZONE="us-east1-b" ;;
    *) REGION="us-central1"; ZONE="us-central1-a" ;;
esac

echo "Selected: $REGION ($ZONE)"
echo ""

# ============================================================================
# SSH KEY SETUP
# ============================================================================

echo "🔑 Setting up SSH keys..."

SSH_KEY_PATH="$HOME/.ssh/snailmail_deploy"

if [ ! -f "$SSH_KEY_PATH" ]; then
    echo "Generating SSH key pair..."
    ssh-keygen -t ed25519 -f "$SSH_KEY_PATH" -C "snailmail-free-tier" -N ""
    echo -e "${GREEN}✓ SSH keys generated${NC}"
else
    echo -e "${GREEN}✓ SSH keys already exist${NC}"
fi

# Update terraform.tfvars
cat > infrastructure/terraform/terraform.tfvars << EOF
project_id          = "$PROJECT_ID"
region              = "$REGION"
zone                = "$ZONE"
ssh_user            = "deploy"
ssh_public_key_path = "$SSH_KEY_PATH.pub"
EOF

echo "✓ Terraform configuration updated"
echo ""

# ============================================================================
# ENABLE APIS
# ============================================================================

echo "🔌 Enabling Google Cloud APIs..."

gcloud services enable compute.googleapis.com 2>/dev/null || true
gcloud services enable iam.googleapis.com 2>/dev/null || true

echo -e "${GREEN}✓ APIs enabled${NC}"
echo ""

# ============================================================================
# TERRAFORM DEPLOYMENT
# ============================================================================

echo "🏗️  Deploying infrastructure with Terraform..."
echo ""

cd infrastructure/terraform

# Initialize
echo "Initializing Terraform..."
terraform init -input=false

# Plan
echo ""
echo "Planning changes..."
terraform plan -input=false -out=tfplan

echo ""
read -p "Apply these changes? (Y/n): " APPLY
if [[ $APPLY =~ ^[Nn] ]]; then
    echo "Deployment cancelled"
    exit 0
fi

# Apply
echo ""
echo "Applying changes (this takes 5-10 minutes)..."
terraform apply -input=false tfplan

# Get outputs
VM_IP=$(terraform output -raw instance_ip 2>/dev/null || echo "pending")
VM_NAME=$(terraform output -raw instance_name 2>/dev/null || echo "snailmail-server")

cd ../..

echo ""
echo -e "${GREEN}✓ Infrastructure deployed!${NC}"
echo ""

# ============================================================================
# WAIT FOR VM TO BOOT
# ============================================================================

echo "⏳ Waiting for VM to boot and initialize (5-10 minutes)..."
echo ""

# Wait for VM to be running
echo "Checking VM status..."
for i in {1..60}; do
    STATUS=$(gcloud compute instances describe $VM_NAME --zone=$ZONE --format='value(status)' 2>/dev/null || echo "UNKNOWN")
    if [ "$STATUS" = "RUNNING" ]; then
        echo -e "${GREEN}✓ VM is running${NC}"
        break
    fi
    echo "  Status: $STATUS (waiting...)"
    sleep 10
done

echo ""
echo "Waiting for startup script to complete..."
sleep 30

# ============================================================================
# TEST SSH CONNECTION
# ============================================================================

echo "🔐 Testing SSH connection..."

for i in {1..10}; do
    if gcloud compute ssh deploy@$VM_NAME --zone=$ZONE --command="echo 'SSH OK'" 2>/dev/null; then
        echo -e "${GREEN}✓ SSH connection successful${NC}"
        break
    fi
    echo "  Attempt $i/10 failed, retrying..."
    sleep 10
done

echo ""

# ============================================================================
# GET PUBLIC IP
# ============================================================================

echo "🌐 Getting public IP address..."

VM_IP=$(gcloud compute instances describe $VM_NAME --zone=$ZONE --format='value(networkInterfaces[0].accessConfigs[0].natIP)')

echo ""
echo "╔════════════════════════════════════════════════════════════════╗"
echo "║                    SETUP COMPLETE!                             ║"
echo "╚════════════════════════════════════════════════════════════════╝"
echo ""
echo -e "${GREEN}✓ Snailmail is running on FREE TIER!${NC}"
echo ""
echo "📍 VM Details:"
echo "   Name: $VM_NAME"
echo "   Zone: $ZONE"
echo "   Type: e2-micro (FREE)"
echo "   IP:   $VM_IP (ephemeral)"
echo ""
echo "🌐 Access your site:"
echo "   http://$VM_IP"
echo ""
echo "🔐 SSH into VM:"
echo "   gcloud compute ssh deploy@$VM_NAME --zone=$ZONE"
echo ""

# ============================================================================
# NEXT STEPS
# ============================================================================

echo "📋 Next Steps:"
echo ""
echo "1️⃣  Configure DNS:"
echo "   • Point snailmail.org to $VM_IP"
echo "   • Or use dynamic DNS (DuckDNS) for ephemeral IP"
echo ""
echo "2️⃣  Set up SMTP relay (required - port 25 blocked):"
echo "   • Sign up for SendGrid free tier"
echo "   • Edit /opt/snailmail/app/config.yaml"
echo "   • Add SMTP relay credentials"
echo ""
echo "3️⃣  Set up SSL certificate:"
echo "   • SSH into VM"
echo "   • Run: sudo certbot --nginx -d snailmail.org"
echo ""
echo "4️⃣  Monitor memory usage:"
echo "   • SSH into VM"
echo "   • Run: /opt/snailmail/check-memory.sh"
echo ""

# ============================================================================
# FREE TIER WARNINGS
# ============================================================================

echo -e "${YELLOW}⚠️  FREE TIER IMPORTANT NOTES:${NC}"
echo ""
echo "• IP is EPHEMERAL (changes on VM restart)"
echo "  → Use dynamic DNS or upgrade to static IP (\$3/month)"
echo ""
echo "• SMTP port 25 is BLOCKED"
echo "  → Use SendGrid/Mailgun relay (100 emails/day free)"
echo ""
echo "• Limited to 1 GB RAM"
echo "  → 2 GB swap configured automatically"
echo "  → Monitor with: /opt/snailmail/check-memory.sh"
echo ""
echo "• 1 GB network egress per month"
echo "  → Monitor: gcloud compute instances describe $VM_NAME"
echo ""

# ============================================================================
# SAVE SETUP INFO
# ============================================================================

cat > SETUP_INFO.txt << EOF
Snailmail FREE TIER Setup Information
======================================

Date: $(date)
Project: $PROJECT_ID
Region: $REGION
Zone: $ZONE

VM Details:
  Name: $VM_NAME
  Type: e2-micro (FREE)
  IP: $VM_IP (ephemeral)
  RAM: 1 GB + 2 GB swap

SSH Access:
  gcloud compute ssh deploy@$VM_NAME --zone=$ZONE

Web Access:
  http://$VM_IP

Useful Commands:
  Check memory:    /opt/snailmail/check-memory.sh
  View logs:       sudo journalctl -u snailmail -f
  Restart app:     sudo systemctl restart snailmail
  Deploy update:   /opt/snailmail/deploy.sh

Configuration Files:
  App config:      /opt/snailmail/app/config.yaml
  Nginx config:    /etc/nginx/sites-available/snailmail
  PostgreSQL:      /etc/postgresql/14/main/conf.d/snailmail.conf

Documentation:
  FREE_TIER_GUIDE.md - Complete free tier guide
  DNS_CONFIG.md - DNS setup instructions
  QUICKSTART.md - Quick start guide

Cost: \$0/month (FREE TIER)
EOF

echo "💾 Setup info saved to SETUP_INFO.txt"
echo ""
echo "📚 Read FREE_TIER_GUIDE.md for detailed instructions"
echo ""
echo -e "${GREEN}🎉 Enjoy your free mail server! 🐌✉️${NC}"
