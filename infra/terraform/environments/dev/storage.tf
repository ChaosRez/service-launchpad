resource "google_storage_bucket" "artifacts" {
  name                        = local.artifact_bucket_name
  project                     = var.project_id
  location                    = var.artifact_bucket_location
  storage_class               = var.artifact_bucket_storage_class
  uniform_bucket_level_access = true
  public_access_prevention    = "enforced"
  force_destroy               = var.artifact_bucket_force_destroy
  labels                      = local.common_labels

  versioning {
    enabled = true
  }

  lifecycle_rule {
    condition {
      age        = var.artifact_bucket_noncurrent_retention_days
      with_state = "ARCHIVED"
    }

    action {
      type = "Delete"
    }
  }

  depends_on = [
    google_project_service.core,
  ]
}
