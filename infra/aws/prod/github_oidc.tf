# GitHub Actions OIDC trust and the role that .github/workflows/deploy-prod.yml assumes. No long-lived AWS keys exist
# anywhere: the workflow exchanges its GitHub-issued token for short-lived credentials of this role.
#
# This repository issues IMMUTABLE subject claims (GitHub API: use_immutable_subject = true), so the `sub` of every
# token has the owner and repository IDs next to the names:
#   repo:cog-forge@211464466/rolecue@1357382102:ref:refs/heads/main
# instead of repo:cog-forge/rolecue:ref:refs/heads/main. The name-only form is NOT accepted below: it is never issued
# for this repository, and the IDs survive a rename or transfer (a re-created repo with the same name cannot match).
# Re-check with: gh api repos/cog-forge/rolecue/actions/oidc/customization/sub
#
# The suffix is the same for both triggers the workflow uses, because the job declares no `environment:` (no
# `environment:<name>` segment) and
#   - workflow_dispatch: GITHUB_REF is the branch the run was started on, so only a run started on main matches;
#   - workflow_run: such a workflow always runs on the repository's default branch, so GITHUB_REF is refs/heads/main.
# Pull requests (`ref:refs/pull/*`), other branches, tags and environments therefore cannot assume the role. Because the
# `workflow_run` token carries the default branch no matter whose commit triggered it, deploy-prod.yml itself refuses
# runs whose commit is not on main (see its guard job); the trust policy cannot tell the difference.

locals {
  # owner id@owner/repo id@repo, from `gh api repos/cog-forge/rolecue --jq '.owner.id, .id'`
  github_repo_subject = "repo:cog-forge@211464466/rolecue@1357382102"

  # The workflow hardcodes the same cluster, service and family names.
  ecs_cluster_arn = aws_ecs_cluster.main.arn
}

# AWS verifies the GitHub OIDC provider against its own trusted CA library, so no thumbprint is configured.
resource "aws_iam_openid_connect_provider" "github" {
  url            = "https://token.actions.githubusercontent.com"
  client_id_list = ["sts.amazonaws.com"]

  tags = { Name = "github-actions" }
}

data "aws_iam_policy_document" "github_deploy_trust" {
  statement {
    sid     = "GitHubMainOnly"
    effect  = "Allow"
    actions = ["sts:AssumeRoleWithWebIdentity"]

    principals {
      type        = "Federated"
      identifiers = [aws_iam_openid_connect_provider.github.arn]
    }

    condition {
      test     = "StringEquals"
      variable = "token.actions.githubusercontent.com:aud"
      values   = ["sts.amazonaws.com"]
    }

    # StringEquals, not StringLike: no wildcard can widen this to other refs.
    condition {
      test     = "StringEquals"
      variable = "token.actions.githubusercontent.com:sub"
      values   = ["${local.github_repo_subject}:ref:refs/heads/main"]
    }
  }
}

resource "aws_iam_role" "github_deploy" {
  name                 = "${local.name}-github-deploy"
  description          = "Assumed by .github/workflows/deploy-prod.yml on main: build and push images, sync secrets, migrate, roll the ECS services"
  assume_role_policy   = data.aws_iam_policy_document.github_deploy_trust.json
  max_session_duration = 3600
}

# The key behind alias/rolecue-prod-sops lives in the bootstrap state; look it up by alias instead of importing it.
data "aws_kms_alias" "sops" {
  name = "alias/rolecue-prod-sops"
}

