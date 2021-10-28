terraform {
  required_version = ">= 0.12"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 3.63.0"
    }
  }
}

provider "aws" {
  region = var.primary-region
}

module "system-permissions" {
  source = "./system-permissions"
}

module "artifacts-buckets" {
  source = "./artifacts-bucket"

  for_each   = toset(var.regions)
  region     = each.key
  account-id = data.aws_caller_identity.current.account_id
}

module "github-events" {
  source = "./github-events"

  role-arn       = module.system-permissions.arn
  role-name      = module.system-permissions.name
  bucket-name    = module.artifacts-buckets[var.primary-region].name
  webhook-secret = var.webhook-secret
}

module "github-app" {
  source = "./github-app"

  role-arn    = module.system-permissions.arn
  role-name   = module.system-permissions.name
  bucket-name = module.artifacts-buckets[var.primary-region].name
  region      = var.primary-region
  account-id  = data.aws_caller_identity.current.account_id
}
