# fastapi-service Helm chart

Packages `services/fastapi-service` as a Helm chart. With default values it renders the same `ConfigMap`, `Deployment`, `Service`, and `HorizontalPodAutoscaler` as `kubectl kustomize k8s/base`. Kustomize stays the main path for the dev and prod overlays. See Decision 15 in [`docs/decisions.md`](../../docs/decisions.md) for why both exist.

## Differences from `k8s/base`

- **No `Namespace` object.** Helm installs into the release namespace (`-n <ns> --create-namespace`). It does not own the namespace, so `helm uninstall` never deletes it.
- **No `spec.replicas` while the HPA is enabled.** Otherwise every `helm upgrade` would reset the replica count the HPA picked. Set `autoscaling.enabled=false` to use `replicaCount`.
- **Extra labels:** `app.kubernetes.io/instance`, `managed-by`, `version`, and `helm.sh/chart`. The selector is `name` + `instance`, so two releases can share a namespace if their names differ.
- **`checksum/config` pod annotation.** A changed value in the ConfigMap rolls the pods. With Kustomize, a ConfigMap edit does not restart pods, because `envFrom` is only read when the container starts.

## Values

| Key | Default | Purpose |
| --- | --- | --- |
| `image.repository` / `image.tag` / `image.pullPolicy` | `service-launchpad/fastapi-service` / `dev` / `IfNotPresent` | Container image |
| `replicaCount` | `1` | Only used when `autoscaling.enabled=false` |
| `autoscaling.enabled` / `minReplicas` / `maxReplicas` / `targetCPUUtilizationPercentage` | `true` / `1` / `5` / `60` | HPA |
| `probes.startup` / `probes.readiness` / `probes.liveness` | same as `k8s/base` | Full probe specs; set one to `null` to drop it |
| `resources` | requests `100m`/`128Mi`, limits `1000m`/`256Mi` | Container resources |
| `config.model` / `config.defaultProfile` / `config.logLevel` | `tinyllama-1.1b-chat-q4_k_m` / `medium` / `INFO` | App settings (ConfigMap) |
| `config.otel.serviceName` / `config.otel.tracesEndpoint` | `fastapi-service` / in-cluster Tempo OTLP HTTP | Trace export; an empty endpoint disables export |
| `service.type` / `service.port` | `ClusterIP` / `8000` | Service |

`values-prod.yaml` mirrors `k8s/overlays/prod`: the Artifact Registry image and smaller requests. CI renders and validates it. It has **not** been installed on GKE.

## Commands

Run these against a development cluster. Always name the context explicitly, because the default kubeconfig context on the workstation may point at production GKE.

```bash
# Static checks (also run in CI)
helm lint charts/fastapi-service --strict
helm template fastapi-service charts/fastapi-service -n service-launchpad-dev | kubeconform -strict -summary -

# Install, change a value, roll back
helm install fastapi-service charts/fastapi-service \
  --kube-context dev-k3s -n service-launchpad-dev --create-namespace --wait
helm upgrade fastapi-service charts/fastapi-service \
  --kube-context dev-k3s -n service-launchpad-dev --reuse-values \
  --set autoscaling.maxReplicas=3 --wait
helm history fastapi-service --kube-context dev-k3s -n service-launchpad-dev
helm rollback fastapi-service 1 --kube-context dev-k3s -n service-launchpad-dev --wait
```

Do not install the chart into a namespace where `kubectl apply -k k8s/base` already created the same resources. Helm refuses to adopt objects it does not own. Pick one tool per namespace.

## Verification record

On 2026-11-08, on a throwaway local k3d cluster (k3s v1.35.5, separate kubeconfig, deleted afterwards). This was not the shared `dev-k3s` host and not GKE.

- `helm install` → revision 1 deployed, 1/1 pod Ready, HPA min 1 / max 5.
- `helm upgrade --set autoscaling.maxReplicas=3 --set config.defaultProfile=short` → revision 2: HPA max 3, ConfigMap updated, pods rolled to a new ReplicaSet through the config checksum.
- `helm rollback fastapi-service 1` → revision 3 ("Rollback to 1"): HPA max 5, profile `medium`, pods back on the original ReplicaSet hash.
