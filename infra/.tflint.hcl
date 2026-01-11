# .tflint.hcl
plugin "terraform" {
  enabled = true
  preset = "recommended"
}

# AWS Plugin
plugin "aws" {
  enabled = true
  version = "0.45.0"
  source  = "github.com/terraform-linters/tflint-ruleset-aws"
}

# GCP Plugin
plugin "google" {
  enabled = true
  version = "0.38.0"
  source  = "github.com/terraform-linters/tflint-ruleset-google"
}
