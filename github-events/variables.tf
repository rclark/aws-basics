variable "role-arn" {
  type = string
}

variable "role-name" {
  type = string
}

variable "bucket-name" {
  type = string
}

variable "bundle-version" {
  type    = string
  default = "latest"
}

variable "webhook-secret" {
  type = string
}
