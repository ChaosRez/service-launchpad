# CI Pipeline

The baseline GitHub Actions workflow lives in `.github/workflows/ci.yml` and runs on "pull requests" and "pushes" to `main`.

## Validated

- Go control-plane formatting with `gofmt`
- Go control-plane tests with `go test ./services/control-plane`
- Control-plane container image build
- FastAPI service dependency installation
- FastAPI service tests with `pytest`
- FastAPI service container image build
- Terraform formatting for `infra/terraform`
- Terraform initialization without a backend
- Terraform validation for `infra/terraform/environments/dev`
- Kustomize rendering for `k8s/base` and `k8s/monitoring`
- Offline Kubernetes schema validation for rendered manifests with `kubeconform`

## Deferred

- No `terraform apply`
- No authenticated GCP deployment
- No GKE deployment
- No remote Minikube access from CI
- No Kubernetes server-side dry-run because CI does not create a cluster

The workflow is intended to catch local code, manifest, container, and Terraform shape regressions without requiring secrets.
