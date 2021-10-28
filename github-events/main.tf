resource "aws_apigatewayv2_api" "api" {
  name            = "github-events"
  protocol_type   = "HTTP"
  target          = aws_lambda_function.handler.arn
  credentials_arn = var.role-arn
}

resource "aws_lambda_function" "handler" {
  depends_on = [
    aws_s3_bucket_object.bundle
  ]

  function_name = "github-events"
  role          = var.role-arn
  handler       = "github-events"
  runtime       = "go1.x"
  timeout       = 60

  s3_bucket = var.bucket-name
  s3_key    = "aws-basics/github-events/${var.bundle-version}.zip"

  source_code_hash = filebase64sha256("dist/github-events.zip")

  environment {
    variables = {
      "GITHUB_EVENT_BUS_NAME" = aws_cloudwatch_event_bus.bus.name
      "GITHUB_WEBHOOK_SECRET" = var.webhook-secret
    }
  }
}

resource "aws_s3_bucket_object" "bundle" {
  # lifecycle {
  #   ignore_changes = all
  # }

  bucket = var.bucket-name
  key    = "aws-basics/github-events/${var.bundle-version}.zip"
  source = "dist/github-events.zip"
  etag   = filemd5("dist/github-events.zip")
}

resource "aws_cloudwatch_log_group" "logs" {
  name              = "/aws/lambda/${aws_lambda_function.handler.function_name}"
  retention_in_days = 14
}

resource "aws_cloudwatch_event_bus" "bus" {
  name = "github-events"
}

resource "aws_iam_role_policy" "permissions" {
  name = "github-events"
  role = var.role-name

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action   = "lambda:InvokeFunction"
        Effect   = "Allow"
        Resource = aws_lambda_function.handler.arn
      },
      {
        Action   = "events:PutEvents"
        Effect   = "Allow"
        Resource = aws_cloudwatch_event_bus.bus.arn
      },
      {
        Action   = "logs:*"
        Effect   = "Allow"
        Resource = "${aws_cloudwatch_log_group.logs.arn}*"
      }
    ]
  })
}
