output "cluster_name" {
  description = "GKE cluster name."
  value       = var.name
}

output "implementation_status" {
  description = "Current implementation status for this module."
  value       = local.implementation_status
}

output "cluster_location" {
  description = "GKE cluster location."
  value       = var.region
}

output "endpoint" {
  description = "GKE Kubernetes API endpoint when the cluster is enabled."
  value       = var.enabled ? google_container_cluster.this[0].endpoint : null
}

output "cluster_ca_certificate" {
  description = "Base64-encoded GKE cluster CA certificate when the cluster is enabled."
  value       = var.enabled ? google_container_cluster.this[0].master_auth[0].cluster_ca_certificate : null
  sensitive   = true
}

output "node_pool_name" {
  description = "Primary node pool name when the cluster is enabled."
  value       = var.enabled ? google_container_node_pool.primary[0].name : null
}

output "private_endpoint_enabled" {
  description = "Whether private-only endpoint access is enabled."
  value       = var.enable_private_endpoint
}

output "private_nodes_enabled" {
  description = "Whether private nodes are enabled."
  value       = var.enable_private_nodes
}
