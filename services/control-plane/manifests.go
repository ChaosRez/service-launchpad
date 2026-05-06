package main

func renderManifestBundle(def serviceDefinition, namespace string) manifestBundle {
	deployment := renderDeploymentManifest(def, namespace)
	service := renderServiceManifest(def, namespace)

	var hpa map[string]any
	manifests := []map[string]any{deployment, service}
	if def.Autoscaling.Enabled {
		hpa = renderHPAManifest(def, namespace)
		manifests = append(manifests, hpa)
	}

	return manifestBundle{
		Namespace:  namespace,
		Deployment: deployment,
		Service:    service,
		HPA:        hpa,
		YAML:       renderYAMLDocuments(manifests),
	}
}

func renderDeploymentManifest(def serviceDefinition, namespace string) map[string]any {
	labels := standardLabels(def.Name)

	return map[string]any{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": map[string]any{
			"name":        def.Name,
			"namespace":   namespace,
			"labels":      labels,
			"annotations": standardAnnotations(),
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
					"labels":      labels,
					"annotations": standardPodAnnotations(),
				},
				"spec": map[string]any{
					"containers": []any{
						map[string]any{
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
						},
					},
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
			"name":        def.Name,
			"namespace":   namespace,
			"labels":      standardLabels(def.Name),
			"annotations": standardAnnotations(),
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
			"name":        def.Name,
			"namespace":   namespace,
			"labels":      standardLabels(def.Name),
			"annotations": standardAnnotations(),
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
		"app.kubernetes.io/component": "managed-service",
		"app.kubernetes.io/part-of":   "service-launchpad",
	}
}

func standardAnnotations() map[string]any {
	return map[string]any{
		"service-launchpad.io/managed-by": "control-plane",
	}
}

func standardPodAnnotations() map[string]any {
	return map[string]any{
		"service-launchpad.io/metrics-path": "/metrics",
		"service-launchpad.io/metrics-port": "http",
	}
}
