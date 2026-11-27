# Alerting and the dashboard. Every alarm sends to one SNS topic that emails the owner. The owner confirms the
# subscription from the email AWS sends after the first apply; until then nothing is delivered.
#
# treat_missing_data = "notBreaching" everywhere: a sleeping stack (no ALB, no tasks, RDS stopped, NAT stopped) reports
# no data, and that must not alarm. The ALB alarms exist only while awake (the ALB itself does not exist otherwise).

resource "aws_sns_topic" "alerts" {
  name = "${local.name}-alerts"

  # No KMS key on purpose: CloudWatch alarms cannot publish to a topic encrypted with the AWS-managed aws/sns key.
}

resource "aws_sns_topic_subscription" "alerts_email" {
  topic_arn = aws_sns_topic.alerts.arn
  protocol  = "email"
  endpoint  = var.alert_email
}

# --- ALB (only while awake) ------------------------------------------------------------------------------------------

resource "aws_cloudwatch_metric_alarm" "alb_elb_5xx" {
  count = var.awake ? 1 : 0

  alarm_name          = "${local.name}-alb-elb-5xx"
  alarm_description   = "The ALB itself returned more than 5 5xx responses in 5 minutes."
  namespace           = "AWS/ApplicationELB"
  metric_name         = "HTTPCode_ELB_5XX_Count"
  dimensions          = { LoadBalancer = aws_lb.main[0].arn_suffix }
  statistic           = "Sum"
  period              = 300
  evaluation_periods  = 1
  threshold           = 5
  comparison_operator = "GreaterThanThreshold"
  treat_missing_data  = "notBreaching"
  alarm_actions       = [aws_sns_topic.alerts.arn]
}

resource "aws_cloudwatch_metric_alarm" "alb_target_5xx" {
  count = var.awake ? 1 : 0

  alarm_name          = "${local.name}-alb-target-5xx"
  alarm_description   = "The targets (frontend, auth, api) returned more than 5 5xx responses in 5 minutes."
  namespace           = "AWS/ApplicationELB"
  metric_name         = "HTTPCode_Target_5XX_Count"
  dimensions          = { LoadBalancer = aws_lb.main[0].arn_suffix }
  statistic           = "Sum"
  period              = 300
  evaluation_periods  = 1
  threshold           = 5
  comparison_operator = "GreaterThanThreshold"
  treat_missing_data  = "notBreaching"
  alarm_actions       = [aws_sns_topic.alerts.arn]
}

resource "aws_cloudwatch_metric_alarm" "unhealthy_hosts" {
  for_each = var.awake ? local.target_groups : {}

  alarm_name        = "${local.name}-${each.key}-unhealthy-hosts"
  alarm_description = "The ${each.key} target group had an unhealthy target for 5 minutes."
  namespace         = "AWS/ApplicationELB"
  metric_name       = "UnHealthyHostCount"
  dimensions = {
    LoadBalancer = aws_lb.main[0].arn_suffix
    TargetGroup  = aws_lb_target_group.app[each.key].arn_suffix
  }
  statistic           = "Maximum"
  period              = 60
  evaluation_periods  = 5
  datapoints_to_alarm = 5
  threshold           = 0
  comparison_operator = "GreaterThanThreshold"
  treat_missing_data  = "notBreaching"
  alarm_actions       = [aws_sns_topic.alerts.arn]
}

# --- ECS (api) -------------------------------------------------------------------------------------------------------

resource "aws_cloudwatch_metric_alarm" "api_cpu" {
  alarm_name        = "${local.name}-api-cpu"
  alarm_description = "api CPU above 85% for 10 minutes."
  namespace         = "AWS/ECS"
  metric_name       = "CPUUtilization"
  dimensions = {
    ClusterName = aws_ecs_cluster.main.name
    ServiceName = aws_ecs_service.client["api"].name
  }
  statistic           = "Average"
  period              = 300
  evaluation_periods  = 2
  threshold           = 85
  comparison_operator = "GreaterThanThreshold"
  treat_missing_data  = "notBreaching"
  alarm_actions       = [aws_sns_topic.alerts.arn]
}

