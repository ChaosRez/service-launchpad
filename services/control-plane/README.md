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

- renders a standardized Kubernetes `Deployment`
- renders a standardized Kubernetes `Service`
- renders an `HorizontalPodAutoscaler` when autoscaling is enabled
- includes standard labels, annotations, probes, and resource defaults

Example manifest request:

```bash
curl http://127.0.0.1:8080/services/fastapi-service/manifests
```
