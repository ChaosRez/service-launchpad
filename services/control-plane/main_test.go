package main

// for validation, duplicate rejection, and JSON persistence reload

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestValidateServiceDefinition(t *testing.T) {
	tests := []struct {
		name    string
		def     serviceDefinition
		wantErr bool
	}{
		{
			name: "valid minimal definition",
			def: serviceDefinition{
				Name:     "fastapi-service",
				Image:    "service-launchpad/fastapi-service:dev",
				Port:     8000,
				Replicas: 1,
			},
		},
		{
			name: "valid autoscaling definition",
			def: serviceDefinition{
				Name:     "fastapi-service",
				Image:    "service-launchpad/fastapi-service:dev",
				Port:     8000,
				Replicas: 1,
				Autoscaling: autoscalingConfig{
					Enabled:              true,
					MinReplicas:          1,
					MaxReplicas:          5,
					TargetCPUUtilization: 60,
				},
			},
		},
		{
			name: "missing name",
			def: serviceDefinition{
				Image:    "service-launchpad/fastapi-service:dev",
				Port:     8000,
				Replicas: 1,
			},
			wantErr: true,
		},
		{
			name: "unsafe name",
			def: serviceDefinition{
				Name:     "FastAPI_Service",
				Image:    "service-launchpad/fastapi-service:dev",
				Port:     8000,
				Replicas: 1,
			},
			wantErr: true,
		},
		{
			name: "missing image",
			def: serviceDefinition{
				Name:     "fastapi-service",
				Port:     8000,
				Replicas: 1,
			},
			wantErr: true,
		},
		{
			name: "invalid port",
			def: serviceDefinition{
				Name:     "fastapi-service",
				Image:    "service-launchpad/fastapi-service:dev",
				Port:     0,
				Replicas: 1,
			},
			wantErr: true,
		},
		{
			name: "invalid replicas",
			def: serviceDefinition{
				Name:     "fastapi-service",
				Image:    "service-launchpad/fastapi-service:dev",
				Port:     8000,
				Replicas: 0,
			},
			wantErr: true,
		},
		{
			name: "invalid autoscaling range",
			def: serviceDefinition{
				Name:     "fastapi-service",
				Image:    "service-launchpad/fastapi-service:dev",
				Port:     8000,
				Replicas: 1,
				Autoscaling: autoscalingConfig{
					Enabled:              true,
					MinReplicas:          3,
					MaxReplicas:          1,
					TargetCPUUtilization: 60,
				},
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateServiceDefinition(tc.def)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error but got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected no error but got %v", err)
			}
		})
	}
}

func TestServiceStorePersistsAndLoads(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "control-plane", "services.json")

	store, err := newServiceStore(storePath)
	if err != nil {
		t.Fatalf("newServiceStore returned error: %v", err)
	}

	created, err := store.create(serviceDefinition{
		Name:     "fastapi-service",
		Image:    "service-launchpad/fastapi-service:dev",
		Port:     8000,
		Replicas: 1,
		Autoscaling: autoscalingConfig{
			Enabled:              true,
			MinReplicas:          1,
			MaxReplicas:          5,
			TargetCPUUtilization: 60,
		},
	})
	if err != nil {
		t.Fatalf("create returned error: %v", err)
	}
	if created.CreatedAt.IsZero() {
		t.Fatalf("expected CreatedAt to be set")
	}

	if _, err := os.Stat(storePath); err != nil {
		t.Fatalf("expected persisted file to exist: %v", err)
	}

	reloaded, err := newServiceStore(storePath)
	if err != nil {
		t.Fatalf("reloading store returned error: %v", err)
	}

	got, ok := reloaded.get("fastapi-service")
	if !ok {
		t.Fatalf("expected reloaded service to exist")
	}
	if got.Image != "service-launchpad/fastapi-service:dev" {
		t.Fatalf("unexpected image after reload: %s", got.Image)
	}
	if time.Since(got.CreatedAt) > time.Minute {
		t.Fatalf("expected CreatedAt to survive reload")
	}
}

func TestServiceStoreRejectsDuplicates(t *testing.T) {
	store, err := newServiceStore("")
	if err != nil {
		t.Fatalf("newServiceStore returned error: %v", err)
	}

	def := serviceDefinition{
		Name:     "fastapi-service",
		Image:    "service-launchpad/fastapi-service:dev",
		Port:     8000,
		Replicas: 1,
	}

	if _, err := store.create(def); err != nil {
		t.Fatalf("first create returned error: %v", err)
	}

	if _, err := store.create(def); err == nil {
		t.Fatalf("expected duplicate create to fail")
	}
}

func TestRenderManifestBundleWithoutAutoscaling(t *testing.T) {
	def := serviceDefinition{
		Name:     "fastapi-service",
		Image:    "service-launchpad/fastapi-service:dev",
		Port:     8000,
		Replicas: 2,
	}

	bundle := renderManifestBundle(def, defaultNamespace)

	if bundle.HPA != nil {
		t.Fatalf("expected no HPA manifest when autoscaling is disabled")
	}
	if bundle.NamespaceManifest["kind"] != "Namespace" {
		t.Fatalf("expected namespace manifest")
	}
	if bundle.ConfigMap["kind"] != "ConfigMap" {
		t.Fatalf("expected configmap manifest for fastapi-service")
	}
	if bundle.Deployment["kind"] != "Deployment" {
		t.Fatalf("expected deployment manifest")
	}
	if bundle.Service["kind"] != "Service" {
		t.Fatalf("expected service manifest")
	}
	if !strings.Contains(bundle.YAML, "kind: Deployment") {
		t.Fatalf("expected YAML to contain Deployment manifest")
	}
	if !strings.Contains(bundle.YAML, "kind: ConfigMap") {
		t.Fatalf("expected YAML to contain ConfigMap manifest")
	}
	if !strings.Contains(bundle.YAML, "kind: Service") {
		t.Fatalf("expected YAML to contain Service manifest")
	}
	if strings.Contains(bundle.YAML, "kind: Namespace") {
		t.Fatalf("expected resource YAML to exclude Namespace manifest")
	}
}

