package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const (
	defaultMaxReplicas             = 10
	defaultMaxAutoscalingReplicas  = 10
	defaultMinCPUUtilizationTarget = 1
	defaultMaxCPUUtilizationTarget = 90
	defaultCPURequest              = "100m"
	defaultCPULimit                = "1000m"
	defaultMemoryRequest           = "128Mi"
	defaultMemoryLimit             = "256Mi"
	defaultAllowedImagePrefix      = "service-launchpad/"
	envAllowedImagePrefixes        = "CONTROL_PLANE_ALLOWED_IMAGE_PREFIXES"
	envMaxReplicas                 = "CONTROL_PLANE_POLICY_MAX_REPLICAS"
	envMaxAutoscalingReplicas      = "CONTROL_PLANE_POLICY_MAX_AUTOSCALING_REPLICAS"
)

var imageReferencePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9./:_@-]*$`)

type deploymentPolicy struct {
	AllowedImagePrefixes       []string
	MaxReplicas                int
	MaxAutoscalingReplicas     int
	MinCPUUtilizationTarget    int
	MaxCPUUtilizationTarget    int
	RequiredCPURequest         string
	RequiredCPULimit           string
	RequiredMemoryRequest      string
	RequiredMemoryLimit        string
	AllowedResourceKinds       map[string]bool
	RequireClusterIPServices   bool
	RequireWorkloadProbes      bool
	RequireDefaultResourceSpec bool
}

func defaultDeploymentPolicy() deploymentPolicy {
	return deploymentPolicy{
		AllowedImagePrefixes:       []string{defaultAllowedImagePrefix},
		MaxReplicas:                defaultMaxReplicas,
		MaxAutoscalingReplicas:     defaultMaxAutoscalingReplicas,
		MinCPUUtilizationTarget:    defaultMinCPUUtilizationTarget,
		MaxCPUUtilizationTarget:    defaultMaxCPUUtilizationTarget,
		RequiredCPURequest:         defaultCPURequest,
		RequiredCPULimit:           defaultCPULimit,
		RequiredMemoryRequest:      defaultMemoryRequest,
		RequiredMemoryLimit:        defaultMemoryLimit,
		AllowedResourceKinds:       defaultAllowedResourceKinds(),
		RequireClusterIPServices:   true,
		RequireWorkloadProbes:      true,
		RequireDefaultResourceSpec: true,
	}
}

func loadDeploymentPolicyFromEnv(getenv func(string) string) (deploymentPolicy, error) {
	policy := defaultDeploymentPolicy()

	if rawPrefixes := strings.TrimSpace(getenv(envAllowedImagePrefixes)); rawPrefixes != "" {
		prefixes := splitCommaSeparated(rawPrefixes)
		if len(prefixes) == 0 {
			return policy, fmt.Errorf("%s must include at least one image prefix when set", envAllowedImagePrefixes)
		}
		policy.AllowedImagePrefixes = prefixes
	}

	if rawMaxReplicas := strings.TrimSpace(getenv(envMaxReplicas)); rawMaxReplicas != "" {
		maxReplicas, err := strconv.Atoi(rawMaxReplicas)
		if err != nil || maxReplicas < 1 {
			return policy, fmt.Errorf("%s must be a positive integer", envMaxReplicas)
		}
		policy.MaxReplicas = maxReplicas
	}

	if rawMaxAutoscaling := strings.TrimSpace(getenv(envMaxAutoscalingReplicas)); rawMaxAutoscaling != "" {
		maxAutoscaling, err := strconv.Atoi(rawMaxAutoscaling)
		if err != nil || maxAutoscaling < 1 {
			return policy, fmt.Errorf("%s must be a positive integer", envMaxAutoscalingReplicas)
		}
		policy.MaxAutoscalingReplicas = maxAutoscaling
	}

	if policy.MaxAutoscalingReplicas < policy.MaxReplicas {
		return policy, fmt.Errorf("%s must be greater than or equal to %s", envMaxAutoscalingReplicas, envMaxReplicas)
	}

	return policy, nil
}

func defaultAllowedResourceKinds() map[string]bool {
	return map[string]bool{
		"Namespace":               true,
		"ConfigMap":               true,
		"Deployment":              true,
		"Service":                 true,
		"HorizontalPodAutoscaler": true,
	}
}

func splitCommaSeparated(raw string) []string {
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value != "" {
			values = append(values, value)
		}
	}
	return values
}

func (p deploymentPolicy) validateServiceDefinition(def serviceDefinition) error {
	if err := validateServiceDefinition(def); err != nil {
		return err
	}
	if def.Replicas > p.MaxReplicas {
		return fmt.Errorf("replicas must be less than or equal to %d", p.MaxReplicas)
	}
	if err := p.validateImageReference(def.Image); err != nil {
		return err
	}
	if !def.Autoscaling.Enabled {
		return nil
	}
	if def.Autoscaling.MaxReplicas > p.MaxAutoscalingReplicas {
		return fmt.Errorf("autoscaling.maxReplicas must be less than or equal to %d", p.MaxAutoscalingReplicas)
	}
	if def.Autoscaling.TargetCPUUtilization < p.MinCPUUtilizationTarget || def.Autoscaling.TargetCPUUtilization > p.MaxCPUUtilizationTarget {
		return fmt.Errorf("autoscaling.targetCpuUtilization must be between %d and %d when autoscaling is enabled", p.MinCPUUtilizationTarget, p.MaxCPUUtilizationTarget)
	}
	return nil
}

func (p deploymentPolicy) validateManifestBundle(bundle manifestBundle) error {
	if strings.TrimSpace(bundle.Namespace) == "" {
		return fmt.Errorf("policy violation: manifest bundle namespace is required")
	}
	if err := p.validateNamespaceManifest(bundle.NamespaceManifest, bundle.Namespace); err != nil {
		return err
	}
	if bundle.ConfigMap != nil {
		if err := p.validateNamespacedResource(bundle.ConfigMap, "ConfigMap", bundle.Namespace); err != nil {
			return err
		}
	}
	if err := p.validateDeploymentManifest(bundle.Deployment, bundle.Namespace); err != nil {
		return err
	}
	if err := p.validateServiceManifest(bundle.Service, bundle.Namespace); err != nil {
		return err
	}
	if bundle.HPA != nil {
		if err := p.validateHPAManifest(bundle.HPA, bundle.Namespace); err != nil {
			return err
		}
	}
	if err := validateBundleYAMLMatchesStructuredIntent(bundle); err != nil {
		return err
	}
	return nil
}

func validateBundleYAMLMatchesStructuredIntent(bundle manifestBundle) error {
	manifests := make([]map[string]any, 0, 4)
	if bundle.ConfigMap != nil {
		manifests = append(manifests, bundle.ConfigMap)
	}
	manifests = append(manifests, bundle.Deployment, bundle.Service)
	if bundle.HPA != nil {
		manifests = append(manifests, bundle.HPA)
	}

	expected := strings.TrimSpace(renderYAMLDocuments(manifests))
	actual := strings.TrimSpace(bundle.YAML)
	if actual == "" {
		return fmt.Errorf("policy violation: manifest bundle YAML is required for fallback deployer parity")
	}
	if actual != expected {
		return fmt.Errorf("policy violation: manifest bundle YAML must match structured resource intent")
	}
	return nil
}

func (p deploymentPolicy) validateImageReference(image string) error {
	image = strings.TrimSpace(image)
	if !imageReferencePattern.MatchString(image) || strings.Contains(image, "..") || strings.Contains(image, "//") {
		return fmt.Errorf("image must be a lowercase container image reference without whitespace or traversal")
	}
	for _, prefix := range p.AllowedImagePrefixes {
		if strings.HasPrefix(image, prefix) {
			return nil
		}
	}
	return fmt.Errorf("image must start with one of the allowed prefixes: %s", strings.Join(p.AllowedImagePrefixes, ", "))
}

func (p deploymentPolicy) validateNamespaceManifest(manifest map[string]any, namespace string) error {
	if err := p.validateKind(manifest, "Namespace"); err != nil {
		return err
	}
	metadata, err := metadataMap(manifest)
	if err != nil {
		return err
	}
	if metadataString(metadata, "name") != namespace {
		return fmt.Errorf("policy violation: Namespace name must match managed namespace %q", namespace)
	}
	if ns := metadataString(metadata, "namespace"); ns != "" {
		return fmt.Errorf("policy violation: Namespace manifest must not set metadata.namespace")
	}
	return validateProjectLabels(metadata, "service-launchpad")
}

func (p deploymentPolicy) validateNamespacedResource(manifest map[string]any, kind string, namespace string) error {
	if err := p.validateKind(manifest, kind); err != nil {
		return err
	}
	metadata, err := metadataMap(manifest)
	if err != nil {
		return err
	}
	if metadataString(metadata, "namespace") != namespace {
		return fmt.Errorf("policy violation: %s must stay in managed namespace %q", kind, namespace)
	}
	if metadataString(metadata, "name") == "" {
		return fmt.Errorf("policy violation: %s metadata.name is required", kind)
	}
	if kind == "ConfigMap" {
		return validateProjectLabels(metadata, "")
	}
	return validateProjectLabels(metadata, metadataString(metadata, "name"))
}

func (p deploymentPolicy) validateDeploymentManifest(manifest map[string]any, namespace string) error {
	if err := p.validateNamespacedResource(manifest, "Deployment", namespace); err != nil {
		return err
	}
	spec, err := requiredMap(manifest, "spec")
	if err != nil {
		return err
	}
	replicas, ok := intValue(spec["replicas"])
	if !ok || replicas < 1 || replicas > p.MaxReplicas {
		return fmt.Errorf("policy violation: Deployment replicas must be between 1 and %d", p.MaxReplicas)
	}
	template, err := requiredMap(spec, "template")
	if err != nil {
		return err
	}
	templateMetadata, err := requiredMap(template, "metadata")
	if err != nil {
		return err
	}
	if err := validateProjectLabels(templateMetadata, ""); err != nil {
		return err
	}
	templateSpec, err := requiredMap(template, "spec")
	if err != nil {
		return err
	}
	for _, field := range []string{"hostNetwork", "hostPID", "hostIPC"} {
		if boolValue(templateSpec[field]) {
			return fmt.Errorf("policy violation: Deployment template must not set %s", field)
		}
	}
	if metadataString(templateSpec, "serviceAccountName") != "" {
		return fmt.Errorf("policy violation: Deployment template must not select an arbitrary service account")
	}
	if volumes, ok := sliceValue(templateSpec["volumes"]); ok {
		for _, volume := range volumes {
			volumeMap, ok := volume.(map[string]any)
			if !ok {
				return fmt.Errorf("policy violation: Deployment volume entries must be objects")
			}
			if _, hasHostPath := volumeMap["hostPath"]; hasHostPath {
				return fmt.Errorf("policy violation: hostPath volumes are not allowed")
			}
		}
	}
	containers, ok := sliceValue(templateSpec["containers"])
	if !ok || len(containers) != 1 {
		return fmt.Errorf("policy violation: Deployment must define exactly one container")
	}
	container, ok := containers[0].(map[string]any)
	if !ok {
		return fmt.Errorf("policy violation: Deployment container must be an object")
	}
	return p.validateContainer(container)
}

func (p deploymentPolicy) validateServiceManifest(manifest map[string]any, namespace string) error {
	if err := p.validateNamespacedResource(manifest, "Service", namespace); err != nil {
		return err
	}
	spec, err := requiredMap(manifest, "spec")
	if err != nil {
		return err
	}
	if p.RequireClusterIPServices && metadataString(spec, "type") != "ClusterIP" {
		return fmt.Errorf("policy violation: Service type must remain ClusterIP")
	}
	return nil
}

func (p deploymentPolicy) validateHPAManifest(manifest map[string]any, namespace string) error {
	if err := p.validateNamespacedResource(manifest, "HorizontalPodAutoscaler", namespace); err != nil {
		return err
	}
	spec, err := requiredMap(manifest, "spec")
	if err != nil {
		return err
	}
	minReplicas, ok := intValue(spec["minReplicas"])
	if !ok || minReplicas < 1 {
		return fmt.Errorf("policy violation: HorizontalPodAutoscaler minReplicas must be at least 1")
	}
	maxReplicas, ok := intValue(spec["maxReplicas"])
	if !ok || maxReplicas < minReplicas || maxReplicas > p.MaxAutoscalingReplicas {
		return fmt.Errorf("policy violation: HorizontalPodAutoscaler maxReplicas must be between minReplicas and %d", p.MaxAutoscalingReplicas)
	}
	metrics, ok := sliceValue(spec["metrics"])
	if !ok || len(metrics) == 0 {
		return fmt.Errorf("policy violation: HorizontalPodAutoscaler must define metrics")
	}
	return nil
}

func (p deploymentPolicy) validateContainer(container map[string]any) error {
	if name := metadataString(container, "name"); !serviceNamePattern.MatchString(name) {
		return fmt.Errorf("policy violation: container name must match service naming policy")
	}
	if err := p.validateImageReference(metadataString(container, "image")); err != nil {
		return fmt.Errorf("policy violation: %w", err)
	}
	if p.RequireWorkloadProbes {
		for _, probe := range []string{"startupProbe", "readinessProbe", "livenessProbe"} {
			if _, err := requiredMap(container, probe); err != nil {
				return fmt.Errorf("policy violation: container must include %s", probe)
			}
		}
	}
	if p.RequireDefaultResourceSpec {
		resources, err := requiredMap(container, "resources")
		if err != nil {
			return fmt.Errorf("policy violation: container resources are required")
		}
		requests, err := requiredMap(resources, "requests")
		if err != nil {
			return fmt.Errorf("policy violation: container resource requests are required")
		}
		limits, err := requiredMap(resources, "limits")
		if err != nil {
			return fmt.Errorf("policy violation: container resource limits are required")
		}
		if metadataString(requests, "cpu") != p.RequiredCPURequest || metadataString(requests, "memory") != p.RequiredMemoryRequest {
			return fmt.Errorf("policy violation: container resource requests must be cpu=%s memory=%s", p.RequiredCPURequest, p.RequiredMemoryRequest)
		}
		if metadataString(limits, "cpu") != p.RequiredCPULimit || metadataString(limits, "memory") != p.RequiredMemoryLimit {
			return fmt.Errorf("policy violation: container resource limits must be cpu=%s memory=%s", p.RequiredCPULimit, p.RequiredMemoryLimit)
		}
	}
	if securityContext, ok := mapValue(container["securityContext"]); ok {
		if boolValue(securityContext["privileged"]) {
			return fmt.Errorf("policy violation: privileged containers are not allowed")
		}
		if boolValue(securityContext["allowPrivilegeEscalation"]) {
			return fmt.Errorf("policy violation: privilege escalation is not allowed")
		}
		if capabilities, ok := mapValue(securityContext["capabilities"]); ok {
			if add, ok := sliceValue(capabilities["add"]); ok && len(add) > 0 {
				return fmt.Errorf("policy violation: added Linux capabilities are not allowed")
			}
		}
	}
	return nil
}

func (p deploymentPolicy) validateKind(manifest map[string]any, want string) error {
	if manifest == nil {
		return fmt.Errorf("policy violation: %s manifest is required", want)
	}
	kind := metadataString(manifest, "kind")
	if kind != want {
		return fmt.Errorf("policy violation: expected kind %s, got %q", want, kind)
	}
	if !p.AllowedResourceKinds[kind] {
		return fmt.Errorf("policy violation: resource kind %s is not allowed", kind)
	}
	return nil
}

func validateProjectLabels(metadata map[string]any, expectedName string) error {
	labels, err := requiredMap(metadata, "labels")
	if err != nil {
		return fmt.Errorf("policy violation: labels are required")
	}
	if metadataString(labels, "app.kubernetes.io/part-of") != "service-launchpad" {
		return fmt.Errorf("policy violation: app.kubernetes.io/part-of label must be service-launchpad")
	}
	if expectedName != "" && metadataString(labels, "app.kubernetes.io/name") != expectedName {
		return fmt.Errorf("policy violation: app.kubernetes.io/name label must be %s", expectedName)
	}
	if value := metadataString(labels, "app.kubernetes.io/managed-by"); value != "" && value != "service-launchpad" {
		return fmt.Errorf("policy violation: app.kubernetes.io/managed-by cannot override service-launchpad ownership")
	}
	if annotations, ok := mapValue(metadata["annotations"]); ok {
		for key := range annotations {
			if !strings.HasPrefix(key, "service-launchpad.io/") {
				return fmt.Errorf("policy violation: annotation %q must use the service-launchpad.io/ prefix", key)
			}
		}
	}
	return nil
}

func metadataMap(manifest map[string]any) (map[string]any, error) {
	return requiredMap(manifest, "metadata")
}

func requiredMap(parent map[string]any, key string) (map[string]any, error) {
	value, ok := mapValue(parent[key])
	if !ok {
		return nil, fmt.Errorf("policy violation: %s must be an object", key)
	}
	return value, nil
}

func mapValue(value any) (map[string]any, bool) {
	switch typed := value.(type) {
	case map[string]any:
		return typed, true
	case map[string]string:
		converted := make(map[string]any, len(typed))
		for key, value := range typed {
			converted[key] = value
		}
		return converted, true
	default:
		return nil, false
	}
}

func sliceValue(value any) ([]any, bool) {
	switch typed := value.(type) {
	case []any:
		return typed, true
	case []map[string]any:
		converted := make([]any, 0, len(typed))
		for _, item := range typed {
			converted = append(converted, item)
		}
		return converted, true
	default:
		return nil, false
	}
}

func metadataString(parent map[string]any, key string) string {
	value, ok := parent[key]
	if !ok {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	default:
		return fmt.Sprint(typed)
	}
}

func boolValue(value any) bool {
	typed, ok := value.(bool)
	return ok && typed
}

func intValue(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int32:
		return int(typed), true
	case int64:
		return int(typed), true
	case float64:
		if typed == float64(int(typed)) {
			return int(typed), true
		}
	}
	return 0, false
}
