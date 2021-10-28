variable "regions" {
  type = list(any)
}

variable "primary-region" {
  type = string
}

variable "webhook-secret" {
  type = string
}

data "aws_caller_identity" "current" {}