func TestRenderManifestBundleWithAutoscaling(t *testing.T) {
	def := serviceDefinition{
		Name:     "fastapi-service",
		Image:    "service-launchpad/fastapi-service:dev",
		Port:     8000,
		Replicas: 1,
		Autoscaling: autoscalingConfig{
			Enabled:              true,
			MinReplicas:          1,
			MaxReplicas:          5,
			TargetCPUUtilization: 60,
		},
	}

	bundle := renderManifestBundle(def, defaultNamespace)

	if bundle.HPA == nil {
		t.Fatalf("expected HPA manifest when autoscaling is enabled")
	}
	if bundle.HPA["kind"] != "HorizontalPodAutoscaler" {
		t.Fatalf("expected HPA kind, got %v", bundle.HPA["kind"])
	}
	if !strings.Contains(bundle.YAML, "kind: HorizontalPodAutoscaler") {
		t.Fatalf("expected YAML to contain HPA manifest")
	}
	metadata, ok := bundle.Deployment["metadata"].(map[string]any)
	if !ok {
		t.Fatalf("expected deployment metadata")
	}
	labels, ok := metadata["labels"].(map[string]any)
	if !ok || labels["app.kubernetes.io/component"] != "inference-simulator" {
		t.Fatalf("expected deployment to use the base component label")
	}
	configMapMetadata, ok := bundle.ConfigMap["metadata"].(map[string]any)
	if !ok || configMapMetadata["name"] != "fastapi-service-config" {
		t.Fatalf("expected fastapi-service config map metadata")
	}
	spec, ok := bundle.Deployment["spec"].(map[string]any)
	if !ok {
		t.Fatalf("expected deployment spec")
	}
	template, ok := spec["template"].(map[string]any)
	if !ok {
		t.Fatalf("expected deployment template")
	}
	templateSpec, ok := template["spec"].(map[string]any)
	if !ok {
		t.Fatalf("expected deployment template spec")
	}
	containers, ok := templateSpec["containers"].([]any)
	if !ok || len(containers) == 0 {
		t.Fatalf("expected deployment containers")
	}
	container, ok := containers[0].(map[string]any)
	if !ok {
		t.Fatalf("expected first container to be an object")
	}
	if _, ok := container["envFrom"].([]any); !ok {
		t.Fatalf("expected deployment to reference the config map via envFrom")
	}
}

func TestClientGoResourceIntentWithAutoscaling(t *testing.T) {
	def := serviceDefinition{
		Name:     "fastapi-service",
		Image:    "service-launchpad/fastapi-service:dev",
		Port:     8000,
		Replicas: 1,
		Autoscaling: autoscalingConfig{
			Enabled:              true,
			MinReplicas:          1,
			MaxReplicas:          5,
			TargetCPUUtilization: 60,
		},
	}

	intent, err := clientGoResourceIntent(renderManifestBundle(def, defaultNamespace))
	if err != nil {
		t.Fatalf("clientGoResourceIntent returned error: %v", err)
	}

	want := []schema.GroupVersionKind{
		{Version: "v1", Kind: "Namespace"},
		{Version: "v1", Kind: "ConfigMap"},
		{Group: "apps", Version: "v1", Kind: "Deployment"},
		{Version: "v1", Kind: "Service"},
		{Group: "autoscaling", Version: "v2", Kind: "HorizontalPodAutoscaler"},
	}

	if !reflect.DeepEqual(intent, want) {
		t.Fatalf("unexpected resource intent:\nwant: %#v\n got: %#v", want, intent)
	}
}

func TestClientGoResourceIntentWithoutAutoscaling(t *testing.T) {
	def := serviceDefinition{
		Name:     "generic-service",
		Image:    "service-launchpad/generic-service:dev",
		Port:     8080,
		Replicas: 2,
	}

	intent, err := clientGoResourceIntent(renderManifestBundle(def, defaultNamespace))
	if err != nil {
		t.Fatalf("clientGoResourceIntent returned error: %v", err)
	}

	want := []schema.GroupVersionKind{
		{Version: "v1", Kind: "Namespace"},
		{Group: "apps", Version: "v1", Kind: "Deployment"},
		{Version: "v1", Kind: "Service"},
	}

	if !reflect.DeepEqual(intent, want) {
		t.Fatalf("unexpected resource intent:\nwant: %#v\n got: %#v", want, intent)
	}
}

func TestDeploymentPolicyValidatesRenderedManifestBundle(t *testing.T) {
	policy := defaultDeploymentPolicy()
	def := serviceDefinition{
		Name:     "fastapi-service",
		Image:    "service-launchpad/fastapi-service:dev",
		Port:     8000,
		Replicas: 1,
		Autoscaling: autoscalingConfig{
			Enabled:              true,
			MinReplicas:          1,
			MaxReplicas:          5,
			TargetCPUUtilization: 60,
		},
	}

	bundle := renderManifestBundle(def, defaultNamespace)
	if err := policy.validateManifestBundle(bundle); err != nil {
		t.Fatalf("expected rendered bundle to pass policy, got %v", err)
	}
}

