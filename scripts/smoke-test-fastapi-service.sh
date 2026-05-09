#!/usr/bin/env bash

set -euo pipefail

PROFILE="${MINIKUBE_PROFILE:-service-launchpad}"
NAMESPACE="${K8S_NAMESPACE:-service-launchpad-dev}"
IMAGE="${FASTAPI_SERVICE_IMAGE:-service-launchpad/fastapi-service:dev}"
LOCAL_PORT="${FASTAPI_SERVICE_LOCAL_PORT:-8000}"
CONTROL_PLANE_PORT="${CONTROL_PLANE_PORT:-8080}"
CONTROL_PLANE_ADDR="${CONTROL_PLANE_LISTEN_ADDR:-127.0.0.1:${CONTROL_PLANE_PORT}}"
CONTROL_PLANE_URL="http://${CONTROL_PLANE_ADDR}"
SERVICE_NAME="fastapi-service"
STORE_PATH="${CONTROL_PLANE_STORE_PATH:-$(mktemp /tmp/control-plane-store.XXXXXX.json)}"
CONTROL_PLANE_LOG="${CONTROL_PLANE_LOG:-/tmp/control-plane-smoke-test.log}"
PORT_FORWARD_LOG="${PORT_FORWARD_LOG:-/tmp/fastapi-service-port-forward.log}"

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

assert_contains() {
  local response="$1"
  local expected="$2"

  if [[ "${response}" != *"${expected}"* ]]; then
    echo "Expected response to contain: ${expected}" >&2
    echo "Actual response: ${response}" >&2
    exit 1
  fi
}

trap cleanup EXIT

require_command minikube
require_command kubectl
require_command docker
require_command curl
require_command go

echo "==> Starting Minikube"
./scripts/bootstrap-minikube.sh --docker-env

echo "==> Pointing Docker at Minikube"
eval "$(minikube -p "${PROFILE}" docker-env)"

echo "==> Building fastapi-service image"
docker build -t "${IMAGE}" services/fastapi-service

echo "==> Starting control plane on ${CONTROL_PLANE_ADDR}"
CONTROL_PLANE_LISTEN_ADDR="${CONTROL_PLANE_ADDR}" \
CONTROL_PLANE_STORE_PATH="${STORE_PATH}" \
CONTROL_PLANE_KUBECTL_CONTEXT="${PROFILE}" \
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

echo "==> Control-plane smoke test passed"
