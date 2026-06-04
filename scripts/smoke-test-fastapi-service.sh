#!/usr/bin/env bash

set -euo pipefail

PROFILE="${MINIKUBE_PROFILE:-service-launchpad}"
NAMESPACE="${K8S_NAMESPACE:-service-launchpad-dev}"
IMAGE="${FASTAPI_SERVICE_IMAGE:-service-launchpad/fastapi-service:dev}"
LOCAL_PORT="${FASTAPI_SERVICE_LOCAL_PORT:-8000}"
CONTROL_PLANE_PORT="${CONTROL_PLANE_PORT:-8080}"
SERVICE_NAME="fastapi-service"
STORE_PATH="${CONTROL_PLANE_STORE_PATH:-$(mktemp /tmp/control-plane-store.XXXXXX.json)}"
CONTROL_PLANE_LOG="${CONTROL_PLANE_LOG:-/tmp/control-plane-smoke-test.log}"
PORT_FORWARD_LOG="${PORT_FORWARD_LOG:-/tmp/fastapi-service-port-forward.log}"
SKIP_BOOTSTRAP="${SKIP_BOOTSTRAP:-false}"
SKIP_IMAGE_BUILD="${SKIP_IMAGE_BUILD:-false}"
CLEANUP_RESOURCES="${CLEANUP_RESOURCES:-false}"

usage() {
  cat <<'EOF'
Usage: ./scripts/smoke-test-fastapi-service.sh [options]

Options:
  --profile <name>               Minikube profile / kube context (default: service-launchpad)
  --namespace <name>             Target Kubernetes namespace (default: service-launchpad-dev)
  --image <name>                 fastapi-service image tag (default: service-launchpad/fastapi-service:dev)
  --local-port <port>            Local port-forward port (default: 8000)
  --control-plane-port <port>    Local control-plane port (default: 8080)
  --skip-bootstrap               Do not run scripts/bootstrap-minikube.sh
  --skip-image-build             Do not build the fastapi-service image
  --cleanup                      Delete the target namespace after the smoke test
  --help                         Show this help text
EOF
}

cleanup() {
  if [[ -n "${PORT_FORWARD_PID:-}" ]]; then
    kill "${PORT_FORWARD_PID}" >/dev/null 2>&1 || true
  fi
  if [[ -n "${CONTROL_PLANE_PID:-}" ]]; then
    kill "${CONTROL_PLANE_PID}" >/dev/null 2>&1 || true
    wait "${CONTROL_PLANE_PID}" >/dev/null 2>&1 || true
  fi
}

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "Missing required command: $1" >&2
    exit 1
  fi
}

wait_for_http() {
  local url="$1"
  local attempts="${2:-30}"

  for _ in $(seq 1 "${attempts}"); do
    if curl -fsS "${url}" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done

  echo "Timed out waiting for ${url}" >&2
  exit 1
}

http_available() {
  local url="$1"

  curl -fsS "${url}" >/dev/null 2>&1
}

assert_contains() {
  local response="$1"
  local expected="$2"

  if [[ "${response}" != *"${expected}"* ]]; then
    echo "Expected response to contain: ${expected}" >&2
    echo "Actual response: ${response}" >&2
    exit 1
  fi
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --profile)
      PROFILE="$2"
      shift 2
      ;;
    --namespace)
      NAMESPACE="$2"
      shift 2
      ;;
    --image)
      IMAGE="$2"
      shift 2
      ;;
    --local-port)
      LOCAL_PORT="$2"
      shift 2
      ;;
    --control-plane-port)
      CONTROL_PLANE_PORT="$2"
      shift 2
      ;;
    --skip-bootstrap)
      SKIP_BOOTSTRAP="true"
      shift
      ;;
    --skip-image-build)
      SKIP_IMAGE_BUILD="true"
      shift
      ;;
    --cleanup)
      CLEANUP_RESOURCES="true"
      shift
      ;;
    --help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      usage
      exit 1
      ;;
  esac
done

CONTROL_PLANE_ADDR="${CONTROL_PLANE_LISTEN_ADDR:-127.0.0.1:${CONTROL_PLANE_PORT}}"
CONTROL_PLANE_URL="http://${CONTROL_PLANE_ADDR}"

trap cleanup EXIT

require_command minikube
require_command kubectl
require_command docker
require_command curl
require_command go

if [[ "${SKIP_BOOTSTRAP}" == "true" ]]; then
  echo "==> Skipping Minikube bootstrap"
else
  echo "==> Starting Minikube"
  ./scripts/bootstrap-minikube.sh --profile "${PROFILE}" --docker-env
fi

if [[ "${SKIP_IMAGE_BUILD}" == "true" ]]; then
  echo "==> Skipping fastapi-service image build"
else
  echo "==> Pointing Docker at Minikube"
  eval "$(minikube -p "${PROFILE}" docker-env)"

  echo "==> Building fastapi-service image"
  docker build -t "${IMAGE}" services/fastapi-service
fi

if http_available "${CONTROL_PLANE_URL}/health"; then
  echo "Control plane already responds at ${CONTROL_PLANE_URL}/health" >&2
  echo "Use a different --control-plane-port, or stop the existing process before running this smoke test." >&2
  exit 1
