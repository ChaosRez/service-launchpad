# Service Launchpad

Service Launchpad is a small internal-platform prototype for registering and operating services on Kubernetes.

- `Go` control plane for service registration and deployment workflows
- `FastAPI` workload that simulates a `llama.cpp` chat completion API
- `Minikube` as the default local Kubernetes path
- `GKE` path for cloud deployment
- `Terraform` for minimal `GCP` and `IAM` resources
- `Prometheus` and `Grafana` for observability
- `Victoria Metrics`, `Grafana Tempo`, and `Grafana` for observability with defined SLOs

