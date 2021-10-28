resource "aws_lambda_function" "handler" {
  depends_on = [
    aws_s3_bucket_object.bundle
  ]

  function_name = "github-app-tokens"
  role          = var.role-arn
  handler       = "github-app-tokens"
  runtime       = "go1.x"
  timeout       = 60

  s3_bucket = var.bucket-name
  s3_key    = "aws-basics/github-app/${var.bundle-version}.zip"

  source_code_hash = filebase64sha256("dist/github-app.zip")
}

resource "aws_s3_bucket_object" "bundle" {
  # lifecycle {
  #   ignore_changes = all
  # }

  bucket = var.bucket-name
  key    = "aws-basics/github-app/${var.bundle-version}.zip"
  source = "dist/github-app.zip"
  etag   = filemd5("dist/github-app.zip")
}

resource "aws_cloudwatch_log_group" "logs" {
  name              = "/aws/lambda/${aws_lambda_function.handler.function_name}"
  retention_in_days = 14
}

resource "aws_iam_role_policy" "permissions" {
  name = "github-app"
  role = var.role-name

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action   = "logs:*"
        Effect   = "Allow"
        Resource = "${aws_cloudwatch_log_group.logs.arn}*"
      },
      {
        Action = [
          "secretsmanager:GetSecretValue",
          "secretsmanager:PutSecretValue"
        ]
        Effect   = "Allow"
        Resource = "arn:aws:secretsmanager:${var.region}:${var.account-id}:secret:aws-basics/github-app/*"
      }
    ]
  })
}

resource "aws_cloudwatch_event_rule" "schedule" {
  name                = "github-app-tokens"
  schedule_expression = "rate(10 minutes)"
}

resource "aws_cloudwatch_event_target" "schedule-target" {
  arn  = aws_lambda_function.handler.arn
  rule = aws_cloudwatch_event_rule.schedule.id
}

resource "aws_lambda_permission" "invoke-permission" {
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.handler.function_name
  principal     = "events.amazonaws.com"
  source_arn    = aws_cloudwatch_event_rule.schedule.arn
}
