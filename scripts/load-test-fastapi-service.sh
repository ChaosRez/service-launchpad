#!/usr/bin/env bash

set -euo pipefail

TEST_NAMESPACE="${TEST_NAMESPACE:-service-launchpad-dev}"
OBSERVABILITY_NAMESPACE="${OBSERVABILITY_NAMESPACE:-service-launchpad-observability}"
SCRIPT_PATH="${SCRIPT_PATH:-loadtests/k6/fastapi-service.js}"
BASE_URL="${BASE_URL:-}"
RUNTIME_PROFILE="${RUNTIME_PROFILE:-long}"
RATE="${RATE:-20}"
DURATION="${DURATION:-5m}"
PRE_ALLOCATED_VUS="${PRE_ALLOCATED_VUS:-20}"
MAX_VUS="${MAX_VUS:-200}"
WAIT_TIMEOUT="${WAIT_TIMEOUT:-15m}"
K6_IMAGE="${K6_IMAGE:-grafana/k6:latest}"
TEST_ID="${TEST_ID:-fastapi-$(date -u +%Y%m%d%H%M%S)}"
KEEP_RESOURCES="false"

usage() {
  cat <<'EOF'
Usage: ./scripts/load-test-fastapi-service.sh [options]

Runs a one-off in-cluster k6 Job against the fastapi-service and streams
k6 metrics to VictoriaMetrics via Prometheus remote write.

Options:
  --test-namespace <name>         Namespace containing fastapi-service
                                  (default: service-launchpad-dev)
  --observability-namespace <n>   Namespace containing VictoriaMetrics
                                  (default: service-launchpad-observability)
  --base-url <url>                Full service base URL inside the cluster
                                  (default: http://fastapi-service.<ns>.svc.cluster.local:8000)
  --profile <name>                runtime_profile to send: short|medium|long
                                  (default: long)
  --rate <n>                      Iterations to start per second
                                  (default: 20)
  --duration <time>               Test duration, e.g. 5m, 90s
                                  (default: 5m)
  --pre-allocated-vus <n>         Initial VU pool for k6
                                  (default: 20)
  --max-vus <n>                   Max VUs k6 may allocate
                                  (default: 200)
  --test-id <value>               Logical test identifier shown in metrics
                                  (default: fastapi-<utc timestamp>)
  --k6-image <image>              k6 runner image
                                  (default: grafana/k6:latest)
  --script-path <path>            Path to the k6 script in this repo
                                  (default: loadtests/k6/fastapi-service.js)
  --wait-timeout <time>           Job completion timeout
                                  (default: 15m)
  --keep                          Keep the Job and ConfigMap after a successful run
  --help                          Show this help text
EOF
}

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "Missing required command: $1" >&2
    exit 1
  fi
}

sanitize_name() {
  printf '%s' "$1" \
    | tr '[:upper:]_' '[:lower:]-' \
    | sed -E 's/[^a-z0-9-]+/-/g; s/^-+//; s/-+$//; s/-+/-/g' \
    | cut -c1-40
}

cleanup() {
  if [[ "${KEEP_RESOURCES}" == "true" ]]; then
    return
  fi

  if [[ "${JOB_CREATED:-false}" == "true" ]] && [[ "${RUN_SUCCESS:-false}" == "true" ]]; then
    kubectl delete job "${JOB_NAME}" -n "${TEST_NAMESPACE}" --ignore-not-found >/dev/null
    kubectl delete configmap "${CONFIGMAP_NAME}" -n "${TEST_NAMESPACE}" --ignore-not-found >/dev/null
  fi
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --test-namespace)
      TEST_NAMESPACE="$2"
      shift 2
      ;;
    --observability-namespace)
      OBSERVABILITY_NAMESPACE="$2"
      shift 2
      ;;
    --base-url)
      BASE_URL="$2"
      shift 2
      ;;
    --profile)
      RUNTIME_PROFILE="$2"
      shift 2
      ;;
    --rate)
      RATE="$2"
      shift 2
      ;;
    --duration)
      DURATION="$2"
      shift 2
      ;;
    --pre-allocated-vus)
      PRE_ALLOCATED_VUS="$2"
      shift 2
      ;;
    --max-vus)
      MAX_VUS="$2"
      shift 2
      ;;
    --test-id)
      TEST_ID="$2"
      shift 2
      ;;
    --k6-image)
      K6_IMAGE="$2"
      shift 2
      ;;
    --script-path)
      SCRIPT_PATH="$2"
      shift 2
      ;;
    --wait-timeout)
      WAIT_TIMEOUT="$2"
      shift 2
      ;;
    --keep)
      KEEP_RESOURCES="true"
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

require_command kubectl

if [[ ! -f "${SCRIPT_PATH}" ]]; then
  echo "k6 script not found: ${SCRIPT_PATH}" >&2
  exit 1
fi

if [[ -z "${BASE_URL}" ]]; then
  BASE_URL="http://fastapi-service.${TEST_NAMESPACE}.svc.cluster.local:8000"
fi

SAFE_TEST_ID="$(sanitize_name "${TEST_ID}")"
if [[ -z "${SAFE_TEST_ID}" ]]; then
  echo "Unable to derive a Kubernetes-safe name from test id: ${TEST_ID}" >&2
  exit 1
