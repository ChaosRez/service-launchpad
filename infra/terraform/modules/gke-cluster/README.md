# GKE Cluster Module

This module provisions the minimal standard GKE cluster used by the production Service Launchpad path.

The module is conditional:

- `enabled = false` creates no cluster and keeps the existing dev root non-provisioning.
- `enabled = true` creates a VPC-native GKE cluster and one managed node pool.

## Resources

When enabled, the module creates:

- `google_container_cluster`
- `google_container_node_pool`

The cluster uses:

- VPC-native networking with supplied pod and service secondary ranges
- separate managed node pool instead of the default node pool
- Workload Identity enabled
- HTTP load balancing addon enabled
- HPA addon enabled
- GCE persistent disk CSI driver enabled
- private nodes by default

## Endpoint Model

The production design prefers a private GKE control-plane endpoint when the Cloud Run to GKE path is ready for VPC egress.

The module supports both:

- `enable_private_endpoint = true` for private-only Kubernetes API access
- `enable_private_endpoint = false` for a public endpoint that should be paired with `master_authorized_networks`

Public endpoint access is acceptable only as a deliberate demo tradeoff. It should not be broad or accidental.

## Cost Profile

The default node pool is intentionally small:

- `node_count = 1`
- `node_machine_type = "e2-standard-2"`
- `node_disk_size_gb = 50`

Task 31 may need to resize this when the production LGTM stack is deployed.
