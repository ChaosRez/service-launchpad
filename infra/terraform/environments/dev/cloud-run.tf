resource "google_cloud_run_v2_service" "control_plane" {
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
      egress = var.control_plane_vpc_egress

      network_interfaces {
        network    = google_compute_network.dev.id
        subnetwork = google_compute_subnetwork.dev.id
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
        value = var.control_plane_kube_api_server
      }

      env {
        name  = "CONTROL_PLANE_KUBE_CA_DATA"
        value = var.control_plane_kube_ca_data
      }

      startup_probe {
        initial_delay_seconds = 0
        period_seconds        = 10
        timeout_seconds       = 2
        failure_threshold     = 6

        http_get {
          path = "/ready"
        }
      }

      liveness_probe {
        period_seconds    = 30
        timeout_seconds   = 2
        failure_threshold = 3

        http_get {
          path = "/health"
        }
      }

      resources {
        limits = {
          cpu    = "1000m"
          memory = "512Mi"
        }
      }
    }
  }

  depends_on = [
    google_project_service.core,
  ]
}
