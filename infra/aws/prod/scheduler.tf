# RDS starts itself again after 7 days stopped. While the stack sleeps, a daily schedule stops it again, so a
# forgotten sleeping stack does not bill a running database. The schedule exists only when awake = false.
# The role is always present (it costs nothing and keeps the sleep/wake applies small).

data "aws_iam_policy_document" "scheduler_trust" {
  statement {
    effect  = "Allow"
    actions = ["sts:AssumeRole"]

    principals {
      type        = "Service"
      identifiers = ["scheduler.amazonaws.com"]
    }

    condition {
      test     = "StringEquals"
      variable = "aws:SourceAccount"
      values   = [local.account_id]
    }
  }
}

data "aws_iam_policy_document" "scheduler_stop_rds" {
  statement {
    sid       = "StopThisDatabase"
    effect    = "Allow"
    actions   = ["rds:StopDBInstance"]
    resources = [aws_db_instance.main.arn]
  }
}

resource "aws_iam_role" "scheduler_stop_rds" {
  name               = "${local.name}-scheduler-stop-rds"
  description        = "EventBridge Scheduler: stop the ${local.name} RDS instance while the stack sleeps"
  assume_role_policy = data.aws_iam_policy_document.scheduler_trust.json
}

resource "aws_iam_role_policy" "scheduler_stop_rds" {
  name   = "stop-rds"
  role   = aws_iam_role.scheduler_stop_rds.id
  policy = data.aws_iam_policy_document.scheduler_stop_rds.json
}

resource "aws_scheduler_schedule" "stop_rds" {
  count = var.awake ? 0 : 1

  name        = "${local.name}-stop-rds"
  description = "Daily stop of ${local.name} RDS while the stack sleeps (RDS auto-starts after 7 days stopped)"
  group_name  = "default"

  schedule_expression          = "cron(0 2 * * ? *)"
  schedule_expression_timezone = "Asia/Ho_Chi_Minh"

  flexible_time_window {
    mode = "OFF"
  }

  target {
    # Universal target: the Scheduler calls the RDS API itself. On an already stopped instance the call fails with
    # InvalidDBInstanceState, which is the expected steady state, so no retries.
    arn      = "arn:aws:scheduler:::aws-sdk:rds:stopDBInstance"
    role_arn = aws_iam_role.scheduler_stop_rds.arn
    input    = jsonencode({ DbInstanceIdentifier = aws_db_instance.main.identifier })

    retry_policy {
      maximum_retry_attempts = 0
    }
  }
}
