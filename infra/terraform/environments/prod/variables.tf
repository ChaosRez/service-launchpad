variable "project_id" {
  description = "GCP project ID where production resources will be created."
  type        = string

  validation {
    condition     = length(trimspace(var.project_id)) > 0
    error_message = "project_id must not be empty."
  }
}

variable "region" {
  description = "Primary GCP region for production resources."
  type        = string
  default     = "europe-west10"
}

variable "zone" {
  description = "Primary GCP zone for zonal production resources."
  type        = string
  default     = "europe-west10-a"
}

variable "name_prefix" {
  description = "Short prefix used when naming production cloud resources."
  type        = string
  default     = "service-launchpad"

  validation {
    condition     = can(regex("^[a-z][a-z0-9-]{1,30}[a-z0-9]$", var.name_prefix))
    error_message = "name_prefix must be 3-32 characters, start with a lowercase letter, and contain only lowercase letters, numbers, and hyphens."
  }
}

variable "labels" {
  description = "Additional labels to apply to supported production resources."
  type        = map(string)
  default     = {}
}

variable "enable_project_services" {
  description = "Whether Terraform should enable the core Google APIs needed by this production environment."
  type        = bool
  default     = true
}

variable "subnet_ip_cidr_range" {
  description = "Primary CIDR range for the production subnet."
  type        = string
  default     = "10.30.0.0/24"

  validation {
    condition     = can(cidrnetmask(var.subnet_ip_cidr_range))
    error_message = "subnet_ip_cidr_range must be a valid CIDR block."
  }
}

variable "secondary_ip_ranges" {
  description = "Secondary subnet ranges for VPC-native GKE pods and services."
  type = list(object({
    range_name    = string
    ip_cidr_range = string
  }))
  default = [
    {
      range_name    = "pods"
      ip_cidr_range = "10.40.0.0/20"
    },
    {
      range_name    = "services"
      ip_cidr_range = "10.41.0.0/24"
    },
  ]

  validation {
    condition     = alltrue([for secondary_range in var.secondary_ip_ranges : can(cidrnetmask(secondary_range.ip_cidr_range))])
    error_message = "Each secondary_ip_ranges entry must use a valid CIDR block."
  }
}

variable "artifact_registry_repository_id" {
  description = "Artifact Registry repository ID for production Service Launchpad images."
  type        = string
  default     = "service-launchpad"

  validation {
    condition     = can(regex("^[a-z][a-z0-9-]{1,61}[a-z0-9]$", var.artifact_registry_repository_id))
    error_message = "artifact_registry_repository_id must be 3-63 characters, start with a lowercase letter, and contain only lowercase letters, numbers, and hyphens."
  }
}

variable "production_image_tag" {
  description = "Container image tag used by production runtime configuration outputs. The publish script also pushes this tag when ALSO_TAGS includes it."
  type        = string
  default     = "prod"

  validation {
    condition     = can(regex("^[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}$", var.production_image_tag))
    error_message = "production_image_tag must be a valid Docker tag."
  }
}

variable "artifact_bucket_name" {
  description = "Optional globally unique name for the production artifact bucket. Defaults to <project_id>-<name_prefix>-prod-artifacts."
  type        = string
  default     = null

  validation {
    condition = (
      var.artifact_bucket_name == null ||
      can(regex("^[a-z0-9][a-z0-9._-]{1,61}[a-z0-9]$", var.artifact_bucket_name))
    )
    error_message = "artifact_bucket_name must be 3-63 characters and use only lowercase letters, numbers, dots, underscores, and hyphens."
  }
}

variable "artifact_bucket_location" {
  description = "GCS location for the production artifact bucket."
  type        = string
  default     = "EU"
}

variable "artifact_bucket_storage_class" {
  description = "Storage class for the production artifact bucket."
  type        = string
  default     = "STANDARD"
}

variable "artifact_bucket_force_destroy" {
  description = "Whether Terraform may delete the production artifact bucket even when it contains objects."
  type        = bool
  default     = false
}

