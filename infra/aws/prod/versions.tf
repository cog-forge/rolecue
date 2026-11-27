terraform {
  # 1.11 is the first release with write-only attributes (password_wo); ephemeral resources arrived in 1.10.
  required_version = ">= 1.11"

  backend "s3" {
    bucket       = "rolecue-prod-tfstate-399643456405"
    key          = "prod/terraform.tfstate"
    region       = "ap-southeast-1"
    encrypt      = true
    use_lockfile = true
  }

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.0"
    }
    cloudflare = {
      source  = "cloudflare/cloudflare"
      version = "~> 5.0"
    }
  }
}
