#!/bin/bash
set -e

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${BLUE}╔════════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║           Enable Google Cloud Billing (Required)              ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════════════════╝${NC}"
echo ""
echo -e "${YELLOW}IMPORTANT: Billing is required even for FREE TIER usage${NC}"
echo -e "${GREEN}But you will NOT be charged for free tier resources!${NC}"
echo ""
echo "Free tier includes:"
echo "  ✓ e2-micro VM (2 vCPUs, 1GB RAM)"
echo "  ✓ 30GB standard disk"
echo "  ✓ 1GB network egress/month"
echo "  ✓ Ephemeral IP address"
echo ""
echo "Estimated cost: \$0/month"
echo ""

PROJECT_ID=$(gcloud config get-value project 2>/dev/null)

if [ -z "$PROJECT_ID" ]; then
    echo -e "${RED}✗ No project set${NC}"
    echo "Run: gcloud config set project snailmail-free-tier"
    exit 1
fi

echo -e "${GREEN}✓ Project: ${PROJECT_ID}${NC}"
echo ""

# Open billing console
echo -e "${BLUE}Opening billing setup page...${NC}"
echo ""
echo "Please:"
echo "1. Click 'Link a billing account' or 'Create account'"
echo "2. Add your payment method (credit/debit card)"
echo "3. Select the account for this project"
echo "4. Close the browser tab when done"
echo ""

# Try to open browser
if command -v xdg-open &> /dev/null; then
    xdg-open "https://console.cloud.google.com/billing?project=${PROJECT_ID}" 2>/dev/null || true
elif command -v open &> /dev/null; then
    open "https://console.cloud.google.com/billing?project=${PROJECT_ID}" 2>/dev/null || true
fi

echo "If browser didn't open, visit:"
echo "https://console.cloud.google.com/billing?project=${PROJECT_ID}"
echo ""

# Wait for user to confirm
read -p "Have you enabled billing? (y/n): " BILLING_DONE

if [ "$BILLING_DONE" != "y" ]; then
    echo ""
    echo "Please enable billing before continuing."
    echo "You can run this script again when ready."
    exit 0
fi

echo ""
echo -e "${BLUE}Verifying billing is enabled...${NC}"

# Check if billing is enabled
BILLING_INFO=$(gcloud beta billing projects describe $PROJECT_ID --format="value(billingEnabled)" 2>/dev/null || echo "false")

if [ "$BILLING_INFO" != "True" ]; then
    echo -e "${YELLOW}⚠️  Billing not yet enabled${NC}"
    echo ""
    echo "Please wait a few minutes for billing to activate,"
    echo "or check: https://console.cloud.google.com/billing?project=${PROJECT_ID}"
    echo ""
    read -p "Try again in a few minutes? (y/n): " RETRY
    if [ "$RETRY" = "y" ]; then
        sleep 10
        BILLING_INFO=$(gcloud beta billing projects describe $PROJECT_ID --format="value(billingEnabled)" 2>/dev/null || echo "false")
    fi
    
    if [ "$BILLING_INFO" != "True" ]; then
        echo -e "${RED}✗ Billing still not enabled${NC}"
        echo "Please check the billing console and try again."
        exit 1
    fi
fi

echo -e "${GREEN}✓ Billing is enabled!${NC}"
echo ""

# Enable APIs
echo -e "${BLUE}Enabling required APIs...${NC}"
echo ""

echo "Enabling Compute Engine API..."
gcloud services enable compute.googleapis.com --quiet
echo -e "${GREEN}✓ Compute Engine API enabled${NC}"

echo ""
echo "Enabling Cloud Resource Manager API..."
gcloud services enable cloudresourcemanager.googleapis.com --quiet
echo -e "${GREEN}✓ Cloud Resource Manager API enabled${NC}"

echo ""
echo -e "${BLUE}╔════════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║                  Setup Complete!                              ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════════════════╝${NC}"
echo ""
echo -e "${GREEN}✓ Billing enabled${NC}"
echo -e "${GREEN}✓ APIs enabled${NC}"
echo -e "${GREEN}✓ Ready to deploy!${NC}"
echo ""
echo "Next steps:"
echo ""
echo "1. Deploy infrastructure:"
echo "   cd infrastructure/terraform"
echo "   terraform plan"
echo "   terraform apply"
echo ""
echo "2. Get VM IP address:"
echo "   gcloud compute instances describe snailmail-server \\"
echo "     --zone=us-central1-a \\"
echo "     --format='get(networkInterfaces[0].accessConfigs[0].natIP)'"
echo ""
echo "3. Configure DNS and deploy application"
echo ""
echo -e "${YELLOW}Remember: You're on the FREE TIER - no charges!${NC}"
echo ""
