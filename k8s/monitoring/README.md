# Monitoring Stack

This directory contains a lightweight local observability stack for Service Launchpad:

- `VictoriaMetrics` as the fast local metrics backend
- `Grafana Mimir` as the heavier long-term metrics backend
- `vmagent` as the shared scrape and remote-write fanout layer
- `kube-state-metrics` for deployment and HPA state
- `Grafana Tempo` as the trace backend
- `Grafana Loki` as the log backend
- `Promtail` as the log shipper
- `Grafana` with datasources and starter dashboards pre-provisioned

For local development, `Mimir` is intentionally deployed in a single-process, filesystem-backed form. That keeps Task 11b lightweight enough for Minikube while still demonstrating the ingestion and storage tradeoffs.

## Deploy

```bash
kubectl apply -k k8s/monitoring
kubectl rollout status deployment/kube-state-metrics -n service-launchpad-observability
kubectl rollout status deployment/victoriametrics -n service-launchpad-observability
kubectl rollout status deployment/mimir -n service-launchpad-observability
kubectl rollout status deployment/vmagent -n service-launchpad-observability
kubectl rollout status deployment/tempo -n service-launchpad-observability
kubectl rollout status deployment/loki -n service-launchpad-observability
kubectl rollout status daemonset/promtail -n service-launchpad-observability
kubectl rollout status deployment/grafana -n service-launchpad-observability
```

## Access

```bash
kubectl port-forward svc/grafana 3000:3000 -n service-launchpad-observability
kubectl port-forward svc/victoriametrics 8428:8428 -n service-launchpad-observability
kubectl port-forward svc/mimir 9009:9009 -n service-launchpad-observability
kubectl port-forward svc/vmagent 8429:8429 -n service-launchpad-observability
kubectl port-forward svc/tempo 3200:3200 -n service-launchpad-observability
kubectl port-forward svc/loki 3100:3100 -n service-launchpad-observability
```


## Notes

- `vmagent` scrapes `fastapi-service`, `control-plane`, `kube-state-metrics`, `VictoriaMetrics`, and `Mimir`, then remote-writes the resulting series to both metric stores
- `kube-state-metrics` is scraped for replica and autoscaler views
- trace export is configured through the `fastapi-service` ConfigMap in `k8s/base`
- `promtail` tails Kubernetes pod logs from `/var/log/pods` and ships them to `Loki` with pod metadata labels
- the Promtail DaemonSet exports `HOSTNAME` from the Kubernetes node name so pod discovery only keeps log files local to each node
- log-to-trace correlation requires applications to include a `trace_id` in their log lines
- the starter dashboards include:
  - `FastAPI Service Observability` for application latency, errors, SLOs, and replicas
  - `k6 Load Testing` for generator request rate, duration, failure rate, and VUs
  - `Metrics Storage Comparison` for side-by-side local vs long-term store queries
- Grafana provisions the dashboard into the `Service Launchpad` folder on startup
- `VictoriaMetrics` stores Prometheus-format metrics, but it does not support exemplar-backed jump-to-trace navigation the same way Prometheus/Mimir do. Use Grafana Explore with the `Tempo` datasource or configure Grafana correlations to move from metrics toward traces.
- `k6` is used inside the cluster and sends `k6_*` metrics to `vmagent` via Prometheus remote write at `http://vmagent.service-launchpad-observability.svc.cluster.local:8429/api/v1/write`
- `vmagent` then replicates those samples to both `VictoriaMetrics` and `Mimir`, which is the Task 11b local-vs-global storage path

## Promtail Readiness Note

In Minikube, a few `kube-system` static control-plane pods such as `etcd`, `kube-apiserver`, `kube-controller-manager`, or `kube-scheduler` to remain `not ready` in the Promtail targets even when the main logging pipeline is working.

## Footprint Comparison

Use `metrics-server` to compare live CPU and memory footprint in Minikube:

```bash
kubectl top pods -n service-launchpad-observability
kubectl top pods -n service-launchpad-observability | egrep 'victoriametrics|vmagent|mimir'
```

For this local development setup, `Mimir` is intentionally heavier than `VictoriaMetrics`, so the comparison is expected to highlight the operational tradeoff between a lightweight local store and a longer-term distributed-style store.
