resource "google_compute_network" "prod" {
  name                    = "${local.resource_name_prefix}-vpc"
  description             = "Dedicated production VPC for Service Launchpad Cloud Run, GKE, and observability resources."
  auto_create_subnetworks = false
  routing_mode            = "REGIONAL"

  depends_on = [
    google_project_service.core,
  ]
}

resource "google_compute_subnetwork" "prod" {
  name          = "${local.resource_name_prefix}-subnet"
  description   = "Production subnet with secondary ranges for VPC-native GKE pods and services."
  ip_cidr_range = var.subnet_ip_cidr_range
  network       = google_compute_network.prod.id
  region        = var.region

  dynamic "secondary_ip_range" {
    for_each = var.secondary_ip_ranges

    content {
      range_name    = secondary_ip_range.value.range_name
      ip_cidr_range = secondary_ip_range.value.ip_cidr_range
    }
  }
}

resource "google_compute_firewall" "allow_internal" {
  name        = "${local.resource_name_prefix}-allow-internal"
  description = "Allow private traffic within the production VPC primary and reserved GKE secondary ranges."
  network     = google_compute_network.prod.name
  direction   = "INGRESS"

  source_ranges = local.internal_source_ranges

  allow {
    protocol = "icmp"
  }

  allow {
    protocol = "tcp"
    ports    = ["0-65535"]
  }

  allow {
    protocol = "udp"
    ports    = ["0-65535"]
  }
}
