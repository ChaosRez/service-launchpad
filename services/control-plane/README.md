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

Implemented endpoints:

- `GET /health`
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

- creates the target namespace first when it does not exist yet
- uses `kubectl apply -f -`
- targets the current `kubectl` context by default
- can target an explicit context with `CONTROL_PLANE_KUBECTL_CONTEXT`

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
