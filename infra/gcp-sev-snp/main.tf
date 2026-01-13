terraform {
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

module "gcp_sev_snp" {
  source = "git::https://github.com/tahardi/bearclave-tf.git//modules/gcp-sev-snp?ref=b4651353d566acfd71ae5f05ecea394f0da8e6a8"

  project_id            = "bearclave"
  service_account_email = var.service_account_email
  ssh_public_key        = var.ssh_public_key
  instance_name         = "bearclave-sev-snp"
  container_image       = "us-east1-docker.pkg.dev/bearclave/bearclave/hello-https-enclave-sev@sha256:de8845f0139f49acb3927c598cd6ecd4c4f374f6b595467021964995d6e8b9a8"
}
