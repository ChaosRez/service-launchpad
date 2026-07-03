#!/usr/bin/env bash
set -euo pipefail

PROJECT_ID="${PROJECT_ID:-${GOOGLE_CLOUD_PROJECT:-}}"
REGION="${REGION:-europe-west10}"
REPOSITORY="${REPOSITORY:-service-launchpad}"
TAG="${TAG:-}"
PLATFORM="${PLATFORM:-linux/amd64}"
ALSO_TAGS="${ALSO_TAGS:-prod}"
CONFIGURE_DOCKER=true
DRY_RUN=false

usage() {
  cat <<'USAGE'
Usage: ./scripts/publish-production-images.sh [options]

Builds and publishes the production control-plane and fastapi-service images
to Google Artifact Registry.

Options:
  --project-id <id>        GCP project ID. Defaults to PROJECT_ID or GOOGLE_CLOUD_PROJECT.
  --region <region>       Artifact Registry region. Default: europe-west10.
  --repository <name>     Artifact Registry Docker repository. Default: service-launchpad.
  --tag <tag>             Primary image tag. Default: git-<short-sha>.
  --also-tag <tags>       Comma-separated extra tags. Default: prod. Use "" to disable.
  --platform <platform>   Build platform. Default: linux/amd64.
  --skip-docker-auth      Skip gcloud auth configure-docker.
  --dry-run               Print commands without running them.
  -h, --help              Show this help.

Environment equivalents:
  PROJECT_ID, GOOGLE_CLOUD_PROJECT, REGION, REPOSITORY, TAG, ALSO_TAGS, PLATFORM
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --project-id)
      PROJECT_ID="${2:-}"
      shift 2
      ;;
    --region)
      REGION="${2:-}"
      shift 2
      ;;
    --repository)
      REPOSITORY="${2:-}"
      shift 2
      ;;
    --tag)
      TAG="${2:-}"
      shift 2
      ;;
    --also-tag)
      ALSO_TAGS="${2:-}"
      shift 2
      ;;
    --platform)
      PLATFORM="${2:-}"
      shift 2
      ;;
    --skip-docker-auth)
      CONFIGURE_DOCKER=false
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

if [[ -z "${PROJECT_ID}" ]]; then
  echo "PROJECT_ID is required. Pass --project-id or set PROJECT_ID/GOOGLE_CLOUD_PROJECT." >&2
  exit 2
fi

if [[ -z "${TAG}" ]]; then
  git_sha="$(git rev-parse --short=12 HEAD)"
  TAG="git-${git_sha}"
fi

registry="${REGION}-docker.pkg.dev"
image_prefix="${registry}/${PROJECT_ID}/${REPOSITORY}"
control_plane_image="${image_prefix}/control-plane:${TAG}"
fastapi_service_image="${image_prefix}/fastapi-service:${TAG}"

split_tags() {
  local raw="$1"
  local -a result=()
  local item
  IFS=',' read -r -a parts <<<"${raw}"
  for item in "${parts[@]}"; do
    item="${item#"${item%%[![:space:]]*}"}"
    item="${item%"${item##*[![:space:]]}"}"
    if [[ -n "${item}" ]]; then
      result+=("${item}")
    fi
  done
  printf '%s\n' "${result[@]}"
}

run() {
  printf '+'
  printf ' %q' "$@"
  printf '\n'
  if [[ "${DRY_RUN}" == "false" ]]; then
    "$@"
  fi
}

if [[ "${DRY_RUN}" == "false" ]]; then
  command -v docker >/dev/null || {
    echo "docker is required." >&2
    exit 1
  }
  if [[ "${CONFIGURE_DOCKER}" == "true" ]]; then
    command -v gcloud >/dev/null || {
      echo "gcloud is required unless --skip-docker-auth is used." >&2
      exit 1
    }
  fi
fi

echo "Publishing Service Launchpad production images"
echo "  project: ${PROJECT_ID}"
echo "  region: ${REGION}"
echo "  repository: ${REPOSITORY}"
echo "  platform: ${PLATFORM}"
echo "  primary tag: ${TAG}"
echo "  control-plane: ${control_plane_image}"
echo "  fastapi-service: ${fastapi_service_image}"

if [[ "${CONFIGURE_DOCKER}" == "true" ]]; then
  run gcloud auth configure-docker "${registry}" --quiet
fi

control_tags=(-t "${control_plane_image}")
fastapi_tags=(-t "${fastapi_service_image}")
while IFS= read -r extra_tag; do
  control_tags+=(-t "${image_prefix}/control-plane:${extra_tag}")
  fastapi_tags+=(-t "${image_prefix}/fastapi-service:${extra_tag}")
done < <(split_tags "${ALSO_TAGS}")

run docker buildx build \
  --platform "${PLATFORM}" \
  -f services/control-plane/Dockerfile \
  "${control_tags[@]}" \
  --push \
  .

run docker buildx build \
  --platform "${PLATFORM}" \
  "${fastapi_tags[@]}" \
  --push \
  services/fastapi-service

echo "Published images:"
echo "  CONTROL_PLANE_IMAGE=${control_plane_image}"
echo "  FASTAPI_SERVICE_IMAGE=${fastapi_service_image}"
