resource "aws_ecs_cluster" "main" {
  name = local.name

  setting {
    name  = "containerInsights"
    value = "enabled" # standard Container Insights ("enhanced" costs more)
  }
}

resource "aws_ecs_cluster_capacity_providers" "main" {
  cluster_name       = aws_ecs_cluster.main.name
  capacity_providers = ["FARGATE"]

  default_capacity_provider_strategy {
    capacity_provider = "FARGATE"
    weight            = 1
  }
}

# Service Connect namespace. HTTP namespaces have no DNS records of their own: the ECS agent's proxy sidecar resolves
# the client alias (auth:3001) and routes to a healthy auth task.
resource "aws_service_discovery_http_namespace" "main" {
  name        = "rolecue.local"
  description = "RoleCue prod Service Connect"
}

# --- Services ----------------------------------------------------------------------------------------------------
# frontend, auth and api. Task definitions are created here, but the deploy workflow registers every later revision, so
# the services ignore task_definition. desired_count is 0 until Task 10 sets run_services (no images yet) and while
# the stack sleeps.

locals {
  desired_count = var.awake && var.run_services ? 1 : 0

  # Seconds after a task starts during which a failing ALB health check does not count. The api image starts
  # Blender-backed code and gets longer.
  health_check_grace = { frontend = 60, auth = 60, api = 180 }

  service_security_groups = {
    frontend = aws_security_group.frontend.id
    auth     = aws_security_group.auth.id
    api      = aws_security_group.api.id
  }

  # Only auth (server) and api (client) join Service Connect; the frontend never calls auth, so it gets no proxy sidecar.
  service_connect_log_options = { for s in ["auth", "api"] : s => {
    "awslogs-group"         = aws_cloudwatch_log_group.app[s].name
    "awslogs-region"        = var.region
    "awslogs-stream-prefix" = "service-connect"
  } }
}

# auth is the Service Connect server (alias auth:3001), reached through the ALB for /api/auth/* and through Service
# Connect by frontend and api.
resource "aws_ecs_service" "auth" {
  name                   = "auth"
  cluster                = aws_ecs_cluster.main.id
  task_definition        = aws_ecs_task_definition.app["auth"].arn
  desired_count          = local.desired_count
  enable_execute_command = true
  propagate_tags         = "SERVICE"

  # Rolling deploys start the new task before stopping the old one; a failed rollout rolls back.
  deployment_minimum_healthy_percent = 100
  deployment_maximum_percent         = 200
  health_check_grace_period_seconds  = local.health_check_grace["auth"]

  deployment_circuit_breaker {
    enable   = true
    rollback = true
  }

  capacity_provider_strategy {
    capacity_provider = "FARGATE"
    weight            = 1
  }

  network_configuration {
    subnets          = aws_subnet.private_app[*].id
    security_groups  = [local.service_security_groups["auth"]]
    assign_public_ip = false
  }

  load_balancer {
    target_group_arn = aws_lb_target_group.app["auth"].arn
    container_name   = "auth"
    container_port   = local.target_groups["auth"].port
  }

  service_connect_configuration {
    enabled   = true
    namespace = aws_service_discovery_http_namespace.main.arn

    service {
      port_name = "auth"
      client_alias {
        dns_name = "auth"
        port     = 3001
      }
    }

    log_configuration {
      log_driver = "awslogs"
      options    = local.service_connect_log_options["auth"]
    }
  }

  # ECS rejects a target group that no load balancer uses, so the listener rule (present only while awake) must exist
  # first. That also orders sleep and wake.
  depends_on = [
    aws_lb_listener_rule.auth,
    aws_ecs_cluster_capacity_providers.main,
  ]

  lifecycle {
    ignore_changes = [task_definition]
  }
}

# api is a Service Connect client of auth and depends on the auth service so the alias resolves from the first
# request. The frontend is in the same for_each but has no Service Connect block (no proxy sidecar).
resource "aws_ecs_service" "client" {
  for_each = toset(["frontend", "api"])

  name                   = each.key
  cluster                = aws_ecs_cluster.main.id
  task_definition        = aws_ecs_task_definition.app[each.key].arn
  desired_count          = local.desired_count
  enable_execute_command = true
  propagate_tags         = "SERVICE"

  deployment_minimum_healthy_percent = 100
  deployment_maximum_percent         = 200
  health_check_grace_period_seconds  = local.health_check_grace[each.key]

  deployment_circuit_breaker {
    enable   = true
    rollback = true
  }

  capacity_provider_strategy {
    capacity_provider = "FARGATE"
    weight            = 1
  }

  network_configuration {
    subnets          = aws_subnet.private_app[*].id
    security_groups  = [local.service_security_groups[each.key]]
    assign_public_ip = false
  }

  load_balancer {
    target_group_arn = aws_lb_target_group.app[each.key].arn
    container_name   = each.key
    container_port   = local.target_groups[each.key].port
  }

  dynamic "service_connect_configuration" {
    for_each = each.key == "api" ? [1] : []
    content {
      enabled   = true
      namespace = aws_service_discovery_http_namespace.main.arn

      log_configuration {
        log_driver = "awslogs"
        options    = local.service_connect_log_options["api"]
      }
    }
  }

  depends_on = [
    aws_ecs_service.auth,
    aws_lb_listener_rule.api,
    aws_lb_listener_rule.frontend,
    aws_ecs_cluster_capacity_providers.main,
  ]

  lifecycle {
    ignore_changes = [task_definition]
  }
}
