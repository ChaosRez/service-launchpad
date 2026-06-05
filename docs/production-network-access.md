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

The preferred production direction is a private GKE control-plane endpoint reachable from Cloud Run through VPC egress. This keeps Kubernetes API access off the public internet and gives the project a clear network boundary.

For a lower-friction demo, a public GKE endpoint is acceptable only if it is intentionally chosen and paired with strong authentication and restrictive authorized access. It should not be the accidental default.

Decision for the first production implementation:

- default design: private nodes and private GKE API endpoint
- acceptable demo fallback: public endpoint with explicit authorized access and documented tradeoff
- never acceptable: broad public Kubernetes API access without a written justification

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

- private-only GKE endpoint
- Cloud NAT or stricter egress policy
- private service access where needed
- organization policy constraints
- VPC Service Controls around GCS or telemetry data if the project grows
- Identity-Aware Proxy or internal HTTPS load balancer for operator-facing Grafana
- network policies between workload and observability namespaces

These are not required for the first production demo, but the custom VPC and explicit ingress/egress choices leave room for them.

## Open Decisions

Task 29 turned the first GKE foundation into Terraform with a private endpoint default. Remaining decisions for later production tasks:

- whether the first live demo keeps the private endpoint or temporarily uses the public endpoint fallback with authorized networks
- how operators and CI reach the private Kubernetes API for Task 31 manifest deployment: VPN, Cloud Workstations, bastion VM, or private runner inside the production VPC
- how trusted operators reach production Grafana remotely: VPN, Cloud Workstations, bastion tunnel, or internal HTTPS load balancer with Identity-Aware Proxy
- Cloud Run ingress enum
- Cloud Run VPC egress mode
- whether Grafana needs an Internal Load Balancer immediately
- initial Cloud Run invoker members
