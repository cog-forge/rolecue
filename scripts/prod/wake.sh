#!/usr/bin/env bash
# Wakes the prod stack: starts RDS and waits for it, sets awake = true, then plans, asks for an interactive "yes" and
# applies (NAT, ALB, listener, rules and, with go_live, DNS come back; ECS scales to 1 when run_services is true).
# Then waits for the services to be stable and smoke-tests the API when go_live is true.
# If you answer anything else, or the plan fails, awake goes back to its previous value (RDS stays running).
#
# Usage: AWS_PROFILE=rolecue-prod scripts/prod/wake.sh
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
need curl

previous="$(env_value "$AWAKE_FILE" awake)"
[ -n "$previous" ] || die "no awake value in ${AWAKE_FILE}"

echo "Starting RDS ${DB_ID}"
status="$(rds_status)"
if [ "$status" = "stopping" ]; then
  echo "  still stopping, waiting until it is stopped"
  wait_rds_stopped
  status="stopped"
fi
case "$status" in
  available | starting) echo "  already ${status}, continuing" ;;
  stopped)
    out="$(aws rds start-db-instance --db-instance-identifier "$DB_ID" --query 'DBInstance.DBInstanceStatus' --output text)"
    echo "  status: ${out}"
    ;;
  *) die "RDS is '${status}', which wake.sh does not handle; check the console" ;;
esac

echo "Waiting for RDS to be available (can take several minutes)"
aws rds wait db-instance-available --db-instance-identifier "$DB_ID"

echo "Setting awake = true in ${AWAKE_FILE#"$REPO_ROOT"/}"
set_awake true

rc=0
plan_and_confirm || rc=$?
if [ "$rc" -ne 0 ]; then
  set_awake "$previous"
  [ "$rc" -eq 1 ] && echo "Cancelled. awake restored to ${previous}; RDS is running." || echo "Plan failed. awake restored to ${previous}; RDS is running." >&2
  exit 1
fi

apply_plan

if [ "$(tfvar run_services)" = "true" ]; then
  echo "Waiting for the ECS services to be stable"
  aws ecs wait services-stable --cluster "$CLUSTER" --services frontend auth api
fi

if [ "$(tfvar go_live)" = "true" ]; then
  echo "Smoke test: https://${APP_API_HOST}/health"
  curl -fsS --retry 5 --retry-delay 10 --retry-all-errors "https://${APP_API_HOST}/health" >/dev/null
  echo "  ok"
fi

"$(dirname "${BASH_SOURCE[0]}")/status.sh"
