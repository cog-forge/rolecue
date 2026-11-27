output "state_bucket" {
  description = "Backend bucket for infra/aws/prod (use_lockfile = true, no DynamoDB)."
  value       = aws_s3_bucket.tfstate.bucket
}

output "sops_kms_key_arn" {
  value = aws_kms_key.sops.arn
}

output "sops_kms_alias_arn" {
  description = "Referenced by .sops.yaml at the repository root."
  value       = aws_kms_alias.sops.arn
}
