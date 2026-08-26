# Production GKE Cost Optimization

This runbook keeps the Service Launchpad workload and the complete self-hosted
observability stack while reducing the fixed GKE footprint. It does not turn
the platform into a highly available production system; the current one-node
design is already a learning/demo environment.

## Evidence and Cost Drivers

Live reads on 2026-08-26 showed:

- a regional Standard GKE control plane in `europe-west10`
- one on-demand `e2-standard-2` node with a 50 GB `pd-balanced` boot disk
- node usage of `176m` CPU (9%) and `3054Mi` memory
- the Service Launchpad and LGTM containers using about `62m` CPU and `552Mi`
  memory at steady state
- Google Managed Service for Prometheus, GKE observability metric packages,
  Cloud Logging collectors, and the repository's self-hosted LGTM stack all
  running at the same time
- no Ingress, `LoadBalancer` Service, or PersistentVolumeClaim in the current
  production overlays

The fixed list-price estimate below uses 730 hours/month and current Berlin
prices. It excludes taxes, network transfer, request-based services, storage
growth, and discounts. The zonal estimate assumes this billing account has not
used its monthly GKE free-tier credit on another eligible cluster.

| Fixed resource | Current | Cost-optimized |
| --- | ---: | ---: |
| GKE cluster management | Regional, about $73.00/month | Zonal, about $0 after the eligible free-tier credit |
| Node compute | `e2-standard-2`, about $48.92/month | `e2-medium`, about $24.46/month |
| Node boot disk | 50 GB `pd-balanced`, about $5.00/month | 30 GB `pd-standard`, about $1.20/month |
| Approximate fixed total | **$126.92/month** | **$25.66/month** |

That is an estimated fixed-cost reduction of about **$101/month (80%)**. If the
cluster remains regional, the node and disk changes alone reduce the estimate
to about **$98.66/month**. Current prices and billing-account credits always win
over this estimate.

Pricing references:

