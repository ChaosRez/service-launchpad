variable "enabled" {
  description = "Whether the module should provision a GKE cluster and node pool."
  type        = bool
  default     = false
}

variable "project_id" {
  description = "GCP project ID for the GKE cluster."
  type        = string
}

variable "name" {
  description = "GKE cluster name."
  type        = string
}

variable "description" {
  description = "GKE cluster description."
  type        = string
  default     = "Service Launchpad GKE cluster."
}

variable "region" {
  description = "Regional location for the GKE cluster."
  type        = string
}

variable "network_self_link" {
  description = "Self link of the VPC for the GKE cluster."
  type        = string
}

variable "subnet_self_link" {
  description = "Self link of the subnet for the GKE cluster."
  type        = string
}

variable "pods_secondary_range_name" {
  description = "Secondary range name used for GKE pod IPs."
  type        = string
}

variable "services_secondary_range_name" {
  description = "Secondary range name used for GKE service IPs."
  type        = string
}

variable "labels" {
  description = "Labels to apply to supported GKE resources."
  type        = map(string)
  default     = {}
}

variable "deletion_protection" {
  description = "Whether deletion protection is enabled for the GKE cluster."
  type        = bool
  default     = false
}

variable "enable_private_nodes" {
  description = "Whether GKE nodes should use private IP addresses only."
  type        = bool
  default     = true
}

variable "enable_private_endpoint" {
  description = "Whether the GKE Kubernetes API endpoint should be private only."
  type        = bool
  default     = false
}

variable "master_ipv4_cidr_block" {
  description = "RFC1918 /28 CIDR block for the private GKE control-plane endpoint."
  type        = string
  default     = "172.16.0.0/28"

  validation {
    condition     = can(cidrnetmask(var.master_ipv4_cidr_block))
    error_message = "master_ipv4_cidr_block must be a valid CIDR block."
  }
}

variable "master_authorized_networks" {
  description = "CIDR blocks allowed to reach the public GKE API endpoint when public endpoint access is enabled."
  type = list(object({
    cidr_block   = string
    display_name = string
  }))
  default = []

  validation {
    condition     = alltrue([for network in var.master_authorized_networks : can(cidrnetmask(network.cidr_block))])
    error_message = "Each master_authorized_networks cidr_block must be a valid CIDR block."
  }
}

variable "release_channel" {
  description = "GKE release channel."
  type        = string
  default     = "REGULAR"

  validation {
    condition     = contains(["RAPID", "REGULAR", "STABLE"], var.release_channel)
    error_message = "release_channel must be RAPID, REGULAR, or STABLE."
  }
}

variable "node_pool_name" {
  description = "Name of the primary GKE node pool."
  type        = string
  default     = "primary"
}

variable "node_locations" {
  description = "Zones where the node pool should run. Keep small for the demo production path."
  type        = list(string)
  default     = []
}

variable "node_count" {
  description = "Node count per selected node location."
  type        = number
  default     = 1

  validation {
    condition     = var.node_count >= 1
    error_message = "node_count must be at least 1."
  }
}

variable "node_machine_type" {
  description = "Machine type for the primary node pool."
  type        = string
  default     = "e2-standard-2"
}

variable "node_disk_size_gb" {
  description = "Boot disk size for primary node pool nodes."
  type        = number
  default     = 50

  validation {
    condition     = var.node_disk_size_gb >= 30
    error_message = "node_disk_size_gb must be at least 30."
  }
}

variable "node_disk_type" {
  description = "Boot disk type for primary node pool nodes."
  type        = string
  default     = "pd-balanced"
}

variable "node_image_type" {
  description = "Node image type."
  type        = string
  default     = "COS_CONTAINERD"
}

variable "node_service_account_email" {
  description = "Service account email used by GKE nodes."
  type        = string
}
