data "aws_availability_zones" "available" {
  state = "available"
}

locals {
  name       = "rolecue-prod"
  azs        = slice(sort(data.aws_availability_zones.available.names), 0, 2)
  vpc_cidr   = "10.40.0.0/16"
  account_id = var.account_id

  # /24 per subnet: public 10.40.0-1, private-app 10.40.10-11, private-db 10.40.20-21.
  public_subnet_cidrs      = [for i in range(2) : cidrsubnet(local.vpc_cidr, 8, i)]
  private_app_subnet_cidrs = [for i in range(2) : cidrsubnet(local.vpc_cidr, 8, 10 + i)]
  private_db_subnet_cidrs  = [for i in range(2) : cidrsubnet(local.vpc_cidr, 8, 20 + i)]

  ssm_prefix   = "/rolecue/prod"
  media_bucket = "rolecue-prod-media-${local.account_id}"
  ecr_repos    = ["frontend", "auth", "api", "migrate"]

  app_url = "https://rolecue.dorriss.com"
}
