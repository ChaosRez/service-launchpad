# Decisions and Tradeoffs

This document records the main architecture decisions behind Service Launchpad. It is written for interview discussion: each decision explains the reason, the tradeoff, and what would change in a production version.

## Decision 1: Use Go for the Control Plane

**Decision:** build the control plane in `Go`.

**Why:** Go is a natural fit for Kubernetes-adjacent systems. The Kubernetes API, controllers, operators, and many platform tools are written in Go, so this keeps the project close to the ecosystem it is demonstrating. Go also gives a small static binary, simple HTTP server support, strong standard-library primitives, and good testability without much framework overhead.

**Tradeoff:** Go is more verbose than scripting languages for simple API work. For a tiny demo, Python or Node could move faster.

**Required direction:** document the current local `kubectl` shell-out path and add a shared `client-go` deployer for both Minikube and `GKE production`. The first `client-go` step should cover the resources the control plane already renders: `Namespace`, `ConfigMap`, `Deployment`, `Service`, and `HorizontalPodAutoscaler`. Structured logging, request-scoped context handling, authentication, authorization, and controller-style reconciliation can build on that foundation.

## Decision 2: Keep the Control Plane Outside the Cluster for Now

**Decision:** run the control plane on the developer host during the local `Minikube` phase.

**Why:** the current deployer shells out to local `kubectl`, which uses the developer machine's kubeconfig and active Kubernetes context. Keeping it outside the cluster avoids prematurely designing service accounts, image publishing, and production cloud identity before the core registration and deployment flow is proven.

**Tradeoff:** in-cluster components cannot reach `localhost` on the developer machine. The monitoring stack handles this locally by scraping the control plane through Minikube's host alias, `host.minikube.internal:8080`.

**Required direction:** test the `client-go` deployer against Minikube, then run the production control plane on Cloud Run with a dedicated Google service account and secure Kubernetes API access to `GKE production`. Staging remains a future TODO.

## Decision 3: Use FastAPI for the Sample Workload

**Decision:** build the sample workload as a `FastAPI` service.

**Why:** FastAPI is a fast way to create a realistic HTTP service with typed request and response shapes. It is also familiar in ML and inference-serving contexts, which matches the simulated chat completion API. The service can expose health checks, readiness checks, metrics, and traces without much ceremony.

**Tradeoff:** the workload is not the hard part of the project. Using FastAPI keeps the service simple, but it does not demonstrate advanced model serving, GPU scheduling, batching, or real inference latency.

**Future production direction:** replace the simulator with a real model-serving backend, add stronger request validation, introduce per-model routing, and consider queueing or concurrency controls.

## Decision 4: Use Minikube as the First Kubernetes Target

**Decision:** make `Minikube` the default local platform.

**Why:** Minikube gives a full Kubernetes API locally and keeps the demo runnable without a cloud account. It is enough to demonstrate manifests, services, HPA behavior, kube-state-metrics, load testing, and a full observability stack.

**Tradeoff:** Minikube hides several production concerns: cloud load balancers, managed identity, private networking, managed node pools, DNS, certificates, and multi-zone failure modes.

**Next required direction:** keep Minikube for fast iteration, then validate the same control-plane flow on `GKE production` through Cloud Run once Terraform, IAM, and network boundaries are implemented.

## Decision 5: Use Cloud Run for the Production Control Plane

**Decision:** keep the control plane outside the cluster. Run it locally on the developer machine for Minikube and on Cloud Run for production.

**Why:** the project is meant to demonstrate platform thinking, not cloud setup for its own sake. Starting locally lets the service registration, deployment, scaling, and observability flows become concrete first. Cloud Run keeps the production control plane operationally small while still forcing real cloud architecture decisions: IAM, VPC egress, GKE API access, authenticated API access, private observability access, and deployment identity.

**Tradeoff:** an external control plane is not the same as a Kubernetes controller. It needs secure access to the GKE API and deliberate handling for metrics because Cloud Run instances are ephemeral and can scale to zero.

