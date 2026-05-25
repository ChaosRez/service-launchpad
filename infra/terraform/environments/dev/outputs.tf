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

output "network_name" {
  description = "Name of the dedicated dev VPC."
  value       = google_compute_network.dev.name
}

output "network_self_link" {
  description = "Self link of the dedicated dev VPC."
  value       = google_compute_network.dev.self_link
}

output "subnet_name" {
  description = "Name of the dev subnet."
  value       = google_compute_subnetwork.dev.name
}

output "subnet_region" {
  description = "Region of the dev subnet."
  value       = google_compute_subnetwork.dev.region
}

output "artifact_bucket_name" {
  description = "Name of the dev artifact bucket."
  value       = google_storage_bucket.artifacts.name
}

output "control_plane_service_account_email" {
  description = "Email of the dev control-plane service account."
  value       = google_service_account.control_plane.email
}
