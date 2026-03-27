# Useful Minikube Commands 🐳

These are handy to run manually while learning the local Kubernetes flow for this project.

## Start And Stop

```bash
minikube start --profile service-launchpad --driver docker --cpus 2 --memory 4096
minikube stop --profile service-launchpad
minikube delete --profile service-launchpad
```

## Context And Cluster Status

```bash
minikube update-context --profile service-launchpad
minikube status --profile service-launchpad
kubectl config current-context
kubectl get nodes
kubectl get pods -A
```

## Addons

```bash
minikube addons enable metrics-server --profile service-launchpad
minikube addons list --profile service-launchpad
```

`metrics-server` matters for this project because the HPA demo later depends on resource metrics.

## Build Images Inside Minikube

```bash
eval $(minikube -p service-launchpad docker-env)
docker build -t service-launchpad/fastapi-service:dev services/fastapi-service
```

This is useful because Minikube can then pull the image from its own local Docker daemon without pushing to a remote registry.

## Basic Kubernetes Inspection

```bash
kubectl get namespaces
kubectl get deployments -A
kubectl get services -A
kubectl describe pod <pod-name> -n <namespace>
kubectl logs <pod-name> -n <namespace>
```

## Accessing Services

```bash
minikube service <service-name> -n <namespace>
kubectl port-forward svc/<service-name> 8000:8000 -n <namespace>
```

Use `minikube service` for quick browser access and `kubectl port-forward` when you want a stable local port for `curl` or load testing.

## Debugging

```bash
kubectl get events -A --sort-by=.lastTimestamp
minikube logs --profile service-launchpad
minikube ssh --profile service-launchpad
```

## Cleanup And Reset

```bash
kubectl delete -f k8s/base/
minikube stop --profile service-launchpad
minikube delete --profile service-launchpad
```

Use `delete` only when you want to fully reset the local cluster state.
