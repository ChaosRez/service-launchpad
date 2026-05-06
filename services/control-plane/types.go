package main

import "time"

const defaultListenAddr = ":8080"
const defaultNamespace = "service-launchpad-dev"

type autoscalingConfig struct {
	Enabled              bool `json:"enabled"`
	MinReplicas          int  `json:"minReplicas"`
	MaxReplicas          int  `json:"maxReplicas"`
	TargetCPUUtilization int  `json:"targetCpuUtilization"`
}

type serviceDefinition struct {
	Name        string            `json:"name"`
	Image       string            `json:"image"`
	Port        int               `json:"port"`
	Replicas    int               `json:"replicas"`
	Autoscaling autoscalingConfig `json:"autoscaling"`
	CreatedAt   time.Time         `json:"createdAt"`
}

type manifestBundle struct {
	Namespace  string         `json:"namespace"`
	Deployment map[string]any `json:"deployment"`
	Service    map[string]any `json:"service"`
	HPA        map[string]any `json:"hpa,omitempty"`
	YAML       string         `json:"yaml"`
}