fi

echo "==> Starting control plane on ${CONTROL_PLANE_ADDR}"
CONTROL_PLANE_LISTEN_ADDR="${CONTROL_PLANE_ADDR}" \
CONTROL_PLANE_STORE_PATH="${STORE_PATH}" \
CONTROL_PLANE_DEPLOYER_MODE="client-go" \
CONTROL_PLANE_KUBE_CONTEXT="${PROFILE}" \
CONTROL_PLANE_TARGET_NAMESPACE="${NAMESPACE}" \
go run ./services/control-plane >"${CONTROL_PLANE_LOG}" 2>&1 &
CONTROL_PLANE_PID=$!

wait_for_http "${CONTROL_PLANE_URL}/health" 45

echo "==> Registering service definition"
register_response="$(
  curl -fsS -X POST "${CONTROL_PLANE_URL}/services" \
    -H "Content-Type: application/json" \
    -d '{
      "name": "fastapi-service",
      "image": "'"${IMAGE}"'",
      "port": 8000,
      "replicas": 1,
      "autoscaling": {
        "enabled": true,
        "minReplicas": 1,
        "maxReplicas": 5,
        "targetCpuUtilization": 60
      }
    }'
)"
assert_contains "${register_response}" "\"name\":\"fastapi-service\""

echo "==> Validating rendered manifests"
manifest_response="$(curl -fsS "${CONTROL_PLANE_URL}/services/${SERVICE_NAME}/manifests")"
assert_contains "${manifest_response}" "\"namespace\":\"${NAMESPACE}\""
assert_contains "${manifest_response}" "\"configMap\""
assert_contains "${manifest_response}" "\"deployment\""
assert_contains "${manifest_response}" "\"service\""
assert_contains "${manifest_response}" "\"hpa\""

echo "==> Deploying through the control plane"
deploy_response="$(curl -fsS -X POST "${CONTROL_PLANE_URL}/services/${SERVICE_NAME}/deploy")"
assert_contains "${deploy_response}" "\"status\":\"applied\""
assert_contains "${deploy_response}" "\"deployer\":\"client-go\""
assert_contains "${deploy_response}" "\"Namespace/${NAMESPACE}\""
assert_contains "${deploy_response}" "\"Deployment/${SERVICE_NAME}\""
assert_contains "${deploy_response}" "\"Service/${SERVICE_NAME}\""
assert_contains "${deploy_response}" "\"HorizontalPodAutoscaler/${SERVICE_NAME}\""

echo "==> Waiting for Kubernetes rollout"
kubectl rollout status deployment/"${SERVICE_NAME}" -n "${NAMESPACE}" --timeout=180s

echo "==> Checking namespace, deployment, service, and HPA"
kubectl get namespace "${NAMESPACE}" >/dev/null
kubectl get deployment "${SERVICE_NAME}" -n "${NAMESPACE}" >/dev/null
kubectl get service "${SERVICE_NAME}" -n "${NAMESPACE}" >/dev/null
kubectl get hpa "${SERVICE_NAME}" -n "${NAMESPACE}" >/dev/null

echo "==> Starting port-forward on localhost:${LOCAL_PORT}"
kubectl port-forward svc/"${SERVICE_NAME}" "${LOCAL_PORT}:8000" -n "${NAMESPACE}" >"${PORT_FORWARD_LOG}" 2>&1 &
PORT_FORWARD_PID=$!

wait_for_http "http://127.0.0.1:${LOCAL_PORT}/health"

echo "==> Checking /health"
health_response="$(curl -fsS "http://127.0.0.1:${LOCAL_PORT}/health")"
assert_contains "${health_response}" "\"status\":\"ok\""

echo "==> Checking /ready"
ready_response="$(curl -fsS "http://127.0.0.1:${LOCAL_PORT}/ready")"
assert_contains "${ready_response}" "\"status\":\"ready\""

echo "==> Checking /v1/models"
models_response="$(curl -fsS "http://127.0.0.1:${LOCAL_PORT}/v1/models")"
assert_contains "${models_response}" "\"id\":\"tinyllama-1.1b-chat-q4_k_m\""

echo "==> Checking chat completion"
chat_response="$(
  curl -fsS -X POST "http://127.0.0.1:${LOCAL_PORT}/v1/chat/completions" \
    -H "Content-Type: application/json" \
    -d '{"runtime_profile":"short"}'
)"
assert_contains "${chat_response}" "\"object\":\"chat.completion\""
assert_contains "${chat_response}" "\"runtime_profile\":\"short\""

echo "==> Checking /metrics"
metrics_response="$(curl -fsS "http://127.0.0.1:${LOCAL_PORT}/metrics")"
assert_contains "${metrics_response}" "fastapi_service_requests_total"
assert_contains "${metrics_response}" "fastapi_service_request_duration_seconds"

if [[ "${CLEANUP_RESOURCES}" == "true" ]]; then
  echo "==> Cleaning up namespace ${NAMESPACE}"
  kubectl delete namespace "${NAMESPACE}" --ignore-not-found
fi

echo "==> Control-plane smoke test passed"
