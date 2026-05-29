# GKE Cluster Module Stub

This module is a deliberate placeholder for the later GKE implementation. It defines the input and output boundary expected by the `dev` Terraform root without creating a cluster.

The current project runtime remains remote Minikube. GKE provisioning is deferred so cluster IAM, Workload Identity, node pools, private endpoint decisions, and Kubernetes RBAC can be designed explicitly.

## Current Behavior

- Creates no resources.
- Validates as part of the dev Terraform root.
- Fails if `enabled = true`.
- Outputs the planned cluster name and a `deferred` implementation status.

## TODO

Replace the stub with a real `google_container_cluster` module when the GKE phase starts. At that point the module should add:

- private or explicitly justified public control-plane endpoint settings
- VPC-native pod and service ranges
- least-privilege node or workload identities
- a minimal node pool or autopilot choice
- release channel and version strategy
- Kubernetes RBAC and Workload Identity integration outside broad project IAM
