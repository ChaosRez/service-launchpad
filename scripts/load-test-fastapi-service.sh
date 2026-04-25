#!/usr/bin/env bash

set -euo pipefail

BASE_URL="${FASTAPI_SERVICE_BASE_URL:-http://127.0.0.1:8000}"
TOTAL_REQUESTS="${TOTAL_REQUESTS:-2400}"
CONCURRENCY="${CONCURRENCY:-120}"
ROUNDS="${ROUNDS:-3}"
RUNTIME_PROFILE="${RUNTIME_PROFILE:-long}"

usage() {
  cat <<'EOF'
Usage: ./scripts/load-test-fastapi-service.sh [options]

Options:
  --base-url <url>        Base URL for the service (default: http://127.0.0.1:8000)
  --requests <count>      Requests per round (default: 2400)
  --concurrency <count>   Parallel requests at a time (default: 120)
  --rounds <count>        Number of rounds to run (default: 3)
  --profile <name>        runtime_profile to send (default: long)
  --help                  Show this help text
EOF
}

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "Missing required command: $1" >&2
    exit 1
  fi
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --base-url)
      BASE_URL="$2"
      shift 2
      ;;
    --requests)
      TOTAL_REQUESTS="$2"
      shift 2
      ;;
    --concurrency)
      CONCURRENCY="$2"
      shift 2
      ;;
    --rounds)
      ROUNDS="$2"
      shift 2
      ;;
    --profile)
      RUNTIME_PROFILE="$2"
      shift 2
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

require_command curl
require_command xargs

send_request() {
  curl -fsS -X POST "${BASE_URL}/v1/chat/completions" \
    -H "Content-Type: application/json" \
    -d "{\"runtime_profile\":\"${RUNTIME_PROFILE}\"}" >/dev/null
}

export BASE_URL
export RUNTIME_PROFILE
export -f send_request

echo "Load test configuration:"
echo "  base_url: ${BASE_URL}"
echo "  requests_per_round: ${TOTAL_REQUESTS}"
echo "  concurrency: ${CONCURRENCY}"
echo "  rounds: ${ROUNDS}"
echo "  runtime_profile: ${RUNTIME_PROFILE}"
echo

for round in $(seq 1 "${ROUNDS}"); do
  echo "==> Starting round ${round}/${ROUNDS}"
  seq 1 "${TOTAL_REQUESTS}" | xargs -n 1 -P "${CONCURRENCY}" bash -lc 'send_request'
  echo "==> Completed round ${round}/${ROUNDS}"
done

echo "Load test finished."
