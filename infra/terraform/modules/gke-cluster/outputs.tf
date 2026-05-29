output "cluster_name" {
  description = "Planned GKE cluster name for the future implementation."
  value       = var.name
}

output "implementation_status" {
  description = "Current implementation status for this module."
  value       = local.implementation_status
}
