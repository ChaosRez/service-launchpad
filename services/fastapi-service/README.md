# FastAPI Service

This workload simulates a small `llama.cpp`-style chat completion service. It keeps both the request and response mostly static, and only varies the response time so it stays easy to demo, monitor, and scale in Kubernetes.

## Endpoints

- `GET /health`: liveness-style health check
- `GET /ready`: readiness-style status check
- `GET /metrics`: Prometheus-format metrics endpoint for `VictoriaMetrics`
- `GET /v1/models`: returns the exposed simulator model metadata
- `POST /v1/chat/completions`: returns a fixed chat completion response with one of a few pre-defined runtimes

## Runtime Simulation

The simulator is intentionally predictable. Clients choose one pre-defined runtime profile:

- `short`: quick request
- `medium`: default request
- `long`: slower request

The response body is static apart from the generated id, timestamp, and selected runtime metadata. Each profile also performs a small amount of real CPU work so the CPU-based HPA has something to react to during load tests.

## Metrics

The service exports Prometheus-format metrics for `VictoriaMetrics`:

- total request count
- request latency
- error count

## Tracing

Set an OTLP HTTP traces endpoint before starting the service to enable exporting:

```bash
export OTEL_SERVICE_NAME=fastapi-service
export OTEL_EXPORTER_OTLP_TRACES_ENDPOINT=http://localhost:4318/v1/traces
```

The chat completion response includes a `simulation.trace_id` field that can be used to find the request in Grafana Explore with the `Tempo` datasource.

When `VictoriaMetrics` is the metrics backend, Grafana trace drilldown is not the same as exemplar-backed Prometheus/Mimir workflows. For this stack, use:
- Grafana Explore with the `Tempo` datasource and `simulation.trace_id`
- Grafana correlations, if you want explicit navigation from metric views toward traces

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
kubectl get hpa fastapi-service -n service-launchpad-dev
```

## Autoscaling

The base manifests include a CPU-based `HorizontalPodAutoscaler`:

- minimum replicas: `1`
- maximum replicas: `5`
- target CPU utilization: `60%`

Useful commands while testing:

```bash
kubectl get hpa -n service-launchpad-dev -w
kubectl top pods -n service-launchpad-dev
kubectl describe hpa fastapi-service -n service-launchpad-dev
```
Port forward to localhost
```bash
kubectl port-forward svc/fastapi-service 8000:8000 -n service-launchpad-dev
```

To trigger scale-up, use the load-test script with stronger defaults:

```bash
./scripts/load-test-fastapi-service.sh
```

You can make it more aggressive if needed:

```bash
./scripts/load-test-fastapi-service.sh --requests 4000 --concurrency 200 --rounds 4 --profile long
```

Watch the HPA and pod count:

```bash
kubectl get hpa -n service-launchpad-dev -w
kubectl get pods -n service-launchpad-dev -w
kubectl top pods -n service-launchpad-dev
```

After the burst stops, give the HPA a few minutes and it should scale back down toward `1` replica.

## Example Request

```bash
curl -X POST http://localhost:8000/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "runtime_profile": "long"
  }'
```
