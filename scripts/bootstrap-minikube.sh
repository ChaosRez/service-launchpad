#!/usr/bin/env bash

set -euo pipefail

PROFILE="${MINIKUBE_PROFILE:-service-launchpad}"
DRIVER="${MINIKUBE_DRIVER:-docker}"
KUBERNETES_VERSION="${MINIKUBE_KUBERNETES_VERSION:-stable}"
CPUS="${MINIKUBE_CPUS:-2}"
MEMORY="${MINIKUBE_MEMORY:-4096}"
ENABLE_METRICS_SERVER="${ENABLE_METRICS_SERVER:-true}"
PRINT_DOCKER_ENV="${PRINT_DOCKER_ENV:-false}"

usage() {
  cat <<'EOF'
Usage: ./scripts/bootstrap-minikube.sh [options]

Options:
  --profile <name>           Minikube profile name
  --driver <name>            Minikube driver (default: docker)
  --kubernetes-version <v>   Kubernetes version (default: stable)
  --cpus <count>             CPU allocation (default: 2)
  --memory <mb>              Memory allocation in MB (default: 4096)
  --skip-metrics-server      Do not enable the metrics-server addon
  --docker-env               Print the command to point Docker at Minikube
  --help                     Show this help text
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
    --profile)
      PROFILE="$2"
      shift 2
      ;;
    --driver)
      DRIVER="$2"
      shift 2
      ;;
    --kubernetes-version)
      KUBERNETES_VERSION="$2"
      shift 2
      ;;
    --cpus)
      CPUS="$2"
      shift 2
      ;;
    --memory)
      MEMORY="$2"
      shift 2
      ;;
    --skip-metrics-server)
      ENABLE_METRICS_SERVER="false"
      shift
      ;;
    --docker-env)
      PRINT_DOCKER_ENV="true"
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

require_command minikube
require_command kubectl

echo "Starting Minikube profile '${PROFILE}'..."
minikube start \
  --profile "${PROFILE}" \
  --driver "${DRIVER}" \
  --kubernetes-version "${KUBERNETES_VERSION}" \
  --cpus "${CPUS}" \
  --memory "${MEMORY}"

echo "Switching kubectl context to '${PROFILE}'..."
minikube update-context --profile "${PROFILE}"

if [[ "${ENABLE_METRICS_SERVER}" == "true" ]]; then
  echo "Enabling metrics-server addon..."
  minikube addons enable metrics-server --profile "${PROFILE}"
fi

echo "Minikube is ready."
echo "Status:"
minikube status --profile "${PROFILE}"

if [[ "${PRINT_DOCKER_ENV}" == "true" ]]; then
  echo
  echo "Run this to build images directly inside Minikube's Docker daemon:"
  echo "eval \$(minikube -p ${PROFILE} docker-env)"
fi
