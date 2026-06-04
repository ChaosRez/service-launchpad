# Service Launchpad Staging Terraform Environment

Staging is a future TODO, not part of the current required project path.

This directory exists to make the environment boundary explicit and to avoid overloading production with staging flags later.
When staging is added, it should have separate boundaries for:

- Terraform state
- Cloud Run service
- Cloud Run service account
- GKE cluster or namespace boundary
- artifact bucket or bucket prefix
- Artifact Registry image tags
- Cloud Run invoker IAM members
- Kubernetes RBAC bindings
- observability access

The intended order is:

1. Validate the shared `client-go` deployer against Minikube.
2. Provision and validate `GKE production`.
3. Deploy the production control plane to Cloud Run.
4. Validate production service registration and deployment.
5. Add staging only when promotion and pre-production rollout validation are useful.
