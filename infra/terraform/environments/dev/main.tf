locals {
  environment          = "dev"
  resource_name_prefix = "${var.name_prefix}-${local.environment}"
  artifact_bucket_name = coalesce(var.artifact_bucket_name, "${var.project_id}-${local.resource_name_prefix}-artifacts")
  control_plane_image  = coalesce(var.control_plane_container_image, "${var.region}-docker.pkg.dev/${var.project_id}/${var.artifact_registry_repository_id}/control-plane:dev")

  common_labels = merge(
    {
      application = "service-launchpad"
      environment = local.environment
      managed_by  = "terraform"
    },
    var.labels
  )

  internal_source_ranges = distinct(concat(
    [var.subnet_ip_cidr_range],
    [for secondary_range in var.secondary_ip_ranges : secondary_range.ip_cidr_range]
  ))

  project_services = var.enable_project_services ? toset([
    "artifactregistry.googleapis.com",
    "compute.googleapis.com",
    "iam.googleapis.com",
    "run.googleapis.com",
    "storage.googleapis.com",
  ]) : toset([])
}
