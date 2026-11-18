# Google Cloud Setup Guide

Before you can deploy to the free tier, you need to set up Google Cloud.

## Step 1: Install gcloud CLI

### On Linux (Fedora/RHEL/CentOS)
```bash
# Add the Cloud SDK repository
sudo tee -a /etc/yum.repos.d/google-cloud-sdk.repo << EOF
[google-cloud-cli]
name=Google Cloud CLI
baseurl=https://packages.cloud.google.com/yum/repos/cloud-sdk-el9-x86_64
enabled=1
gpgcheck=1
repo_gpgcheck=0
gpgkey=https://packages.cloud.google.com/yum/doc/rpm-package-key.gpg
EOF

# Install the SDK
sudo yum install google-cloud-cli

# Verify installation
gcloud --version
```

### On macOS
```bash
brew install --cask google-cloud-sdk
```

### On Windows
Download from: https://cloud.google.com/sdk/docs/install

## Step 2: Authenticate with Google Cloud

```bash
# Login to your Google account
gcloud auth login

# This will open a browser window. Sign in with your Google account.
# If you don't have one, create one at https://accounts.google.com/
```

## Step 3: Create a Project

```bash
# Create a new project for Snailmail
gcloud projects create snailmail-free-tier --name="Snailmail Free Tier"

# Set it as the active project
gcloud config set project snailmail-free-tier

# Verify the project is set
gcloud config get-value project
```

## Step 4: Enable Required APIs

```bash
# Enable Compute Engine API (for VMs)
gcloud services enable compute.googleapis.com

# Enable Cloud Resource Manager API
gcloud services enable cloudresourcemanager.googleapis.com
```

## Step 5: Set Up Billing

**Important**: Even though we're using the free tier, Google requires a billing account.

1. Go to: https://console.cloud.google.com/billing
2. Link a billing account to your project
3. You won't be charged for free tier resources

## Step 6: Verify Setup

```bash
# Check your project
gcloud config get-value project

# List available regions (should include us-central1)
gcloud compute regions list | grep us-central1

# Check if you have quota for e2-micro
gcloud compute machine-types list --filter="name=e2-micro" --zones=us-central1-a
```

## Step 7: Update Terraform Configuration

Once you have your project ID, update the terraform.tfvars file:

```bash
cd infrastructure/terraform

# Get your project ID
PROJECT_ID=$(gcloud config get-value project)

# Update the configuration
cat > terraform.tfvars << EOF
project_id          = "$PROJECT_ID"
region              = "us-central1"
zone                = "us-central1-a"
ssh_user            = "deploy"
ssh_public_key_path = "/home/binarygeek119/.ssh/snailmail_deploy.pub"
EOF

echo "✅ Updated terraform.tfvars with project: $PROJECT_ID"
```

## Step 8: Deploy Infrastructure

```bash
# Initialize Terraform (if not already done)
terraform init

# Preview what will be created
terraform plan

# Create the infrastructure
terraform apply

# Type 'yes' when prompted
```

## Step 9: Get Your VM IP

```bash
# Get the public IP address
gcloud compute instances describe snailmail-server \
  --zone=us-central1-a \
  --format='get(networkInterfaces[0].accessConfigs[0].natIP)'
```

## Troubleshooting

### Error: "Project not found"
```bash
# List your projects
gcloud projects list

# Set the correct project
gcloud config set project YOUR_PROJECT_ID
```

### Error: "API not enabled"
```bash
# Enable the required API
gcloud services enable compute.googleapis.com
```

### Error: "Quota exceeded"
- Make sure you're using us-central1, us-west1, or us-east1 (free tier regions)
- Check if you've exceeded the free tier limits
- Wait a few minutes and try again

### Error: "Permission denied"
```bash
# Re-authenticate
gcloud auth login

# Set the project again
gcloud config set project YOUR_PROJECT_ID
```

## Next Steps

Once your infrastructure is deployed:

1. **Configure DNS** - See DNS_CONFIG.md
2. **Set up SSL** - SSH into the VM and run certbot
3. **Configure SMTP** - Set up SendGrid or another relay
4. **Deploy the app** - Push to GitHub for auto-deployment

## Free Tier Limits

Remember, the free tier includes:
- ✅ e2-micro VM (2 vCPUs, 1GB RAM)
- ✅ 30GB standard persistent disk
- ✅ 1GB network egress per month
- ✅ Ephemeral IP (changes on restart)

You won't be charged as long as you stay within these limits.
