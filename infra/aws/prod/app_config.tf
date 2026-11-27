# Plain (non-secret) container configuration: class C of docs/superpowers/plans/2026-11-27-prod-env-inventory.md,
# one map per service. Values that Terraform already knows come from resources and locals; the rest are public
# identifiers (OAuth client IDs, Resend template IDs, an account ID) copied from the owner's local env files.
# Class S/G secrets never appear here: they are injected from SSM (see the *_secrets maps below).
#
# Every value is a string, and no name appears both here and in the same service's secrets map.

locals {
  ssm_parameter_arn_prefix = "arn:aws:ssm:${var.region}:${local.account_id}:parameter${local.ssm_prefix}"

  cookie_prefix = "rolecue-prod" # must be identical in api and auth; differs from staging's so cookies on dorriss.com do not collide

  # The api must see the real client IP in X-Forwarded-For, not the ALB's or a Cloudflare edge's: trust the VPC (ALB to
  # task) and Cloudflare's IPv4 ranges (the ALB is IPv4-only, so edges reach it over IPv4). Comma separated, no spaces.
  # Sorted so the value only changes when Cloudflare publishes new ranges.
  trusted_proxies = join(",", concat([local.vpc_cidr], sort(tolist(data.cloudflare_ip_ranges.main.ipv4_cidrs))))

  s3_endpoint = "https://s3.${var.region}.amazonaws.com"
}

locals {
  app_config_api = {
    APP_ENVIRONMENT        = "production"
    APP_DEBUG              = "false"
    APP_VERSION            = var.image_tag
    SERVER_HOST            = "0.0.0.0"
    SERVER_PORT            = "8080"
    SERVER_TRUSTED_PROXIES = local.trusted_proxies
    LOGGER_LEVEL           = "info"
    LOGGER_FORMAT          = "json"

    DATABASE_REQUIRE_TLS = "true" # the URL carries sslmode=verify-full and the RDS CA bundle path

    AUTH_UPSTREAM_URL         = "http://auth:3001" # Service Connect alias
    AUTH_BASE_URL             = local.app_url
    BETTER_AUTH_URL           = local.app_url # legacy fallback for AUTH_UPSTREAM_URL; ignored while that is set
    AUTH_SECURE_COOKIES       = "true"
    AUTH_SESSION_TIMEOUT      = "5s"
    BETTER_AUTH_COOKIE_PREFIX = local.cookie_prefix
    BETTER_AUTH_COOKIE_DOMAIN = local.zone_name

    CORS_ALLOW_ORIGINS     = local.app_url
    CORS_ALLOW_CREDENTIALS = "true"

    # No S3 access keys: the SDK falls back to the task role.
    S3_ENDPOINT = local.s3_endpoint
    S3_REGION   = var.region
    S3_BUCKET   = aws_s3_bucket.media.bucket

    LLM_PROVIDER          = "gemini"
    LLM_MODEL             = "gemini-3.1-flash-lite"
    LLM_TIMEOUT           = "30s"
    LLM_MAX_RETRIES       = "1"
    QUESTION_BANK_SIZE    = "50"
    CLOUDFLARE_ACCOUNT_ID = "e6aae3d74fae13575fc315a477a57add" # an identifier, not a credential

    AI_JD_EXTRACTION_PROVIDER    = "groq"
    AI_JD_EXTRACTION_MODEL       = "openai/gpt-oss-120b"
    AI_JD_EXTRACTION_TIMEOUT     = "30s"
    AI_JD_EXTRACTION_MAX_RETRIES = "0"
    AI_JD_EXTRACTION_FALLBACK    = "cloudflare,gemini"

    AI_QUESTION_BANK_PROVIDER    = "groq"
    AI_QUESTION_BANK_MODEL       = "openai/gpt-oss-120b"
    AI_QUESTION_BANK_TIMEOUT     = "90s"
    AI_QUESTION_BANK_MAX_RETRIES = "0"
    AI_QUESTION_BANK_FALLBACK    = "cloudflare,gemini"

    AI_FOLLOW_UP_PROVIDER    = "groq"
    AI_FOLLOW_UP_MODEL       = "openai/gpt-oss-20b"
    AI_FOLLOW_UP_TIMEOUT     = "5s"
    AI_FOLLOW_UP_MAX_RETRIES = "0"
    AI_FOLLOW_UP_FALLBACK    = "gemini,cloudflare"

    AI_EVALUATION_PROVIDER    = "groq"
    AI_EVALUATION_MODEL       = "openai/gpt-oss-120b"
    AI_EVALUATION_TIMEOUT     = "30s"
    AI_EVALUATION_MAX_RETRIES = "0"
    AI_EVALUATION_FALLBACK    = "cloudflare,gemini"

    AI_FOLLOW_UP_DECISION_PROVIDER  = "clef"
    AI_FOLLOW_UP_DECISION_MODEL     = "clef-flash"
    AI_FOLLOW_UP_DECISION_THRESHOLD = "0.5"

    INTERVIEW_STT_PROVIDER  = "azure"
    INTERVIEW_STT_TIMEOUT   = "30s"
    AZURE_SPEECH_REGION     = "koreacentral"
    TTS_TIMEOUT             = "10s"
    TTS_SYNC_LOCALES        = "en"
    EVALUATION_WEIGHT_TOTAL = "100"

    AVATAR_CONVERSION_ENABLED   = "true"
    AVATAR_BLENDER_BINARY       = "/opt/blender/blender"
    AVATAR_CONVERSION_TIMEOUT   = "120s"
    AVATAR_MAX_INPUT_BYTES      = "52428800"
    AVATAR_MAX_OUTPUT_BYTES     = "104857600"
    AVATAR_VRM_COMMERCIAL_USAGE = "personalNonProfit"
    AVATAR_VRM_LICENSE_URL      = "https://docs.google.com/document/d/e/2PACX-1vT5_TR6-MNs29LqI-LLKHvIKHVE0iluuapOpHODGRVDaqyfuCsEgaiE3ZIliI1-FN_-9rxJZ3iVo_jJ/pub"

    EMAIL_PROVIDER                        = "resend"
    EMAIL_FROM                            = "RoleCue <no-reply@auth.dorriss.com>"
    EMAIL_MODERATION_APPROVED_TEMPLATE_ID = "rolecue-posting-approved"
    EMAIL_MODERATION_REJECTED_TEMPLATE_ID = "rolecue-posting-rejected"
  }

  app_config_auth = {
    NODE_ENV                  = "production"
    HOST                      = "0.0.0.0"
    PORT                      = "3001"
    APP_URL                   = local.app_url
    AUTH_BASE_URL             = local.app_url
    BETTER_AUTH_COOKIE_PREFIX = local.cookie_prefix
    BETTER_AUTH_COOKIE_DOMAIN = local.zone_name
    AUTH_SESSION_MAX_AGE_DAYS = "14"

    # OAuth client IDs are public identifiers; the secrets are in SSM.
    GOOGLE_CLIENT_ID   = "575924259933-kha72i5h8rqe6vq128t855j1igbbck30.apps.googleusercontent.com"
    GITHUB_CLIENT_ID   = "Ov23li208S57Ni73Y4XF"
    FACEBOOK_CLIENT_ID = "3250440628481851"

    RESEND_FROM_EMAIL                   = "RoleCue <no-reply@auth.dorriss.com>"
    RESEND_VERIFY_EMAIL_TEMPLATE_ID     = "8dff9e5f-8ef9-4e40-9c74-a680d6668e05"
    RESEND_RESET_PASSWORD_TEMPLATE_ID   = "e1fa5248-0bb9-4843-a2da-dd4cf98b9f77" # gitleaks:allow (Resend template id, not a credential)
    RESEND_ACCOUNT_LOCKED_TEMPLATE_ID   = "3667a438-9b17-4696-8b07-d36e382fffe2"
    RESEND_ACCOUNT_UNLOCKED_TEMPLATE_ID = "b7757145-fc97-4153-b926-c904af1edfdd"
  }

  # NEXT_PUBLIC_* values are build arguments of the image (class B), not runtime config.
  app_config_frontend = {
    NODE_ENV              = "production"
    HOSTNAME              = "0.0.0.0"
    PORT                  = "3000"
    AUTH_SOCIAL_PROVIDERS = "google,github,facebook"
    CSP_ENFORCE           = "false" # report-only until the go-live smoke test shows no violations (Task 10)
  }

  # migrate takes everything from its database URL.
  app_config_migrate = {}

  app_config = {
    api      = local.app_config_api
    auth     = local.app_config_auth
    frontend = local.app_config_frontend
    migrate  = local.app_config_migrate
  }
}

