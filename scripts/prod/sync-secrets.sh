#!/usr/bin/env bash
# Syncs infra/aws/prod/secrets.sops.env to SSM Parameter Store as SecureString under /rolecue/prod/<KEY>.
# When the prod stack has an RDS endpoint (terraform output), also writes the two assembled database URLs.
# Idempotent. Prints key names and counts, never values. Values never appear on a command line.
#
# Usage: AWS_PROFILE=rolecue-prod scripts/prod/sync-secrets.sh
set -euo pipefail
# shellcheck source=lib.sh
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

# TLS settings for the database URLs. Change here only (open decision: verify-full needs the CA bundle in the images).
DB_TLS_QUERY="sslmode=verify-full&sslrootcert=/etc/ssl/certs/rds-global-bundle.pem"
DB_NAME="rolecue"
DB_PORT="5432"
APP_DB_USER="rolecue_app"
MASTER_DB_USER="rolecue_admin"

export AWS_REGION="${AWS_REGION:-ap-southeast-1}"
export AWS_DEFAULT_REGION="$AWS_REGION"
TF_DIR="${REPO_ROOT}/infra/aws/prod"

need aws
need sops
need jq

# The AWS CLI cannot read --cli-input-json from a pipe or /dev/fd, and a value on the command line would show up in
# `ps`. So the request JSON goes through a 0600 file in a private tmpfs directory that is removed on exit.
WORK_DIR="$(mktemp -d "${XDG_RUNTIME_DIR:-/dev/shm}/rolecue-sync.XXXXXX")"
trap 'rm -rf -- "$WORK_DIR"' EXIT
REQ="${WORK_DIR}/request.json"

put_param() { # <name> <value>
  # printf is a shell builtin and jq reads the value from stdin, so the value is never in any process's argv.
  (umask 077 && printf '%s' "$2" | jq -n --arg name "${SSM_PREFIX}/$1" --rawfile value /dev/stdin \
    '{Name: $name, Value: $value, Type: "SecureString", Overwrite: true}' >"$REQ")
  aws ssm put-parameter --cli-input-json "file://${REQ}" --output text --query Version >/dev/null
  : >"$REQ"
}

tag_param() { # tags cannot be set together with --overwrite, so tag in a separate (idempotent) call
  aws ssm add-tags-to-resource --resource-type Parameter --resource-id "${SSM_PREFIX}/$1" \
    --tags Key=Project,Value=rolecue Key=Env,Value=prod Key=ManagedBy,Value=terraform >/dev/null
}

PLAIN="$(decrypt_secrets)"

get() { # <KEY> -> value from the decrypted dotenv (split on the first "=")
  local line
  line="$(printf '%s\n' "$PLAIN" | grep -E "^${1}=" | head -n 1)" || true
  [ -n "$line" ] || return 1
  printf '%s' "${line#*=}"
}

written=0
while IFS= read -r line; do
  case "$line" in
    [A-Z]*=*) ;;
    *) continue ;;
  esac
  key="${line%%=*}"
  case "$key" in sops_*) continue ;; esac
  put_param "$key" "${line#*=}"
  tag_param "$key"
  written=$((written + 1))
  printf '  synced %s\n' "$key"
done <<<"$PLAIN"

db_urls=0
if endpoint="$(terraform -chdir="$TF_DIR" output -raw rds_endpoint 2>/dev/null)" && [ -n "$endpoint" ]; then
  endpoint="${endpoint%:*}" # drop a trailing :port if the output has one
  app_pw="$(get APP_DB_PASSWORD)" || die "APP_DB_PASSWORD missing from the secrets file"
  master_pw="$(get RDS_MASTER_PASSWORD)" || die "RDS_MASTER_PASSWORD missing from the secrets file"
  put_param DATABASE_URL_APP "postgres://${APP_DB_USER}:${app_pw}@${endpoint}:${DB_PORT}/${DB_NAME}?${DB_TLS_QUERY}"
  tag_param DATABASE_URL_APP
  put_param DATABASE_URL_MIGRATE "postgres://${MASTER_DB_USER}:${master_pw}@${endpoint}:${DB_PORT}/${DB_NAME}?${DB_TLS_QUERY}"
  tag_param DATABASE_URL_MIGRATE
  db_urls=2
  printf '  synced DATABASE_URL_APP, DATABASE_URL_MIGRATE\n'
else
  echo "RDS endpoint not available from terraform output; skipping DATABASE_URL_APP and DATABASE_URL_MIGRATE."
fi

total="$(aws ssm get-parameters-by-path --path "$SSM_PREFIX" --recursive --no-with-decryption \
  --query 'Parameters[].Name' --output json | jq 'length')"
echo "Wrote ${written} secrets + ${db_urls} database URLs; ${total} parameters now under ${SSM_PREFIX}."