resource "aws_cloudwatch_metric_alarm" "api_memory" {
  alarm_name        = "${local.name}-api-memory"
  alarm_description = "api memory above 85% for 10 minutes."
  namespace         = "AWS/ECS"
  metric_name       = "MemoryUtilization"
  dimensions = {
    ClusterName = aws_ecs_cluster.main.name
    ServiceName = aws_ecs_service.client["api"].name
  }
  statistic           = "Average"
  period              = 300
  evaluation_periods  = 2
  threshold           = 85
  comparison_operator = "GreaterThanThreshold"
  treat_missing_data  = "notBreaching"
  alarm_actions       = [aws_sns_topic.alerts.arn]
}

# --- RDS -------------------------------------------------------------------------------------------------------------

resource "aws_cloudwatch_metric_alarm" "rds_cpu" {
  alarm_name          = "${local.name}-rds-cpu"
  alarm_description   = "RDS CPU above 80% for 15 minutes."
  namespace           = "AWS/RDS"
  metric_name         = "CPUUtilization"
  dimensions          = { DBInstanceIdentifier = aws_db_instance.main.identifier }
  statistic           = "Average"
  period              = 300
  evaluation_periods  = 3
  threshold           = 80
  comparison_operator = "GreaterThanThreshold"
  treat_missing_data  = "notBreaching"
  alarm_actions       = [aws_sns_topic.alerts.arn]
}

resource "aws_cloudwatch_metric_alarm" "rds_free_storage" {
  alarm_name          = "${local.name}-rds-free-storage"
  alarm_description   = "RDS free storage below 2 GB (of 20 GB)."
  namespace           = "AWS/RDS"
  metric_name         = "FreeStorageSpace"
  dimensions          = { DBInstanceIdentifier = aws_db_instance.main.identifier }
  statistic           = "Minimum"
  period              = 300
  evaluation_periods  = 1
  threshold           = 2 * 1024 * 1024 * 1024 # bytes
  comparison_operator = "LessThanThreshold"
  treat_missing_data  = "notBreaching"
  alarm_actions       = [aws_sns_topic.alerts.arn]
}

# --- NAT instance ----------------------------------------------------------------------------------------------------
# A failed system status check (the host, not the OS) triggers EC2 auto-recovery, which restarts the instance on new
# hardware with the same id, private IP and volumes, and the topic hears about it.

resource "aws_cloudwatch_metric_alarm" "nat_status" {
  alarm_name          = "${local.name}-nat-status"
  alarm_description   = "NAT instance failed its system status check; EC2 auto-recovery runs."
  namespace           = "AWS/EC2"
  metric_name         = "StatusCheckFailed_System"
  dimensions          = { InstanceId = aws_instance.nat.id }
  statistic           = "Maximum"
  period              = 60
  evaluation_periods  = 2
  threshold           = 0
  comparison_operator = "GreaterThanThreshold"
  treat_missing_data  = "notBreaching"
  alarm_actions = [
    "arn:aws:automate:${var.region}:ec2:recover",
    aws_sns_topic.alerts.arn,
  ]
}

# --- Dashboard -------------------------------------------------------------------------------------------------------
# Widgets are 8 columns wide (3 per row), 6 rows high. The ALB row is dropped while asleep and the others move up.

