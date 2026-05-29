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

variable "gke_cluster_stub_enabled" {
  description = "Reserved switch for the deferred GKE module. Must remain false until the real cluster module is implemented."
  type        = bool
  default     = false
}
