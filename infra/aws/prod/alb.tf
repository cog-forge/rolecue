# --- Target groups: always present (free; ECS in Task 7 references them) ---------------------------------------------
# ip targets for Fargate/awsvpc. HTTP/1.1 to the targets (the default), which keeps WebSocket upgrades working for the
# interview socket (GET /interviews/:id/live): an ALB HTTP target group passes Upgrade requests through, and nothing
# here (no gRPC / HTTP2 protocol version, no stickiness) changes that.

locals {
  target_groups = {
    frontend = { port = 3000, health_path = "/", matcher = "200-399" }
    auth     = { port = 3001, health_path = "/readyz", matcher = "200" }
    api      = { port = 8080, health_path = "/health", matcher = "200" }
  }
}

resource "aws_lb_target_group" "app" {
  for_each = local.target_groups

  name                 = "${local.name}-${each.key}"
  vpc_id               = aws_vpc.main.id
  target_type          = "ip"
  protocol             = "HTTP"
  port                 = each.value.port
  deregistration_delay = 30

  health_check {
    path                = each.value.health_path
    matcher             = each.value.matcher
    interval            = 15
    timeout             = 5
    healthy_threshold   = 2
    unhealthy_threshold = 3
  }

  stickiness {
    type    = "lb_cookie"
    enabled = false
  }

  tags = { Name = "${local.name}-${each.key}" }
}

# --- Load balancer: only while awake ---------------------------------------------------------------------------------

resource "aws_lb" "main" {
  count = var.awake ? 1 : 0

  name               = local.name
  load_balancer_type = "application"
  internal           = false
  ip_address_type    = "ipv4"
  subnets            = aws_subnet.public[*].id
  security_groups    = [aws_security_group.alb.id]

  # The interview WebSocket pings every 25 s; 300 s leaves slack for a stalled client.
  idle_timeout               = 300
  drop_invalid_header_fields = true
  # Sleeping (awake = false) removes the ALB, so deletion protection must stay off.
  enable_deletion_protection = false

  tags = { Name = local.name }
}

# HTTPS only. There is no port-80 listener: Cloudflare connects to the origin on 443 and redirects visitors itself.
resource "aws_lb_listener" "https" {
  count = var.awake ? 1 : 0

  load_balancer_arn = aws_lb.main[0].arn
  port              = 443
  protocol          = "HTTPS"
  ssl_policy        = "ELBSecurityPolicy-TLS13-1-2-2021-06"
  certificate_arn   = aws_acm_certificate_validation.main.certificate_arn

  default_action {
    type = "fixed-response"

    fixed_response {
      content_type = "text/plain"
      message_body = "Not found"
      status_code  = "404"
    }
  }
}

resource "aws_lb_listener_rule" "api" {
  count = var.awake ? 1 : 0

  listener_arn = aws_lb_listener.https[0].arn
  priority     = 10

  action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.app["api"].arn
  }

  condition {
    host_header {
      values = [local.api_host]
    }
  }
}

# Must stay ahead of the frontend rule: both match the app host, and the lowest priority number wins.
resource "aws_lb_listener_rule" "auth" {
  count = var.awake ? 1 : 0

  listener_arn = aws_lb_listener.https[0].arn
  priority     = 20

  action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.app["auth"].arn
  }

  condition {
    host_header {
      values = [local.app_host]
    }
  }

  condition {
    path_pattern {
      values = ["/api/auth/*"]
    }
  }
}

resource "aws_lb_listener_rule" "frontend" {
  count = var.awake ? 1 : 0

  listener_arn = aws_lb_listener.https[0].arn
  priority     = 30

  action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.app["frontend"].arn
  }

  condition {
    host_header {
      values = [local.app_host]
    }
  }
}

# --- ALB security group ingress: 443 from Cloudflare's IPv4 ranges only ----------------------------------------------
# IPv4 only on purpose: the VPC and the ALB are IPv4-only (ip_address_type = "ipv4"), and Cloudflare reaches
# origins over IPv4, so the IPv6 ranges would never match a packet. No 0.0.0.0/0. The list comes from Cloudflare's
# published ranges; when Cloudflare changes them, a later plan adds or removes rules here.

resource "aws_vpc_security_group_ingress_rule" "alb_from_cloudflare" {
  for_each = toset(data.cloudflare_ip_ranges.main.ipv4_cidrs)

  security_group_id = aws_security_group.alb.id
  cidr_ipv4         = each.value
  ip_protocol       = "tcp"
  from_port         = 443
  to_port           = 443
  description       = "HTTPS from Cloudflare"
}
