resource "aws_s3_bucket" "artifacts-bucket" {
  bucket = "artifacts-${var.account-id}-${var.region}"

  lifecycle_rule {
    id                                     = "expire-multipart"
    enabled                                = true
    abort_incomplete_multipart_upload_days = 1
  }
}

resource "aws_s3_bucket_public_access_block" "private" {
  bucket                  = aws_s3_bucket.artifacts-bucket.id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}
