resource "google_service_account" "control_plane" {
  project      = var.project_id
  account_id   = var.control_plane_service_account_id
  display_name = "Service Launchpad production control plane"
  description  = "Least-privilege identity for the production Cloud Run control plane."

  depends_on = [
    google_project_service.core,
  ]
}

resource "google_service_account" "gke_node" {
  project      = var.project_id
  account_id   = var.gke_node_service_account_id
  display_name = "Service Launchpad production GKE nodes"
  description  = "Node identity for the production Service Launchpad GKE cluster."

  depends_on = [
    google_project_service.core,
  ]
}

resource "google_storage_bucket_iam_member" "control_plane_object_creator" {
  bucket = google_storage_bucket.artifacts.name
  role   = "roles/storage.objectCreator"
  member = google_service_account.control_plane.member
}

resource "google_storage_bucket_iam_member" "control_plane_object_viewer" {
  bucket = google_storage_bucket.artifacts.name
  role   = "roles/storage.objectViewer"
  member = google_service_account.control_plane.member
}

resource "google_project_iam_member" "gke_node_default_service_account" {
  project = var.project_id
  role    = "roles/container.defaultNodeServiceAccount"
  member  = google_service_account.gke_node.member
}

resource "google_artifact_registry_repository_iam_member" "gke_node_artifact_reader" {
  project    = var.project_id
  location   = google_artifact_registry_repository.service_launchpad.location
  repository = google_artifact_registry_repository.service_launchpad.repository_id
  role       = "roles/artifactregistry.reader"
  member     = google_service_account.gke_node.member
}
