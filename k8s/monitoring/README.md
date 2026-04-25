# Monitoring Stack

This directory contains a lightweight local observability stack for Service Launchpad:

- `VictoriaMetrics` as the metrics backend, scraping Prometheus-format endpoints
- `Grafana Tempo` as the trace backend
- `Grafana` with both datasources pre-provisioned

## Deploy

```bash
kubectl apply -k k8s/monitoring
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
- trace export is configured through the `fastapi-service` ConfigMap in `k8s/base`
- dashboards and SLO views will be added is TODO
