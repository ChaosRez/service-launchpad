module "gke_cluster" {
  source = "../../modules/gke-cluster"

  enabled = var.gke_cluster_enabled

  project_id                    = var.project_id
  name                          = coalesce(var.gke_cluster_name, "${local.resource_name_prefix}-gke")
  description                   = "Production GKE cluster for Service Launchpad workloads and self-hosted LGTM observability."
  region                        = var.region
  network_self_link             = google_compute_network.prod.self_link
  subnet_self_link              = google_compute_subnetwork.prod.self_link
  pods_secondary_range_name     = "pods"
  services_secondary_range_name = "services"
  labels                        = local.common_labels

  deletion_protection        = var.gke_deletion_protection
  enable_private_nodes       = var.gke_enable_private_nodes
  enable_private_endpoint    = var.gke_enable_private_endpoint
  enable_dns_endpoint        = var.gke_enable_dns_endpoint
  master_ipv4_cidr_block     = var.gke_master_ipv4_cidr_block
  master_authorized_networks = var.gke_master_authorized_networks
  release_channel            = var.gke_release_channel

  node_pool_name             = "primary"
  node_locations             = var.gke_node_locations
  node_count                 = var.gke_node_count
  node_machine_type          = var.gke_node_machine_type
  node_disk_size_gb          = var.gke_node_disk_size_gb
  node_disk_type             = var.gke_node_disk_type
  node_image_type            = var.gke_node_image_type
  node_service_account_email = google_service_account.gke_node.email

  depends_on = [
    google_project_service.core,
    google_project_iam_member.gke_node_default_service_account,
    google_artifact_registry_repository_iam_member.gke_node_artifact_reader,
  ]
}
