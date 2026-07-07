#!/usr/bin/env bash
set -euo pipefail

KUBE_CONTEXT="${KUBE_CONTEXT:-}"
FASTAPI_SERVICE_IMAGE="${FASTAPI_SERVICE_IMAGE:-}"
OBSERVABILITY_OVERLAY="${OBSERVABILITY_OVERLAY:-k8s/overlays/prod-observability}"
WORKLOAD_OVERLAY="${WORKLOAD_OVERLAY:-k8s/overlays/prod}"
TERRAFORM_PROD_DIR="${TERRAFORM_PROD_DIR:-infra/terraform/environments/prod}"
APPLY_OBSERVABILITY=true
APPLY_WORKLOAD=true
WAIT_FOR_ROLLOUT=true
DRY_RUN=false

usage() {
  cat <<'USAGE'
Usage: ./scripts/deploy-production-platform.sh [options]

Applies the production observability stack and fastapi-service workload overlay
to GKE. Run this from a machine that can reach the private GKE Kubernetes API.

Options:
  --context <name>          kubectl context to use.
  --fastapi-image <image>   Published Artifact Registry fastapi-service image.
  --skip-observability      Do not apply the production observability overlay.
  --skip-workload           Do not apply the production workload overlay.
  --no-wait                 Do not wait for rollouts.
  --dry-run                 Render/apply with kubectl server-side dry-run.
  -h, --help                Show this help.

Environment equivalents:
  KUBE_CONTEXT, FASTAPI_SERVICE_IMAGE, OBSERVABILITY_OVERLAY, WORKLOAD_OVERLAY, TERRAFORM_PROD_DIR

If --fastapi-image is not set, the script tries:
  terraform -chdir=infra/terraform/environments/prod output -raw fastapi_service_container_image
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --context)
      KUBE_CONTEXT="${2:-}"
      shift 2
      ;;
    --fastapi-image)
      FASTAPI_SERVICE_IMAGE="${2:-}"
      shift 2
      ;;
    --skip-observability)
      APPLY_OBSERVABILITY=false
      shift
      ;;
    --skip-workload)
      APPLY_WORKLOAD=false
      shift
      ;;
    --no-wait)
      WAIT_FOR_ROLLOUT=false
      shift
      ;;
    --dry-run)
      DRY_RUN=true
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

kubectl_args=()
if [[ -n "${KUBE_CONTEXT}" ]]; then
  kubectl_args+=(--context "${KUBE_CONTEXT}")
fi

run_kubectl() {
  printf '+ kubectl'
  printf ' %q' "${kubectl_args[@]}" "$@"
  printf '\n'
  kubectl "${kubectl_args[@]}" "$@"
}

if ! command -v kubectl >/dev/null; then
  echo "kubectl is required." >&2
  exit 1
fi

if [[ "${APPLY_WORKLOAD}" == "true" && -z "${FASTAPI_SERVICE_IMAGE}" ]]; then
  if command -v terraform >/dev/null && [[ -d "${TERRAFORM_PROD_DIR}" ]]; then
    FASTAPI_SERVICE_IMAGE="$(terraform -chdir="${TERRAFORM_PROD_DIR}" output -raw fastapi_service_container_image 2>/dev/null || true)"
  fi
fi

if [[ "${APPLY_WORKLOAD}" == "true" && -z "${FASTAPI_SERVICE_IMAGE}" ]]; then
  echo "FASTAPI_SERVICE_IMAGE is required. Publish images first or pass --fastapi-image." >&2
  exit 2
fi

dry_run_args=()
if [[ "${DRY_RUN}" == "true" ]]; then
  dry_run_args+=(--dry-run=server)
fi

echo "Deploying Service Launchpad production platform"
echo "  observability overlay: ${OBSERVABILITY_OVERLAY} (enabled: ${APPLY_OBSERVABILITY})"
echo "  workload overlay: ${WORKLOAD_OVERLAY} (enabled: ${APPLY_WORKLOAD})"
if [[ "${APPLY_WORKLOAD}" == "true" ]]; then
  echo "  fastapi-service image: ${FASTAPI_SERVICE_IMAGE}"
fi
if [[ -n "${KUBE_CONTEXT}" ]]; then
  echo "  kubectl context: ${KUBE_CONTEXT}"
fi

if [[ "${APPLY_OBSERVABILITY}" == "true" ]]; then
  run_kubectl apply -k "${OBSERVABILITY_OVERLAY}" "${dry_run_args[@]}"
fi

if [[ "${APPLY_WORKLOAD}" == "true" ]]; then
  run_kubectl apply -k "${WORKLOAD_OVERLAY}" "${dry_run_args[@]}"
  run_kubectl set image deployment/fastapi-service "fastapi-service=${FASTAPI_SERVICE_IMAGE}" -n service-launchpad-prod "${dry_run_args[@]}"
fi

if [[ "${DRY_RUN}" == "true" || "${WAIT_FOR_ROLLOUT}" == "false" ]]; then
  exit 0
fi

if [[ "${APPLY_OBSERVABILITY}" == "true" ]]; then
  run_kubectl rollout status deployment/victoriametrics -n service-launchpad-observability --timeout=180s
  run_kubectl rollout status deployment/mimir -n service-launchpad-observability --timeout=240s
  run_kubectl rollout status deployment/vmagent -n service-launchpad-observability --timeout=180s
  run_kubectl rollout status deployment/kube-state-metrics -n service-launchpad-observability --timeout=180s
  run_kubectl rollout status deployment/tempo -n service-launchpad-observability --timeout=180s
  run_kubectl rollout status deployment/loki -n service-launchpad-observability --timeout=180s
  run_kubectl rollout status daemonset/promtail -n service-launchpad-observability --timeout=180s
  run_kubectl rollout status deployment/grafana -n service-launchpad-observability --timeout=180s
fi

if [[ "${APPLY_WORKLOAD}" == "true" ]]; then
  run_kubectl rollout status deployment/fastapi-service -n service-launchpad-prod --timeout=180s
  run_kubectl get deployment,service,hpa -n service-launchpad-prod
fi

echo "Production platform deploy checks completed."
echo "Private Grafana access:"
echo "  kubectl ${kubectl_args[*]} port-forward svc/grafana 3000:3000 -n service-launchpad-observability"
