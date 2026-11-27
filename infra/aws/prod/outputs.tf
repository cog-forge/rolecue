output "vpc_id" {
  value = aws_vpc.main.id
}

output "vpc_cidr" {
  value = aws_vpc.main.cidr_block
}

output "public_subnet_ids" {
  value = aws_subnet.public[*].id
}

output "private_app_subnet_ids" {
  value = aws_subnet.private_app[*].id
}

output "private_db_subnet_ids" {
  value = aws_subnet.private_db[*].id
}

output "sg_ids" {
  value = {
    alb      = aws_security_group.alb.id
    frontend = aws_security_group.frontend.id
    auth     = aws_security_group.auth.id
    api      = aws_security_group.api.id
    migrate  = aws_security_group.migrate.id
    rds      = aws_security_group.rds.id
  }
}

output "rds_endpoint" {
  description = "Hostname only, without the port; scripts/prod/sync-secrets.sh builds the database URLs from it."
  value       = aws_db_instance.main.address
}

output "rds_instance_id" {
  value = aws_db_instance.main.identifier
}

output "media_bucket" {
  value = aws_s3_bucket.media.bucket
}

output "ecr_repository_urls" {
  value = { for k, r in aws_ecr_repository.app : k => r.repository_url }
}

output "nat_instance_id" {
  value = aws_instance.nat.id
}
