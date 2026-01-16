terraform {
  required_version = ">= 1.14.0, < 2.0.0"
  required_providers {
    aws = {
      source = "hashicorp/aws"
      # Allow minor/patch updates (e.g., 6.29) but prevent major (e.g., v7)
      version = "~> 6.28"
    }
  }
}

provider "aws" {
  region = "us-east-2"
}

module "aws_nitro_enclave" {
  source = "git::https://github.com/tahardi/bearclave-tf.git//modules/aws-nitro-enclaves?ref=v0.1.0"

  instance_name = "bearclave-nitro"
  key_pair_name = "ec2-key--tahardi-bearclave"

  tags = {
    Environment = "development"
    Project     = "bearclave"
  }
}

output "instance_id" {
  value       = module.aws_nitro_enclave.instance_id
  description = "The EC2 instance ID"
}
