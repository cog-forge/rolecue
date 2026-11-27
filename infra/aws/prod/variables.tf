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
