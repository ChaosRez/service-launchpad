module "gke_cluster" {
  source = "../../modules/gke-cluster"

  enabled = var.gke_cluster_stub_enabled

  project_id                    = var.project_id
  name                          = "${local.resource_name_prefix}-gke"
  region                        = var.region
  network_self_link             = google_compute_network.dev.self_link
  subnet_self_link              = google_compute_subnetwork.dev.self_link
  pods_secondary_range_name     = "pods"
  services_secondary_range_name = "services"
  node_service_account_email    = google_service_account.control_plane.email
}
