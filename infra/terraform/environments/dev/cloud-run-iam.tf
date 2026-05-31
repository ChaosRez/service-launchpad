resource "google_cloud_run_v2_service_iam_member" "control_plane_invoker" {
  for_each = toset(var.control_plane_invoker_members)

  project  = var.project_id
  location = google_cloud_run_v2_service.control_plane.location
  name     = google_cloud_run_v2_service.control_plane.name
  role     = "roles/run.invoker"
  member   = each.value
}