**Required direction:** validate the `client-go` deployer against Minikube, provision `GKE production`, deploy the control plane to Cloud Run, use the same `client-go` path for Kubernetes operations in both environments, and document staging as a future TODO. `kubectl` remains only a local fallback / debug deployer after `client-go` exists.

**Runtime model:** Cloud Run runs the same Go binary as local development, but with production configuration for the target GKE cluster. The Cloud Run service receives the Kubernetes API endpoint, cluster CA certificate, target namespace, and deployer mode through environment variables or mounted configuration. If the GKE API endpoint is private, Cloud Run needs VPC egress into the production VPC. If the endpoint is public, it must still be protected by explicit IAM authentication and a deliberate network access policy.

**Identity model:** Cloud Run uses a dedicated service account such as `service-launchpad-prod-control-plane@<project-id>.iam.gserviceaccount.com`. Google IAM authenticates that service account to reach or discover the cluster, but Kubernetes RBAC authorizes the actual deployment operations. If cluster endpoint and CA settings are supplied directly, Google IAM can stay narrow. If the service discovers cluster metadata dynamically, grant read-only GKE cluster viewer access rather than broad project roles.

**Kubernetes permissions:** bind the Cloud Run service account in `GKE production` to the smallest permissions needed by the deployer: cluster-scoped `get`, `create`, `update`, and `patch` on the managed `Namespace`; namespace-scoped `get`, `list`, `watch`, `create`, `update`, and `patch` on `ConfigMap`, `Service`, `Deployment`, and `HorizontalPodAutoscaler`. Do not grant `cluster-admin`, broad secret access, node access, pod exec, or mutation rights outside the managed namespace. Delete permissions should wait until service deletion or rollback cleanup is explicitly implemented.

**Client access model:** production clients call the Cloud Run HTTPS endpoint with an identity token and must be granted `roles/run.invoker` on the Cloud Run service. Local `127.0.0.1` development can remain unauthenticated; production cannot. Do not grant `allUsers` or `allAuthenticatedUsers`. If the demo needs a private-only path, restrict Cloud Run ingress to internal or internal-load-balancer traffic while keeping the same explicit IAM invocation model.

**Observability model:** keep `/health`, `/ready`, and `/metrics` in the Cloud Run runtime so the same binary stays inspectable in local and production paths. Production should run the self-hosted LGTM stack in `GKE production`, similar to the Minikube stack. Cloud Logging remains the first capture point for Cloud Run request and application logs, then can be integrated into self-hosted Loki through a Cloud Logging sink to Pub/Sub and a small GKE subscriber or collector. Pull scraping `/metrics` from Cloud Run is not a durable metrics strategy because instances are ephemeral, can scale to zero, and may sit behind IAM or internal ingress. Durable production control-plane metrics and traces require a later OTLP exporter, sidecar/exporter, pushgateway, or remote-write path into the self-hosted stack.

**Future TODOs:** staging, richer authorization beyond Cloud Run IAM, queryable audit history, progressive delivery, rollback gates, controller-style reconciliation, durable production control-plane metrics and traces, and stricter egress or private endpoint hardening remain separate follow-up work. Optional extensions can include extra node pools, private-only endpoints, managed certificates, DNS automation, stricter egress, IAP, or Pub/Sub.

**Deployment policy:** before production image publishing and Cloud Run deployment, the control plane must enforce a deterministic Go allow-list around both incoming service definitions and rendered Kubernetes manifests. This is intentionally not a general policy engine. It bounds replicas and autoscaling, restricts image prefixes by environment, keeps rendered resources inside the managed namespace, limits resource kinds to the control-plane contract, requires probes and resource requests/limits, keeps services `ClusterIP`, and rejects privileged or host-level workload settings. Cluster-side admission policy through Kubernetes admission policies, OPA Gatekeeper, or Kyverno remains a future hardening option after the first production path is working.

