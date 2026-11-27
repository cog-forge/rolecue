variable "region" {
  description = "Only region the account's SCP allows for RoleCue resources."
  type        = string
  default     = "ap-southeast-1"
}

variable "account_id" {
  description = "Guards against applying with credentials for another account."
  type        = string
  default     = "399643456405"
}

variable "awake" {
  description = "false puts the stack to sleep: the NAT instance is stopped here, and later tasks scale ECS to 0 and remove the ALB and DNS. RDS is stopped by scripts/prod/sleep.sh."
  type        = bool
  default     = true
}

variable "go_live" {
  description = "true publishes the proxied Cloudflare CNAMEs for rolecue and rolecue-api (and only takes effect while awake). Stays false until the smoke tests through the ALB DNS name pass (plan Task 10)."
  type        = bool
  default     = false
}

variable "run_services" {
  description = "false keeps the three ECS services at desired_count 0. No images exist in ECR before the first deploy workflow run, so Task 10 flips this to true after the first deploy."
  type        = bool
  default     = false
}

variable "image_tag" {
  description = "Tag of the images in the initial task definitions and the value of APP_VERSION. The deploy workflow registers new revisions with the git SHA; the services ignore task_definition changes, so this only matters until the first deploy."
  type        = string
  default     = "bootstrap"
}
