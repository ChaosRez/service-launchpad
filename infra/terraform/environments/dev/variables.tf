variable "project_id" {
  description = "GCP project ID where dev resources will be created."
  type        = string

  validation {
    condition     = length(trimspace(var.project_id)) > 0
    error_message = "project_id must not be empty."
  }
}

variable "region" {
  description = "Primary GCP region for dev resources."
  type        = string
  default     = "europe-west10"
}

variable "zone" {
  description = "Primary GCP zone for zonal dev resources."
  type        = string
  default     = "europe-west10-a"
}

variable "name_prefix" {
  description = "Short prefix used when naming dev cloud resources."
  type        = string
  default     = "service-launchpad"

  validation {
    condition     = can(regex("^[a-z][a-z0-9-]{1,30}[a-z0-9]$", var.name_prefix))
    error_message = "name_prefix must be 3-32 characters, start with a lowercase letter, and contain only lowercase letters, numbers, and hyphens."
  }
}

variable "labels" {
  description = "Additional labels to apply to supported GCP resources."
  type        = map(string)
  default     = {}
}

variable "enable_project_services" {
  description = "Whether Terraform should enable the core Google APIs needed by this dev environment."
  type        = bool
  default     = true
}

variable "subnet_ip_cidr_range" {
  description = "Primary CIDR range for the dev subnet."
  type        = string
  default     = "10.10.0.0/24"

  validation {
    condition     = can(cidrnetmask(var.subnet_ip_cidr_range))
    error_message = "subnet_ip_cidr_range must be a valid CIDR block."
  }
}

variable "secondary_ip_ranges" {
  description = "Secondary subnet ranges reserved for future VPC-native GKE pods and services."
  type = list(object({
    range_name    = string
    ip_cidr_range = string
  }))
  default = [
    {
      range_name    = "pods"
      ip_cidr_range = "10.20.0.0/20"
    },
    {
      range_name    = "services"
      ip_cidr_range = "10.21.0.0/24"
    },
  ]

  validation {
    condition     = alltrue([for secondary_range in var.secondary_ip_ranges : can(cidrnetmask(secondary_range.ip_cidr_range))])
    error_message = "Each secondary_ip_ranges entry must use a valid CIDR block."
  }
}

variable "control_plane_service_account_id" {
  description = "Account ID for the dev control-plane service account."
  type        = string
  default     = "slp-dev-control-plane"

  validation {
    condition     = can(regex("^[a-z][a-z0-9-]{4,28}[a-z0-9]$", var.control_plane_service_account_id))
    error_message = "control_plane_service_account_id must be 6-30 characters, start with a lowercase letter, and contain only lowercase letters, numbers, and hyphens."
  }
}

variable "artifact_bucket_name" {
  description = "Optional globally unique name for the dev artifact bucket. Defaults to <project_id>-<name_prefix>-dev-artifacts."
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
  description = "GCS location for the dev artifact bucket."
  type        = string
  default     = "EU"
}

variable "artifact_bucket_storage_class" {
  description = "Storage class for the dev artifact bucket."
  type        = string
  default     = "STANDARD"
}

variable "artifact_bucket_force_destroy" {
  description = "Whether Terraform may delete the artifact bucket even when it contains objects. Useful for disposable dev environments."
  type        = bool
  default     = false
}

variable "artifact_bucket_noncurrent_retention_days" {
  description = "Age in days after which old object versions are deleted from the artifact bucket."
  type        = number
  default     = 30

  validation {
    condition     = var.artifact_bucket_noncurrent_retention_days >= 1
    error_message = "artifact_bucket_noncurrent_retention_days must be at least 1."
  }
}

variable "artifact_registry_repository_id" {
  description = "Artifact Registry repository ID for Service Launchpad container images."
  type        = string
  default     = "service-launchpad"

  validation {
    condition     = can(regex("^[a-z][a-z0-9-]{1,61}[a-z0-9]$", var.artifact_registry_repository_id))
    error_message = "artifact_registry_repository_id must be 3-63 characters, start with a lowercase letter, and contain only lowercase letters, numbers, and hyphens."
  }
}

variable "gke_cluster_stub_enabled" {
  description = "Reserved switch for the deferred GKE module. Must remain false until the real cluster module is implemented."
  type        = bool
  default     = false
}

variable "control_plane_container_image" {
  description = "Container image for the Cloud Run control-plane service. Defaults to the expected Artifact Registry image path."
  type        = string
  default     = null
}

variable "control_plane_ingress" {
  description = "Cloud Run ingress policy for the control-plane service."
  type        = string
  default     = "INGRESS_TRAFFIC_ALL"

  validation {
    condition = contains([
      "INGRESS_TRAFFIC_ALL",
      "INGRESS_TRAFFIC_INTERNAL_ONLY",
      "INGRESS_TRAFFIC_INTERNAL_LOAD_BALANCER",
    ], var.control_plane_ingress)
    error_message = "control_plane_ingress must be a valid Cloud Run ingress enum."
  }
}

variable "control_plane_enable_vpc_egress" {
  description = "Whether Cloud Run should use direct VPC egress. Keep false for easy demo teardown; enable when targeting a private GKE API endpoint."
  type        = bool
  default     = false
}

variable "control_plane_vpc_egress" {
  description = "Cloud Run VPC egress policy used when control_plane_enable_vpc_egress is true."
  type        = string
  default     = "PRIVATE_RANGES_ONLY"

  validation {
    condition = contains([
      "PRIVATE_RANGES_ONLY",
      "ALL_TRAFFIC",
    ], var.control_plane_vpc_egress)
    error_message = "control_plane_vpc_egress must be PRIVATE_RANGES_ONLY or ALL_TRAFFIC."
  }
}

variable "control_plane_min_instances" {
  description = "Minimum Cloud Run instances for the control-plane service."
  type        = number
  default     = 0

  validation {
    condition     = var.control_plane_min_instances >= 0
    error_message = "control_plane_min_instances must be greater than or equal to 0."
  }
}

variable "control_plane_max_instances" {
  description = "Maximum Cloud Run instances for the control-plane service."
  type        = number
  default     = 2

  validation {
    condition     = var.control_plane_max_instances >= 1
    error_message = "control_plane_max_instances must be greater than or equal to 1."
  }
}

variable "control_plane_deletion_protection" {
  description = "Whether deletion protection is enabled for the Cloud Run control-plane service."
  type        = bool
  default     = false
}

variable "control_plane_target_namespace" {
  description = "Kubernetes namespace targeted by the Cloud Run control-plane deployer."
  type        = string
  default     = "service-launchpad-prod"
}

variable "control_plane_kube_api_server" {
  description = "GKE Kubernetes API endpoint used by the Cloud Run control-plane deployer."
  type        = string
  default     = ""
}

variable "control_plane_kube_ca_data" {
  description = "Base64-encoded GKE cluster CA data used by the Cloud Run control-plane deployer."
  type        = string
  default     = ""
  sensitive   = true
}

variable "control_plane_invoker_members" {
  description = "IAM members allowed to call the Cloud Run control-plane API. Use user:, group:, or serviceAccount: members; never allUsers."
  type        = list(string)
  default     = []

  validation {
    condition = alltrue([
      for member in var.control_plane_invoker_members :
      !contains(["allUsers", "allAuthenticatedUsers"], member)
    ])
    error_message = "Do not grant public or broad authenticated access to the production control-plane API."
  }
}
