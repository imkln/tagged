resource "aws_iam_role" "worker" {
  name_prefix = "tagged-worker-"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "lambda.amazonaws.com" }
      Action    = "sts:AssumeRole"
    }]
  })
}

resource "aws_cloudwatch_log_group" "worker" {
  name              = "/aws/lambda/tagged-worker"
  retention_in_days = 7
}

resource "aws_iam_role_policy" "worker" {
  role = aws_iam_role.worker.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect   = "Allow"
        Action   = ["logs:CreateLogStream", "logs:PutLogEvents"]
        Resource = "${aws_cloudwatch_log_group.worker.arn}:*"
      },
      {
        Effect   = "Allow"
        Action   = ["dynamodb:GetItem", "dynamodb:PutItem"]
        Resource = aws_dynamodb_table.photos.arn
      },
      {
        Effect   = "Allow"
        Action   = ["s3:GetObject"]
        Resource = "${aws_s3_bucket.photos.arn}/originals/*"
      },
      {
        Effect   = "Allow"
        Action   = ["s3:PutObject"]
        Resource = "${aws_s3_bucket.photos.arn}/thumbnails/*"
      }
    ]
  })
}

resource "aws_lambda_function" "worker" {
  function_name    = "tagged-worker"
  role             = aws_iam_role.worker.arn
  runtime          = "provided.al2023"
  handler          = "bootstrap"
  architectures    = ["arm64"]
  filename         = "${path.module}/../build/worker.zip"
  source_code_hash = filebase64sha256("${path.module}/../build/worker.zip")
  timeout          = 180
  memory_size      = 2048

  environment {
    variables = {
      PHOTO_BUCKET     = aws_s3_bucket.photos.id
      PHOTO_TABLE      = aws_dynamodb_table.photos.name
      MAX_UPLOAD_BYTES = tostring(var.max_upload_bytes)
    }
  }

  depends_on = [aws_iam_role_policy.worker]
}

resource "aws_lambda_function_event_invoke_config" "worker" {
  function_name          = aws_lambda_function.worker.function_name
  maximum_retry_attempts = 0
}

resource "aws_lambda_permission" "photos" {
  statement_id  = "AllowPhotoUploads"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.worker.function_name
  principal     = "s3.amazonaws.com"
  source_arn    = aws_s3_bucket.photos.arn
}

resource "aws_s3_bucket_notification" "photos" {
  bucket = aws_s3_bucket.photos.id

  lambda_function {
    lambda_function_arn = aws_lambda_function.worker.arn
    events              = ["s3:ObjectCreated:*"]
    filter_prefix       = "originals/"
  }

  depends_on = [aws_lambda_permission.photos, aws_lambda_function_event_invoke_config.worker]
}
