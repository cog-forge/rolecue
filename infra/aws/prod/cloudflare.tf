# Cloudflare zone lookup and the IP ranges the ALB security group admits. The provider reads CLOUDFLARE_API_TOKEN
# from the environment (providers.tf).
#
# Zone-wide settings are deliberately NOT managed here (no cloudflare_zone_setting / zone_settings_override for
# ssl, always_use_https, min_tls_version, ...). The zone is already in SSL mode "full", which works with the ACM
# certificate on the ALB (Cloudflare connects to the origin over HTTPS and does not validate the chain), and other
# sites in this zone depend on the current settings. Changing them from this stack could break those sites.

data "cloudflare_zone" "main" {
  filter = {
    name = local.zone_name
  }
}

data "cloudflare_ip_ranges" "main" {}
