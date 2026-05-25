output "environment" {
  description = "Terraform environment represented by this root module."
  value       = local.environment
}

output "project_id" {
  description = "GCP project ID configured for the dev environment."
  value       = var.project_id
}

output "region" {
  description = "Primary GCP region configured for dev resources."
  value       = var.region
}

output "zone" {
  description = "Primary GCP zone configured for zonal dev resources."
  value       = var.zone
}

output "resource_name_prefix" {
  description = "Prefix future resources should use for consistent dev naming."
  value       = local.resource_name_prefix
}

output "common_labels" {
  description = "Labels applied to supported GCP resources by default."
  value       = local.common_labels
}
