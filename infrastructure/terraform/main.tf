terraform {
  required_version = ">= 1.0"
  
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 5.0"
    }
  }
  
  # Use local backend instead of GCS to avoid authentication issues
  # GCS backend requires credentials, local is simpler for free tier setup
  backend "local" {
    path = "terraform.tfstate"
  }
}

provider "google" {
  project = var.project_id
  region  = var.region
}

# VPC Network (Free tier includes VPC)
resource "google_compute_network" "vpc" {
  name                    = "snailmail-vpc"
  auto_create_subnetworks = false
}

# Subnet
resource "google_compute_subnetwork" "subnet" {
  name          = "snailmail-subnet"
  ip_cidr_range = "10.0.1.0/24"
  region        = var.region
  network       = google_compute_network.vpc.id
  
  private_ip_google_access = true
}

# Firewall Rules (Free tier includes firewall)
resource "google_compute_firewall" "allow_http" {
  name    = "snailmail-allow-http"
  network = google_compute_network.vpc.name
  
  allow {
    protocol = "tcp"
    ports    = ["80", "443", "1143", "25"]
  }
  
  source_ranges = ["0.0.0.0/0"]
  target_tags   = ["snailmail-server"]
}

resource "google_compute_firewall" "allow_ssh" {
  name    = "snailmail-allow-ssh"
  network = google_compute_network.vpc.name
  
  allow {
    protocol = "tcp"
    ports    = ["22"]
  }
  
  source_ranges = ["0.0.0.0/0"]
  target_tags   = ["snailmail-server"]
}

# Service Account
resource "google_service_account" "snailmail" {
  account_id   = "snailmail-service"
  display_name = "Snailmail Service Account"
}

# IAM Binding for service account
resource "google_project_iam_member" "snailmail_logs" {
  project = var.project_id
  role    = "roles/logging.logWriter"
  member  = "serviceAccount:${google_service_account.snailmail.email}"
}

# Compute Engine Instance - FREE TIER (e2-micro)
resource "google_compute_instance" "snailmail" {
  name         = "snailmail-server"
  machine_type = "e2-micro"  # Free tier machine type
  zone         = var.zone
  
  boot_disk {
    initialize_params {
      image = "ubuntu-os-cloud/ubuntu-2204-lts"
      size  = 30  # Free tier includes 30GB standard disk
      type  = "pd-standard"  # Standard disk (free tier)
    }
  }
  
  network_interface {
    network    = google_compute_network.vpc.id
    subnetwork = google_compute_subnetwork.subnet.id
    
    # Ephemeral IP (free) instead of static IP ($3/month)
    access_config {
      # No nat_ip = ephemeral IP
    }
  }
  
  metadata = {
    ssh-keys = "${var.ssh_user}:${file(var.ssh_public_key_path)}"
  }
  
  metadata_startup_script = file("${path.module}/../scripts/startup.sh")
  
  service_account {
    email  = google_service_account.snailmail.email
    scopes = ["cloud-platform"]
  }
  
  tags = ["snailmail-server"]
  
  labels = {
    environment = "production"
    project     = "snailmail"
    tier        = "free"
  }
  
  # Allow stopping for machine type changes
  allow_stopping_for_update = true
}

# Outputs
output "instance_ip" {
  value       = google_compute_instance.snailmail.network_interface[0].access_config[0].nat_ip
  description = "Public IP address of the Snailmail server (ephemeral)"
}

output "instance_name" {
  value       = google_compute_instance.snailmail.name
  description = "Name of the VM instance"
}

output "ssh_command" {
  value       = "gcloud compute ssh ${var.ssh_user}@${google_compute_instance.snailmail.name} --zone=${var.zone}"
  description = "Command to SSH into the instance"
}

output "free_tier_notes" {
  value = <<-EOT
    FREE TIER CONFIGURATION:
    - Machine: e2-micro (2 vCPUs, 1 GB RAM)
    - Disk: 30 GB standard persistent disk
    - Network: Ephemeral IP (changes on restart)
    - Region: ${var.region} (must be us-west1, us-central1, or us-east1)
    
    FREE TIER LIMITATIONS:
    - IP address changes if VM is stopped/restarted
    - Limited to 1 GB RAM (optimize PostgreSQL and app)
    - No SMTP port 25 outbound (use SMTP relay like SendGrid)
    - 1 GB network egress per month (outside North America)
    
    To get static IP later, add ~$3/month
  EOT
  description = "Important notes about free tier limitations"
}
