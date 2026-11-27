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

output "alb_dns_name" {
  description = "null while asleep."
  value       = var.awake ? aws_lb.main[0].dns_name : null
}

output "target_group_arns" {
  value = { for k, tg in aws_lb_target_group.app : k => tg.arn }
}

output "certificate_arn" {
  value = aws_acm_certificate_validation.main.certificate_arn
}

output "cluster_name" {
  value = aws_ecs_cluster.main.name
}

output "service_names" {
  value = merge(
    { auth = aws_ecs_service.auth.name },
    { for k, s in aws_ecs_service.client : k => s.name }
  )
}

output "task_definition_families" {
  description = "Family of every task, migrate included; the deploy workflow registers revisions under these names."
  value       = { for k, t in aws_ecs_task_definition.app : k => t.family }
}

output "migrate_run_task_network" {
  description = "Network settings for `aws ecs run-task` of the migrate task."
  value = {
    subnets         = aws_subnet.private_app[*].id
    security_groups = [aws_security_group.migrate.id]
  }
}

output "execution_role_arn" {
  value = aws_iam_role.execution.arn
}

output "task_role_arns" {
  value = { for k, r in aws_iam_role.task : k => r.arn }
}
