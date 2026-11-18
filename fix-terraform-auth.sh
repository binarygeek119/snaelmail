#!/bin/bash
# Fix Terraform Authentication Issue
# This script switches from GCS backend to local backend

set -e

echo "🔧 Fixing Terraform authentication issue..."
echo ""

cd infrastructure/terraform

# Clean up old GCS backend files
echo "🧹 Cleaning up old configuration..."
rm -rf .terraform
rm -f .terraform.lock.hcl
rm -f terraform.tfstate.backup

# Keep the main state file if it exists
if [ -f "terraform.tfstate" ]; then
    echo "✓ Found existing state file"
    mv terraform.tfstate terraform.tfstate.backup.$(date +%Y%m%d_%H%M%S) 2>/dev/null || true
fi

echo "✓ Cleaned up old files"
echo ""

# Initialize with local backend
echo "🔄 Initializing Terraform with local backend..."
terraform init

echo ""
echo "✅ Terraform initialized successfully!"
echo ""
echo "Next steps:"
echo "  1. Review your terraform.tfvars file"
echo "  2. Run: terraform plan"
echo "  3. Run: terraform apply"
echo ""
echo "💡 The local backend stores state in: infrastructure/terraform/terraform.tfstate"
echo "   Make sure to back up this file!"
echo ""
