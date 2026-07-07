# Production Platform Runbook

This runbook covers the first production `GKE` platform deployment for Task 31.

## Access Assumption

The production cluster currently uses a private Kubernetes API endpoint. Run the commands from a machine that can reach that private endpoint, such as:

- a bastion VM in the production VPC
- Cloud Workstations attached to the production VPC
- a VPN-connected operator workstation
- a private CI runner in the production VPC

Do not temporarily expose the Kubernetes API publicly unless that fallback is explicitly chosen with restrictive authorized networks.

## Prerequisites

- Production Terraform has been applied.
- Production images have been published with `scripts/publish-production-images.sh`.
- `kubectl` is authenticated to `service-launchpad-prod-gke`.

Recommended credential command from an eligible private-network host:

```bash
gcloud container clusters get-credentials service-launchpad-prod-gke \
  --project geofaas-411316 \
  --region europe-west10 \
  --internal-ip
```

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

Open `http://127.0.0.1:3000` from the operator host or through the chosen tunnel.

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

The first production Grafana access path is private `kubectl port-forward` from the same machine that can reach the private GKE API. Internal Load Balancer, IAP, DNS, and managed certificates remain later hardening work.
