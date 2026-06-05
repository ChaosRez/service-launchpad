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
