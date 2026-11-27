#!/usr/bin/env bash
# Shared helpers for the prod secret scripts. Source it; do not run it.
# Rules: never `set -x`, never print a secret value, never put a secret on a command line.

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SECRETS_FILE_REL="infra/aws/prod/secrets.sops.env"
SECRETS_FILE="${REPO_ROOT}/${SECRETS_FILE_REL}"
SSM_PREFIX="/rolecue/prod"

die() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

need() {
  command -v "$1" >/dev/null 2>&1 || die "missing required tool: $1"
}

# env_value <file> <KEY>
# Prints the value of KEY from a dotenv file without executing it. Uses the last assignment, splits on the first
# "=" only (values may end in "="), tolerates "export ", CRLF and one pair of surrounding quotes. For unquoted
# values a trailing " # comment" is dropped. Prints nothing when the key is absent.
env_value() {
  local file="$1" key="$2" line value
  line="$(grep -E "^[[:space:]]*(export[[:space:]]+)?${key}[[:space:]]*=" "$file" | tail -n 1)" || true
  [ -n "$line" ] || return 0
  value="${line#*=}"
  value="${value%$'\r'}"
  value="${value#"${value%%[![:space:]]*}"}" # trim leading whitespace
  case "$value" in
    \"*\") value="${value#\"}"; value="${value%\"}" ;;
    \'*\') value="${value#\'}"; value="${value%\'}" ;;
    *)
      value="${value%%[[:space:]]#*}"
      value="${value%"${value##*[![:space:]]}"}" # trim trailing whitespace
      ;;
  esac
  printf '%s' "$value"
}

# rand_alnum <length>
# Cryptographically random [A-Za-z0-9] string of exactly <length> characters.
rand_alnum() {
  local want="$1" out="" chunk
  while [ "${#out}" -lt "$want" ]; do
    chunk="$(openssl rand -base64 96 | tr -d '\n' | tr -dc 'A-Za-z0-9')"
    out="${out}${chunk}"
  done
  printf '%s' "${out:0:want}"
}

# decrypt_secrets
# Prints the decrypted dotenv on stdout (hold it in a shell variable; never write it to disk).
decrypt_secrets() {
  [ -f "$SECRETS_FILE" ] || die "${SECRETS_FILE_REL} does not exist; run scripts/prod/gen-secrets.sh first"
  (cd "$REPO_ROOT" && sops --decrypt --input-type dotenv --output-type dotenv "$SECRETS_FILE_REL")
}
