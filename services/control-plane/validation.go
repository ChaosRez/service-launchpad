package main

import (
	"errors"
	"regexp"
	"strings"
)

var serviceNamePattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)

func validateServiceDefinition(def serviceDefinition) error {
	if strings.TrimSpace(def.Name) == "" {
		return errors.New("name is required")
	}
	if !serviceNamePattern.MatchString(def.Name) {
		return errors.New("name must be lowercase letters, numbers, or hyphens, and must start and end with an alphanumeric character")
	}
	if strings.TrimSpace(def.Image) == "" {
		return errors.New("image is required")
	}
	if def.Port < 1 || def.Port > 65535 {
		return errors.New("port must be between 1 and 65535")
	}
	if def.Replicas < 1 {
		return errors.New("replicas must be at least 1")
	}
	if !def.Autoscaling.Enabled {
		return nil
	}
	if def.Autoscaling.MinReplicas < 1 {
		return errors.New("autoscaling.minReplicas must be at least 1 when autoscaling is enabled")
	}
	if def.Autoscaling.MaxReplicas < def.Autoscaling.MinReplicas {
		return errors.New("autoscaling.maxReplicas must be greater than or equal to autoscaling.minReplicas")
	}
	if def.Autoscaling.TargetCPUUtilization < 1 || def.Autoscaling.TargetCPUUtilization > 100 {
		return errors.New("autoscaling.targetCpuUtilization must be between 1 and 100 when autoscaling is enabled")
	}

	return nil
}
