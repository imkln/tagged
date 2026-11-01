locals {
  origin = aws_apigatewayv2_api.web.api_endpoint
}

resource "random_id" "cookie_key" {
  byte_length = 32
}

resource "aws_cognito_user_pool" "owner" {
  name                = "tagged"
  user_pool_tier      = "LITE"
  deletion_protection = "ACTIVE"

  admin_create_user_config {
    allow_admin_create_user_only = true
  }

  username_configuration {
    case_sensitive = false
  }
}

resource "aws_cognito_user_pool_domain" "login" {
  domain                = var.cognito_domain_prefix
  user_pool_id          = aws_cognito_user_pool.owner.id
  managed_login_version = 1
}

resource "aws_cognito_identity_provider" "google" {
  user_pool_id  = aws_cognito_user_pool.owner.id
  provider_name = "Google"
  provider_type = "Google"

  provider_details = {
    client_id        = var.google_client_id
    client_secret    = var.google_client_secret
    authorize_scopes = "openid email profile"
  }

  attribute_mapping = {
    email          = "email"
    email_verified = "email_verified"
    username       = "sub"
  }
}

resource "aws_cognito_user_pool_client" "web" {
  name                                 = "tagged-web"
  user_pool_id                         = aws_cognito_user_pool.owner.id
  generate_secret                      = false
  supported_identity_providers         = [aws_cognito_identity_provider.google.provider_name]
  allowed_oauth_flows_user_pool_client = true
  allowed_oauth_flows                  = ["code"]
  allowed_oauth_scopes                 = ["openid", "email", "profile"]
  prevent_user_existence_errors        = "ENABLED"
  enable_token_revocation              = true
  explicit_auth_flows                  = ["ALLOW_REFRESH_TOKEN_AUTH"]
  read_attributes                      = ["email", "email_verified"]
  write_attributes                     = ["email", "email_verified"]
  id_token_validity                    = 1
  access_token_validity                = 1

  callback_urls = [
    "${local.origin}/auth/callback",
    "http://localhost:5173/auth/callback",
  ]
  logout_urls = [local.origin, "${local.origin}/", "http://localhost:5173/"]

  token_validity_units {
    id_token     = "hours"
    access_token = "hours"
  }
}

resource "aws_iam_role" "api" {
  name_prefix = "tagged-api-"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "lambda.amazonaws.com" }
      Action    = "sts:AssumeRole"
    }]
  })
}

resource "aws_cloudwatch_log_group" "api" {
  name              = "/aws/lambda/tagged-api"
  retention_in_days = 7
}

resource "aws_iam_role_policy" "logs" {
  role = aws_iam_role.api.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect   = "Allow"
      Action   = ["logs:CreateLogStream", "logs:PutLogEvents"]
      Resource = "${aws_cloudwatch_log_group.api.arn}:*"
    }]
  })
}

resource "aws_lambda_function" "api" {
  function_name                  = "tagged-api"
  role                           = aws_iam_role.api.arn
  runtime                        = "provided.al2023"
  handler                        = "bootstrap"
  architectures                  = ["arm64"]
  filename                       = "${path.module}/../build/tagged.zip"
  source_code_hash               = filebase64sha256("${path.module}/../build/tagged.zip")
  timeout                        = 30
  memory_size                    = 256
  reserved_concurrent_executions = 2

  environment {
    variables = {
      APP_ORIGIN     = local.origin
      OIDC_ISSUER    = "https://cognito-idp.${var.aws_region}.amazonaws.com/${aws_cognito_user_pool.owner.id}"
      OIDC_CLIENT_ID = aws_cognito_user_pool_client.web.id
      AUTH_DOMAIN    = "https://${aws_cognito_user_pool_domain.login.domain}.auth.${var.aws_region}.amazoncognito.com"
      OWNER_EMAIL    = var.owner_email
      OWNER_SUBJECT  = var.owner_subject
      COOKIE_KEY     = random_id.cookie_key.hex
    }
  }

  depends_on = [aws_iam_role_policy.logs]
}

resource "aws_apigatewayv2_api" "web" {
  name          = "tagged"
  protocol_type = "HTTP"
}

resource "aws_apigatewayv2_integration" "api" {
  api_id                 = aws_apigatewayv2_api.web.id
  integration_type       = "AWS_PROXY"
  integration_uri        = aws_lambda_function.api.invoke_arn
  payload_format_version = "2.0"
  timeout_milliseconds   = 30000
}

resource "aws_apigatewayv2_route" "all" {
  api_id    = aws_apigatewayv2_api.web.id
  route_key = "$default"
  target    = "integrations/${aws_apigatewayv2_integration.api.id}"
}

resource "aws_apigatewayv2_stage" "web" {
  api_id      = aws_apigatewayv2_api.web.id
  name        = "$default"
  auto_deploy = true

  default_route_settings {
    throttling_burst_limit = 10
    throttling_rate_limit  = 2
  }
}

resource "aws_lambda_permission" "gateway" {
  statement_id  = "APIGatewayInvoke"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.api.function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${aws_apigatewayv2_api.web.execution_arn}/*/*"
}
