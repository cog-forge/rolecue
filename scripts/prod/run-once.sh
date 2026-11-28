#!/usr/bin/env bash
# Runs a one-off command as a Fargate task in the prod ECS cluster, waits for it, prints the log tail and exits with
# the container's exit code. Meant for admin chores that have no service of their own (creating the app database role,
# inspecting the database). It never touches the deploy families: the task family is rolecue-prod-<name>.
#
# Usage: AWS_PROFILE=rolecue-prod scripts/prod/run-once.sh <name> [--sql FILE] -- <command...>
#
# The container runs postgres:18-alpine (psql, busybox wget/base64) with these secrets injected from SSM:
#   DATABASE_URL_MIGRATE (master user), DATABASE_URL_APP (rolecue_app), APP_DB_PASSWORD
# Before your command, the RDS CA bundle is fetched, its sha256 is checked against api/certs/rds-global-bundle.pem and
# it is installed at /etc/ssl/certs/rds-global-bundle.pem, so the URLs work unchanged with sslmode=verify-full.
# --sql FILE makes the file's content available gzipped and base64-encoded in $SQL_B64 (no secrets belong in it; ECS caps
# the whole override at 8192 characters). Put the command in single quotes so the variables expand in the container:
#   run-once.sh app-role --sql scripts/prod/sql/00-app-role.sql -- \
#     'printf %s "$SQL_B64" | base64 -d | gunzip | psql "$DATABASE_URL_MIGRATE" -X -q -v ON_ERROR_STOP=1 -f -'
# Secret values are never printed; the log tail is scrubbed of anything shaped like a connection string.
set -euo pipefail
# shellcheck source=lib.sh
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

REGION="${AWS_REGION:-ap-southeast-1}"
export AWS_REGION="$REGION" AWS_DEFAULT_REGION="$REGION"
CLUSTER="rolecue-prod"
IMAGE="public.ecr.aws/docker/library/postgres:18-alpine" # ECR Public: no Docker Hub rate limit
CA_BUNDLE="${REPO_ROOT}/api/certs/rds-global-bundle.pem"
CA_URL="https://truststore.pki.rds.amazonaws.com/global/global-bundle.pem"
LOG_GROUP="/rolecue/prod/migrate"
CONTAINER="run"

usage() {
  sed -n '2,/^set -euo/p' "${BASH_SOURCE[0]}" | sed '$d' | sed -E 's/^# ?//' >&2
  exit 2
}

sql_file=""
name=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    --) shift; break ;;
    --sql) [ "$#" -ge 2 ] || usage; sql_file="$2"; shift 2 ;;
    -h | --help) usage ;;
    -*) usage ;;
    *) [ -z "$name" ] || usage; name="$1"; shift ;;
  esac
done
[ -n "$name" ] && [ "$#" -ge 1 ] || usage
command_str="$*"

[[ "$name" =~ ^[a-z][a-z0-9-]{0,40}$ ]] || die "name must be lowercase letters, digits and dashes, starting with a letter"
# A revision registered into a deploy family would be copied by the next deploy (new_revision takes the latest).
case "$name" in
  frontend | auth | api | migrate | migrate-drill) die "'$name' is reserved for the deploy workflow" ;;
esac
need aws
need jq
need base64
need gzip
need sha256sum
[ -f "$CA_BUNDLE" ] || die "missing ${CA_BUNDLE#"$REPO_ROOT"/}"

account="$(aws sts get-caller-identity --query Account --output text)"
family="rolecue-prod-${name}"
ssm="arn:aws:ssm:${REGION}:${account}:parameter${SSM_PREFIX}"
ca_sha="$(sha256sum "$CA_BUNDLE" | cut -d' ' -f1)"

sql_b64=""
if [ -n "$sql_file" ]; then
  [ -f "$sql_file" ] || die "no such file: $sql_file"
  sql_b64="$(gzip -9n <"$sql_file" | base64 -w0)"
fi

# Container start: install and verify the RDS CA bundle, then run the caller's command.
bootstrap='set -eu
wget -q -O /tmp/rds-ca.pem "$RDS_CA_URL"
echo "$RDS_CA_SHA256  /tmp/rds-ca.pem" | sha256sum -c - >/dev/null || { echo "RDS CA bundle checksum mismatch" >&2; exit 97; }
mv /tmp/rds-ca.pem /etc/ssl/certs/rds-global-bundle.pem
'
script="${bootstrap}${command_str}"

work="$(mktemp -d)"
trap 'rm -rf -- "$work"' EXIT

jq -n --arg family "$family" --arg image "$IMAGE" --arg group "$LOG_GROUP" --arg region "$REGION" \
  --arg prefix "$name" --arg container "$CONTAINER" --arg exec "arn:aws:iam::${account}:role/rolecue-prod-ecs-execution" \
  --arg ssm "$ssm" --arg ca_url "$CA_URL" --arg ca_sha "$ca_sha" '{
  family: $family, requiresCompatibilities: ["FARGATE"], networkMode: "awsvpc", cpu: "256", memory: "512",
  executionRoleArn: $exec,
  runtimePlatform: {operatingSystemFamily: "LINUX", cpuArchitecture: "X86_64"},
  containerDefinitions: [{
    name: $container, image: $image, essential: true, entryPoint: ["sh", "-c"],
    environment: [{name: "RDS_CA_URL", value: $ca_url}, {name: "RDS_CA_SHA256", value: $ca_sha}],
    secrets: [("DATABASE_URL_MIGRATE", "APP_DB_PASSWORD", "DATABASE_URL_APP") | {name: ., valueFrom: "\($ssm)/\(.)"}],
    logConfiguration: {logDriver: "awslogs", options: {
      "awslogs-group": $group, "awslogs-region": $region, "awslogs-stream-prefix": $prefix}}
  }]
}' >"$work/taskdef.json"