func TestDeploymentPolicyRejectsUnsafeManifestShapes(t *testing.T) {
	policy := defaultDeploymentPolicy()
	def := serviceDefinition{
		Name:     "fastapi-service",
		Image:    "service-launchpad/fastapi-service:dev",
		Port:     8000,
		Replicas: 1,
		Autoscaling: autoscalingConfig{
			Enabled:              true,
			MinReplicas:          1,
			MaxReplicas:          5,
			TargetCPUUtilization: 60,
		},
	}

	tests := []struct {
		name   string
		mutate func(*manifestBundle)
		want   string
	}{
		{
			name: "deployment outside managed namespace",
			mutate: func(bundle *manifestBundle) {
				metadata := bundle.Deployment["metadata"].(map[string]any)
				metadata["namespace"] = "default"
			},
			want: "managed namespace",
		},
		{
			name: "service type load balancer",
			mutate: func(bundle *manifestBundle) {
				spec := bundle.Service["spec"].(map[string]any)
				spec["type"] = "LoadBalancer"
			},
			want: "ClusterIP",
		},
		{
			name: "missing readiness probe",
			mutate: func(bundle *manifestBundle) {
				container := firstRenderedContainer(t, bundle)
				delete(container, "readinessProbe")
			},
			want: "readinessProbe",
		},
		{
			name: "missing resource limits",
			mutate: func(bundle *manifestBundle) {
				container := firstRenderedContainer(t, bundle)
				resources := container["resources"].(map[string]any)
				delete(resources, "limits")
			},
			want: "resource limits",
		},
		{
			name: "excessive resource limit",
			mutate: func(bundle *manifestBundle) {
				container := firstRenderedContainer(t, bundle)
				resources := container["resources"].(map[string]any)
				limits := resources["limits"].(map[string]any)
				limits["memory"] = "2Gi"
			},
			want: "resource limits",
		},
		{
			name: "privileged container",
			mutate: func(bundle *manifestBundle) {
				container := firstRenderedContainer(t, bundle)
				container["securityContext"] = map[string]any{
					"privileged": true,
				}
			},
			want: "privileged",
		},
		{
			name: "host network",
			mutate: func(bundle *manifestBundle) {
				templateSpec := renderedTemplateSpec(t, bundle)
				templateSpec["hostNetwork"] = true
			},
			want: "hostNetwork",
		},
		{
			name: "host path volume",
			mutate: func(bundle *manifestBundle) {
				templateSpec := renderedTemplateSpec(t, bundle)
				templateSpec["volumes"] = []any{
					map[string]any{
						"name": "host",
						"hostPath": map[string]any{
							"path": "/var/run/docker.sock",
						},
					},
				}
			},
			want: "hostPath",
		},
		{
			name: "arbitrary service account",
			mutate: func(bundle *manifestBundle) {
				templateSpec := renderedTemplateSpec(t, bundle)
				templateSpec["serviceAccountName"] = "cluster-admin"
			},
			want: "service account",
		},
		{
			name: "bad ownership label",
			mutate: func(bundle *manifestBundle) {
				metadata := bundle.Deployment["metadata"].(map[string]any)
				labels := metadata["labels"].(map[string]any)
				labels["app.kubernetes.io/part-of"] = "other-system"
			},
			want: "part-of",
		},
		{
			name: "arbitrary annotation",
			mutate: func(bundle *manifestBundle) {
				metadata := bundle.Deployment["metadata"].(map[string]any)
				metadata["annotations"] = map[string]any{
					"container.apparmor.security.beta.kubernetes.io/fastapi-service": "unconfined",
				}
			},
			want: "service-launchpad.io",
		},
		{
			name: "yaml drift",
			mutate: func(bundle *manifestBundle) {
				bundle.YAML += "\n---\napiVersion: v1\nkind: Secret\nmetadata:\n  name: unexpected\n"
			},
			want: "structured resource intent",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			base := renderManifestBundle(def, defaultNamespace)
			bundle := manifestBundle{
				Namespace:         base.Namespace,
				NamespaceManifest: cloneManifest(base.NamespaceManifest),
				ConfigMap:         cloneManifest(base.ConfigMap),
				Deployment:        cloneManifest(base.Deployment),
				Service:           cloneManifest(base.Service),
				HPA:               cloneManifest(base.HPA),
				YAML:              base.YAML,
			}
			tc.mutate(&bundle)

			err := policy.validateManifestBundle(bundle)
			if err == nil {
				t.Fatalf("expected policy error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected error to contain %q, got %v", tc.want, err)
			}
		})
	}
}

func TestNewManifestDeployerFromConfigSelectsKubectl(t *testing.T) {
	deployer, err := newManifestDeployerFromConfig(controlPlaneConfig{
		DeployerMode:  "kubectl",
		KubectlBinary: "kubectl",
		KubeContext:   "service-launchpad",
	})
	if err != nil {
		t.Fatalf("newManifestDeployerFromConfig returned error: %v", err)
	}

	if _, ok := deployer.(*kubectlDeployer); !ok {
		t.Fatalf("expected kubectlDeployer, got %T", deployer)
	}
}

