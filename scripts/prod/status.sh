#!/usr/bin/env bash
# Prints the state of the prod stack: RDS, NAT instance, ECS services, ALB, and the awake/run_services/go_live values
# Terraform will use. Read-only; needs no Cloudflare token.
#
# Usage: AWS_PROFILE=rolecue-prod scripts/prod/status.sh
set -euo pipefail
# shellcheck source=lib.sh
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
# shellcheck source=tf-helpers.sh
source "$(dirname "${BASH_SOURCE[0]}")/tf-helpers.sh"

need aws

echo "== Terraform variables (tfvars files, else defaults) =="
for v in awake run_services go_live; do
  printf '  %-13s %s\n' "$v" "$(tfvar "$v")"
done

echo "== RDS =="
rds="$(aws rds describe-db-instances --db-instance-identifier "$DB_ID" \
  --query 'DBInstances[0].DBInstanceStatus' --output text 2>/dev/null)" || rds="not found"
printf '  %-13s %s\n' "$DB_ID" "$rds"

echo "== NAT instance =="
nat="$(aws ec2 describe-instances \
  --filters "Name=tag:Name,Values=rolecue-prod-nat" "Name=instance-state-name,Values=pending,running,stopping,stopped" \
  --query 'Reservations[].Instances[].[InstanceId,State.Name]' --output text 2>/dev/null)" || nat=""
printf '  %s\n' "${nat:-not found}"

echo "== ECS services (cluster ${CLUSTER}) =="
aws ecs describe-services --cluster "$CLUSTER" --services frontend auth api \
  --query 'services[].[serviceName,desiredCount,runningCount,pendingCount]' --output text 2>/dev/null |
  awk 'BEGIN { printf "  %-10s %8s %8s %8s\n", "service", "desired", "running", "pending" }
       { printf "  %-10s %8s %8s %8s\n", $1, $2, $3, $4 }' || echo "  not found"

echo "== ALB =="
alb="$(aws elbv2 describe-load-balancers --names "$CLUSTER" \
  --query 'LoadBalancers[0].[State.Code,DNSName]' --output text 2>/dev/null)" || alb=""
if [ -n "$alb" ]; then
  printf '  exists: %s\n' "$alb"
else
  echo "  does not exist (asleep)"
fi
