# Initial task definitions. The deploy workflow (Task 9) registers every later revision from the latest one, swapping
# the image, so the services ignore task_definition and the old revisions stay registered for rollback (skip_destroy).
#
# readonlyRootFilesystem stays false everywhere: the SSM agent that ECS Exec injects needs a writable root filesystem
# (AWS re:Post, "TargetNotConnectedException"), and the api's Blender writes under its home directory. The compose
# read_only hardening is therefore not carried over; the images already run as non-root users. initProcessEnabled
# reaps ECS Exec's helper processes.
#
# Container health checks exist only where the image ships a tool to run one: frontend (node) and auth (bun), as in
# compose and the auth Dockerfile. api (a wolfi base with no confirmed curl/wget) and migrate (no server) rely on the ALB
# health check, which also drives the deployment circuit breaker.

locals {
  task_sizes = {
    frontend = { cpu = 256, memory = 512, port = 3000 }
    auth     = { cpu = 256, memory = 512, port = 3001 }
    api      = { cpu = 1024, memory = 3072, port = 8080 }
    migrate  = { cpu = 256, memory = 512, port = null }
  }

  container_health_checks = {
    frontend = {
      command = ["CMD", "node", "-e", "fetch('http://127.0.0.1:3000').then(r=>process.exit(r.ok?0:1)).catch(()=>process.exit(1))"]
    }
    auth = {
      command = ["CMD", "bun", "-e", "const r=await fetch('http://127.0.0.1:3001/readyz');process.exit(r.ok?0:1)"]
    }
  }
}

resource "aws_ecs_task_definition" "app" {
  for_each = toset(local.ecs_tasks)

  family                   = "${local.name}-${each.key}"
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  cpu                      = local.task_sizes[each.key].cpu
  memory                   = local.task_sizes[each.key].memory
  execution_role_arn       = aws_iam_role.execution.arn
  task_role_arn            = aws_iam_role.task[each.key].arn
  skip_destroy             = true

  runtime_platform {
    operating_system_family = "LINUX"
    cpu_architecture        = "X86_64" # GitHub runners build amd64, and the api bundles x86 Blender
  }

  container_definitions = jsonencode([
    merge(
      {
        name      = each.key
        image     = "${aws_ecr_repository.app[each.key].repository_url}:${var.image_tag}"
        essential = true

        portMappings = local.task_sizes[each.key].port == null ? [] : [merge(
          {
            name          = each.key # Service Connect refers to auth's mapping by this name
            containerPort = local.task_sizes[each.key].port
            protocol      = "tcp"
          },
          each.key == "auth" ? { appProtocol = "http" } : {}
        )]

        environment = [for k in sort(keys(local.app_config[each.key])) : { name = k, value = local.app_config[each.key][k] }]
        secrets = [for k in sort(keys(local.app_secrets[each.key])) : {
          name      = k
          valueFrom = "${local.ssm_parameter_arn_prefix}/${local.app_secrets[each.key][k]}"
        }]

        readonlyRootFilesystem = false
        linuxParameters        = { initProcessEnabled = true }

        logConfiguration = {
          logDriver = "awslogs"
          options = {
            "awslogs-group"         = aws_cloudwatch_log_group.app[each.key].name
            "awslogs-region"        = var.region
            "awslogs-stream-prefix" = each.key
          }
        }
      },
      contains(keys(local.container_health_checks), each.key) ? {
        healthCheck = {
          command     = local.container_health_checks[each.key].command
          interval    = 15
          timeout     = 5
          retries     = 3
          startPeriod = 60
        }
      } : {}
    )
  ])
}