**Production image publishing:** production workloads and the Cloud Run control plane must use Artifact Registry images, not Minikube-local tags. The first publishing path is a manual script that builds and pushes `control-plane` and `fastapi-service` images with an immutable `git-<short-sha>` tag plus an optional mutable `prod` demo tag. GitHub Actions publishing should wait until scoped GCP authentication, preferably Workload Identity Federation, is configured.

## Decision 6: Keep Terraform Scope Minimal

**Decision:** keep Terraform focused on a small set of GCP resources and environment scaffolding.

**Why:** the project should show that infrastructure can be managed intentionally without turning into a broad cloud platform build. Minimal Terraform scope keeps the interview story focused on why each resource exists: IAM, networking, cluster boundaries, and environment-specific configuration.

**Priority slice:** the minimal GCP / Terraform slice is not low priority. It must include a custom VPC, Cloud Run service account, IAM notes, Artifact Registry, and either a minimal GKE cluster module or a documented cluster-module stub before the cloud path is considered credible.

**Tradeoff:** a minimal scope does not include every production concern, such as full DNS automation, certificate management, external secrets, multi-region topology, or disaster recovery.

**Cloud resource integration:** use the Terraform-managed artifact bucket as the first production integration point. The Cloud Run control plane writes best-effort deployment audit JSON objects to GCS after deploy attempts using its dedicated service account and bucket-level `roles/storage.objectCreator`. This proves cloud identity and resource access without introducing a database or broad project permissions.

**Future production direction:** expand Terraform only where the platform flow needs it: GKE cluster, node pools, Cloud Run service, service accounts, VPC egress, VPC-native networking, subnet ranges, firewall rules, Artifact Registry, storage buckets for telemetry/audit records, and environment-specific variables.

## Decision 7: Add CI Before More Cloud Complexity

**Decision:** add GitHub Actions CI immediately after the initial Terraform / GCP foundation.

**Why:** at this point the repo has meaningful surfaces to validate: Go control-plane behavior, FastAPI service dependencies, Kubernetes manifests, Docker builds, and Terraform configuration. CI should protect the platform path before the required GKE work adds more moving pieces.

**Priority checks:** the first CI version must run Go tests and Kubernetes manifest validation. Terraform formatting / validation and image build checks should be included as soon as practical. Live `terraform apply` and authenticated GKE deploys stay out of the first CI increment.

## Decision 8: Use VictoriaMetrics for Local Metrics

**Decision:** use VictoriaMetrics as the primary local metrics store.

**Why:** VictoriaMetrics is lightweight, fast, and compatible with Prometheus-style scraping and query workflows. It works well for a local demo where operational simplicity matters.

**Tradeoff:** VictoriaMetrics is not the same operational model as a larger multi-tenant metrics platform. Some Grafana exemplar and correlation workflows are more commonly demonstrated with Prometheus or Mimir.

**Future production direction:** decide whether local VictoriaMetrics remains a dev-only store, whether Mimir becomes the long-term central store, and what retention and cardinality policies apply by environment.

## Decision 9: Include Mimir, Tempo, and Loki for Observability Breadth

**Decision:** include Mimir, Tempo, and Loki alongside VictoriaMetrics.

**Why:** the project is stronger when it shows the full telemetry story: metrics, traces, and logs. Mimir provides a conversation about long-term metrics storage and remote write. Tempo demonstrates distributed tracing. Loki and Promtail complete the logging path.

**Tradeoff:** the local monitoring stack is heavier than a minimal demo. Running VictoriaMetrics, Mimir, Tempo, Loki, Promtail, kube-state-metrics, vmagent, and Grafana uses more Minikube CPU and memory.

**Future production direction:** define retention tiers, sampling rules, log retention, metric cardinality budgets, and cost-aware defaults per environment.

## Decision 10: Use k6 for Load Testing

**Decision:** use `k6` for workload load testing.

**Why:** k6 is scriptable, container-friendly, and produces useful metrics. Running it as an in-cluster job makes service DNS and network paths realistic for the local cluster.

