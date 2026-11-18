#!/bin/bash
set -e

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${BLUE}╔════════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║         Google Cloud Setup for Snailmail Free Tier            ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════════════════╝${NC}"
echo ""

# Check if gcloud is installed
if ! command -v gcloud &> /dev/null; then
    echo -e "${YELLOW}⚠️  gcloud CLI not found${NC}"
    echo ""
    echo "Please install gcloud CLI first:"
    echo ""
    echo "On Fedora/RHEL/CentOS:"
    echo "  sudo tee -a /etc/yum.repos.d/google-cloud-sdk.repo << EOF"
    echo "[google-cloud-cli]"
    echo "name=Google Cloud CLI"
    echo "baseurl=https://packages.cloud.google.com/yum/repos/cloud-sdk-el9-x86_64"
    echo "enabled=1"
    echo "gpgcheck=1"
    echo "repo_gpgcheck=0"
    echo "gpgkey=https://packages.cloud.google.com/yum/doc/rpm-package-key.gpg"
    echo "EOF"
    echo ""
    echo "  sudo yum install google-cloud-cli"
    echo ""
    echo "On macOS:"
    echo "  brew install --cask google-cloud-sdk"
    echo ""
    echo "On Windows:"
    echo "  Download from: https://cloud.google.com/sdk/docs/install"
    echo ""
    echo "After installation, run this script again."
    exit 1
fi

echo -e "${GREEN}✓ gcloud CLI found${NC}"
echo ""

# Check if authenticated
echo -e "${BLUE}Checking authentication...${NC}"
if ! gcloud auth list --filter=status:ACTIVE --format="value(account)" | grep -q "@"; then
    echo -e "${YELLOW}⚠️  Not authenticated${NC}"
    echo ""
    echo "Opening browser for authentication..."
    gcloud auth login
else
    ACCOUNT=$(gcloud auth list --filter=status:ACTIVE --format="value(account)")
    echo -e "${GREEN}✓ Authenticated as: ${ACCOUNT}${NC}"
fi
echo ""

# Check for existing project
echo -e "${BLUE}Checking for existing projects...${NC}"
PROJECTS=$(gcloud projects list --format="value(projectId)" 2>/dev/null || echo "")

if [ -z "$PROJECTS" ]; then
    echo -e "${YELLOW}⚠️  No projects found${NC}"
    echo ""
    read -p "Create a new project? (y/n): " CREATE_PROJECT
    if [ "$CREATE_PROJECT" = "y" ]; then
        PROJECT_ID="snailmail-free-tier-$(date +%s)"
        echo ""
        echo "Creating project: $PROJECT_ID"
        gcloud projects create $PROJECT_ID --name="Snailmail Free Tier"
        gcloud config set project $PROJECT_ID
        echo -e "${GREEN}✓ Project created and set${NC}"
    else
        echo ""
        echo "Please create a project in the Google Cloud Console:"
        echo "https://console.cloud.google.com/projectcreate"
        echo ""
        echo "Then set it with:"
        echo "  gcloud config set project YOUR_PROJECT_ID"
        exit 1
    fi
else
    echo -e "${GREEN}✓ Found projects:${NC}"
    gcloud projects list --format="table(projectId,name)" | head -10
    echo ""
    
    CURRENT_PROJECT=$(gcloud config get-value project 2>/dev/null || echo "")
    
    if [ -z "$CURRENT_PROJECT" ]; then
        read -p "Enter project ID to use: " PROJECT_ID
        gcloud config set project $PROJECT_ID
    else
        echo -e "${GREEN}✓ Using project: ${CURRENT_PROJECT}${NC}"
        read -p "Use this project? (y/n): " USE_CURRENT
        if [ "$USE_CURRENT" = "n" ]; then
            read -p "Enter project ID to use: " PROJECT_ID
            gcloud config set project $PROJECT_ID
        else
            PROJECT_ID=$CURRENT_PROJECT
        fi
    fi
fi

echo ""
echo -e "${BLUE}Enabling required APIs...${NC}"
gcloud services enable compute.googleapis.com --quiet
gcloud services enable cloudresourcemanager.googleapis.com --quiet
echo -e "${GREEN}✓ APIs enabled${NC}"
echo ""

# Update terraform.tfvars
echo -e "${BLUE}Updating Terraform configuration...${NC}"
cd infrastructure/terraform

cat > terraform.tfvars << EOF
project_id          = "$PROJECT_ID"
region              = "us-central1"
zone                = "us-central1-a"
ssh_user            = "deploy"
ssh_public_key_path = "/home/binarygeek119/.ssh/snailmail_deploy.pub"
EOF

echo -e "${GREEN}✓ Updated terraform.tfvars${NC}"
echo ""

# Check billing
echo -e "${BLUE}Checking billing setup...${NC}"
echo -e "${YELLOW}⚠️  IMPORTANT: You must set up billing to use Google Cloud${NC}"
echo ""
echo "Even though we're using the FREE TIER, Google requires a billing account."
echo "You will NOT be charged for free tier resources."
echo ""
echo "Please ensure billing is set up:"
echo "1. Go to: https://console.cloud.google.com/billing"
echo "2. Link a billing account to your project"
echo "3. Verify your payment method"
echo ""
read -p "Have you set up billing? (y/n): " BILLING_SETUP

if [ "$BILLING_SETUP" != "y" ]; then
    echo ""
    echo "Please set up billing before continuing."
    echo "Open: https://console.cloud.google.com/billing"
    exit 1
fi

echo ""
echo -e "${GREEN}✓ Billing confirmed${NC}"
echo ""

# Initialize Terraform
echo -e "${BLUE}Initializing Terraform...${NC}"
terraform init
echo -e "${GREEN}✓ Terraform initialized${NC}"
echo ""

echo -e "${BLUE}╔════════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║                    Setup Complete!                             ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════════════════╝${NC}"
echo ""
echo -e "${GREEN}Project ID: ${PROJECT_ID}${NC}"
echo -e "${GREEN}Region: us-central1${NC}"
echo -e "${GREEN}Zone: us-central1-a${NC}"
echo ""
echo "Next steps:"
echo ""
echo "1. Preview the infrastructure:"
echo "   cd infrastructure/terraform"
echo "   terraform plan"
echo ""
echo "2. Deploy the infrastructure:"
echo "   terraform apply"
echo ""
echo "3. Get your VM IP address:"
echo "   gcloud compute instances describe snailmail-server \\"
echo "     --zone=us-central1-a \\"
echo "     --format='get(networkInterfaces[0].accessConfigs[0].natIP)'"
echo ""
echo "4. Configure DNS (see DNS_CONFIG.md)"
echo ""
echo "5. Set up SSL and SMTP relay"
echo ""
echo -e "${YELLOW}⚠️  Remember: This is using the FREE TIER${NC}"
echo "- e2-micro VM (2 vCPUs, 1GB RAM)"
echo "- 30GB disk"
echo "- 1GB network egress/month"
echo "- Ephemeral IP (changes on restart)"
echo ""
echo "You will NOT be charged as long as you stay within these limits."
echo ""
