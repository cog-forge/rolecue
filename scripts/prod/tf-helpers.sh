#!/usr/bin/env bash
# Terraform helpers shared by sleep.sh, wake.sh and status.sh. Source it; do not run it. Needs lib.sh sourced first.
# Terraform always runs through `sops exec-env`, so the Cloudflare token reaches it only through its environment.

TF_DIR="${REPO_ROOT}/infra/aws/prod"
OPS_ENV_FILE="${TF_DIR}/ops.sops.env"
AWAKE_FILE="${TF_DIR}/awake.auto.tfvars"
TF_PLAN="tf.plan" # relative to TF_DIR (-chdir); gitignored

export AWS_REGION="${AWS_REGION:-ap-southeast-1}"
export AWS_DEFAULT_REGION="$AWS_REGION"
CLUSTER="rolecue-prod"
DB_ID="rolecue-prod"
APP_API_HOST="rolecue-api.dorriss.com"

# tf <terraform args...>
tf() {
  local cmd
  printf -v cmd '%q ' terraform -chdir="$TF_DIR" "$@"
  sops exec-env "$OPS_ENV_FILE" "$cmd"
}

# tfvar <name>
# The value Terraform will use: the last assignment in a tfvars file loaded automatically (*.auto.tfvars,
# terraform.tfvars), else the default in variables.tf. -var flags passed by hand are not visible here.
tfvar() {
  local name="$1" f v=""
  for f in "$TF_DIR"/terraform.tfvars "$TF_DIR"/*.auto.tfvars; do
    [ -f "$f" ] || continue
    local found
    found="$(env_value "$f" "$name")"
    [ -n "$found" ] && v="$found"
  done
  if [ -z "$v" ]; then
    v="$(awk -v n="$name" '
      $0 ~ "^variable \"" n "\"" { inblock = 1 }
      inblock && $1 == "default" { gsub(/"/, "", $3); print $3; exit }
    ' "$TF_DIR/variables.tf")"
  fi
  printf '%s' "$v"
}

# set_awake <true|false>
set_awake() {
  case "$1" in true | false) ;; *) die "set_awake: expected true or false" ;; esac
  [ -f "$AWAKE_FILE" ] || die "missing ${AWAKE_FILE}"
  sed -i -E "s/^awake[[:space:]]*=.*/awake = $1/" "$AWAKE_FILE"
  [ "$(env_value "$AWAKE_FILE" awake)" = "$1" ] || die "could not set awake = $1 in ${AWAKE_FILE}"
}

# plan_and_confirm
# Plans into tf.plan, prints the change summary, asks for an interactive "yes". Returns 0 on yes, 1 otherwise.
plan_and_confirm() {
  [ -t 0 ] || die "needs an interactive terminal to confirm"
  local log
  log="$(mktemp)"
  if ! tf plan -input=false -no-color -out="$TF_PLAN" >"$log" 2>&1; then
    cat "$log" >&2
    rm -f "$log"
    return 2
  fi
  rm -f "$log"
  echo "--- plan summary ---"
  tf show -no-color "$TF_PLAN" | grep -E '^Plan:|^No changes' || true
  tf show -json "$TF_PLAN" |
    jq -r '.resource_changes[] | select(.change.actions != ["no-op"]) | "  \(.change.actions | join("/")) \(.address)"' | sort
  echo "--------------------"
  local answer
  read -r -p 'Apply this plan? Type "yes" to continue: ' answer
  [ "$answer" = "yes" ]
}

apply_plan() {
  tf apply -input=false -no-color "$TF_PLAN"
}

# rds_status
# Prints the status of the prod DB instance (available, stopped, stopping, starting, ...).
rds_status() {
  aws rds describe-db-instances --db-instance-identifier "$DB_ID" --query 'DBInstances[0].DBInstanceStatus' --output text
}

# wait_rds_stopped
# The CLI has no waiter for "stopped", so poll (up to 20 minutes).
wait_rds_stopped() {
  local i status
  for ((i = 0; i < 80; i++)); do
    status="$(rds_status)"
    [ "$status" = "stopped" ] && return 0
    sleep 15
  done
  die "RDS did not reach 'stopped' in time (last status: ${status})"
}
