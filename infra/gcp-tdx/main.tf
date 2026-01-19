terraform {
  required_version = ">= 1.14.0, < 2.0.0"
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 7.15"
    }
  }
}

provider "google" {
  project = "bearclave"
  zone    = "us-central1-a"
}

variable "service_account_email" {
  description = "GCP service account email"
  type        = string
  sensitive   = true
}

variable "ssh_public_key" {
  description = "SSH public key for accessing instance"
  type        = string
  sensitive   = true
}

module "gcp_tdx" {
  source = "git::https://github.com/tahardi/bearclave-tf.git//modules/gcp-tdx?ref=v0.1.1"

  project_id            = "bearclave"
  service_account_email = var.service_account_email
  ssh_public_key        = var.ssh_public_key
  instance_name         = "bearclave-tdx"
  container_image       = "us-east1-docker.pkg.dev/bearclave/bearclave/hello-https-enclave-tdx@sha256:e9ad9cafc59b6c0291f0b79b78da9132f3fb325d6de0459ce6a7b343abb51bc0"
}
