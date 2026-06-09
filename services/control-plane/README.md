# Control Plane

This directory contains `Go` control-plane service for Service Launchpad.

Current scope:

- accepts service definitions over HTTP
- keeps them in memory by default
  - optionally persist service definitions to a JSON file
- exposes read endpoints for registered services

Current validation:

- `name` is required and must use lowercase letters, numbers, or hyphens
- `image` is required
- `port` must be between `1` and `65535`
- `replicas` must be at least `1`
- autoscaling settings are validated when autoscaling is enabled
- deployment policy bounds replicas, autoscaling, image prefixes, and rendered Kubernetes resource intent before apply

Implemented endpoints:

- `GET /health`
- `GET /ready`
- `GET /metrics`
- `POST /services`
- `GET /services`
- `GET /services/{name}`
- `GET /services/{name}/manifests`
- `POST /services/{name}/deploy`

Current service definition shape:

- `name`
- `image`
- `port`
- `replicas`
- `autoscaling`

Example request:

```bash
curl -X POST http://127.0.0.1:8080/services \
  -H "Content-Type: application/json" \
  -d '{
    "name": "fastapi-service",
    "image": "service-launchpad/fastapi-service:dev",
    "port": 8000,
    "replicas": 1,
    "autoscaling": {
      "enabled": true,
      "minReplicas": 1,
      "maxReplicas": 5,
      "targetCpuUtilization": 60
    }
  }'
```

Run locally:

```bash
go run ./services/control-plane
```

For the Minikube monitoring stack to scrape the local control plane, keep it reachable from the Minikube node. The default `CONTROL_PLANE_LISTEN_ADDR=:8080` listens on all interfaces, which works with the `host.minikube.internal:8080` scrape target used by `vmagent`.

Optional file-backed storage:

```bash
CONTROL_PLANE_STORE_PATH=./tmp/control-plane-services.json go run ./services/control-plane
```

When `CONTROL_PLANE_STORE_PATH` is unset, the service stays in-memory only.

Manifest rendering:

- currently mirrors the sample inference-simulator shape from [`k8s/base`](k8s/base)
- renders a standardized Kubernetes `ConfigMap` for `fastapi-service`
- renders a standardized Kubernetes `Deployment`
- renders a standardized Kubernetes `Service`
- renders an `HorizontalPodAutoscaler` when autoscaling is enabled
- includes the same labels, probes, resource defaults, and `envFrom` wiring used by the base manifests (`k8s/base`)

Example manifest request:

```bash
curl http://127.0.0.1:8080/services/fastapi-service/manifests
```

Cluster apply:

- creates or updates the target namespace first when it does not exist yet
- uses the shared `client-go` deployer by default
- can use local kubeconfig for Minikube
- can use explicit API endpoint, CA, and bearer-token settings for external runtimes
- can fall back to `kubectl apply -f -` for local debug use

The `client-go` deployer is the shared implementation path for Minikube and the production Cloud Run to GKE model. The local `kubectl` deployer remains available as a fallback / debug option only.

Deployer modes:

```bash
# default
CONTROL_PLANE_DEPLOYER_MODE=client-go

# local fallback / debug
CONTROL_PLANE_DEPLOYER_MODE=kubectl

# API-only mode, disables deploy calls
CONTROL_PLANE_DEPLOYER_MODE=disabled
```

Local Minikube through kubeconfig:

```bash
CONTROL_PLANE_DEPLOYER_MODE=client-go \
CONTROL_PLANE_KUBECONFIG="$HOME/.kube/config" \
CONTROL_PLANE_KUBE_CONTEXT=service-launchpad \
go run ./services/control-plane
```

External cluster settings:

```bash
CONTROL_PLANE_DEPLOYER_MODE=client-go
CONTROL_PLANE_KUBE_API_SERVER=https://<cluster-api-endpoint>
CONTROL_PLANE_KUBE_CA_FILE=/var/run/service-launchpad/cluster-ca.crt
CONTROL_PLANE_KUBE_BEARER_TOKEN_FILE=/var/run/service-launchpad/token
CONTROL_PLANE_TARGET_NAMESPACE=service-launchpad-prod
```

The external settings are the shape intended for Cloud Run once the production GKE access model is configured. The Cloud Run service account authenticates the runtime; Kubernetes RBAC still controls which resources can be created or updated.

Optional deployment audit records:

```bash
CONTROL_PLANE_AUDIT_BUCKET=<gcs-bucket-name>
CONTROL_PLANE_AUDIT_PREFIX=control-plane/deployments
```

When `CONTROL_PLANE_AUDIT_BUCKET` is set, every successful or failed deploy attempt writes a JSON audit artifact to GCS. The artifact includes the service definition, target namespace, generated manifests, deploy result, status, duration, and error details when deployment fails. Audit storage is best-effort: a GCS write failure is logged but does not change the deploy response status.

Cloud Run uses its runtime service account and the metadata server to obtain the GCS access token. For local tests or controlled debugging, `CONTROL_PLANE_GCS_BEARER_TOKEN` and `CONTROL_PLANE_GCS_ENDPOINT` can override token and endpoint behavior.

Current prerequisite:

- the referenced container image must already be available to the target cluster
- for local `Minikube`, that usually means:

```bash
eval "$(minikube -p service-launchpad docker-env)"
docker build -t service-launchpad/fastapi-service:dev services/fastapi-service
```

Project direction:

- keep image availability as a documented prerequisite for the current local-control-plane phase
- later add a dev-friendly workflow around image loading or chart values, without turning the control plane itself into a generic image builder

Example deploy request:

```bash
curl -X POST http://127.0.0.1:8080/services/fastapi-service/deploy
```

Health and readiness:

- `GET /health` returns a minimal liveness response
- `GET /ready` returns readiness/status details including target namespace, managed service count, deployment availability, metrics availability, and whether file persistence is enabled

Metrics:

- exposed from `GET /metrics` in Prometheus text format for `vmagent` / `VictoriaMetrics`
- `service_launchpad_control_plane_service_registrations_total{result="success|failure"}`
- `service_launchpad_control_plane_deployments_total{result="success|failure"}`
- `service_launchpad_control_plane_deployment_duration_seconds`
- `service_launchpad_control_plane_managed_services`

The local monitoring stack keeps the control plane outside Kubernetes for now and scrapes it from in-cluster `vmagent` through Minikube's host alias:

```text
host.minikube.internal:8080
```

Cloud Run keeps the same endpoints, but the production observability model is different:

- Cloud Run startup checks use `/ready`
- Cloud Run liveness checks use `/health`
- `/metrics` stays available for authenticated ad hoc inspection
- stdout, stderr, and request logs are the baseline production signal through Cloud Logging

Do not treat Cloud Run `/metrics` as durable production telemetry. Cloud Run instances are ephemeral, can scale to zero, and may be reachable only through IAM or internal ingress, so pull-based scraping is less reliable than the local Minikube scrape path. Add OTLP export, a sidecar/exporter, pushgateway, or remote-write path later if production control-plane metrics need retention and dashboards.
