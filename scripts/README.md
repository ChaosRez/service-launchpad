# Scripts

This directory will hold local helper scripts for Minikube bootstrapping, deployment, and demo workflows.

## Available Scripts

- `bootstrap-minikube.sh`: starts a local Minikube cluster, updates the `kubectl` context, enables `metrics-server`, and can print the Docker environment command for local image builds

## Manual Minikube Commands

```bash
minikube start --profile service-launchpad --driver docker --cpus 2 --memory 4096
minikube update-context --profile service-launchpad
minikube addons enable metrics-server --profile service-launchpad
eval $(minikube -p service-launchpad docker-env)
minikube status --profile service-launchpad
kubectl get nodes
kubectl get pods -A
```
