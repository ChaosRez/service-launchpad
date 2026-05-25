locals {
  environment          = "dev"
  resource_name_prefix = "${var.name_prefix}-${local.environment}"

  common_labels = merge(
    {
      application = "service-launchpad"
      environment = local.environment
      managed_by  = "terraform"
    },
    var.labels
  )
}
