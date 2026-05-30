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

output "gke_cluster_module_status" {
  description = "Status of the deferred GKE module stub."
  value       = module.gke_cluster.implementation_status
}

output "gke_cluster_planned_name" {
  description = "Planned name for the future dev GKE cluster."
  value       = module.gke_cluster.cluster_name
}

output "control_plane_cloud_run_service_name" {
  description = "Name of the Cloud Run control-plane service."
  value       = google_cloud_run_v2_service.control_plane.name
}

output "control_plane_cloud_run_uri" {
  description = "URI of the Cloud Run control-plane service."
  value       = google_cloud_run_v2_service.control_plane.uri
}

output "control_plane_container_image" {
  description = "Container image configured for the Cloud Run control-plane service."
  value       = local.control_plane_image
}

output "artifact_registry_repository" {
  description = "Artifact Registry repository for Service Launchpad container images."
  value       = google_artifact_registry_repository.service_launchpad.name
}
