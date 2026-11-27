terraform {
  required_version = ">= 1.11"

  # This stack was first applied with local state (it creates the bucket the prod stack uses as its backend), then
  # migrated into that same bucket under its own key. If the bucket is ever lost: remove this backend block,
  # re-import the resources, and run `terraform init -migrate-state` to go back to a bucket-backed state.
  backend "s3" {
    bucket       = "rolecue-prod-tfstate-399643456405"
    key          = "bootstrap/terraform.tfstate"
    region       = "ap-southeast-1"
    encrypt      = true
    use_lockfile = true
  }

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.0"
    }
  }
}

provider "aws" {
  region              = var.region
  allowed_account_ids = [var.account_id]

  default_tags {
    tags = {
      Project   = "rolecue"
      Env       = "prod"
      ManagedBy = "terraform"
      Stack     = "bootstrap"
    }
  }
}