func TestNewManifestDeployerFromConfigSelectsClientGo(t *testing.T) {
	deployer, err := newManifestDeployerFromConfig(controlPlaneConfig{
		DeployerMode: "client-go",
		KubeContext:  "service-launchpad",
	})
	if err != nil {
		t.Fatalf("newManifestDeployerFromConfig returned error: %v", err)
	}

	if _, ok := deployer.(*clientGoDeployer); !ok {
		t.Fatalf("expected clientGoDeployer, got %T", deployer)
	}
}

func TestNewManifestDeployerFromConfigSupportsDisabledMode(t *testing.T) {
	deployer, err := newManifestDeployerFromConfig(controlPlaneConfig{DeployerMode: "disabled"})
	if err != nil {
		t.Fatalf("newManifestDeployerFromConfig returned error: %v", err)
	}
	if deployer != nil {
		t.Fatalf("expected nil deployer for disabled mode, got %T", deployer)
	}
}

func TestNewManifestDeployerFromConfigRejectsUnknownMode(t *testing.T) {
	_, err := newManifestDeployerFromConfig(controlPlaneConfig{DeployerMode: "shell"})
	if err == nil {
		t.Fatalf("expected unsupported deployer mode error")
	}
	if !strings.Contains(err.Error(), "unsupported CONTROL_PLANE_DEPLOYER_MODE") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadControlPlaneConfigReadsEnvironment(t *testing.T) {
	t.Setenv("CONTROL_PLANE_LISTEN_ADDR", "127.0.0.1:9090")
	t.Setenv("CONTROL_PLANE_STORE_PATH", "/tmp/services.json")
	t.Setenv(envTargetNamespace, "service-launchpad-prod")
	t.Setenv(envDeployerMode, "kubectl")
	t.Setenv("CONTROL_PLANE_KUBECTL_BIN", "/usr/local/bin/kubectl")
	t.Setenv(envKubeContext, "prod")
	t.Setenv(envAuditBucket, "service-launchpad-audit")
	t.Setenv(envAuditPrefix, "control-plane/audit")
	t.Setenv(envGCSEndpoint, "http://storage.example.test")
	t.Setenv(envGCSBearerToken, "test-gcs-token")

	cfg, err := loadControlPlaneConfig()
	if err != nil {
		t.Fatalf("loadControlPlaneConfig returned error: %v", err)
	}

	if cfg.ListenAddr != "127.0.0.1:9090" {
		t.Fatalf("unexpected listen addr: %s", cfg.ListenAddr)
	}
	if cfg.StorePath != "/tmp/services.json" {
		t.Fatalf("unexpected store path: %s", cfg.StorePath)
	}
	if cfg.TargetNamespace != "service-launchpad-prod" {
		t.Fatalf("unexpected namespace: %s", cfg.TargetNamespace)
	}
	if cfg.DeployerMode != "kubectl" {
		t.Fatalf("unexpected deployer mode: %s", cfg.DeployerMode)
	}
	if cfg.KubectlBinary != "/usr/local/bin/kubectl" {
		t.Fatalf("unexpected kubectl binary: %s", cfg.KubectlBinary)
	}
	if cfg.KubeContext != "prod" {
		t.Fatalf("unexpected kube context: %s", cfg.KubeContext)
	}
	if cfg.AuditBucket != "service-launchpad-audit" {
		t.Fatalf("unexpected audit bucket: %s", cfg.AuditBucket)
	}
	if cfg.AuditPrefix != "control-plane/audit" {
		t.Fatalf("unexpected audit prefix: %s", cfg.AuditPrefix)
	}
	if cfg.GCSEndpoint != "http://storage.example.test" {
		t.Fatalf("unexpected GCS endpoint: %s", cfg.GCSEndpoint)
	}
	if cfg.GCSBearerToken != "test-gcs-token" {
		t.Fatalf("unexpected GCS bearer token: %s", cfg.GCSBearerToken)
	}
	if !reflect.DeepEqual(cfg.DeploymentPolicy.AllowedImagePrefixes, []string{defaultAllowedImagePrefix}) {
		t.Fatalf("unexpected default image prefixes: %#v", cfg.DeploymentPolicy.AllowedImagePrefixes)
	}
}

func TestLoadControlPlaneConfigReadsDeploymentPolicyEnvironment(t *testing.T) {
	t.Setenv(envAllowedImagePrefixes, "europe-west10-docker.pkg.dev/geofaas-411316/service-launchpad/,service-launchpad/")
	t.Setenv(envMaxReplicas, "4")
	t.Setenv(envMaxAutoscalingReplicas, "6")

	cfg, err := loadControlPlaneConfig()
	if err != nil {
		t.Fatalf("loadControlPlaneConfig returned error: %v", err)
	}

	wantPrefixes := []string{
		"europe-west10-docker.pkg.dev/geofaas-411316/service-launchpad/",
		"service-launchpad/",
	}
	if !reflect.DeepEqual(cfg.DeploymentPolicy.AllowedImagePrefixes, wantPrefixes) {
		t.Fatalf("unexpected image prefixes:\nwant: %#v\n got: %#v", wantPrefixes, cfg.DeploymentPolicy.AllowedImagePrefixes)
	}
	if cfg.DeploymentPolicy.MaxReplicas != 4 {
		t.Fatalf("unexpected max replicas: %d", cfg.DeploymentPolicy.MaxReplicas)
	}
	if cfg.DeploymentPolicy.MaxAutoscalingReplicas != 6 {
		t.Fatalf("unexpected max autoscaling replicas: %d", cfg.DeploymentPolicy.MaxAutoscalingReplicas)
	}
}

func TestDeploymentPolicyValidatesServiceDefinitions(t *testing.T) {
	policy := defaultDeploymentPolicy()

	valid := serviceDefinition{
		Name:     "fastapi-service",
		Image:    "service-launchpad/fastapi-service:dev",
		Port:     8000,
		Replicas: 1,
		Autoscaling: autoscalingConfig{
			Enabled:              true,
			MinReplicas:          1,
			MaxReplicas:          5,
			TargetCPUUtilization: 60,
		},
	}
	if err := policy.validateServiceDefinition(valid); err != nil {
		t.Fatalf("expected valid definition to pass policy, got %v", err)
	}

	tests := []struct {
		name string
		def  serviceDefinition
		want string
	}{
		{
			name: "image outside allowed prefix",
			def: serviceDefinition{
				Name:     "fastapi-service",
				Image:    "nginx:latest",
				Port:     8000,
				Replicas: 1,
			},
			want: "allowed prefixes",
		},
		{
			name: "replicas above policy bound",
			def: serviceDefinition{
				Name:     "fastapi-service",
				Image:    "service-launchpad/fastapi-service:dev",
				Port:     8000,
				Replicas: defaultMaxReplicas + 1,
			},
			want: "replicas must be less than or equal",
		},
		{
			name: "autoscaling above policy bound",
			def: serviceDefinition{
				Name:     "fastapi-service",
				Image:    "service-launchpad/fastapi-service:dev",
				Port:     8000,
				Replicas: 1,
				Autoscaling: autoscalingConfig{
					Enabled:              true,
					MinReplicas:          1,
					MaxReplicas:          defaultMaxAutoscalingReplicas + 1,
					TargetCPUUtilization: 60,
				},
			},
			want: "autoscaling.maxReplicas",
		},
		{
			name: "cpu target above policy bound",
			def: serviceDefinition{
				Name:     "fastapi-service",
				Image:    "service-launchpad/fastapi-service:dev",
				Port:     8000,
				Replicas: 1,
				Autoscaling: autoscalingConfig{
					Enabled:              true,
					MinReplicas:          1,
					MaxReplicas:          3,
					TargetCPUUtilization: defaultMaxCPUUtilizationTarget + 1,
				},
			},
			want: "targetCpuUtilization",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := policy.validateServiceDefinition(tc.def)
			if err == nil {
				t.Fatalf("expected policy error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected error to contain %q, got %v", tc.want, err)
			}
		})
	}
}

