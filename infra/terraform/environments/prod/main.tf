locals {
  environment           = "prod"
  resource_name_prefix  = "${var.name_prefix}-${local.environment}"
  artifact_bucket_name  = coalesce(var.artifact_bucket_name, "${var.project_id}-${local.resource_name_prefix}-artifacts")
  artifact_image_prefix = "${var.region}-docker.pkg.dev/${var.project_id}/${var.artifact_registry_repository_id}"
  control_plane_image   = "${local.artifact_image_prefix}/control-plane:${var.production_image_tag}"
  fastapi_service_image = "${local.artifact_image_prefix}/fastapi-service:${var.production_image_tag}"

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
    "container.googleapis.com",
    "iam.googleapis.com",
    "run.googleapis.com",
    "storage.googleapis.com",
  ]) : toset([])
}
