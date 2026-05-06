# Control Plane

This directory contains `Go` control-plane service for Service Launchpad.

Current scope:

- accepts service definitions over HTTP
- keeps them in memory for now
- exposes read endpoints for registered services

TODO:
Validation rules, persistent storage, and Kubernetes manifest rendering.

Implemented endpoints:

- `GET /health`
- `POST /services`
- `GET /services`
- `GET /services/{name}`

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
