variable "project_id" {
  description = "Google Cloud project ID"
  type        = string
}

variable "region" {
  description = "Google Cloud region (must be free tier eligible: us-west1, us-central1, or us-east1)"
  type        = string
  default     = "us-central1"
}

variable "zone" {
  description = "Google Cloud zone (must be in free tier region)"
  type        = string
  default     = "us-central1-a"
}

variable "ssh_user" {
  description = "SSH username"
  type        = string
  default     = "deploy"
}

variable "ssh_public_key_path" {
  description = "Path to SSH public key"
  type        = string
  default     = "~/.ssh/snailmail_deploy.pub"
}
