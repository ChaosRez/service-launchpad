# Service Launchpad Production Terraform Environment

This directory is the required Terraform environment for the production Cloud Run + GKE path.

Do not copy the `dev` root blindly and apply it as production. Production needs explicit decisions for GKE sizing, endpoint exposure, Cloud Run egress, API access, observability access, and rollout safety.

## Current Status

Status: GKE production foundation added for Task 29. Network and access requirements are documented in [../../../../docs/production-network-access.md](../../../../docs/production-network-access.md).

This directory is now the production Terraform root for the GKE foundation. It provisions:

- required project services
- production VPC, subnet, secondary ranges, and internal firewall rule
- Artifact Registry repository
- GCS artifact bucket
- production Cloud Run control-plane service account
- production GKE node service account
- bucket-level control-plane audit/config IAM
- repository-level GKE node image-pull IAM
- standard GKE cluster and primary node pool through `../../modules/gke-cluster`

TODO:

- publish production images
- deploy production observability and workload stack to GKE
- deploy the production control plane to Cloud Run
- route Cloud Run deployment events into production Loki
- validate production registration and deployment

## Production Defaults

Production should differ from local Minikube and the current dev cloud foundation in these ways:

- workloads run on `GKE production`
- the control plane runs on Cloud Run
- images come from Artifact Registry
- operators call the Cloud Run API with IAM identity tokens
- Cloud Run uses a dedicated production service account
- Kubernetes RBAC grants only the managed namespace permissions required by the deployer
- observability runs as a self-hosted LGTM stack in GKE production
- Cloud Run logs are captured by Cloud Logging and later routed to Loki through Pub/Sub and a GKE collector
- deployment audit artifacts are written to a production GCS artifact bucket

The first GKE configuration is cost-aware:

- one selected node location by default
- one node per selected node location
- `e2-standard-2` node type by default
- private nodes enabled by default
- private Kubernetes API endpoint enabled by default
- public Kubernetes API endpoint allowed only with explicit authorized networks as a demo fallback

Set `gke_enable_private_endpoint = false` only when the public endpoint fallback is intentional and `gke_master_authorized_networks` is populated.

## Expected Inputs

The first production Terraform implementation should make these inputs explicit:

- `project_id`
- `region`
- `zone`
- production VPC and subnet CIDR ranges
- GKE pod and service secondary ranges
- GKE cluster name and node pool sizing
- GKE endpoint exposure settings
- GKE master authorized networks when public endpoint access is enabled
- artifact bucket name or naming convention

Use [terraform.tfvars.example](terraform.tfvars.example) as the initial operator-facing shape. Cloud Run service deployment is still deferred to Task 32.