func TestBuildKubernetesRESTConfigFromExplicitEndpoint(t *testing.T) {
	t.Setenv(envKubeAPIServer, "https://10.0.0.1")
	t.Setenv(envKubeCAData, base64.StdEncoding.EncodeToString([]byte("test-ca")))
	t.Setenv(envKubeBearerToken, "test-token")

	cfg, err := buildKubernetesRESTConfig("")
	if err != nil {
		t.Fatalf("buildKubernetesRESTConfig returned error: %v", err)
	}

	if cfg.Host != "https://10.0.0.1" {
		t.Fatalf("unexpected host: %s", cfg.Host)
	}
	if string(cfg.CAData) != "test-ca" {
		t.Fatalf("unexpected CA data: %q", string(cfg.CAData))
	}
	if cfg.BearerToken != "test-token" {
		t.Fatalf("unexpected bearer token: %s", cfg.BearerToken)
	}
}

func TestBuildKubernetesRESTConfigRejectsInvalidADCSetting(t *testing.T) {
	t.Setenv(envKubeAPIServer, "https://10.0.0.1")
	t.Setenv(envKubeUseADC, "sometimes")

	_, err := buildKubernetesRESTConfig("")
	if err == nil || !strings.Contains(err.Error(), envKubeUseADC) {
		t.Fatalf("expected invalid %s error, got %v", envKubeUseADC, err)
	}
}

func TestBuildKubernetesRESTConfigRejectsADCWithStaticToken(t *testing.T) {
	t.Setenv(envKubeAPIServer, "https://10.0.0.1")
	t.Setenv(envKubeUseADC, "true")
	t.Setenv(envKubeBearerToken, "test-token")

	_, err := buildKubernetesRESTConfig("")
	if err == nil || !strings.Contains(err.Error(), "cannot be combined") {
		t.Fatalf("expected conflicting Kubernetes credentials error, got %v", err)
	}
}

