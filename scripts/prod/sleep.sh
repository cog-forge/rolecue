#!/usr/bin/env bash
# Puts the prod stack to sleep: awake = false (ECS to 0, ALB, listener, rules and DNS removed, NAT stopped, daily
# RDS-stop schedule created), then stops RDS with the CLI. Shows the plan and waits for an interactive "yes".
# If you answer anything else, or the plan fails, awake goes back to its previous value.
#
# Usage: AWS_PROFILE=rolecue-prod scripts/prod/sleep.sh
set -euo pipefail
# shellcheck source=lib.sh
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
# shellcheck source=tf-helpers.sh
source "$(dirname "${BASH_SOURCE[0]}")/tf-helpers.sh"

[ -t 0 ] || die "needs an interactive terminal to confirm; nothing was changed"
need aws
need sops
need terraform
need jq

previous="$(env_value "$AWAKE_FILE" awake)"
[ -n "$previous" ] || die "no awake value in ${AWAKE_FILE}"

echo "Setting awake = false in ${AWAKE_FILE#"$REPO_ROOT"/}"
set_awake false

rc=0
plan_and_confirm || rc=$?
if [ "$rc" -ne 0 ]; then
  set_awake "$previous"
  if [ "$rc" -eq 1 ]; then
    echo "Cancelled. awake restored to ${previous}."
  else
    echo "Plan failed. awake restored to ${previous}." >&2
  fi
  exit 1
fi

apply_plan

echo "Stopping RDS ${DB_ID}"
if ! out="$(aws rds stop-db-instance --db-instance-identifier "$DB_ID" --query 'DBInstance.DBInstanceStatus' --output text 2>&1)"; then
  case "$out" in
    *InvalidDBInstanceState*)
      # The same error covers every state that is not "available". Only stopped/stopping is fine.
      status="$(rds_status)"
      case "$status" in
        stopped | stopping) echo "  already ${status}, nothing to do" ;;
        *) die "RDS is '${status}' and cannot be stopped now; wait for 'available' and run: aws rds stop-db-instance --db-instance-identifier ${DB_ID}" ;;
      esac
      ;;
    *)
      printf '%s\n' "$out" >&2
      die "could not stop RDS"
      ;;
  esac
else
  echo "  status: ${out}"
fi

"$(dirname "${BASH_SOURCE[0]}")/status.sh"
