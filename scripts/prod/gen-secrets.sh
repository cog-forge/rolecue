#!/usr/bin/env bash
# Builds infra/aws/prod/secrets.sops.env (SOPS + KMS alias/rolecue-prod-sops):
#   class S: copied from the owner's .env and auth/.env (14 keys, current third-party values)
#   class G: generated fresh for prod (4 keys, 48 alphanumeric characters)
# See docs/superpowers/plans/2026-11-27-prod-env-inventory.md. Prints key names and counts, never values.
#
# Usage: scripts/prod/gen-secrets.sh [--force]
#   ENV_SRC_DIR overrides the directory that holds .env and auth/.env (default: the owner's main checkout).
#
# WARNING: --force regenerates RDS_MASTER_PASSWORD and the other class G values. Once RDS exists, doing that makes
# the stored password disagree with the database. Rotate deliberately instead.
set -euo pipefail
# shellcheck source=lib.sh
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

FORCE=0
case "${1:-}" in
  "") ;;
  --force) FORCE=1 ;;
  *) die "usage: $0 [--force]" ;;
esac

ENV_SRC_DIR="${ENV_SRC_DIR:-/home/dorriss/Projects/university/sep490/ai-interview-practice}"
ROOT_ENV="${ENV_SRC_DIR}/.env"
AUTH_ENV="${ENV_SRC_DIR}/auth/.env"

ROOT_KEYS=(
  PAYOS_CLIENT_ID PAYOS_API_KEY PAYOS_CHECKSUM_KEY
  LLM_API_KEY AZURE_SPEECH_KEY GROQ_API_KEY
  CLOUDFLARE_AI_API_TOKEN OPENROUTER_API_KEY AWS_BEARER_TOKEN_BEDROCK
)
AUTH_KEYS=(GOOGLE_CLIENT_SECRET GITHUB_CLIENT_SECRET FACEBOOK_CLIENT_SECRET RESEND_API_KEY SUPPORT_EMAIL)
GENERATED_KEYS=(RDS_MASTER_PASSWORD APP_DB_PASSWORD BETTER_AUTH_SECRET ADMIN_AUTH_INTERNAL_SECRET)
EXPECTED=$((${#ROOT_KEYS[@]} + ${#AUTH_KEYS[@]} + ${#GENERATED_KEYS[@]}))

need sops
need openssl
[ -r "$ROOT_ENV" ] || die "cannot read ${ROOT_ENV}"
[ -r "$AUTH_ENV" ] || die "cannot read ${AUTH_ENV}"
if [ -e "$SECRETS_FILE" ] && [ "$FORCE" -ne 1 ]; then
  die "${SECRETS_FILE_REL} already exists; pass --force to regenerate (this rotates the class G values)"
fi

umask 077
TMP_PLAIN="$(mktemp)"
TMP_ENC="$(mktemp "${SECRETS_FILE}.XXXXXX.tmp")"
trap 'rm -f "$TMP_PLAIN" "$TMP_ENC"' EXIT

count=0
missing=()

append_from() { # <file> <keys...>
  local file="$1" key value
  shift
  for key in "$@"; do
    value="$(env_value "$file" "$key")"
    if [ -z "$value" ]; then
      missing+=("$key")
      continue
    fi
    case "$value" in *$'\n'*) die "${key} contains a newline, which dotenv cannot hold" ;; esac
    printf '%s=%s\n' "$key" "$value" >>"$TMP_PLAIN"
    count=$((count + 1))
    printf '  S  %s\n' "$key"
  done
}

echo "Class S keys:"
append_from "$ROOT_ENV" "${ROOT_KEYS[@]}"
append_from "$AUTH_ENV" "${AUTH_KEYS[@]}"

if [ "${#missing[@]}" -gt 0 ]; then
  die "missing or empty in the source .env files: ${missing[*]}"
fi

echo "Class G keys (generated, 48 alphanumeric):"
for key in "${GENERATED_KEYS[@]}"; do
  printf '%s=%s\n' "$key" "$(rand_alnum 48)" >>"$TMP_PLAIN"
  count=$((count + 1))
  printf '  G  %s\n' "$key"
done

[ "$count" -eq "$EXPECTED" ] || die "expected ${EXPECTED} keys, built ${count}"

# Run from the repo root so the creation rule in .sops.yaml matches the relative path.
(
  cd "$REPO_ROOT"
  sops --encrypt --input-type dotenv --output-type dotenv \
    --filename-override "$SECRETS_FILE_REL" "$TMP_PLAIN" >"$TMP_ENC"
)
mv -f "$TMP_ENC" "$SECRETS_FILE"
chmod 644 "$SECRETS_FILE"

echo "Wrote ${SECRETS_FILE_REL} with ${count} keys (plaintext temp file removed on exit)."
