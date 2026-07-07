# Production Observability Overlay

This overlay deploys the self-hosted LGTM-style stack to `GKE production`:

- VictoriaMetrics
- Grafana Mimir
- vmagent
- kube-state-metrics
- Grafana Tempo
- Grafana Loki
- Promtail
- Grafana

The first production version keeps every observability service private with Kubernetes `ClusterIP` services. Remote operator access should use the same private cluster access path selected for Task 31, then `kubectl port-forward` for the first demo. Internal Load Balancers, IAP, DNS, and managed certificates remain later hardening work.

The overlay also replaces the local `vmagent` scrape config so production scrapes:

- `fastapi-service.service-launchpad-prod.svc.cluster.local:8000`
- in-cluster observability components
- kube-state-metrics

Cloud Run control-plane metrics are intentionally not scraped here; Task 32/32a handles production control-plane telemetry separately.

Apply from a machine that can reach the private GKE Kubernetes API:

```bash
kubectl apply -k k8s/overlays/prod-observability
```

Check rollout:

```bash
kubectl rollout status deployment/victoriametrics -n service-launchpad-observability
kubectl rollout status deployment/mimir -n service-launchpad-observability
kubectl rollout status deployment/vmagent -n service-launchpad-observability
kubectl rollout status deployment/kube-state-metrics -n service-launchpad-observability
kubectl rollout status deployment/tempo -n service-launchpad-observability
kubectl rollout status deployment/loki -n service-launchpad-observability
kubectl rollout status daemonset/promtail -n service-launchpad-observability
kubectl rollout status deployment/grafana -n service-launchpad-observability
```

Access Grafana privately:

```bash
kubectl port-forward svc/grafana 3000:3000 -n service-launchpad-observability
```
