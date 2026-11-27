# --- Trust policy shared by every ECS role -----------------------------------------------------------------------
# aws:SourceAccount / aws:SourceArn stop another account's (or another service's) ECS tasks from assuming these roles
# through the confused-deputy path. ECS documents arn:aws:ecs:<region>:<account>:* as the SourceArn form.

data "aws_iam_policy_document" "ecs_tasks_trust" {
  statement {
    effect  = "Allow"
    actions = ["sts:AssumeRole"]

    principals {
      type        = "Service"
      identifiers = ["ecs-tasks.amazonaws.com"]
    }

    condition {
      test     = "StringEquals"
      variable = "aws:SourceAccount"
      values   = [local.account_id]
    }

    condition {
      test     = "ArnLike"
      variable = "aws:SourceArn"
      values   = ["arn:aws:ecs:${var.region}:${local.account_id}:*"]
    }
  }
}

# The execution role keeps a plain trust policy: the ECS docs only show the SourceAccount/SourceArn conditions for task
# roles, and a wrong condition here would stop every task from starting.
data "aws_iam_policy_document" "ecs_tasks_trust_plain" {
  statement {
    effect  = "Allow"
    actions = ["sts:AssumeRole"]

    principals {
      type        = "Service"
      identifiers = ["ecs-tasks.amazonaws.com"]
    }
  }
}

# --- Execution role: pulls images, writes logs, reads the secrets injected at task start ---------------------------

resource "aws_iam_role" "execution" {
  name               = "${local.name}-ecs-execution"
  description        = "ECS agent: ECR pull, CloudWatch Logs and /rolecue/prod SSM parameters"
  assume_role_policy = data.aws_iam_policy_document.ecs_tasks_trust_plain.json
}

resource "aws_iam_role_policy_attachment" "execution_managed" {
  role       = aws_iam_role.execution.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy"
}

# SecureStrings use the AWS-managed alias/aws/ssm key, whose key policy already lets the account's principals decrypt
# through SSM, so no kms:Decrypt statement is needed here.
data "aws_iam_policy_document" "execution_ssm" {
  statement {
    sid       = "ReadAppParameters"
    effect    = "Allow"
    actions   = ["ssm:GetParameters"]
    resources = ["${local.ssm_parameter_arn_prefix}/*"]
  }
}

resource "aws_iam_role_policy" "execution_ssm" {
  name   = "read-app-parameters"
  role   = aws_iam_role.execution.id
  policy = data.aws_iam_policy_document.execution_ssm.json
}

# --- Task roles ---------------------------------------------------------------------------------------------------
# One role per task. Only api gets AWS permissions (the media bucket); every role carries the ssmmessages permissions
# ECS Exec needs, so go-live debugging works with `aws ecs execute-command`.

resource "aws_iam_role" "task" {
  for_each = toset(local.ecs_tasks)

  name               = "${local.name}-${each.key}-task"
  description        = "Task role of the ${each.key} container"
  assume_role_policy = data.aws_iam_policy_document.ecs_tasks_trust.json
}

data "aws_iam_policy_document" "ecs_exec" {
  statement {
    sid    = "EcsExecChannels"
    effect = "Allow"
    actions = [
      "ssmmessages:CreateControlChannel",
      "ssmmessages:CreateDataChannel",
      "ssmmessages:OpenControlChannel",
      "ssmmessages:OpenDataChannel",
    ]
    resources = ["*"]
  }
}

resource "aws_iam_role_policy" "ecs_exec" {
  for_each = aws_iam_role.task

  name   = "ecs-exec"
  role   = each.value.id
  policy = data.aws_iam_policy_document.ecs_exec.json
}

# Presigned URLs are signed by the api's task credentials, so these permissions also bound what browsers can do.
# ListBucket makes a missing key return 404 instead of 403.
data "aws_iam_policy_document" "api_media" {
  statement {
    sid       = "MediaObjects"
    effect    = "Allow"
    actions   = ["s3:GetObject", "s3:PutObject", "s3:DeleteObject"]
    resources = ["${aws_s3_bucket.media.arn}/*"]
  }

  statement {
    sid       = "MediaList"
    effect    = "Allow"
    actions   = ["s3:ListBucket"]
    resources = [aws_s3_bucket.media.arn]
  }
}

resource "aws_iam_role_policy" "api_media" {
  name   = "media-bucket"
  role   = aws_iam_role.task["api"].id
  policy = data.aws_iam_policy_document.api_media.json
}
