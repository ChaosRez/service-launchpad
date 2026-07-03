# Environment Model

Service Launchpad has three environment concepts:

- local `Minikube` for development and pre-production deployer validation
- `GKE production` for the required cloud runtime
- `GKE staging` as a future TODO

The project does not require a separate `GKE dev` cluster. Local development should stay fast and cheap while still exercising the same `client-go` deployer path that production uses.

## Environment Summary

| Concern | Local Minikube | GKE production | Future GKE staging |
| --- | --- | --- | --- |
| Terraform environment | Not managed by Terraform | `infra/terraform/environments/prod` | `infra/terraform/environments/staging` |
| Control plane runtime | Developer host, usually `127.0.0.1:8080` or `:8080` | Cloud Run | Cloud Run |
| Workload runtime | Minikube | GKE | GKE |
| Deployer path | `client-go` against local kubeconfig; `kubectl` fallback for debug | `client-go` against GKE API | Same as production |
| Images | Minikube-local images are acceptable | Artifact Registry images published by `scripts/publish-production-images.sh` | Artifact Registry images only |
| API access | Local unauthenticated access only | Cloud Run IAM with explicit `roles/run.invoker` members | Same model as production, separate identities |
| Control-plane identity | Local developer identity and kubeconfig | Dedicated Cloud Run service account | Separate Cloud Run service account |
| Kubernetes permissions | Local developer permissions | Minimal RBAC for managed namespace resources | Same minimal RBAC in staging namespace or cluster |
| Observability stack | Self-hosted LGTM in Minikube | Self-hosted LGTM in GKE production | Self-hosted LGTM or shared production-like stack |
| Cloud Run logs | Not applicable | Cloud Logging first; later route to Loki through Pub/Sub and a GKE collector | Same as production |
| Deployment audit artifacts | Optional local override | GCS artifact bucket written by Cloud Run service account | Separate staging bucket |
| Rollout safety | Manual validation is acceptable | Explicit rollout and rollback expectations required before shared use | Pre-production rollout validation |

## Local Minikube

Local Minikube is the primary development loop. It should validate:

- service registration
- manifest rendering
- `client-go` apply behavior
- workload health and readiness
- HPA resource generation
- local LGTM observability dashboards

The control plane stays outside Kubernetes. This keeps the local loop simple and matches the production boundary where Cloud Run also stays outside the workload cluster.

Local Minikube may use:

- local kubeconfig
- Minikube-local Docker images
- unauthenticated `127.0.0.1` control-plane calls
- `kubectl` fallback for debugging

Local Minikube should not define production IAM, production GKE networking, public ingress, or long-lived cloud resources.

## GKE Production

`GKE production` is the required cloud runtime for the project.

The production environment should use:

- Terraform root: `infra/terraform/environments/prod`
- custom VPC and production subnet ranges
- GKE for workloads and the production LGTM stack
- Cloud Run for the external control plane
- Artifact Registry for all production images
- production images published as immutable `git-<short-sha>` tags, with `prod` available only as a demo convenience tag
- explicit Cloud Run invoker IAM
- a dedicated Cloud Run service account
- minimal Kubernetes RBAC for the managed namespace resources
- GCS deployment audit artifacts written by the Cloud Run service account

Production must make these decisions explicit before it is considered complete:

- whether the GKE Kubernetes API endpoint is public or private
- how Cloud Run reaches the GKE API
- whether Cloud Run needs VPC egress
- how operators reach the Cloud Run API
- how Grafana and observability endpoints remain private
- how control-plane deployment events move from Cloud Logging into Loki
- what rollback or cleanup behavior exists after a failed deployment

## Future GKE Staging

`GKE staging` is intentionally not part of the current required path.

When added, staging should be a separate environment, not a set of production flags. It should have:

- separate Terraform root under `infra/terraform/environments/staging`
- separate Cloud Run service
- separate Cloud Run service account
- separate GKE cluster or clearly documented shared-cluster namespace boundary
- separate artifact bucket or prefix
- separate Cloud Run invoker membership
- production-like observability and deployment validation

Staging should exist to test promotion, rollout safety, and production-like IAM/networking before production changes. Until those workflows are implemented, adding staging would increase maintenance without improving the current platform story.