data "aws_iam_policy_document" "github_deploy" {
  # --- ECR: push to the four repositories ---------------------------------------------------------------------------
  statement {
    sid       = "EcrLogin"
    effect    = "Allow"
    actions   = ["ecr:GetAuthorizationToken"]
    resources = ["*"] # the action has no resource-level permissions
  }

  # Push needs the layer upload actions plus the read ones buildx uses to check which blobs the registry already has.
  # DescribeImages lets the workflow skip a build whose (immutable) tag already exists.
  statement {
    sid    = "EcrPushPull"
    effect = "Allow"
    actions = [
      "ecr:BatchCheckLayerAvailability",
      "ecr:BatchGetImage",
      "ecr:CompleteLayerUpload",
      "ecr:DescribeImages",
      "ecr:GetDownloadUrlForLayer",
      "ecr:InitiateLayerUpload",
      "ecr:PutImage",
      "ecr:UploadLayerPart",
    ]
    resources = [for r in aws_ecr_repository.app : r.arn]
  }

  # --- ECS: register revisions, roll the services, run the migrate task ----------------------------------------------
  statement {
    sid    = "EcsTaskDefinitions"
    effect = "Allow"
    actions = [
      "ecs:DescribeTaskDefinition",
      "ecs:ListTaskDefinitions",
      "ecs:RegisterTaskDefinition",
    ]
    resources = ["*"] # task definition actions have no resource-level permissions
  }

  statement {
    sid       = "EcsRollServices"
    effect    = "Allow"
    actions   = ["ecs:DescribeServices", "ecs:UpdateService"]
    resources = [for s in local.ecs_services : "arn:aws:ecs:${var.region}:${local.account_id}:service/${aws_ecs_cluster.main.name}/${s}"]
  }

  # RunTask is allowed for any revision of the migrate family and of its failure-drill sibling (the workflow's
  # simulate_migrate_failure input registers the drill family), and only into this cluster.
  statement {
    sid     = "EcsRunMigrate"
    effect  = "Allow"
    actions = ["ecs:RunTask"]
    resources = [
      "arn:aws:ecs:${var.region}:${local.account_id}:task-definition/${aws_ecs_task_definition.app["migrate"].family}:*",
      "arn:aws:ecs:${var.region}:${local.account_id}:task-definition/${aws_ecs_task_definition.app["migrate"].family}-drill:*",
    ]

    condition {
      test     = "ArnEquals"
      variable = "ecs:cluster"
      values   = [local.ecs_cluster_arn]
    }
  }

  statement {
    sid       = "EcsDescribeTasks"
    effect    = "Allow"
    actions   = ["ecs:DescribeTasks"]
    resources = ["arn:aws:ecs:${var.region}:${local.account_id}:task/${aws_ecs_cluster.main.name}/*"]
  }

  # ListTasks has no resource-level permissions; the cluster condition is what scopes it.
  statement {
    sid       = "EcsListTasks"
    effect    = "Allow"
    actions   = ["ecs:ListTasks"]
    resources = ["*"]

    condition {
      test     = "ArnEquals"
      variable = "ecs:cluster"
      values   = [local.ecs_cluster_arn]
    }
  }

  # --- IAM: registering a task definition passes its roles to ECS --------------------------------------------------
  statement {
    sid     = "PassEcsRoles"
    effect  = "Allow"
    actions = ["iam:PassRole"]
    resources = concat(
      [aws_iam_role.execution.arn],
      [for r in aws_iam_role.task : r.arn],
    )

    condition {
      test     = "StringEquals"
      variable = "iam:PassedToService"
      values   = ["ecs-tasks.amazonaws.com"]
    }
  }

  # --- SSM and KMS: scripts/prod/sync-secrets.sh ---------------------------------------------------------------------
  # SecureStrings use the AWS-managed alias/aws/ssm key, whose key policy already lets principals of this account use
  # it through SSM, so no kms permission is needed for the writes. AddTagsToResource and GetParametersByPath are the
  # two extra calls the script makes (tagging, and counting what is under the prefix). GetParametersByPath is
  # authorized against the path itself, so the prefix is listed with and without /*.
  statement {
    sid    = "SyncSecrets"
    effect = "Allow"
    actions = [
      "ssm:AddTagsToResource",
      "ssm:GetParameter",
      "ssm:GetParametersByPath",
      "ssm:PutParameter",
    ]
    resources = [
      local.ssm_parameter_arn_prefix,
      "${local.ssm_parameter_arn_prefix}/*",
    ]
  }

  statement {
    sid       = "DecryptSopsFile"
    effect    = "Allow"
    actions   = ["kms:Decrypt"]
    resources = [data.aws_kms_alias.sops.target_key_arn]
  }

  # --- Guard, migrate logs, network lookup --------------------------------------------------------------------------
  statement {
    sid       = "RdsStateForGuard"
    effect    = "Allow"
    actions   = ["rds:DescribeDBInstances"]
    resources = [aws_db_instance.main.arn]
  }

  statement {
    sid       = "ReadMigrateLogs"
    effect    = "Allow"
    actions   = ["logs:FilterLogEvents", "logs:GetLogEvents"]
    resources = ["${aws_cloudwatch_log_group.app["migrate"].arn}:*"]
  }

  # The workflow finds the private-app subnets and the migrate security group by tag/name.
  statement {
    sid       = "NetworkLookup"
    effect    = "Allow"
    actions   = ["ec2:DescribeSecurityGroups", "ec2:DescribeSubnets"]
    resources = ["*"] # Describe* has no resource-level permissions
  }
}

resource "aws_iam_role_policy" "github_deploy" {
  name   = "deploy-prod"
  role   = aws_iam_role.github_deploy.id
  policy = data.aws_iam_policy_document.github_deploy.json
}

output "github_deploy_role_arn" {
  description = "Hardcoded in .github/workflows/deploy-prod.yml (a public value)."
  value       = aws_iam_role.github_deploy.arn
}