fi

CONFIGMAP_NAME="k6-script-${SAFE_TEST_ID}"
JOB_NAME="k6-loadtest-${SAFE_TEST_ID}"
REMOTE_WRITE_URL="http://vmagent.${OBSERVABILITY_NAMESPACE}.svc.cluster.local:8429/api/v1/write"
RUN_SUCCESS="false"
JOB_CREATED="false"

trap cleanup EXIT

echo "k6 load test configuration:"
echo "  test_namespace: ${TEST_NAMESPACE}"
echo "  observability_namespace: ${OBSERVABILITY_NAMESPACE}"
echo "  base_url: ${BASE_URL}"
echo "  runtime_profile: ${RUNTIME_PROFILE}"
echo "  rate_per_second: ${RATE}"
echo "  duration: ${DURATION}"
echo "  pre_allocated_vus: ${PRE_ALLOCATED_VUS}"
echo "  max_vus: ${MAX_VUS}"
echo "  test_id: ${TEST_ID}"
echo "  k6_image: ${K6_IMAGE}"
echo

kubectl create configmap "${CONFIGMAP_NAME}" \
  -n "${TEST_NAMESPACE}" \
  --from-file=fastapi-service.js="${SCRIPT_PATH}" \
  --dry-run=client \
  -o yaml \
  | kubectl apply -f -

kubectl delete job "${JOB_NAME}" -n "${TEST_NAMESPACE}" --ignore-not-found >/dev/null

kubectl apply -f - <<EOF
apiVersion: batch/v1
kind: Job
metadata:
  name: ${JOB_NAME}
  namespace: ${TEST_NAMESPACE}
  labels:
    app.kubernetes.io/name: k6-loadtest
    app.kubernetes.io/component: load-generator
    app.kubernetes.io/part-of: service-launchpad
    service-launchpad/test-id: ${SAFE_TEST_ID}
spec:
  backoffLimit: 0
  ttlSecondsAfterFinished: 3600
  template:
    metadata:
      labels:
        app.kubernetes.io/name: k6-loadtest
        app.kubernetes.io/component: load-generator
        app.kubernetes.io/part-of: service-launchpad
        service-launchpad/test-id: ${SAFE_TEST_ID}
    spec:
      restartPolicy: Never
      containers:
        - name: k6
          image: ${K6_IMAGE}
          imagePullPolicy: IfNotPresent
          command:
            - k6
          args:
            - run
            - -o
            - experimental-prometheus-rw
            - /scripts/fastapi-service.js
          env:
            - name: BASE_URL
              value: "${BASE_URL}"
            - name: RUNTIME_PROFILE
              value: "${RUNTIME_PROFILE}"
            - name: RATE
              value: "${RATE}"
            - name: DURATION
              value: "${DURATION}"
            - name: PRE_ALLOCATED_VUS
              value: "${PRE_ALLOCATED_VUS}"
            - name: MAX_VUS
              value: "${MAX_VUS}"
            - name: TESTID
              value: "${TEST_ID}"
            - name: K6_PROMETHEUS_RW_SERVER_URL
              value: "${REMOTE_WRITE_URL}"
            - name: K6_PROMETHEUS_RW_TREND_STATS
              value: "p(50),p(90),p(95),p(99),min,max"
            - name: K6_PROMETHEUS_RW_PUSH_INTERVAL
              value: "5s"
            - name: K6_PROMETHEUS_RW_STALE_MARKERS
              value: "true"
          volumeMounts:
            - name: script
              mountPath: /scripts
              readOnly: true
      volumes:
        - name: script
          configMap:
            name: ${CONFIGMAP_NAME}
EOF

JOB_CREATED="true"

echo "Created Job ${JOB_NAME}."
echo "Watch autoscaling:"
echo "  kubectl get hpa -n ${TEST_NAMESPACE} -w"
echo "  kubectl get pods -n ${TEST_NAMESPACE} -w"
echo "  kubectl top pods -n ${TEST_NAMESPACE}"
echo

kubectl wait --for=condition=Ready "pod" -n "${TEST_NAMESPACE}" -l "job-name=${JOB_NAME}" --timeout=120s >/dev/null
kubectl logs -n "${TEST_NAMESPACE}" -f "job/${JOB_NAME}"

if kubectl wait --for=condition=complete "job/${JOB_NAME}" -n "${TEST_NAMESPACE}" --timeout="${WAIT_TIMEOUT}" >/dev/null; then
  RUN_SUCCESS="true"
  echo
  echo "k6 Job completed successfully."
  echo "Grafana tips:"
  echo "  - FastAPI dashboard: watch service latency, throughput, and HPA behavior"
echo "  - Explore / VictoriaMetrics or Mimir: query k6_* metrics filtered by testid=\"${TEST_ID}\""
  echo "  - Explore / Tempo: traces still come from fastapi-service itself"
else
  echo
  echo "k6 Job did not reach Completed within ${WAIT_TIMEOUT}." >&2
  echo "Inspect it with:" >&2
  echo "  kubectl describe job ${JOB_NAME} -n ${TEST_NAMESPACE}" >&2
  echo "  kubectl get pods -n ${TEST_NAMESPACE} -l job-name=${JOB_NAME}" >&2
  exit 1
fi
