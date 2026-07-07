# CI Pipeline

The baseline GitHub Actions workflow lives in `.github/workflows/ci.yml`. It runs on pull requests and pushes to `main`.

The goal is quick, secret-free validation of the current repository surfaces: Go, Python, Docker builds, Terraform shape, and Kubernetes manifests.

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
- Kustomize rendering for the production workload overlay at `k8s/overlays/prod`
- Kustomize rendering for the production observability overlay at `k8s/overlays/prod-observability`
- Offline Kubernetes schema validation for rendered manifests with `kubeconform`

## Deferred

- No `terraform apply`
- No authenticated GCP deployment
- No Artifact Registry push from CI; production image publishing is handled by `scripts/publish-production-images.sh`
- No Cloud Run deployment
- No `GKE` deployment
- No remote Minikube access from CI
- No Kubernetes server-side dry-run because CI does not create a cluster
- No `k6` load test execution


## Future Additions

- Add authenticated `terraform plan` against a dedicated GCP project.
- Add authenticated GitHub Actions publishing to Artifact Registry after Workload Identity Federation or scoped deploy credentials are configured.
- Add a small Kubernetes smoke test with `kind`.
- Add explicit Cloud Run / `GKE` validation once the production path is implemented.
