#!/usr/bin/env bash

set -euo pipefail

PROFILE="${MINIKUBE_PROFILE:-service-launchpad}"
NAMESPACE="${K8S_NAMESPACE:-service-launchpad-dev}"
IMAGE="${FASTAPI_SERVICE_IMAGE:-service-launchpad/fastapi-service:dev}"
LOCAL_PORT="${FASTAPI_SERVICE_LOCAL_PORT:-8000}"
SERVICE_NAME="fastapi-service"

cleanup() {
  if [[ -n "${PORT_FORWARD_PID:-}" ]]; then
    kill "${PORT_FORWARD_PID}" >/dev/null 2>&1 || true
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
  local attempts=20

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

echo "==> Starting Minikube"
./scripts/bootstrap-minikube.sh --docker-env

echo "==> Pointing Docker at Minikube"
eval "$(minikube -p "${PROFILE}" docker-env)"

echo "==> Building fastapi-service image"
docker build -t "${IMAGE}" services/fastapi-service

echo "==> Applying Kubernetes manifests"
kubectl apply -k k8s/base

echo "==> Waiting for deployment rollout"
kubectl rollout status deployment/"${SERVICE_NAME}" -n "${NAMESPACE}" --timeout=180s

echo "==> Starting port-forward on localhost:${LOCAL_PORT}"
kubectl port-forward svc/"${SERVICE_NAME}" "${LOCAL_PORT}:8000" -n "${NAMESPACE}" >/tmp/fastapi-service-port-forward.log 2>&1 &
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

echo "==> Smoke test passed"