locals {
  widget_w = 8
  widget_h = 6
  # Row index of the first non-ALB row.
  dash_row0 = var.awake ? 1 : 0

  dash_alb = flatten([for _ in range(var.awake ? 1 : 0) : [
    {
      type = "metric", x = 0, y = 0, width = local.widget_w, height = local.widget_h
      properties = {
        title  = "ALB requests"
        region = var.region
        view   = "timeSeries"
        stat   = "Sum"
        period = 300
        metrics = [
          ["AWS/ApplicationELB", "RequestCount", "LoadBalancer", aws_lb.main[0].arn_suffix],
        ]
      }
    },
    {
      type = "metric", x = 8, y = 0, width = local.widget_w, height = local.widget_h
      properties = {
        title  = "ALB 5xx"
        region = var.region
        view   = "timeSeries"
        stat   = "Sum"
        period = 300
        metrics = [
          ["AWS/ApplicationELB", "HTTPCode_ELB_5XX_Count", "LoadBalancer", aws_lb.main[0].arn_suffix],
          ["AWS/ApplicationELB", "HTTPCode_Target_5XX_Count", "LoadBalancer", aws_lb.main[0].arn_suffix],
        ]
      }
    },
    {
      type = "metric", x = 16, y = 0, width = local.widget_w, height = local.widget_h
      properties = {
        title  = "ALB target latency"
        region = var.region
        view   = "timeSeries"
        period = 300
        metrics = [
          ["AWS/ApplicationELB", "TargetResponseTime", "LoadBalancer", aws_lb.main[0].arn_suffix, { stat = "Average" }],
          ["AWS/ApplicationELB", "TargetResponseTime", "LoadBalancer", aws_lb.main[0].arn_suffix, { stat = "p95" }],
        ]
      }
    },
  ]])

  dash_rest = [
    {
      type = "metric", x = 0, y = local.dash_row0 * local.widget_h, width = local.widget_w, height = local.widget_h
      properties = {
        title  = "ECS CPU (%) per service"
        region = var.region
        view   = "timeSeries"
        stat   = "Average"
        period = 300
        metrics = [for s in local.ecs_services :
          ["AWS/ECS", "CPUUtilization", "ClusterName", aws_ecs_cluster.main.name, "ServiceName", s]
        ]
      }
    },
    {
      type = "metric", x = 8, y = local.dash_row0 * local.widget_h, width = local.widget_w, height = local.widget_h
      properties = {
        title  = "ECS memory (%) per service"
        region = var.region
        view   = "timeSeries"
        stat   = "Average"
        period = 300
        metrics = [for s in local.ecs_services :
          ["AWS/ECS", "MemoryUtilization", "ClusterName", aws_ecs_cluster.main.name, "ServiceName", s]
        ]
      }
    },
    {
      type = "metric", x = 16, y = local.dash_row0 * local.widget_h, width = local.widget_w, height = local.widget_h
      properties = {
        title  = "NAT instance network (bytes)"
        region = var.region
        view   = "timeSeries"
        stat   = "Sum"
        period = 300
        metrics = [
          ["AWS/EC2", "NetworkIn", "InstanceId", aws_instance.nat.id],
          ["AWS/EC2", "NetworkOut", "InstanceId", aws_instance.nat.id],
        ]
      }
    },
    {
      type = "metric", x = 0, y = (local.dash_row0 + 1) * local.widget_h, width = local.widget_w, height = local.widget_h
      properties = {
        title  = "RDS CPU (%)"
        region = var.region
        view   = "timeSeries"
        stat   = "Average"
        period = 300
        metrics = [
          ["AWS/RDS", "CPUUtilization", "DBInstanceIdentifier", aws_db_instance.main.identifier],
        ]
      }
    },
    {
      type = "metric", x = 8, y = (local.dash_row0 + 1) * local.widget_h, width = local.widget_w, height = local.widget_h
      properties = {
        title  = "RDS connections"
        region = var.region
        view   = "timeSeries"
        stat   = "Maximum"
        period = 300
        metrics = [
          ["AWS/RDS", "DatabaseConnections", "DBInstanceIdentifier", aws_db_instance.main.identifier],
        ]
      }
    },
    {
      type = "metric", x = 16, y = (local.dash_row0 + 1) * local.widget_h, width = local.widget_w, height = local.widget_h
      properties = {
        title  = "RDS free storage (bytes)"
        region = var.region
        view   = "timeSeries"
        stat   = "Minimum"
        period = 300
        metrics = [
          ["AWS/RDS", "FreeStorageSpace", "DBInstanceIdentifier", aws_db_instance.main.identifier],
        ]
      }
    },
  ]
}

resource "aws_cloudwatch_dashboard" "main" {
  dashboard_name = local.name
  dashboard_body = jsonencode({ widgets = concat(local.dash_alb, local.dash_rest) })
}