// cover successful and failed deploy requests with a fake deployer
func TestHandleServiceDeploy(t *testing.T) {
	store, err := newServiceStore("")
	if err != nil {
		t.Fatalf("newServiceStore returned error: %v", err)
	}

	service, err := store.create(serviceDefinition{
		Name:     "fastapi-service",
		Image:    "service-launchpad/fastapi-service:dev",
		Port:     8000,
		Replicas: 1,
		Autoscaling: autoscalingConfig{
			Enabled:              true,
			MinReplicas:          1,
			MaxReplicas:          5,
			TargetCPUUtilization: 60,
		},
	})
	if err != nil {
		t.Fatalf("create returned error: %v", err)
	}

	server := newAPIServer(store, defaultNamespace, fakeDeployer{
		result: applyResult{
			Command: "kubectl apply -f -",
			Output:  "deployment.apps/fastapi-service configured",
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/services/"+service.Name+"/deploy", nil)
	rec := httptest.NewRecorder()

	server.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var response map[string]any
	if err := json.NewDecoder(bytes.NewReader(rec.Body.Bytes())).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["status"] != "applied" {
		t.Fatalf("expected status=applied, got %v", response["status"])
	}

	result, ok := response["result"].(map[string]any)
	if !ok {
		t.Fatalf("expected result object in response")
	}
	if result["command"] != "kubectl apply -f -" {
		t.Fatalf("unexpected command value: %v", result["command"])
	}
}

func TestHandleServiceDeployWritesAuditRecord(t *testing.T) {
	store, err := newServiceStore("")
	if err != nil {
		t.Fatalf("newServiceStore returned error: %v", err)
	}

	service, err := store.create(serviceDefinition{
		Name:     "fastapi-service",
		Image:    "service-launchpad/fastapi-service:dev",
		Port:     8000,
		Replicas: 1,
	})
	if err != nil {
		t.Fatalf("create returned error: %v", err)
	}

	audit := &fakeAuditRecorder{
		result: auditResult{
			Bucket: "test-bucket",
			Object: "control-plane/deployments/test.json",
		},
	}
	server := newAPIServer(store, defaultNamespace, fakeDeployer{
		result: applyResult{
			Deployer: "client-go",
			Applied:  []string{"Deployment/fastapi-service"},
		},
	}, audit)

	req := httptest.NewRequest(http.MethodPost, "/services/"+service.Name+"/deploy", nil)
	rec := httptest.NewRecorder()

	server.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if audit.calls != 1 {
		t.Fatalf("expected one audit record, got %d", audit.calls)
	}
	if audit.lastRecord.Service.Name != "fastapi-service" {
		t.Fatalf("unexpected audited service: %s", audit.lastRecord.Service.Name)
	}
	if audit.lastRecord.Status != "success" {
		t.Fatalf("unexpected audit status: %s", audit.lastRecord.Status)
	}
	if audit.lastRecord.Manifests.Deployment["kind"] != "Deployment" {
		t.Fatalf("expected deployment manifest in audit record")
	}
	if !strings.Contains(rec.Body.String(), `"object":"control-plane/deployments/test.json"`) {
		t.Fatalf("expected audit object in response, got %s", rec.Body.String())
	}
}

func TestHandleServiceDeployFailure(t *testing.T) {
	store, err := newServiceStore("")
	if err != nil {
		t.Fatalf("newServiceStore returned error: %v", err)
	}

	if _, err := store.create(serviceDefinition{
		Name:     "fastapi-service",
		Image:    "service-launchpad/fastapi-service:dev",
		Port:     8000,
		Replicas: 1,
	}); err != nil {
		t.Fatalf("create returned error: %v", err)
	}

	server := newAPIServer(store, defaultNamespace, fakeDeployer{
		err: errors.New("kubectl apply failed"),
	})

	req := httptest.NewRequest(http.MethodPost, "/services/fastapi-service/deploy", nil)
	rec := httptest.NewRecorder()

	server.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected status 502, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "failed to apply manifests") {
		t.Fatalf("expected deploy failure message, got %s", rec.Body.String())
	}
}

func TestHandleServiceDeployFailureWritesAuditRecord(t *testing.T) {
	store, err := newServiceStore("")
	if err != nil {
		t.Fatalf("newServiceStore returned error: %v", err)
	}

	if _, err := store.create(serviceDefinition{
		Name:     "fastapi-service",
		Image:    "service-launchpad/fastapi-service:dev",
		Port:     8000,
		Replicas: 1,
	}); err != nil {
		t.Fatalf("create returned error: %v", err)
	}

	audit := &fakeAuditRecorder{}
	server := newAPIServer(store, defaultNamespace, fakeDeployer{
		err: errors.New("kubectl apply failed"),
	}, audit)

	req := httptest.NewRequest(http.MethodPost, "/services/fastapi-service/deploy", nil)
	rec := httptest.NewRecorder()

	server.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected status 502, got %d", rec.Code)
	}
	if audit.calls != 1 {
		t.Fatalf("expected one audit record, got %d", audit.calls)
	}
	if audit.lastRecord.Status != "failure" {
		t.Fatalf("unexpected audit status: %s", audit.lastRecord.Status)
	}
	if audit.lastRecord.Error != "kubectl apply failed" {
		t.Fatalf("unexpected audit error: %s", audit.lastRecord.Error)
	}
}

func TestHandleServiceRegistrationRejectsPolicyViolation(t *testing.T) {
	store, err := newServiceStore("")
	if err != nil {
		t.Fatalf("newServiceStore returned error: %v", err)
	}

	server := newAPIServer(store, defaultNamespace, nil)
	req := httptest.NewRequest(http.MethodPost, "/services", strings.NewReader(`{
		"name": "fastapi-service",
		"image": "docker.io/library/nginx:latest",
		"port": 8000,
		"replicas": 1
	}`))
	rec := httptest.NewRecorder()

	server.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "allowed prefixes") {
		t.Fatalf("expected policy message, got %s", rec.Body.String())
	}
	if store.count() != 0 {
		t.Fatalf("expected rejected service not to be stored")
	}
}

