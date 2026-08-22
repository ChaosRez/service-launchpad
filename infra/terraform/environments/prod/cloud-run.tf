resource "google_cloud_run_v2_service" "control_plane" {
  count = var.control_plane_enabled ? 1 : 0

  name                = "${local.resource_name_prefix}-control-plane"
  location            = var.region
  project             = var.project_id
  ingress             = var.control_plane_ingress
  deletion_protection = var.control_plane_deletion_protection
  labels              = local.common_labels

  template {
    service_account = google_service_account.control_plane.email

    scaling {
      min_instance_count = var.control_plane_min_instances
      max_instance_count = var.control_plane_max_instances
    }

    vpc_access {
      egress = "PRIVATE_RANGES_ONLY"

      network_interfaces {
        network    = google_compute_network.prod.id
        subnetwork = google_compute_subnetwork.prod.id
      }
    }

    containers {
      image = local.control_plane_image

      ports {
        name           = "http1"
        container_port = 8080
      }

      env {
        name  = "CONTROL_PLANE_LISTEN_ADDR"
        value = ":8080"
      }
      env {
        name  = "CONTROL_PLANE_DEPLOYER_MODE"
        value = "client-go"
      }
      env {
        name  = "CONTROL_PLANE_TARGET_NAMESPACE"
        value = var.control_plane_target_namespace
      }
      env {
        name  = "CONTROL_PLANE_KUBE_API_SERVER"
        value = "https://${module.gke_cluster.endpoint}"
      }
      env {
        name  = "CONTROL_PLANE_KUBE_CA_DATA"
        value = module.gke_cluster.cluster_ca_certificate
      }
      env {
        name  = "CONTROL_PLANE_KUBE_USE_ADC"
        value = "true"
      }
      env {
        name  = "CONTROL_PLANE_ALLOWED_IMAGE_PREFIXES"
        value = "${local.artifact_image_prefix}/"
      }
      env {
        name  = "CONTROL_PLANE_AUDIT_BUCKET"
        value = google_storage_bucket.artifacts.name
      }
      env {
        name  = "CONTROL_PLANE_AUDIT_PREFIX"
        value = "control-plane/deployments"
      }

      startup_probe {
        period_seconds    = 10
        timeout_seconds   = 2
        failure_threshold = 12
        http_get { path = "/ready" }
      }
      liveness_probe {
        period_seconds    = 30
        timeout_seconds   = 2
        failure_threshold = 3
        http_get { path = "/health" }
      }
      resources {
        limits = {
          cpu    = "1000m"
          memory = "512Mi"
        }
      }
    }
  }

  depends_on = [google_project_service.core]
}

resource "google_cloud_run_v2_service_iam_member" "control_plane_invoker" {
  for_each = var.control_plane_enabled ? toset(var.control_plane_invoker_members) : toset([])
  project  = var.project_id
  location = google_cloud_run_v2_service.control_plane[0].location
  name     = google_cloud_run_v2_service.control_plane[0].name
  role     = "roles/run.invoker"
  member   = each.value
}
