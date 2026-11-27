# Certificate for both public names, validated through DNS records created in Cloudflare. It exists regardless of
# `awake`: it is free, and issuing it again after every wake-up would need a new validation round.

resource "aws_acm_certificate" "main" {
  domain_name               = local.app_host
  subject_alternative_names = [local.api_host]
  validation_method         = "DNS"

  tags = { Name = "${local.name}-cert" }

  lifecycle {
    create_before_destroy = true
  }
}

# Validation records must not be proxied, or ACM cannot read them. ACM returns the name and value with a trailing
# dot; Cloudflare stores names without it.
resource "cloudflare_dns_record" "acm_validation" {
  for_each = {
    for o in aws_acm_certificate.main.domain_validation_options : o.domain_name => {
      name    = trimsuffix(o.resource_record_name, ".")
      content = trimsuffix(o.resource_record_value, ".")
      type    = o.resource_record_type
    }
  }

  zone_id = data.cloudflare_zone.main.zone_id
  name    = each.value.name
  type    = each.value.type
  content = each.value.content
  ttl     = 60
  proxied = false
  comment = "ACM DNS validation for ${each.key} (terraform, rolecue-prod)"
}

# Waits until ACM reports ISSUED.
resource "aws_acm_certificate_validation" "main" {
  certificate_arn         = aws_acm_certificate.main.arn
  validation_record_fqdns = [for r in cloudflare_dns_record.acm_validation : r.name]
}