func TestHandleServiceDeployRejectsPolicyViolationAndWritesAuditRecord(t *testing.T) {
	store, err := newServiceStore("")
	if err != nil {
		t.Fatalf("newServiceStore returned error: %v", err)
	}

	service, err := store.create(serviceDefinition{
		Name:     "fastapi-service",
		Image:    "service-launchpad/fastapi-service:dev",
		Port:     8000,
		Replicas: 1,
	})
	if err != nil {
		t.Fatalf("create returned error: %v", err)
	}

	policy := defaultDeploymentPolicy()
	policy.AllowedImagePrefixes = []string{"europe-west10-docker.pkg.dev/geofaas-411316/service-launchpad/"}
	audit := &fakeAuditRecorder{}
	server := newAPIServerWithPolicy(store, defaultNamespace, fakeDeployer{}, policy, audit)

	req := httptest.NewRequest(http.MethodPost, "/services/"+service.Name+"/deploy", nil)
	rec := httptest.NewRecorder()

	server.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "deployment policy rejected request") {
		t.Fatalf("expected policy rejection response, got %s", rec.Body.String())
	}
	if audit.calls != 1 {
		t.Fatalf("expected one audit record, got %d", audit.calls)
	}
	if audit.lastRecord.Status != "failure" {
		t.Fatalf("unexpected audit status: %s", audit.lastRecord.Status)
	}
	if !strings.Contains(audit.lastRecord.Error, "allowed prefixes") {
		t.Fatalf("unexpected audit error: %s", audit.lastRecord.Error)
	}
}

func TestHealthReadyAndMetricsEndpoints(t *testing.T) {
	store, err := newServiceStore("")
	if err != nil {
		t.Fatalf("newServiceStore returned error: %v", err)
	}

	if _, err := store.create(serviceDefinition{
		Name:     "fastapi-service",
		Image:    "service-launchpad/fastapi-service:dev",
		Port:     8000,
		Replicas: 1,
	}); err != nil {
		t.Fatalf("create returned error: %v", err)
	}

	server := newAPIServer(store, defaultNamespace, fakeDeployer{})
	handler := server.routes()

	tests := []struct {
		name       string
		path       string
		wantStatus int
		wantBody   []string
	}{
		{
			name:       "health",
			path:       "/health",
			wantStatus: http.StatusOK,
			wantBody:   []string{`"status":"ok"`},
		},
		{
			name:       "ready",
			path:       "/ready",
			wantStatus: http.StatusOK,
			wantBody: []string{
				`"status":"ready"`,
				`"namespace":"service-launchpad-dev"`,
				`"managedServices":1`,
				`"deploymentEnabled":true`,
				`"metricsEnabled":true`,
			},
		},
		{
			name:       "metrics",
			path:       "/metrics",
			wantStatus: http.StatusOK,
			wantBody:   []string{"# HELP service_launchpad_control_plane_managed_services"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("expected status %d, got %d", tc.wantStatus, rec.Code)
			}

			body := rec.Body.String()
			for _, want := range tc.wantBody {
				if !strings.Contains(body, want) {
					t.Fatalf("expected response body to contain %q, got:\n%s", want, body)
				}
			}
		})
	}
}

func TestMetricsEndpointTracksRegistrationsAndManagedServices(t *testing.T) {
	store, err := newServiceStore("")
	if err != nil {
		t.Fatalf("newServiceStore returned error: %v", err)
	}

	server := newAPIServer(store, defaultNamespace, nil)
	handler := server.routes()

	createReq := httptest.NewRequest(http.MethodPost, "/services", strings.NewReader(`{
		"name": "fastapi-service",
		"image": "service-launchpad/fastapi-service:dev",
		"port": 8000,
		"replicas": 1
	}`))
	createRec := httptest.NewRecorder()
	handler.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", createRec.Code)
	}

	invalidReq := httptest.NewRequest(http.MethodPost, "/services", strings.NewReader(`{"name": "Bad_Name"}`))
	invalidRec := httptest.NewRecorder()
	handler.ServeHTTP(invalidRec, invalidReq)
	if invalidRec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", invalidRec.Code)
	}

	metricsReq := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	metricsRec := httptest.NewRecorder()
	handler.ServeHTTP(metricsRec, metricsReq)

	if metricsRec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", metricsRec.Code)
	}
	if contentType := metricsRec.Header().Get("Content-Type"); !strings.Contains(contentType, "text/plain") {
		t.Fatalf("expected text/plain metrics content type, got %q", contentType)
	}

	body := metricsRec.Body.String()
	expectedLines := []string{
		`service_launchpad_control_plane_service_registrations_total{result="success"} 1`,
		`service_launchpad_control_plane_service_registrations_total{result="failure"} 1`,
		`service_launchpad_control_plane_managed_services 1`,
	}
	for _, line := range expectedLines {
		if !strings.Contains(body, line) {
			t.Fatalf("expected metrics body to contain %q, got:\n%s", line, body)
		}
	}
}

func TestMetricsEndpointTracksDeploymentResultsAndDuration(t *testing.T) {
	store, err := newServiceStore("")
	if err != nil {
		t.Fatalf("newServiceStore returned error: %v", err)
	}

	service, err := store.create(serviceDefinition{
		Name:     "fastapi-service",
		Image:    "service-launchpad/fastapi-service:dev",
		Port:     8000,
		Replicas: 1,
	})
	if err != nil {
		t.Fatalf("create returned error: %v", err)
	}

	server := newAPIServer(store, defaultNamespace, fakeDeployer{
		result: applyResult{Command: "kubectl apply -f -"},
	})
	handler := server.routes()

	deployReq := httptest.NewRequest(http.MethodPost, "/services/"+service.Name+"/deploy", nil)
	deployRec := httptest.NewRecorder()
	handler.ServeHTTP(deployRec, deployReq)
	if deployRec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", deployRec.Code)
	}

	metricsReq := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	metricsRec := httptest.NewRecorder()
	handler.ServeHTTP(metricsRec, metricsReq)

	body := metricsRec.Body.String()
	expectedLines := []string{
		`service_launchpad_control_plane_deployments_total{result="success"} 1`,
		`service_launchpad_control_plane_deployment_duration_seconds_bucket{result="success",le="+Inf"} 1`,
		`service_launchpad_control_plane_deployment_duration_seconds_count{result="success"} 1`,
	}
	for _, line := range expectedLines {
		if !strings.Contains(body, line) {
			t.Fatalf("expected metrics body to contain %q, got:\n%s", line, body)
		}
	}
}

