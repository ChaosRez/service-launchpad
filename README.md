# Service Launchpad

Service Launchpad is a small platform-engineering project that demonstrates how an external control plane can register, deploy, scale, and observe services on Kubernetes.

The current implementation focuses on a realistic local `Minikube` platform loop, then extends the design toward a production `GKE` path where the same Go control plane runs externally on Cloud Run.

- `Go` control plane for service registration, validation, manifest rendering, and deployment workflows
- shared `client-go` deployer path for Kubernetes resources, with `kubectl` kept as a local fallback/debug option
- `FastAPI` workload that simulates a `llama.cpp` / OpenAI-style chat completion API
- `Minikube` as the default validated Kubernetes path
- `GKE` production path for workloads, with the control plane running externally on Cloud Run
- `Terraform` foundation for `GCP`, custom networking, IAM, Artifact Registry, Cloud Run, and a future `GKE` module boundary
- `VictoriaMetrics`, `Grafana Mimir`, `Grafana Tempo`, `Grafana Loki`, and `Grafana` for metrics, traces, logs, SLOs, and dashboards
- in-cluster `k6` load testing to show workload behavior and autoscaling signals

More detail is available in [docs](docs/).

## Architecture Overview

The project is split into a few focused areas:

- `services/control-plane`: Go API for managing service definitions and deployment flow
- `services/fastapi-service`: `llama.cpp`-style chat completion simulator used for deployment, monitoring, and scaling demos
- `k8s/base`: base Kubernetes manifests for workloads and platform components
- `k8s/monitoring`: local observability stack manifests and configuration
- `loadtests/k6`: reusable `k6` scripts for in-cluster load generation
- `infra/terraform`: Terraform modules and environment-specific configuration
- `docs`: diagrams, decisions, and walkthrough notes
- `scripts`: local helper scripts for bootstrapping and demos

At a high level, an operator registers a service definition through the control-plane API, the control plane renders Kubernetes resources, and the deployer applies those resources to the target cluster. The workload, control plane, load generator, Kubernetes state, metrics, logs, and traces are then visible through the local Grafana stack.

## Local Quickstart

The local workflow is the primary validated path:

1. Start `Minikube` with `./scripts/bootstrap-minikube.sh --docker-env`
2. Build and run the `FastAPI` chat completion simulator
3. Deploy manifests from `k8s/base`
4. Add monitoring from `k8s/monitoring`
5. Exercise scaling with the in-cluster `k6` load-test script

## GKE Production Path

After the local `Minikube` workflow is stable, the production path moves workloads to `GKE` using the Terraform configuration under `infra/terraform`. The control plane stays outside the cluster: locally it runs on the developer machine, and in production it runs on Cloud Run.

The intended environment progression is:

- local `Minikube` for development and validation of the shared `client-go` deployer
- `production` for the required Cloud Run + `GKE` path
- `staging` as a future TODO
The current cloud foundation includes the external Cloud Run control-plane shape, authenticated Cloud Run invocation model, custom VPC, service account, Artifact Registry repository, GCS artifact bucket, and a deliberately disabled `GKE` module stub. Actual production `GKE` provisioning and end-to-end Cloud Run-to-GKE validation are the next major runtime milestones.

## Tech Stack

- `Go`
- `FastAPI`
- `Kubernetes`
- `Minikube`
- `Terraform`
- `GCP` (`GKE`, `Cloud Run`)
- `IAM`
- `VictoriaMetrics` (and `vmagent`)
- `Grafana Mimir`
- `Grafana`
- `Grafana Tempo`
- `Grafana Loki`
- `k6`
- `GitHub Actions`

## Repository Status

Implemented so far:

- local `FastAPI` simulator with health, readiness, OpenAI-style chat completion endpoint, metrics, and traces
- Kubernetes workload manifests for namespace, config, deployment, service, and HPA
- local observability stack with VictoriaMetrics, Mimir, vmagent, kube-state-metrics, Tempo, Loki, Promtail, and Grafana dashboards
- in-cluster `k6` load-test workflow
- Go control-plane API for service registration, service inspection, manifest preview, and deployment
- control-plane metrics, health, and readiness endpoints
- shared deployer abstraction with `client-go`, `kubectl`, and disabled modes
- Terraform `dev` foundation for custom VPC, IAM, GCS, Artifact Registry, Cloud Run, and a disabled `GKE` module boundary
- authenticated Cloud Run API access model for production clients
- baseline GitHub Actions CI for the repository

Planned next milestones:

1. Preserve and document production control-plane observability from Cloud Run
2. Connect the control plane to a small cloud resource path such as GCS audit/config artifacts
3. Validate the `client-go` deployer against Minikube
4. Provision and deploy to `GKE production`
5. Publish production images through Artifact Registry
6. Validate service registration and deployment through the Cloud Run control plane
7. Write observability cost-analysis and telemetry strategy documentation
8. Add more demo material: API flow, Kubernetes resource views, load-test runbook, and autoscaling screenshots

## Monitoring Stack

The first observability-stack increment lives in `k8s/monitoring` and deploys:

- `VictoriaMetrics`
- `Grafana Mimir`
- `vmagent`
- `kube-state-metrics`
- `Grafana Tempo`
- `Grafana Loki`
- `Promtail`
- `Grafana` with datasources and dashboards pre-provisioned

Deploy it with:

```bash
kubectl apply -k k8s/monitoring
kubectl rollout status deployment/victoriametrics -n service-launchpad-observability
kubectl rollout status deployment/mimir -n service-launchpad-observability
kubectl rollout status deployment/vmagent -n service-launchpad-observability
kubectl rollout status deployment/kube-state-metrics -n service-launchpad-observability
kubectl rollout status deployment/tempo -n service-launchpad-observability
kubectl rollout status deployment/loki -n service-launchpad-observability
kubectl rollout status deployment/grafana -n service-launchpad-observability
```

## Load Testing

Task 11 uses `k6` as the load generator. The supported path is:

```bash
./scripts/load-test-fastapi-service.sh --profile long --rate 35 --duration 9999m
```

That script:

- runs `k6` as a Kubernetes `Job`
- targets the in-cluster `fastapi-service` DNS name directly
- pushes `k6_*` metrics into `vmagent`, which replicates them to both `VictoriaMetrics` and `Mimir`
- lets you watch both the service dashboard and the generator dashboard in Grafana during the same run

## Screenshots

### Control-plane service

![Grafana dashboard for the Service Launchpad control plane](docs/Control%20Plane%20-%20Service%20Launchpad.png)

The control-plane dashboard shows scrape health, managed service count, service registration rate, deployment attempt rate, and deployment duration. This is the platform layer of the project: the API that accepts service definitions and applies Kubernetes resources.

### FastAPI service

![Grafana dashboard for the Service Launchpad FastAPI service](docs/FastAPI%20Service%20-%20Service%20Launchpad.png)

The FastAPI dashboard shows workload availability, latency SLI, P95 latency, request rate, error rate, and replica behavior. This is the deployed service side of the project: an observable inference-style workload running on Kubernetes and exercised under load.

