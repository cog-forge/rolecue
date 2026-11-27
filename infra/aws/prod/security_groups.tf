# Ingress from the ALB is added in Task 6 (Cloudflare ranges); the ALB group only exists here so the others can
# reference it.

resource "aws_security_group" "alb" {
  name        = "${local.name}-alb"
  description = "Public load balancer"
  vpc_id      = aws_vpc.main.id

  tags = { Name = "${local.name}-alb" }
}

resource "aws_security_group" "frontend" {
  name        = "${local.name}-frontend"
  description = "frontend tasks"
  vpc_id      = aws_vpc.main.id

  tags = { Name = "${local.name}-frontend" }
}

resource "aws_security_group" "auth" {
  name        = "${local.name}-auth"
  description = "auth tasks"
  vpc_id      = aws_vpc.main.id

  tags = { Name = "${local.name}-auth" }
}

resource "aws_security_group" "api" {
  name        = "${local.name}-api"
  description = "api tasks"
  vpc_id      = aws_vpc.main.id

  tags = { Name = "${local.name}-api" }
}

resource "aws_security_group" "migrate" {
  name        = "${local.name}-migrate"
  description = "one-off migrate tasks"
  vpc_id      = aws_vpc.main.id

  tags = { Name = "${local.name}-migrate" }
}

resource "aws_security_group" "rds" {
  name        = "${local.name}-rds"
  description = "PostgreSQL"
  vpc_id      = aws_vpc.main.id

  tags = { Name = "${local.name}-rds" }
}

resource "aws_security_group" "nat" {
  name        = "${local.name}-nat"
  description = "NAT instance"
  vpc_id      = aws_vpc.main.id

  tags = { Name = "${local.name}-nat" }
}

# --- Ingress ----------------------------------------------------------------------------------------------------

resource "aws_vpc_security_group_ingress_rule" "frontend_from_alb" {
  security_group_id            = aws_security_group.frontend.id
  referenced_security_group_id = aws_security_group.alb.id
  ip_protocol                  = "tcp"
  from_port                    = 3000
  to_port                      = 3000
  description                  = "ALB to frontend"
}

resource "aws_vpc_security_group_ingress_rule" "api_from_alb" {
  security_group_id            = aws_security_group.api.id
  referenced_security_group_id = aws_security_group.alb.id
  ip_protocol                  = "tcp"
  from_port                    = 8080
  to_port                      = 8080
  description                  = "ALB to api"
}

resource "aws_vpc_security_group_ingress_rule" "auth_from_alb" {
  security_group_id            = aws_security_group.auth.id
  referenced_security_group_id = aws_security_group.alb.id
  ip_protocol                  = "tcp"
  from_port                    = 3001
  to_port                      = 3001
  description                  = "ALB to auth (/api/auth/*)"
}

resource "aws_vpc_security_group_ingress_rule" "auth_from_frontend" {
  security_group_id            = aws_security_group.auth.id
  referenced_security_group_id = aws_security_group.frontend.id
  ip_protocol                  = "tcp"
  from_port                    = 3001
  to_port                      = 3001
  description                  = "frontend to auth (Service Connect)"
}

resource "aws_vpc_security_group_ingress_rule" "auth_from_api" {
  security_group_id            = aws_security_group.auth.id
  referenced_security_group_id = aws_security_group.api.id
  ip_protocol                  = "tcp"
  from_port                    = 3001
  to_port                      = 3001
  description                  = "api to auth (Service Connect)"
}

resource "aws_vpc_security_group_ingress_rule" "rds_from_api" {
  security_group_id            = aws_security_group.rds.id
  referenced_security_group_id = aws_security_group.api.id
  ip_protocol                  = "tcp"
  from_port                    = 5432
  to_port                      = 5432
  description                  = "api to PostgreSQL"
}

resource "aws_vpc_security_group_ingress_rule" "rds_from_auth" {
  security_group_id            = aws_security_group.rds.id
  referenced_security_group_id = aws_security_group.auth.id
  ip_protocol                  = "tcp"
  from_port                    = 5432
  to_port                      = 5432
  description                  = "auth to PostgreSQL"
}

resource "aws_vpc_security_group_ingress_rule" "rds_from_migrate" {
  security_group_id            = aws_security_group.rds.id
  referenced_security_group_id = aws_security_group.migrate.id
  ip_protocol                  = "tcp"
  from_port                    = 5432
  to_port                      = 5432
  description                  = "migrate to PostgreSQL"
}

resource "aws_vpc_security_group_ingress_rule" "nat_from_private_app" {
  count = 2

  security_group_id = aws_security_group.nat.id
  cidr_ipv4         = local.private_app_subnet_cidrs[count.index]
  ip_protocol       = "-1"
  description       = "private-app subnet ${count.index} egress through NAT"
}

# --- Egress: all, for every group -------------------------------------------------------------------------------

resource "aws_vpc_security_group_egress_rule" "all" {
  for_each = {
    alb      = aws_security_group.alb.id
    frontend = aws_security_group.frontend.id
    auth     = aws_security_group.auth.id
    api      = aws_security_group.api.id
    migrate  = aws_security_group.migrate.id
    rds      = aws_security_group.rds.id
    nat      = aws_security_group.nat.id
  }

  security_group_id = each.value
  cidr_ipv4         = "0.0.0.0/0"
  ip_protocol       = "-1"
  description       = "all egress"
}
