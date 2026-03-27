# FastAPI Service

This workload simulates a small `llama.cpp`-style chat completion service. It keeps both the request and response mostly static, and only varies the response time so it stays easy to demo, monitor, and scale in Kubernetes.

## Endpoints

- `GET /health`: liveness-style health check
- `GET /ready`: readiness-style status check
- `GET /metrics`: Prometheus metrics endpoint
- `GET /v1/models`: returns the exposed simulator model metadata
- `POST /v1/chat/completions`: returns a fixed chat completion response with one of a few pre-defined runtimes

## Runtime Simulation

The simulator is intentionally predictable. Clients choose one pre-defined runtime profile:

- `short`: quick request
- `medium`: default request
- `long`: slower request

The response body is static apart from the generated id, timestamp, and selected runtime metadata.

## Metrics

The service exports Prometheus metrics for:

- total request count
- request latency
- error count

## Local Run

```bash
python3 -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt
uvicorn app.main:app --reload --host 0.0.0.0 --port 8000
```

## Docker Run

```bash
docker build -t service-launchpad/fastapi-service .
docker run --rm -p 8000:8000 service-launchpad/fastapi-service
```

## Kubernetes Deploy

Build the image into Minikube's Docker daemon first, then apply the base manifests:

```bash
eval $(minikube -p service-launchpad docker-env)
docker build -t service-launchpad/fastapi-service:dev services/fastapi-service
kubectl apply -k k8s/base
```

Quick sanity check:
```bash
kubectl get configmap fastapi-service-config -n service-launchpad-dev -o yaml
kubectl get svc fastapi-service -n service-launchpad-dev
kubectl get deployment fastapi-service -n service-launchpad-dev -o yaml
```

## Example Request

```bash
curl -X POST http://localhost:8000/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "runtime_profile": "long"
  }'
```
