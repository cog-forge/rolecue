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
