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