variable "artifact_bucket_noncurrent_retention_days" {
  description = "Age in days after which old object versions are deleted from the production artifact bucket."
  type        = number
  default     = 90

  validation {
    condition     = var.artifact_bucket_noncurrent_retention_days >= 1
    error_message = "artifact_bucket_noncurrent_retention_days must be at least 1."
  }
}

variable "control_plane_service_account_id" {
  description = "Account ID for the production Cloud Run control-plane service account."
  type        = string
  default     = "slp-prod-control-plane"

  validation {
    condition     = can(regex("^[a-z][a-z0-9-]{4,28}[a-z0-9]$", var.control_plane_service_account_id))
    error_message = "control_plane_service_account_id must be 6-30 characters, start with a lowercase letter, and contain only lowercase letters, numbers, and hyphens."
  }
}

variable "gke_node_service_account_id" {
  description = "Account ID for the production GKE node service account."
  type        = string
  default     = "slp-prod-gke-node"

  validation {
    condition     = can(regex("^[a-z][a-z0-9-]{4,28}[a-z0-9]$", var.gke_node_service_account_id))
    error_message = "gke_node_service_account_id must be 6-30 characters, start with a lowercase letter, and contain only lowercase letters, numbers, and hyphens."
  }
}

variable "gke_cluster_enabled" {
  description = "Whether to provision the production GKE cluster."
  type        = bool
  default     = true
}

variable "gke_cluster_name" {
  description = "Optional production GKE cluster name. Defaults to <name_prefix>-prod-gke."
  type        = string
  default     = null
}

variable "gke_deletion_protection" {
  description = "Whether deletion protection is enabled for the production GKE cluster."
  type        = bool
  default     = false
}

variable "gke_enable_private_nodes" {
  description = "Whether production GKE nodes should use private IP addresses only."
  type        = bool
  default     = true
}

variable "gke_enable_private_endpoint" {
  description = "Whether the production GKE Kubernetes API endpoint should be private only."
  type        = bool
  default     = true
}

variable "gke_master_ipv4_cidr_block" {
  description = "RFC1918 /28 CIDR block for the private GKE control-plane endpoint."
  type        = string
  default     = "172.16.0.0/28"

  validation {
    condition     = can(cidrnetmask(var.gke_master_ipv4_cidr_block))
    error_message = "gke_master_ipv4_cidr_block must be a valid CIDR block."
  }
}

variable "gke_master_authorized_networks" {
  description = "CIDR blocks allowed to reach the public GKE API endpoint when public endpoint access is enabled."
  type = list(object({
    cidr_block   = string
    display_name = string
  }))
  default = []

  validation {
    condition     = alltrue([for network in var.gke_master_authorized_networks : can(cidrnetmask(network.cidr_block))])
    error_message = "Each gke_master_authorized_networks cidr_block must be a valid CIDR block."
  }
}

check "public_gke_endpoint_has_authorized_networks" {
  assert {
    condition     = var.gke_enable_private_endpoint || length(var.gke_master_authorized_networks) > 0
    error_message = "Set gke_master_authorized_networks when gke_enable_private_endpoint is false."
  }
}

variable "gke_release_channel" {
  description = "GKE release channel."
  type        = string
  default     = "REGULAR"
}

variable "gke_node_locations" {
  description = "Zones where the production node pool should run. Keep small for the demo production path."
  type        = list(string)
  default     = ["europe-west10-a"]
}

variable "gke_node_count" {
  description = "Node count per selected node location."
  type        = number
  default     = 1
}

variable "gke_node_machine_type" {
  description = "Machine type for the production node pool."
  type        = string
  default     = "e2-standard-2"
}

variable "gke_node_disk_size_gb" {
  description = "Boot disk size for production node pool nodes."
  type        = number
  default     = 50
}

variable "gke_node_disk_type" {
  description = "Boot disk type for production node pool nodes."
  type        = string
  default     = "pd-balanced"
}

variable "gke_node_image_type" {
  description = "Node image type for production node pool nodes."
  type        = string
  default     = "COS_CONTAINERD"
}
