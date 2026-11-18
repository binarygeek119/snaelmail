#!/bin/bash
set -e

GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}Verifying billing and enabling APIs...${NC}"
echo ""

PROJECT_ID=$(gcloud config get-value project)

# Check billing status
echo "Checking billing status..."
BILLING_ENABLED=$(gcloud beta billing projects describe $PROJECT_ID --format="value(billingEnabled)" 2>/dev/null || echo "false")

if [ "$BILLING_ENABLED" != "True" ]; then
    echo -e "${BLUE}⚠️  Billing not yet enabled${NC}"
    echo ""
    echo "Please enable billing at:"
    echo "https://console.cloud.google.com/billing?project=${PROJECT_ID}"
    echo ""
    echo "After enabling, run this script again."
    exit 1
fi

echo -e "${GREEN}✓ Billing is enabled!${NC}"
echo ""

# Enable APIs
echo -e "${BLUE}Enabling Compute Engine API...${NC}"
gcloud services enable compute.googleapis.com --quiet
echo -e "${GREEN}✓ Compute Engine API enabled${NC}"

echo ""
echo -e "${BLUE}Enabling Cloud Resource Manager API...${NC}"
gcloud services enable cloudresourcemanager.googleapis.com --quiet
echo -e "${GREEN}✓ Cloud Resource Manager API enabled${NC}"

echo ""
echo -e "${GREEN}╔════════════════════════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║              Ready to Deploy Infrastructure!                   ║${NC}"
echo -e "${GREEN}╚════════════════════════════════════════════════════════════════╝${NC}"
echo ""
echo "Next step: Deploy your free tier VM"
echo ""
echo "Run:"
echo "  cd infrastructure/terraform"
echo "  terraform plan"
echo "  terraform apply"
echo ""
