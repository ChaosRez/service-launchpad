# Scripts

This directory will hold local helper scripts for Minikube bootstrapping, deployment, and demo workflows.

## Available Scripts

- `bootstrap-minikube.sh`: starts a local Minikube cluster, updates the `kubectl` context, enables `metrics-server`, and can print the Docker environment command for local image builds
- `load-test-fastapi-service.sh`: runs an in-cluster `k6` Job against `fastapi-service` and remote-writes `k6_*` metrics to `vmagent`, which fans them out to both `VictoriaMetrics` and `Mimir`
- `smoke-test-fastapi-service.sh`: boots Minikube, builds the service image into Minikube, starts the control plane, registers `fastapi-service`, deploys it through the control-plane API, and validates the main endpoints

## Control Plane Smoke Test

The preferred deployment smoke test now exercises the control plane instead of applying [`k8s/base`](k8s/base) directly.

```bash
./scripts/smoke-test-fastapi-service.sh
```

The script:

- starts `Minikube`
- points Docker at Minikube's daemon
- builds `service-launchpad/fastapi-service:dev`
- starts the control plane locally with a temporary JSON store
- registers `fastapi-service`
- validates the rendered manifests
- deploys through `POST /services/fastapi-service/deploy`
- waits for rollout and verifies the service endpoints

Current prerequisite: the referenced image must already be available to the cluster. The smoke test handles that automatically for local Minikube by building the image into Minikube's Docker daemon first.

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
