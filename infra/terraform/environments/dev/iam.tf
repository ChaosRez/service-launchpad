resource "google_service_account" "control_plane" {
  project      = var.project_id
  account_id   = var.control_plane_service_account_id
  display_name = "Service Launchpad dev control plane"
  description  = "Least-privilege identity for Service Launchpad dev control-plane cloud integrations."

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
