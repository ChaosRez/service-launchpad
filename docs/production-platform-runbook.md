# Production Platform Runbook

This runbook covers the first production `GKE` platform deployment for Task 31.

## Access Assumption

The production cluster keeps its public IP endpoint disabled and exposes two authenticated control-plane paths:

- operators use the Google-managed DNS endpoint from a local workstation, Cloud Shell, or a compatible Kubernetes IDE
- Cloud Run uses the private IP endpoint through production VPC egress

The DNS endpoint requires Google IAM authentication and Kubernetes RBAC authorization. It does not expose nodes, workloads, Grafana, or observability services.

## Prerequisites

- Production Terraform has been applied.
- Production images have been published with `scripts/publish-production-images.sh`.
- `gke-gcloud-auth-plugin` is installed on the operator workstation.
- `kubectl` is authenticated to `service-launchpad-prod-gke` through the DNS endpoint.

Configure the local kubeconfig entry:

```bash
gcloud container clusters get-credentials service-launchpad-prod-gke \
  --project geofaas-411316 \
  --region europe-west10 \
  --dns-endpoint
```

Verify the target before any apply:

```bash
kubectl config current-context
kubectl config view --minify --output='jsonpath={.clusters[0].cluster.server}{"\n"}'
kubectl cluster-info --request-timeout=10s
kubectl get nodes -o wide
```

The server should use `https://gke-....gke.goog`, the context should be `gke_geofaas-411316_europe-west10_service-launchpad-prod-gke`, and the production node should report `Ready`.

GoLand uses the same kubeconfig and `gke-gcloud-auth-plugin`; no separate GKE network configuration is required after the DNS-backed context works with `kubectl`.

## Deploy

Use the published `fastapi-service` image from Terraform outputs:

```bash
FASTAPI_SERVICE_IMAGE="$(terraform -chdir=infra/terraform/environments/prod output -raw fastapi_service_container_image)"
./scripts/deploy-production-platform.sh \
  --context gke_geofaas-411316_europe-west10_service-launchpad-prod-gke \
  --fastapi-image "${FASTAPI_SERVICE_IMAGE}"
```

The script applies:

- `k8s/overlays/prod-observability`
- `k8s/overlays/prod`

It then waits for the observability stack and workload deployment rollouts.

## Verify

Check workload readiness:

```bash
kubectl get pods,svc,hpa -n service-launchpad-prod
kubectl rollout status deployment/fastapi-service -n service-launchpad-prod
```

Check observability readiness:

```bash
kubectl get pods,svc -n service-launchpad-observability
kubectl rollout status deployment/grafana -n service-launchpad-observability
kubectl rollout status deployment/vmagent -n service-launchpad-observability
kubectl rollout status deployment/tempo -n service-launchpad-observability
kubectl rollout status deployment/loki -n service-launchpad-observability
kubectl rollout status daemonset/promtail -n service-launchpad-observability
```

Check Grafana privately:

```bash
kubectl port-forward svc/grafana 3000:3000 -n service-launchpad-observability
```

Open `http://127.0.0.1:3000` on the operator workstation.

Generate workload traffic:

```bash
kubectl port-forward svc/fastapi-service 8000:8000 -n service-launchpad-prod
curl http://127.0.0.1:8000/health
curl http://127.0.0.1:8000/ready
curl http://127.0.0.1:8000/v1/models
```

Expected observability checks:

- `vmagent` scrapes `fastapi-service` and kube-state-metrics.
- Grafana can query VictoriaMetrics/Mimir for workload metrics.
- Tempo receives traces from `fastapi-service`.
- Loki shows pod logs through Promtail.

## Access Model

The first production Grafana access path is private `kubectl port-forward` over the DNS-backed Kubernetes API connection. Internal Load Balancer, IAP, DNS for Grafana itself, and managed certificates remain later hardening work.
