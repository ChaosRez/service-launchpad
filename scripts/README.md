# Scripts

This directory will hold local helper scripts for Minikube bootstrapping, deployment, and demo workflows.

## Available Scripts

- `bootstrap-minikube.sh`: starts a local Minikube cluster, updates the `kubectl` context, enables `metrics-server`, and can print the Docker environment command for local image builds
- `deploy-production-platform.sh`: applies the production observability and workload overlays to GKE from a host that can reach the private Kubernetes API
- `load-test-fastapi-service.sh`: runs an in-cluster `k6` Job against `fastapi-service` and remote-writes `k6_*` metrics to `vmagent`, which fans them out to both `VictoriaMetrics` and `Mimir`
- `publish-production-images.sh`: builds and pushes the production `control-plane` and `fastapi-service` images to Artifact Registry
- `smoke-test-fastapi-service.sh`: boots Minikube, builds the service image into Minikube, starts the control plane, registers `fastapi-service`, deploys it through the control-plane API, and validates the main endpoints

## Control Plane Smoke Test

The preferred deployment smoke test now exercises the control plane instead of applying [`k8s/base`](k8s/base) directly.

```bash
./scripts/smoke-test-fastapi-service.sh
```

For an already-running Minikube environment, use an isolated namespace and control-plane port:

```bash
./scripts/smoke-test-fastapi-service.sh \
  --skip-bootstrap \
  --skip-image-build \
  --namespace service-launchpad-smoke \
  --control-plane-port 18080 \
  --cleanup
```

The script:

- starts `Minikube`
- points Docker at Minikube's daemon
- builds `service-launchpad/fastapi-service:dev`
- starts the control plane locally with a temporary JSON store
- sets `CONTROL_PLANE_DEPLOYER_MODE=client-go`
- points `CONTROL_PLANE_KUBE_CONTEXT` at the Minikube profile
- sets `CONTROL_PLANE_TARGET_NAMESPACE` to the requested namespace
- registers `fastapi-service`
- validates the rendered manifests
- deploys through `POST /services/fastapi-service/deploy`
- asserts the deploy response reports `"deployer":"client-go"` and the expected applied resources
- waits for rollout and verifies the service endpoints

Current prerequisite: the referenced image must already be available to the cluster. The smoke test handles that automatically for local Minikube by building the image into Minikube's Docker daemon first.

Use `--skip-image-build` only when the image already exists in the Minikube Docker daemon or is pullable by the cluster.

## Production Image Publishing

The production path uses Artifact Registry images instead of Minikube-local images. The Terraform production root creates the repository; this script publishes the two images expected by later GKE and Cloud Run tasks.

Default behavior:

- primary tag: `git-<short-sha>`
- extra tag: `prod`
- platform: `linux/amd64`
- repository: `<region>-docker.pkg.dev/<project-id>/service-launchpad`

Example:

```bash
./scripts/publish-production-images.sh \
  --project-id geofaas-411316 \
  --region europe-west10 \
  --repository service-launchpad
```

To pin a specific production runtime tag:

```bash
./scripts/publish-production-images.sh \
  --project-id geofaas-411316 \
  --tag git-$(git rev-parse --short=12 HEAD) \
  --also-tag ""
```

The script prints the resulting `CONTROL_PLANE_IMAGE` and `FASTAPI_SERVICE_IMAGE`. Use the same tag in `infra/terraform/environments/prod/terraform.tfvars` through `production_image_tag` when a later task wires Cloud Run and GKE runtime configuration.

## k6 Load Testing

The preferred load path is now `k6` inside Kubernetes instead of local `curl` loops or a long-lived custom load pod. This avoids `kubectl port-forward` instability and lets the load generator publish its own metrics into the local observability stack.

Run a test with:

```bash
./scripts/load-test-fastapi-service.sh
```

Useful overrides:

```bash
./scripts/load-test-fastapi-service.sh \
  --profile long \
  --rate 35 \
  --duration 6m \
  --pre-allocated-vus 40 \
  --max-vus 300
```

The script:

- runs `k6` inside the cluster against `http://fastapi-service.service-launchpad-dev.svc.cluster.local:8000`
- streams `k6` metrics to `vmagent` at `/api/v1/write`
- lets `vmagent` replicate them to both `VictoriaMetrics` and `Mimir`
- tags the run with a `testid` so you can filter it in Grafana
- prints the `kubectl` watch commands that are most useful during a scaling demo

Implementation note: the script uses `k6`'s `experimental-prometheus-rw` output, which is the official `k6` path for streaming metrics into a Prometheus-compatible backend. In this project, `vmagent` receives that remote write traffic and forwards it to both metric stores.

## Manual Minikube Commands

```bash
minikube start --profile service-launchpad --driver docker --cpus 2 --memory 4096
minikube update-context --profile service-launchpad
minikube addons enable metrics-server --profile service-launchpad
eval $(minikube -p service-launchpad docker-env)
minikube status --profile service-launchpad
kubectl get nodes
kubectl get pods -A
```