# Container variable name => SSM parameter name under /rolecue/prod. RDS_MASTER_PASSWORD and APP_DB_PASSWORD are
# deliberately absent: they only feed the database URLs (and RDS), never a container.
locals {
  app_secrets_api = {
    DATABASE_URL               = "DATABASE_URL_APP"
    ADMIN_AUTH_INTERNAL_SECRET = "ADMIN_AUTH_INTERNAL_SECRET"
    PAYOS_CLIENT_ID            = "PAYOS_CLIENT_ID"
    PAYOS_API_KEY              = "PAYOS_API_KEY"
    PAYOS_CHECKSUM_KEY         = "PAYOS_CHECKSUM_KEY"
    LLM_API_KEY                = "LLM_API_KEY"
    GROQ_API_KEY               = "GROQ_API_KEY"
    OPENROUTER_API_KEY         = "OPENROUTER_API_KEY"
    AWS_BEARER_TOKEN_BEDROCK   = "AWS_BEARER_TOKEN_BEDROCK"
    CLOUDFLARE_AI_API_TOKEN    = "CLOUDFLARE_AI_API_TOKEN"
    AZURE_SPEECH_KEY           = "AZURE_SPEECH_KEY"
    RESEND_API_KEY             = "RESEND_API_KEY"
  }

  app_secrets_auth = {
    BETTER_AUTH_DATABASE_URL   = "DATABASE_URL_APP"
    BETTER_AUTH_SECRET         = "BETTER_AUTH_SECRET"
    ADMIN_AUTH_INTERNAL_SECRET = "ADMIN_AUTH_INTERNAL_SECRET"
    GOOGLE_CLIENT_SECRET       = "GOOGLE_CLIENT_SECRET"
    GITHUB_CLIENT_SECRET       = "GITHUB_CLIENT_SECRET"
    FACEBOOK_CLIENT_SECRET     = "FACEBOOK_CLIENT_SECRET"
    RESEND_API_KEY             = "RESEND_API_KEY"
    SUPPORT_EMAIL              = "SUPPORT_EMAIL" # the owner's address; the repo is public, so it lives in SOPS
  }

  app_secrets_migrate = {
    DATABASE_URL = "DATABASE_URL_MIGRATE" # master user: DDL rights
  }

  app_secrets = {
    api      = local.app_secrets_api
    auth     = local.app_secrets_auth
    frontend = {}
    migrate  = local.app_secrets_migrate
  }
}
