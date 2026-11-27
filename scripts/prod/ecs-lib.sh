#!/usr/bin/env bash
# ECS helpers shared by the prod deploy workflow. Source it; do not run it. Needs aws and jq, and AWS credentials.

# new_revision <family> <container> <image> [jq-filter]
# Registers a copy of the family's latest ACTIVE revision whose <container> runs <image>, and prints the new revision
# ARN. Where the container has an APP_VERSION variable it is set to $IMAGE_TAG. The optional jq filter is applied last
# to the whole task definition. Prints no task definition content, only the ARN.
new_revision() {
  local family="$1" container="$2" image="$3" extra="${4:-.}" work
  work="$(mktemp -d "${RUNNER_TEMP:-/tmp}/taskdef.XXXXXX")" || return 1
  # Each stage writes a file, so a failure in any of them (unknown container, bad filter) stops here instead of
  # registering a stale or empty definition.
  if aws ecs describe-task-definition --task-definition "$family" --query taskDefinition --output json >"$work/current.json" &&
    jq --arg c "$container" --arg image "$image" --arg v "${IMAGE_TAG:?IMAGE_TAG is not set}" '
      del(.taskDefinitionArn, .revision, .status, .requiresAttributes, .compatibilities,
          .registeredAt, .registeredBy, .deregisteredAt)
      | if ([.containerDefinitions[] | select(.name == $c)] | length) != 1
        then error("container \($c) not found exactly once") else . end
      | .containerDefinitions |= map(
          if .name == $c then
            .image = $image
            | if has("environment")
              then .environment |= map(if .name == "APP_VERSION" then .value = $v else . end)
              else . end
          else . end)' "$work/current.json" >"$work/swapped.json" &&
    jq "$extra" "$work/swapped.json" >"$work/new.json"; then
    aws ecs register-task-definition --cli-input-json "file://$work/new.json" \
      --query taskDefinition.taskDefinitionArn --output text
    local rc=$?
    rm -rf -- "$work"
    return "$rc"
  fi
  rm -rf -- "$work"
  return 1
}
