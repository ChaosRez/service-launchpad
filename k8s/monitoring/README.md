# Monitoring Stack

This directory contains a lightweight local observability stack for Service Launchpad:

- `VictoriaMetrics` as the metrics backend, scraping Prometheus-format endpoints
- `kube-state-metrics` for deployment and HPA state
- `Grafana Tempo` as the trace backend
- `Grafana` with both datasources and a starter dashboard pre-provisioned

## Deploy

```bash
kubectl apply -k k8s/monitoring
kubectl rollout status deployment/kube-state-metrics -n service-launchpad-observability
kubectl rollout status deployment/victoriametrics -n service-launchpad-observability
kubectl rollout status deployment/tempo -n service-launchpad-observability
kubectl rollout status deployment/grafana -n service-launchpad-observability
```

## Access

```bash
kubectl port-forward svc/grafana 3000:3000 -n service-launchpad-observability
kubectl port-forward svc/victoriametrics 8428:8428 -n service-launchpad-observability
kubectl port-forward svc/tempo 3200:3200 -n service-launchpad-observability
```


## Notes

- `fastapi-service` metrics are scraped directly from the service in `service-launchpad-dev`
- `kube-state-metrics` is scraped for replica and autoscaler views
- trace export is configured through the `fastapi-service` ConfigMap in `k8s/base`
- the starter dashboard includes request rate, p95 latency, error rate, availability SLI, latency SLI, and replica behavior
- Grafana provisions the dashboard into the `Service Launchpad` folder on startup