# Register a new revision only when the definition changed (the hash rides along as a tag).
spec_hash="$(sha256sum "$work/taskdef.json" | cut -c1-32)"
current_hash="$(aws ecs describe-task-definition --task-definition "$family" --include TAGS \
  --query "tags[?key=='specHash']|[0].value" --output text 2>/dev/null || true)"
if [ "$current_hash" = "$spec_hash" ]; then
  td="$(aws ecs describe-task-definition --task-definition "$family" --query taskDefinition.taskDefinitionArn --output text)"
  echo "Task definition ${td##*/} is current."
else
  td="$(aws ecs register-task-definition --cli-input-json "file://$work/taskdef.json" \
    --tags "key=specHash,value=$spec_hash" --query taskDefinition.taskDefinitionArn --output text)"
  echo "Registered ${td##*/}."
fi

jq -n --arg container "$CONTAINER" --arg script "$script" --arg sql "$sql_b64" '{
  containerOverrides: [{name: $container, command: [$script],
    environment: (if $sql == "" then [] else [{name: "SQL_B64", value: $sql}] end)}]}' >"$work/overrides.json"
size="$(jq -c . "$work/overrides.json" | wc -c)"
[ "$size" -le 8000 ] || die "container overrides are ${size} characters; ECS allows 8192 (shorten the SQL or the command)"

subnets="$(aws ec2 describe-subnets \
  --filters Name=tag:Project,Values=rolecue Name=tag:Env,Values=prod Name=tag:Tier,Values=private-app \
  --query 'Subnets[].SubnetId' --output text | tr '\t' ',')"
sg="$(aws ec2 describe-security-groups --filters Name=group-name,Values=rolecue-prod-migrate Name=tag:Project,Values=rolecue \
  --query 'SecurityGroups[0].GroupId' --output text)"
[ -n "$subnets" ] && [ -n "$sg" ] && [ "$sg" != None ] || die "cannot find the private-app subnets or the migrate security group"

run="$(aws ecs run-task --cluster "$CLUSTER" --task-definition "$td" --launch-type FARGATE --count 1 \
  --started-by "run-once-${name}" --overrides "file://$work/overrides.json" \
  --network-configuration "awsvpcConfiguration={subnets=[$subnets],securityGroups=[$sg],assignPublicIp=DISABLED}" \
  --output json)"
task_arn="$(jq -r '.tasks[0].taskArn // empty' <<<"$run")"
if [ -z "$task_arn" ]; then
  jq -r '.failures[] | "run-task failure: \(.arn // "-") \(.reason) \(.detail // "")"' <<<"$run" >&2
  exit 1
fi
task_id="${task_arn##*/}"
echo "Task ${task_id} started from ${td##*/}; waiting for it to stop..."

for _ in 1 2 3; do # each wait polls for up to 10 minutes
  aws ecs wait tasks-stopped --cluster "$CLUSTER" --tasks "$task_arn" 2>/dev/null && break
done
info="$(aws ecs describe-tasks --cluster "$CLUSTER" --tasks "$task_arn" --output json)"
[ "$(jq -r '.tasks[0].lastStatus' <<<"$info")" = "STOPPED" ] || die "task ${task_id} is still running; it was not stopped"
exit_code="$(jq -r --arg c "$CONTAINER" '[.tasks[0].containers[] | select(.name == $c) | .exitCode] | first // "none"' <<<"$info")"
stopped="$(jq -r '.tasks[0].stoppedReason // "-"' <<<"$info")"

# CloudWatch lags a few seconds behind the container; retry while the stream is empty.
echo "--- log tail (${LOG_GROUP} ${name}/${CONTAINER}/${task_id}) ---"
for attempt in 1 2 3 4 5 6; do
  logs="$(aws logs get-log-events --log-group-name "$LOG_GROUP" --log-stream-name "${name}/${CONTAINER}/${task_id}" \
    --limit 200 --query 'events[].message' --output text 2>/dev/null || true)"
  [ -n "$logs" ] && break
  sleep 5
done
if [ -n "${logs:-}" ]; then
  # Never let a connection string reach the output, whatever the tool printed.
  aws logs get-log-events --log-group-name "$LOG_GROUP" --log-stream-name "${name}/${CONTAINER}/${task_id}" \
    --limit 200 --query 'events[].message' --output json | jq -r '.[]' | sed -E "s#(://)[^[:space:]\"']*@#\\1***@#g"
else
  echo "(no log events)"
fi
echo "--- end of log ---"

if [ "$exit_code" = "none" ]; then
  echo "Task ${task_id} never produced an exit code (stopped reason: ${stopped})." >&2
  exit 1
fi
echo "Exit code ${exit_code}."
exit "$exit_code"
