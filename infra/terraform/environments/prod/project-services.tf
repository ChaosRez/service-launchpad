resource "google_project_service" "core" {
  for_each = local.project_services

  project = var.project_id
  service = each.value

  disable_on_destroy = false
}
