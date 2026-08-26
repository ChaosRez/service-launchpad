locals {
  implementation_status = var.enabled ? "enabled" : "disabled"
}

resource "google_container_cluster" "this" {
  count = var.enabled ? 1 : 0

  project  = var.project_id
  name     = var.name
  location = var.location

  description = var.description

  network    = var.network_self_link
  subnetwork = var.subnet_self_link

  deletion_protection      = var.deletion_protection
  remove_default_node_pool = true
  initial_node_count       = 1

  networking_mode = "VPC_NATIVE"

  ip_allocation_policy {
    cluster_secondary_range_name  = var.pods_secondary_range_name
    services_secondary_range_name = var.services_secondary_range_name
  }

  private_cluster_config {
    enable_private_nodes    = var.enable_private_nodes
    enable_private_endpoint = var.enable_private_endpoint
    master_ipv4_cidr_block  = var.master_ipv4_cidr_block
  }

  control_plane_endpoints_config {
    dns_endpoint_config {
      allow_external_traffic = var.enable_dns_endpoint
    }
  }

  dynamic "master_authorized_networks_config" {
    for_each = var.enable_private_endpoint || length(var.master_authorized_networks) > 0 ? [1] : []

    content {
      dynamic "cidr_blocks" {
        for_each = var.master_authorized_networks

        content {
          cidr_block   = cidr_blocks.value.cidr_block
          display_name = cidr_blocks.value.display_name
        }
      }
    }
  }

  release_channel {
    channel = var.release_channel
  }

  logging_config {
    enable_components = var.cloud_logging_enabled ? ["SYSTEM_COMPONENTS"] : []
  }

  monitoring_config {
    enable_components = var.cloud_monitoring_enabled ? ["SYSTEM_COMPONENTS"] : []

    managed_prometheus {
      enabled = var.managed_prometheus_enabled
    }
  }

  workload_identity_config {
    workload_pool = "${var.project_id}.svc.id.goog"
  }

  addons_config {
    http_load_balancing {
      disabled = !var.http_load_balancing_enabled
    }

    horizontal_pod_autoscaling {
      disabled = false
    }

    gce_persistent_disk_csi_driver_config {
      enabled = var.gce_persistent_disk_csi_driver_enabled
    }

    dns_cache_config {
      enabled = var.dns_cache_enabled
    }
  }

  resource_labels = var.labels
}

resource "google_container_node_pool" "primary" {
  count = var.enabled ? 1 : 0

  project  = var.project_id
  name     = var.node_pool_name
  location = var.location
  cluster  = google_container_cluster.this[0].name

  node_locations = var.node_locations
  node_count     = var.node_count

  management {
    auto_repair  = true
    auto_upgrade = true
  }

  node_config {
    service_account = var.node_service_account_email
    machine_type    = var.node_machine_type
    disk_size_gb    = var.node_disk_size_gb
    disk_type       = var.node_disk_type
    image_type      = var.node_image_type
    spot            = var.node_spot

    labels = var.labels

    oauth_scopes = [
      "https://www.googleapis.com/auth/cloud-platform",
    ]
  }
}