**Tradeoff:** k6 validates service behavior under load, but it does not replace full performance testing, chaos testing, or production capacity planning.

**Future production direction:** add repeatable demo profiles, capture baseline results, and compare HPA behavior across local and cloud environments.

## Decision 11: Start with Hand-Rendered Manifests

**Decision:** render Kubernetes manifests directly from the Go control plane for the first implementation.

**Why:** direct rendering keeps the early control-plane behavior visible. It is easy to inspect the generated `Deployment`, `Service`, `ConfigMap`, and `HPA`, and it avoids adding Helm before the service definition model is stable.

**Tradeoff:** hand-rendered manifests can become brittle as the deployment model grows. Helm, Kustomize, or a typed Kubernetes client would handle complexity better.

**Future production direction:** package the workload as a Helm chart and have the control plane pass service definition values into `helm upgrade --install`, or move to `client-go` and typed Kubernetes resources.

**Update (2026-11-08):** the control plane now applies the manifests it renders through `client-go` server-side apply. The static workload is now also packaged as a Helm chart. Decision 15 explains why. The control plane does not call Helm.

## Decision 12: Keep Storage Simple

**Decision:** use in-memory storage by default with optional JSON file persistence.

**Why:** the goal is to demonstrate platform flow, not database operations. In-memory storage makes local iteration easy, and file-backed JSON is enough to show restart persistence without introducing Postgres, migrations, or backups.

**Tradeoff:** this is not suitable for multi-user or production operation. It lacks concurrency semantics beyond process memory, history, auditability, and high availability.

**Future production direction:** use a real datastore, add migrations, store deployment history, track service status, and support audit queries.

## Decision 13: Delay Local Authentication and Authorization

**Decision:** leave authn/authz out of the first local implementation, but require authenticated Cloud Run access for production.

**Why:** the early project needs to prove service registration, deployment, autoscaling, and observability. Adding authentication too early would increase setup burden and distract from the platform path.

**Tradeoff:** the unauthenticated API must remain local-only. It should not be exposed on a network without authentication and authorization.

**Future production direction:** add authentication, role-based authorization, audit logging, namespace scoping, and admission policy checks before any shared environment.

## Decision 14: Use Separate GKE DNS and Private-IP Access Paths

**Decision:** use the IAM-authenticated GKE DNS endpoint for operators, Cloud Shell, and Kubernetes-aware IDEs while retaining the private IP endpoint for the Cloud Run control plane. Keep the public IP endpoint disabled.

**Why:** operators need a practical way to use `kubectl`, port-forward private services, and inspect the cluster without maintaining a bastion or VPN solely for Kubernetes API access. The DNS endpoint provides that path through Google IAM and Kubernetes RBAC. Cloud Run already has direct VPC egress and can continue using the private IP endpoint, keeping runtime traffic on the production network path.

**Verified:** on 2026-08-22, a local workstation generated credentials with `gcloud container clusters get-credentials --dns-endpoint`; `kubectl cluster-info` and `kubectl get nodes` succeeded, and GoLand connected through the same kubeconfig context. The cluster retained private nodes, a disabled public IP endpoint, and its private control-plane endpoint at `172.16.0.2`.

**Tradeoff:** the DNS endpoint is reachable from external networks through Google-managed APIs, so IAM permissions such as `container.clusters.connect` and Kubernetes RBAC become central access controls. VPC Service Controls remain a future option for stricter network-aware restrictions.

## Decision 15: Add a Helm Chart for the Workload, Keep Kustomize

**Decision:** package `services/fastapi-service` as `charts/fastapi-service`. Its default values render the same `ConfigMap`, `Deployment`, `Service`, and `HPA` as `k8s/base`. Kustomize stays the source for the dev and prod overlays and for the monitoring stack. CI runs `helm lint --strict`, then `helm template` for the default and `values-prod.yaml` renders, then `kubeconform -strict` on both.

