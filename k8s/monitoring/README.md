# Monitoring Stack

This directory contains a lightweight local observability stack for Service Launchpad:

- `VictoriaMetrics` as the metrics backend, scraping Prometheus-format endpoints
- `kube-state-metrics` for deployment and HPA state
- `Grafana Tempo` as the trace backend
- `Grafana` with both datasources and starter dashboards pre-provisioned

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
- the starter dashboards include:
  - `FastAPI Service Observability` for application latency, errors, SLOs, and replicas
  - `k6 Load Testing` for generator request rate, duration, failure rate, and VUs
- Grafana provisions the dashboard into the `Service Launchpad` folder on startup
- `VictoriaMetrics` stores Prometheus-format metrics, but it does not support exemplar-backed jump-to-trace navigation the same way Prometheus/Mimir do. Use Grafana Explore with the `Tempo` datasource or configure Grafana correlations to move from metrics toward traces.
-`k6` is used inside the cluster and sends `k6_*` metrics to `VictoriaMetrics` via Prometheus remote write at `http://victoriametrics.service-launchpad-observability.svc.cluster.local:8428/api/v1/write`