func TestGCSAuditRecorderUploadsDeploymentAuditRecord(t *testing.T) {
	var requestPath string
	var requestQuery url.Values
	var requestAuth string
	var requestBody []byte

	recorder := newGCSAuditRecorder("test-bucket", "audit-prefix", "https://storage.example.test", "static-token").(*gcsAuditRecorder)
	recorder.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			requestPath = r.URL.Path
			requestQuery = r.URL.Query()
			requestAuth = r.Header.Get("Authorization")

			var err error
			requestBody, err = io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("failed to read request body: %v", err)
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Body:       io.NopCloser(strings.NewReader("{}")),
				Header:     make(http.Header),
				Request:    r,
			}, nil
		}),
	}
	recorder.now = func() time.Time {
		return time.Date(2026, 6, 4, 12, 30, 0, 123456789, time.UTC)
	}

	service := serviceDefinition{
		Name:     "fastapi-service",
		Image:    "service-launchpad/fastapi-service:dev",
		Port:     8000,
		Replicas: 1,
	}
	result, err := recorder.RecordDeployment(context.Background(), deploymentAuditRecord{
		Service:   service,
		Namespace: "service-launchpad-prod",
		Status:    "success",
		Result: applyResult{
			Deployer: "client-go",
			Applied:  []string{"Deployment/fastapi-service"},
		},
		Manifests: renderManifestBundle(service, "service-launchpad-prod"),
	})
	if err != nil {
		t.Fatalf("RecordDeployment returned error: %v", err)
	}

	if requestPath != "/upload/storage/v1/b/test-bucket/o" {
		t.Fatalf("unexpected upload path: %s", requestPath)
	}
	if requestQuery.Get("uploadType") != "media" {
		t.Fatalf("unexpected uploadType: %s", requestQuery.Get("uploadType"))
	}
	if result.Object != "audit-prefix/2026/06/04/fastapi-service/20260604T123000.123456789Z-success.json" {
		t.Fatalf("unexpected object name: %s", result.Object)
	}
	if requestQuery.Get("name") != result.Object {
		t.Fatalf("unexpected object query name: %s", requestQuery.Get("name"))
	}
	if requestAuth != "Bearer static-token" {
		t.Fatalf("unexpected auth header: %s", requestAuth)
	}
	if !bytes.Contains(requestBody, []byte(`"namespace": "service-launchpad-prod"`)) {
		t.Fatalf("expected namespace in audit payload, got %s", string(requestBody))
	}
	if !bytes.Contains(requestBody, []byte(`"deployer": "client-go"`)) {
		t.Fatalf("expected deployer in audit payload, got %s", string(requestBody))
	}
}

type fakeDeployer struct {
	result     applyResult
	err        error
	lastBundle manifestBundle
}

func (f fakeDeployer) Apply(_ context.Context, bundle manifestBundle) (applyResult, error) {
	f.lastBundle = bundle
	return f.result, f.err
}

type fakeAuditRecorder struct {
	result     auditResult
	err        error
	calls      int
	lastRecord deploymentAuditRecord
}

func (f *fakeAuditRecorder) RecordDeployment(_ context.Context, record deploymentAuditRecord) (auditResult, error) {
	f.calls++
	f.lastRecord = record
	return f.result, f.err
}

func renderedTemplateSpec(t *testing.T, bundle *manifestBundle) map[string]any {
	t.Helper()
	spec, ok := bundle.Deployment["spec"].(map[string]any)
	if !ok {
		t.Fatalf("expected deployment spec")
	}
	template, ok := spec["template"].(map[string]any)
	if !ok {
		t.Fatalf("expected deployment template")
	}
	templateSpec, ok := template["spec"].(map[string]any)
	if !ok {
		t.Fatalf("expected deployment template spec")
	}
	return templateSpec
}

func firstRenderedContainer(t *testing.T, bundle *manifestBundle) map[string]any {
	t.Helper()
	templateSpec := renderedTemplateSpec(t, bundle)
	containers, ok := templateSpec["containers"].([]any)
	if !ok || len(containers) == 0 {
		t.Fatalf("expected deployment containers")
	}
	container, ok := containers[0].(map[string]any)
	if !ok {
		t.Fatalf("expected first container")
	}
	return container
}

func cloneManifest(manifest map[string]any) map[string]any {
	if manifest == nil {
		return nil
	}
	cloned := make(map[string]any, len(manifest))
	for key, value := range manifest {
		cloned[key] = cloneValue(value)
	}
	return cloned
}

func cloneValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneManifest(typed)
	case []any:
		cloned := make([]any, len(typed))
		for i, item := range typed {
			cloned[i] = cloneValue(item)
		}
		return cloned
	default:
		return value
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
