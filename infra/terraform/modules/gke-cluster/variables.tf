variable "enabled" {
  description = "Reserved switch for the future GKE implementation. The stub must remain disabled."
  type        = bool
  default     = false

  validation {
    condition     = var.enabled == false
    error_message = "The GKE module is currently a non-provisioning stub. Keep enabled=false until the real module is implemented."
  }
}

variable "project_id" {
  description = "GCP project ID for the future GKE cluster."
  type        = string
}

variable "name" {
  description = "Planned GKE cluster name."
  type        = string
}

variable "region" {
  description = "Planned GKE cluster region."
  type        = string
}

variable "network_self_link" {
  description = "Self link of the VPC planned for the future GKE cluster."
  type        = string
}

variable "subnet_self_link" {
  description = "Self link of the subnet planned for the future GKE cluster."
  type        = string
}

variable "pods_secondary_range_name" {
  description = "Secondary range name reserved for future GKE pod IPs."
  type        = string
}

variable "services_secondary_range_name" {
  description = "Secondary range name reserved for future GKE service IPs."
  type        = string
}
