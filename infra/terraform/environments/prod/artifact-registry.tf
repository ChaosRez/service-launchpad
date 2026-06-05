resource "google_artifact_registry_repository" "service_launchpad" {
  project       = var.project_id
  location      = var.region
  repository_id = var.artifact_registry_repository_id
  description   = "Production Docker images for Service Launchpad services."
  format        = "DOCKER"
  labels        = local.common_labels

  depends_on = [
    google_project_service.core,
  ]
}
