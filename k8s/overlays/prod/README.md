# Production Workload Overlay

This overlay adapts the local `fastapi-service` base manifests for the `GKE production` path.

It changes:

- namespace: `service-launchpad-prod`
- namespace environment label: `prod`
- image source: Artifact Registry instead of the Minikube-local `service-launchpad/fastapi-service:dev`

Before applying it to a real production cluster, set the image to the value printed by `scripts/publish-production-images.sh` or by the Terraform `fastapi_service_container_image` output.

Example:

```bash
FASTAPI_SERVICE_IMAGE="$(terraform -chdir=infra/terraform/environments/prod output -raw fastapi_service_container_image)"
cd k8s/overlays/prod
kustomize edit set image "service-launchpad/fastapi-service=${FASTAPI_SERVICE_IMAGE}"
kubectl kustomize .
```

Task 31 owns applying this overlay to `GKE production` after the private cluster access path is selected.
