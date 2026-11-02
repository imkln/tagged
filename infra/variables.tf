variable "aws_region" {
  description = "AWS deployment region."
  type        = string
}

variable "max_upload_bytes" {
  description = "Maximum JPEG upload size in bytes; this is not a pixel limit."
  type        = number
  default     = 268435456

  validation {
    condition     = var.max_upload_bytes > 0 && floor(var.max_upload_bytes) == var.max_upload_bytes
    error_message = "Set a positive whole number of bytes."
  }
}

variable "owner_email" {
  description = "Verified Google email allowed to access the library."
  type        = string
  sensitive   = true

  validation {
    condition     = can(regex("^[^@\\s]+@[^@\\s]+\\.[^@\\s]+$", var.owner_email))
    error_message = "Set a valid owner email."
  }
}

variable "owner_subject" {
  description = "Optional Cognito subject to additionally bind access to a stable identity after first sign-in."
  type        = string
  sensitive   = true
  default     = ""
}

variable "google_client_id" {
  description = "Web OAuth client ID registered in Google Auth Platform."
  type        = string
}

variable "google_client_secret" {
  description = "Google OAuth client secret. Stored in encrypted remote Terraform state; never commit it."
  type        = string
  sensitive   = true
}

variable "cognito_domain_prefix" {
  description = "A globally unique Cognito domain prefix, also used when registering the Google callback."
  type        = string

  validation {
    condition     = can(regex("^[a-z0-9][a-z0-9-]{2,50}$", var.cognito_domain_prefix))
    error_message = "Use 3–51 lowercase letters, digits, or hyphens, starting with a letter or digit."
  }
}
