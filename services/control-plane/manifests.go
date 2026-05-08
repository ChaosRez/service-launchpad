package main

// Deployment/Service/HPA rendering

func renderManifestBundle(def serviceDefinition, namespace string) manifestBundle {
	namespaceManifest := renderNamespaceManifest(namespace)
	configMap := renderServiceConfigMap(def, namespace)
	deployment := renderDeploymentManifest(def, namespace)
	service := renderServiceManifest(def, namespace)

	resourceManifests := []map[string]any{}
	if configMap != nil {
		resourceManifests = append(resourceManifests, configMap)
	}
	resourceManifests = append(resourceManifests, deployment, service)
	var hpa map[string]any
	if def.Autoscaling.Enabled {
		hpa = renderHPAManifest(def, namespace)
		resourceManifests = append(resourceManifests, hpa)
	}

	return manifestBundle{
		Namespace:         namespace,
		NamespaceManifest: namespaceManifest,
		ConfigMap:         configMap,
		Deployment:        deployment,
		Service:           service,
		HPA:               hpa,
		YAML:              renderYAMLDocuments(resourceManifests),
	}
}

func renderNamespaceManifest(namespace string) map[string]any {
	return map[string]any{
		"apiVersion": "v1",
		"kind":       "Namespace",
		"metadata": map[string]any{
			"name": namespace,
			"labels": map[string]any{
				"app.kubernetes.io/name":        "service-launchpad",
				"app.kubernetes.io/part-of":     "service-launchpad",
				"app.kubernetes.io/environment": "dev",
			},
		},
	}
}

func renderDeploymentManifest(def serviceDefinition, namespace string) map[string]any {
	labels := standardLabels(def.Name)
	container := map[string]any{
		"name":            def.Name,
		"image":           def.Image,
		"imagePullPolicy": "IfNotPresent",
		"ports": []any{
			map[string]any{
				"name":          "http",
				"containerPort": def.Port,
			},
		},
		"startupProbe": map[string]any{
			"httpGet": map[string]any{
				"path": "/ready",
				"port": "http",
			},
			"initialDelaySeconds": 2,
			"periodSeconds":       5,
			"timeoutSeconds":      2,
			"failureThreshold":    12,
		},
		"readinessProbe": map[string]any{
			"httpGet": map[string]any{
				"path": "/ready",
				"port": "http",
			},
			"initialDelaySeconds": 3,
			"periodSeconds":       5,
			"timeoutSeconds":      2,
			"failureThreshold":    6,
		},
		"livenessProbe": map[string]any{
			"httpGet": map[string]any{
				"path": "/health",
				"port": "http",
			},
			"initialDelaySeconds": 20,
			"periodSeconds":       15,
			"timeoutSeconds":      2,
			"failureThreshold":    10,
		},
		"resources": map[string]any{
			"requests": map[string]any{
				"cpu":    "100m",
				"memory": "128Mi",
			},
			"limits": map[string]any{
				"cpu":    "1000m",
				"memory": "256Mi",
			},
		},
	}
	if configMapName := serviceConfigMapName(def); configMapName != "" {
		container["envFrom"] = []any{
			map[string]any{
				"configMapRef": map[string]any{
					"name": configMapName,
				},
			},
		}
	}

	return map[string]any{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": map[string]any{
			"name":      def.Name,
			"namespace": namespace,
			"labels":    labels,
		},
		"spec": map[string]any{
			"replicas": def.Replicas,
			"selector": map[string]any{
				"matchLabels": map[string]any{
					"app.kubernetes.io/name": def.Name,
				},
			},
			"template": map[string]any{
				"metadata": map[string]any{
					"labels": labels,
				},
				"spec": map[string]any{
					"containers": []any{container},
				},
			},
		},
	}
}

func renderServiceManifest(def serviceDefinition, namespace string) map[string]any {
	return map[string]any{
		"apiVersion": "v1",
		"kind":       "Service",
		"metadata": map[string]any{
			"name":      def.Name,
			"namespace": namespace,
			"labels":    standardLabels(def.Name),
		},
		"spec": map[string]any{
			"selector": map[string]any{
				"app.kubernetes.io/name": def.Name,
			},
			"ports": []any{
				map[string]any{
					"name":       "http",
					"port":       def.Port,
					"targetPort": "http",
				},
			},
			"type": "ClusterIP",
		},
	}
}

func renderHPAManifest(def serviceDefinition, namespace string) map[string]any {
	return map[string]any{
		"apiVersion": "autoscaling/v2",
		"kind":       "HorizontalPodAutoscaler",
		"metadata": map[string]any{
			"name":      def.Name,
			"namespace": namespace,
			"labels":    standardLabels(def.Name),
		},
		"spec": map[string]any{
			"scaleTargetRef": map[string]any{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"name":       def.Name,
			},
			"minReplicas": def.Autoscaling.MinReplicas,
			"maxReplicas": def.Autoscaling.MaxReplicas,
			"metrics": []any{
				map[string]any{
					"type": "Resource",
					"resource": map[string]any{
						"name": "cpu",
						"target": map[string]any{
							"type":               "Utilization",
							"averageUtilization": def.Autoscaling.TargetCPUUtilization,
						},
					},
				},
			},
		},
	}
}

func standardLabels(name string) map[string]any {
	return map[string]any{
		"app.kubernetes.io/name":      name,
		"app.kubernetes.io/component": "inference-simulator",
		"app.kubernetes.io/part-of":   "service-launchpad",
	}
}

func renderServiceConfigMap(def serviceDefinition, namespace string) map[string]any {
	if serviceConfigMapName(def) == "" {
		return nil
	}

	return map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]any{
			"name":      serviceConfigMapName(def),
			"namespace": namespace,
			"labels":    standardLabels(def.Name),
		},
		"data": map[string]any{
			"FASTAPI_SERVICE_MODEL":              "tinyllama-1.1b-chat-q4_k_m",
			"FASTAPI_SERVICE_DEFAULT_PROFILE":    "medium",
			"OTEL_SERVICE_NAME":                  def.Name,
			"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT": "http://tempo.service-launchpad-observability.svc.cluster.local:4318/v1/traces",
		},
	}
}

func serviceConfigMapName(def serviceDefinition) string {
	if def.Name != "fastapi-service" {
		return ""
	}
	return def.Name + "-config"
}