- [GKE pricing and free-tier eligibility](https://cloud.google.com/kubernetes-engine/pricing)
- [Berlin E2 VM pricing](https://cloud.google.com/products/compute/pricing/general-purpose)
- [Persistent Disk pricing](https://cloud.google.com/compute/disks-image-pricing)
- [Google Cloud Observability pricing](https://cloud.google.com/products/observability/pricing)

## Cost-Optimized Shape

The checked-in example and the local ignored `terraform.tfvars` select:

```hcl
gke_location          = "europe-west10-a"
gke_node_machine_type = "e2-medium"
gke_node_disk_size_gb = 30
gke_node_disk_type     = "pd-standard"
gke_node_spot          = false

gke_cloud_logging_enabled              = false
gke_cloud_monitoring_enabled           = false
gke_managed_prometheus_enabled         = false
gke_http_load_balancing_enabled        = false
gke_persistent_disk_csi_driver_enabled = false
gke_dns_cache_enabled                  = false
```

The production Kustomize overlays reduce scheduler requests, based on the live
steady-state measurements, but preserve container limits. The complete service
set remains:

- `fastapi-service`
- Grafana
- VictoriaMetrics
- Mimir
- vmagent
- kube-state-metrics
- Tempo
- Loki
- Promtail

An `e2-medium` GKE node currently exposes `940m` allocatable CPU. GKE system
pods request about `571m`, so the observability overlay requests `150m` total
and FastAPI requests `50m`, leaving about `169m` for scheduling headroom. The
single-replica observability Deployments use zero-surge rollouts; updates can
briefly interrupt an individual telemetry service without adding another node.

Disabling Cloud Logging and Cloud Monitoring reduces Google support visibility.
Google documents GKE support as best-effort when these integrations are off.
For this learning cluster, Loki/Promtail and VictoriaMetrics/Mimir/vmagent retain
the application-facing logs and metrics. Re-enable the Terraform switches before
using this design for a service that depends on Google support telemetry.

## Change Impact

Changing `gke_location` from `europe-west10` to `europe-west10-a` replaces the
cluster. The existing services will be unavailable during the replacement, and
the current `emptyDir` observability data will be lost. Expect approximately
15-30 minutes for the Terraform replacement, credential refresh, and platform
redeployment. The node-pool resize and addon changes also recreate nodes.

The tradeoff is a GKE control-plane SLA change from 99.95% for regional Standard
to 99.5% for zonal Standard. This matches the existing single-zone, single-node
workload availability but is not an HA production target.

Spot remains optional. Setting `gke_node_spot = true` can discount node compute
by 60-91%, but a one-node Spot pool can disappear at any time and lose all
`emptyDir` telemetry. Keep it `false` for the stable low-cost profile.

## 1. Preflight on the Local Workstation

These commands only inspect the selected targets:

```bash
hostname
kubectl config current-context
kubectl --context prod-gke cluster-info
kubectl --context prod-gke get nodes -o wide
kubectl --context prod-gke top nodes
kubectl --context prod-gke top pods -A --containers
gcloud auth list --filter=status:ACTIVE
gcloud config get-value project
```

Expected target: local workstation, context `prod-gke`, project
`geofaas-411316`, and cluster `service-launchpad-prod-gke`.

## 2. Apply the Lower Requests Before the Node Change

These commands mutate Kubernetes Deployments and the Promtail DaemonSet in the
existing production cluster. They cause rolling pod restarts but do not change
service names, images, ports, or limits:

```bash
kubectl --context prod-gke apply -k k8s/overlays/prod-observability
kubectl --context prod-gke apply -k k8s/overlays/prod

kubectl --context prod-gke rollout status deployment/grafana \
  -n service-launchpad-observability --timeout=180s
kubectl --context prod-gke rollout status deployment/mimir \
  -n service-launchpad-observability --timeout=240s
kubectl --context prod-gke rollout status deployment/fastapi-service \
  -n service-launchpad-prod --timeout=180s
```

Success means every rollout completes and `kubectl get pods -A` shows no
unexpected `Pending`, `CrashLoopBackOff`, or `ImagePullBackOff` pods.

## 3. Produce and Review a Refreshed Terraform Plan

Run on the local workstation. The normal plan refreshes live GCP state; unlike
`terraform state list`, it can reveal drift. It does not change GCP resources:

```bash
terraform -chdir=infra/terraform/environments/prod init
terraform -chdir=infra/terraform/environments/prod fmt -check
terraform -chdir=infra/terraform/environments/prod validate
terraform -chdir=infra/terraform/environments/prod plan \
  -out=cost-optimized.tfplan
terraform -chdir=infra/terraform/environments/prod show \
  -no-color cost-optimized.tfplan
```

Expected material changes:

- replace the regional GKE cluster with a zonal cluster
- replace the primary node pool with one `e2-medium` on-demand node
- reduce the boot disk to 30 GB `pd-standard`
- disable Cloud Logging, Cloud Monitoring, Managed Prometheus, HTTP load
  balancing, PD CSI, and NodeLocal DNSCache

Stop if the plan deletes the VPC, subnet, Artifact Registry repository, GCS
bucket, IAM identities, or any resource outside the expected GKE replacement.

## 4. Apply During a Maintenance Window

This is the consequential cloud mutation. It replaces GKE and causes downtime:

```bash
terraform -chdir=infra/terraform/environments/prod apply \
  cost-optimized.tfplan
```

Terraform will print the exact resources before asking for confirmation. No
meaningful temporary cost increase is expected because the same secondary IP
ranges prevent both clusters from running concurrently.

## 5. Refresh Credentials and Redeploy the Same Services

The new control plane has a new endpoint and CA. Refresh the local kubeconfig
through the IAM-authenticated DNS endpoint:

```bash
gcloud container clusters get-credentials service-launchpad-prod-gke \
  --project geofaas-411316 \
  --location europe-west10-a \
  --dns-endpoint

kubectl config current-context
kubectl cluster-info --request-timeout=10s
kubectl get nodes -o wide
```

Pass the generated current context directly to the deployment script:

```bash
GKE_CONTEXT="$(kubectl config current-context)"
FASTAPI_SERVICE_IMAGE="$(terraform -chdir=infra/terraform/environments/prod output -raw fastapi_service_container_image)"

./scripts/deploy-production-platform.sh \
  --context "${GKE_CONTEXT}" \
  --fastapi-image "${FASTAPI_SERVICE_IMAGE}"
```

Verify the runtime:

```bash
kubectl --context "${GKE_CONTEXT}" get pods -A
kubectl --context "${GKE_CONTEXT}" top nodes
kubectl --context "${GKE_CONTEXT}" top pods -A --containers

gcloud container clusters describe service-launchpad-prod-gke \
  --project geofaas-411316 \
  --location europe-west10-a \
  --format='yaml(location,nodePools.config.machineType,nodePools.config.diskType,nodePools.config.diskSizeGb,loggingService,monitoringService,monitoringConfig.managedPrometheusConfig)'
```

The cluster is **deployed** only after Terraform apply. It is **verified** only
after all pods are ready, Grafana can query both metrics stores, Loki returns
logs, Tempo returns traces, and the FastAPI health/readiness checks pass.

## Rollback

Changing back to a regional control plane is another destructive replacement,
not an instant rollback. Restore these inputs, create and review a fresh plan,
then apply and redeploy the overlays:

```hcl
gke_location          = "europe-west10"
gke_node_machine_type = "e2-standard-2"
gke_node_disk_size_gb = 50
gke_node_disk_type     = "pd-balanced"

gke_cloud_logging_enabled              = true
gke_cloud_monitoring_enabled           = true
gke_managed_prometheus_enabled         = true
gke_http_load_balancing_enabled        = true
gke_persistent_disk_csi_driver_enabled = true
gke_dns_cache_enabled                  = true
```

For a quicker capacity-only rollback on the new zonal cluster, keep
`gke_location = "europe-west10-a"`, restore only the node/addon inputs, and
review the resulting node-pool replacement plan.
