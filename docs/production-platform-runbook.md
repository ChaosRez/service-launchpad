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
- The pinned `kube-state-metrics` dependency has been mirrored into Artifact Registry because private GKE nodes do not have general internet egress.
- `gke-gcloud-auth-plugin` is installed on the operator workstation.
- `kubectl` has a `prod-gke` context authenticated to `service-launchpad-prod-gke` through the DNS endpoint.

Configure the local kubeconfig entry:

```bash
gcloud container clusters get-credentials service-launchpad-prod-gke \
  --project geofaas-411316 \
  --location europe-west10-a \
  --dns-endpoint

# Use the generated context directly, or rename it to prod-gke after removing
# or renaming the old regional-cluster context.
kubectl config current-context
```

Verify the target before any apply:

```bash
kubectl config current-context
kubectl config view --context prod-gke --minify --output='jsonpath={.clusters[0].cluster.server}{"\n"}'
kubectl --context prod-gke cluster-info --request-timeout=10s
kubectl --context prod-gke get nodes -o wide
```

The rename changes only the local kubeconfig label, not the GKE cluster. The server should use `https://gke-....gke.goog`, the context should be `prod-gke`, and the production node should report `Ready`.

GoLand uses the same kubeconfig and `gke-gcloud-auth-plugin`; no separate GKE network configuration is required after the DNS-backed context works with `kubectl`.

## Mirror Third-Party Image

The private GKE node can pull from Artifact Registry through Google networking but cannot reach `registry.k8s.io` without Cloud NAT. Mirror the pinned AMD64 image from the local workstation:

```bash
SOURCE_IMAGE="registry.k8s.io/kube-state-metrics/kube-state-metrics:v2.13.0"
MIRROR_IMAGE="europe-west10-docker.pkg.dev/geofaas-411316/service-launchpad/kube-state-metrics:v2.13.0"

gcloud auth configure-docker europe-west10-docker.pkg.dev --quiet
docker pull --platform linux/amd64 "${SOURCE_IMAGE}"
docker tag "${SOURCE_IMAGE}" "${MIRROR_IMAGE}"
docker push "${MIRROR_IMAGE}"

gcloud artifacts docker images list \
  europe-west10-docker.pkg.dev/geofaas-411316/service-launchpad/kube-state-metrics \
  --include-tags
```

This adds one pinned image to the existing repository. It does not modify GKE until the production overlay is applied.

## Deploy

Use the published `fastapi-service` image from Terraform outputs:

```bash
FASTAPI_SERVICE_IMAGE="$(terraform -chdir=infra/terraform/environments/prod output -raw fastapi_service_container_image)"
./scripts/deploy-production-platform.sh \
  --context prod-gke \
  --fastapi-image "${FASTAPI_SERVICE_IMAGE}"
```

The script applies:

- `k8s/overlays/prod-observability`
- `k8s/overlays/prod`

It then waits for the observability stack and workload deployment rollouts.

On a fresh cluster, `kubectl apply --dry-run=server` cannot validate namespaced objects in the same operation that dry-runs their Namespace because the Namespace is intentionally not persisted. This can produce `namespaces "service-launchpad-prod" not found` even when the rendered overlay is valid. After the Namespace exists, the same server-side dry run validates cleanly.

## Verify

Check workload readiness:

```bash
kubectl --context prod-gke get pods,svc,hpa -n service-launchpad-prod
kubectl --context prod-gke rollout status deployment/fastapi-service -n service-launchpad-prod
```

Check observability readiness:

```bash
kubectl --context prod-gke get pods,svc -n service-launchpad-observability
kubectl --context prod-gke rollout status deployment/grafana -n service-launchpad-observability
kubectl --context prod-gke rollout status deployment/vmagent -n service-launchpad-observability
kubectl --context prod-gke rollout status deployment/tempo -n service-launchpad-observability
kubectl --context prod-gke rollout status deployment/loki -n service-launchpad-observability
kubectl --context prod-gke rollout status daemonset/promtail -n service-launchpad-observability
```

Check Grafana privately:

```bash
kubectl --context prod-gke port-forward svc/grafana 3000:3000 -n service-launchpad-observability
```

Open `http://127.0.0.1:3000` on the operator workstation.

Generate workload traffic:

```bash
kubectl --context prod-gke port-forward svc/fastapi-service 8000:8000 -n service-launchpad-prod
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