**Why:** Decision 11 deferred Helm until the service definition settled. It has now settled: image, replicas, HPA bounds, probes, limits, and OTLP endpoint. Helm adds what Kustomize does not have:
- release history with `helm upgrade` / `helm rollback` and a recorded revision for each change
- typed, documented values for the knobs that vary between installs
- a config checksum that rolls pods when the ConfigMap changes
- a versioned package that someone else can install without reading the overlay patches

**Tradeoff against Kustomize:**
- Kustomize keeps plain YAML that `kubectl` can apply directly, with no template language. Its overlays read as explicit diffs, which suits the two fixed environments here.
- Helm templates are harder to read and review. Every value is a level of indirection.
- Helm stores release state as Secrets in the namespace.
- Helm cannot adopt objects created by `kubectl apply -k`, so one namespace must use one tool.

Running both means two sources for one workload. A parity check (default `helm template` vs `kubectl kustomize k8s/base`) found exactly two intended differences: no `Namespace` object in the chart, and no `spec.replicas` while the HPA is enabled. That check was run once, by hand, and is not in CI.

**Verified:** `helm install`, `helm upgrade` with changed values, and `helm rollback` were run on 2026-11-08 against a throwaway local k3d cluster. The shared `dev-k3s` host and GKE were not touched. See `charts/fastapi-service/README.md`.

**Future production direction:** pick one tool per environment. If the control plane manages many services, either have it pass service definitions as values to `helm upgrade --install`, or keep `client-go` and drop the chart. Publish the chart to Artifact Registry as an OCI artifact if other teams consume it.

## Decision 16: JSON Logs Carry the OTel Trace ID

**Decision:** `fastapi-service` writes one JSON log line per request. The line includes `trace_id` and `span_id` from the current OpenTelemetry span. The Grafana Loki datasource has a derived field that turns `trace_id` into a link to the `Tempo` datasource. Probe and scrape paths are not logged, and uvicorn's plain-text access log is turned off.

**Why:** before this change, the trace ID appeared only in metric exemplars and the response body. A slow request found in Loki could not be followed to its trace. Putting the ID in the log line closes the logs → traces path without changing Promtail: Loki stores the line as-is, and Grafana's regex picks out the ID at query time.

**Tradeoff:** the regex depends on the exact JSON spelling `"trace_id":"<32 hex>"`. Changing the separators or field name breaks the link silently. Parsing JSON in Promtail would be sturdier, but it would turn `trace_id` into a label if done naively, and that label's cardinality is unbounded. A unit test pins the match between the log line and the response trace ID. The datasource regex is not under test.

**Verified:** on 2026-11-08, on the same local k3d cluster with `k8s/monitoring` applied:
- one request's `trace_id` was the same in the response body, the pod log, the line stored in Loki, and the trace returned by Tempo (`/api/traces/<id>` → 200)
- Grafana provisioned the derived field on the Loki datasource
- the link was not clicked in the Grafana UI

## What Is Intentionally Simplified

- The control plane uses `kubectl` instead of `client-go`.
- The control plane runs outside the cluster for now.
- Storage is in-memory or JSON file backed.
- Generated manifests cover the common happy path, not every Kubernetes option.
- There is no API authentication or authorization yet for local development; production Cloud Run access must be authenticated.
- There is no deployment rollback or progressive delivery strategy yet.
- Image publishing and cluster image loading are manual prerequisites.
- The development Kubernetes path is the only fully validated workload and observability runtime so far.
- GKE operator access is verified through the DNS endpoint; production workloads, observability, and Cloud Run runtime validation remain required. Staging is documented as a future TODO.

## Summary

The project deliberately starts small: one control plane, one workload, one local cluster, and one observability stack. The important design choice is not that every production feature exists today. The important choice is that the local system already has the same conceptual boundaries a production platform would need: a platform API, a managed workload, a cluster runtime, telemetry pipelines, and clear future paths for IAM, networking, and deployment hardening.
