# One log group per container, created here so the execution role needs no logs:CreateLogGroup. The awslogs driver
# and the Service Connect proxy both write to it (the proxy under the "service-connect" stream prefix).

locals {
  ecs_services = ["frontend", "auth", "api"]
  ecs_tasks    = concat(local.ecs_services, ["migrate"])
}

resource "aws_cloudwatch_log_group" "app" {
  for_each = toset(local.ecs_tasks)

  name              = "${local.ssm_prefix}/${each.key}"
  retention_in_days = 14

  tags = { Name = "${local.ssm_prefix}/${each.key}" }
}
