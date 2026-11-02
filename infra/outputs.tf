output "website_url" {
  value = aws_apigatewayv2_api.web.api_endpoint
}

output "cognito_pool_id" {
  value = aws_cognito_user_pool.owner.id
}

output "cognito_client_id" {
  value = aws_cognito_user_pool_client.web.id
}

output "oidc_issuer" {
  value = "https://cognito-idp.${var.aws_region}.amazonaws.com/${aws_cognito_user_pool.owner.id}"
}

output "auth_domain" {
  value = "https://${aws_cognito_user_pool_domain.login.domain}.auth.${var.aws_region}.amazoncognito.com"
}
