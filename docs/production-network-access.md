# Production Network and Access Design

This note defines the target network and access model for the required `GKE production` path. It is a design boundary for the later Terraform work; it does not provision resources by itself.

## Summary

Production uses:

- a custom GCP VPC for Service Launchpad resources
- regional subnets with secondary ranges for VPC-native GKE pods and services
- GKE for workloads and the self-hosted LGTM stack
- Cloud Run for the external control plane
- Cloud Run IAM for operator access to the control-plane API
- Kubernetes RBAC for what the control plane can mutate inside GKE
- private access for Grafana and observability backends by default

The control plane remains outside Kubernetes. Cloud Run calls the GKE Kubernetes API through the same `client-go` deployer path already validated against Minikube.

## Why a Custom VPC

Use a custom VPC rather than the default network because production needs explicit ownership of:

- subnet CIDR ranges
- GKE pod and service secondary ranges
- firewall rules
- Cloud Run egress behavior
- private observability access
- future private endpoint and stricter egress controls

The default VPC creates broad regional defaults that are convenient for experiments but weak for explaining production boundaries. A custom VPC makes the network shape intentional and auditable.

## Production Subnet Layout

The production Terraform root should create at least one regional subnet for GKE and Cloud Run egress:

```text
VPC: service-launchpad-prod-vpc
Subnet: service-launchpad-prod-subnet
Primary range: GKE nodes and VPC-native endpoints
Secondary range: pods
Secondary range: services
```

The example production input shape currently reserves:

- primary subnet: `10.30.0.0/24`
- pods: `10.40.0.0/20`
- services: `10.41.0.0/24`

Those ranges are placeholders for the demo production path. Before applying production Terraform in a shared project, verify they do not overlap with existing VPCs, VPNs, peering ranges, or private service ranges.

## GKE API Endpoint Choice

Production uses two deliberate GKE control-plane access paths:

- operators, Cloud Shell, and Kubernetes-aware IDEs use the DNS endpoint with Google IAM authentication and Kubernetes RBAC authorization
- Cloud Run uses the private IP endpoint through direct VPC egress

The public IP endpoint remains disabled. Enabling the DNS endpoint does not expose GKE nodes, workloads, or an unauthenticated Kubernetes API. It places the Kubernetes API behind a Google-managed `*.gke.goog` endpoint and requires the caller to have `container.clusters.connect` plus the Kubernetes authorization needed for the requested operation.

This hybrid model was verified from a local workstation on 2026-08-22 with `gcloud container clusters get-credentials --dns-endpoint`, `kubectl cluster-info`, `kubectl get nodes`, and GoLand's Kubernetes integration. It avoids a bastion for routine operator access while retaining the private endpoint needed by the Cloud Run runtime design.

Terraform manages both controls independently:

- `gke_enable_private_endpoint = true` disables the public IP endpoint while retaining the private IP endpoint
- `gke_enable_dns_endpoint = true` permits IAM-authenticated traffic through the DNS endpoint

For stricter environments, VPC Service Controls can further restrict the DNS endpoint. A public IP endpoint is acceptable only as an intentional fallback with restrictive authorized networks; broad public IP access is not allowed.

## Cloud Run to GKE API

Cloud Run needs to reach the GKE Kubernetes API so the control plane can apply resources with `client-go`.

If GKE uses a private endpoint:

- enable Cloud Run VPC egress
- attach Cloud Run to the production VPC/subnet
- keep egress to private ranges where possible
- allow Cloud Run egress to the GKE control-plane endpoint

If GKE uses a public endpoint:

- Cloud Run can reach it over normal Google-managed egress
- API authentication and endpoint access restrictions become more important
- document why the public endpoint was chosen

In both cases, Google IAM is not enough by itself. Kubernetes RBAC must authorize the Cloud Run service account for the exact resources it manages.

## Control-Plane API Access

Operators call the control-plane API through the Cloud Run HTTPS URL.

Required access model:

- no unauthenticated public invocation
- explicit `roles/run.invoker` grants for user, group, or service-account members
- identity tokens with the Cloud Run service URL as audience
- local unauthenticated access remains limited to `127.0.0.1` development only

Example production call shape:

```bash
CONTROL_PLANE_URL="https://<cloud-run-service-url>"
TOKEN="$(gcloud auth print-identity-token --audiences="${CONTROL_PLANE_URL}")"

curl -X POST "${CONTROL_PLANE_URL}/services" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d @service-definition.json

curl -X POST "${CONTROL_PLANE_URL}/services/fastapi-service/deploy" \
  -H "Authorization: Bearer ${TOKEN}"
```

If the demo must be fully private, use internal or internal-load-balancer Cloud Run ingress and keep the same explicit invoker IAM model.

## Workload Traffic

Workload traffic should stay separate from control-plane traffic.

Initial production workload exposure:

- internal Kubernetes `Service` for service-to-service traffic
- Ingress or Gateway API only when the workload needs external users
- TLS termination at the cloud load balancer, gateway, or ingress layer
- no direct pod or node exposure

The control plane deploys workloads, but it should not become the data path for workload traffic.

## Observability Access

Production uses a self-hosted LGTM stack in `GKE production`.

Default access model:

- Grafana, Loki, Tempo, Mimir, VictoriaMetrics, and vmagent stay private
- Kubernetes `Service` DNS is preferred for in-cluster access
- use an Internal Load Balancer only where Cloud Run or trusted operators need network reachability
- avoid public Grafana or public observability backend endpoints

The DNS endpoint exposes only the Kubernetes API. It does not expose Grafana or the telemetry backends. Operators use the DNS-backed Kubernetes connection with `kubectl port-forward` for the first production verification path.

Cloud Run logs are captured first by Cloud Logging. Later, Task 32a routes structured Cloud Run deployment events through:

```text
Cloud Logging -> Pub/Sub -> GKE subscriber / collector -> Loki
```

GCS deployment audit artifacts remain durable evidence and are not the primary Loki ingestion path.

## Firewall Boundaries

Production firewall rules should be narrow and explainable:

- allow required internal VPC traffic between GKE nodes and pod/service ranges
- allow Cloud Run VPC egress to private GKE API endpoint if private endpoint is selected
- allow Cloud Run or trusted client access only to required internal observability endpoints
- avoid broad public ingress to nodes, Grafana, Loki, Tempo, Mimir, or VictoriaMetrics
- avoid SSH-style administrative ingress unless there is a specific break-glass design

Workload ingress should be handled by Ingress/Gateway resources rather than generic firewall openings.

## Stricter Future Controls

Later hardening can add:

- DNS endpoint restrictions through VPC Service Controls
- Cloud NAT or stricter egress policy
- private service access where needed
- organization policy constraints
- VPC Service Controls around GCS or telemetry data if the project grows
- Identity-Aware Proxy or internal HTTPS load balancer for operator-facing Grafana
- network policies between workload and observability namespaces

These are not required for the first production demo, but the custom VPC and explicit ingress/egress choices leave room for them.

## Open Decisions

Task 29 turned the first GKE foundation into Terraform. The operator access and first Grafana access decisions are now closed:

- operators use the IAM-authenticated GKE DNS endpoint from local workstations, Cloud Shell, or compatible IDEs
- Cloud Run uses the private IP endpoint through VPC egress
- Grafana remains private and is reached first through `kubectl port-forward`

Remaining decisions for later production tasks:

- Cloud Run ingress enum
- Cloud Run VPC egress mode
- whether Grafana needs an Internal Load Balancer immediately
- initial Cloud Run invoker members
