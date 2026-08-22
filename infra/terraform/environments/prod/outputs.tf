output "environment" {
  description = "Terraform environment represented by this root module."
  value       = local.environment
}

output "project_id" {
  description = "GCP project ID configured for production."
  value       = var.project_id
}

output "region" {
  description = "Primary GCP region configured for production resources."
  value       = var.region
}

output "resource_name_prefix" {
  description = "Prefix resources use for consistent production naming."
  value       = local.resource_name_prefix
}

output "network_name" {
  description = "Name of the dedicated production VPC."
  value       = google_compute_network.prod.name
}

output "network_self_link" {
  description = "Self link of the dedicated production VPC."
  value       = google_compute_network.prod.self_link
}

output "subnet_name" {
  description = "Name of the production subnet."
  value       = google_compute_subnetwork.prod.name
}

output "artifact_bucket_name" {
  description = "Name of the production artifact bucket."
  value       = google_storage_bucket.artifacts.name
}

output "artifact_registry_repository" {
  description = "Artifact Registry repository for production Service Launchpad images."
  value       = google_artifact_registry_repository.service_launchpad.name
}

output "artifact_registry_image_prefix" {
  description = "Artifact Registry image prefix for production Service Launchpad images."
  value       = local.artifact_image_prefix
}

output "production_image_tag" {
  description = "Container image tag selected for production runtime configuration."
  value       = var.production_image_tag
}

output "control_plane_container_image" {
  description = "Production control-plane image expected by the Cloud Run deployment task."
  value       = local.control_plane_image
}

output "control_plane_enabled" {
  description = "Whether the production Cloud Run control plane is enabled."
  value       = var.control_plane_enabled
}

output "control_plane_service_name" {
  description = "Production Cloud Run control-plane service name when enabled."
  value       = var.control_plane_enabled ? google_cloud_run_v2_service.control_plane[0].name : null
}

output "control_plane_service_uri" {
  description = "Authenticated production Cloud Run control-plane URI when enabled."
  value       = var.control_plane_enabled ? google_cloud_run_v2_service.control_plane[0].uri : null
}

output "fastapi_service_container_image" {
  description = "Production fastapi-service image expected by the GKE workload deployment task."
  value       = local.fastapi_service_image
}

output "control_plane_service_account_email" {
  description = "Email of the production Cloud Run control-plane service account."
  value       = google_service_account.control_plane.email
}

output "gke_node_service_account_email" {
  description = "Email of the production GKE node service account."
  value       = google_service_account.gke_node.email
}

output "gke_cluster_module_status" {
  description = "Status of the GKE module."
  value       = module.gke_cluster.implementation_status
}

output "gke_cluster_name" {
  description = "Name of the production GKE cluster."
  value       = module.gke_cluster.cluster_name
}

output "gke_cluster_location" {
  description = "Location of the production GKE cluster."
  value       = module.gke_cluster.cluster_location
}

output "gke_cluster_endpoint" {
  description = "GKE Kubernetes API endpoint used by the Cloud Run control-plane deployer."
  value       = module.gke_cluster.endpoint
}

output "gke_cluster_ca_data" {
  description = "Base64-encoded GKE cluster CA data used by the Cloud Run control-plane deployer."
  value       = module.gke_cluster.cluster_ca_certificate
  sensitive   = true
}

output "gke_private_endpoint_enabled" {
  description = "Whether the production GKE private endpoint is enabled."
  value       = module.gke_cluster.private_endpoint_enabled
}

output "gke_private_nodes_enabled" {
  description = "Whether production GKE private nodes are enabled."
  value       = module.gke_cluster.private_nodes_enabled
}

output "gke_dns_endpoint_enabled" {
  description = "Whether IAM-authenticated external access through the production GKE DNS endpoint is enabled."
  value       = module.gke_cluster.dns_endpoint_enabled
}

output "gke_dns_endpoint" {
  description = "Production GKE DNS endpoint used by operators and compatible Kubernetes IDE clients."
  value       = module.gke_cluster.dns_endpoint
}
