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

The production overlay lowers only scheduler requests to match the measured
steady-state footprint of this demo stack. Container limits are unchanged, so
Grafana, Mimir, Loki, Tempo, VictoriaMetrics, and vmagent can still burst. Recheck
`kubectl top pods -A --containers` after load tests before shrinking further.

On the one-node cost profile, the eight observability containers request `150m`
CPU in total. The overlay uses `maxSurge: 0` and `maxUnavailable: 1` for their
single-replica Deployments so updates do not require a second copy to fit on the
node. This permits brief per-service observability downtime during a rollout and
is not appropriate for a high-availability environment.

Cloud Run control-plane metrics are intentionally not scraped here; Task 32/32a handles production control-plane telemetry separately.

The production GKE nodes have private addresses and no Cloud NAT. The overlay therefore pulls `kube-state-metrics:v2.13.0` from the project Artifact Registry mirror instead of directly from `registry.k8s.io`. Mirror that image before the first deployment by following `docs/production-platform-runbook.md`.

Apply through the IAM-authenticated GKE DNS endpoint:

```bash
kubectl --context prod-gke apply -k k8s/overlays/prod-observability
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
