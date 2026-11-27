provider "aws" {
  region              = var.region
  allowed_account_ids = [var.account_id]

  default_tags {
    tags = {
      Project   = "rolecue"
      Env       = "prod"
      ManagedBy = "terraform"
    }
  }
}

# Reads CLOUDFLARE_API_TOKEN from the environment. Run every terraform command through
#   sops exec-env infra/aws/prod/ops.sops.env 'terraform -chdir=infra/aws/prod ...'
# so the token is never written to disk or printed. Scope: zone dorriss.com only (Zone Read, DNS Edit,
# Zone Settings Edit).
provider "cloudflare" {}
