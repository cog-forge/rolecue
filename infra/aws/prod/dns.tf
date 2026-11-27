# Public app names: proxied CNAMEs to the ALB. Published only while the stack is awake AND go_live is true, so a
# sleeping or not-yet-validated stack never answers on the public names. (The ACM validation records live in acm.tf.)

resource "cloudflare_dns_record" "app" {
  for_each = var.awake && var.go_live ? {
    rolecue     = local.app_host
    rolecue-api = local.api_host
  } : {}

  zone_id = data.cloudflare_zone.main.zone_id
  name    = each.value
  type    = "CNAME"
  content = aws_lb.main[0].dns_name
  ttl     = 1 # "automatic"; the only value Cloudflare accepts for proxied records
  proxied = true
  comment = "RoleCue prod ALB (terraform, rolecue-prod)"
}
