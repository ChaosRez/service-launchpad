package main

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	defaultDeployerMode = "client-go"

	envDeployerMode        = "CONTROL_PLANE_DEPLOYER_MODE"
	envTargetNamespace     = "CONTROL_PLANE_TARGET_NAMESPACE"
	envKubeconfig          = "CONTROL_PLANE_KUBECONFIG"
	envKubeContext         = "CONTROL_PLANE_KUBE_CONTEXT"
	envKubeAPIServer       = "CONTROL_PLANE_KUBE_API_SERVER"
	envKubeCAFile          = "CONTROL_PLANE_KUBE_CA_FILE"
	envKubeCAData          = "CONTROL_PLANE_KUBE_CA_DATA"
	envKubeBearerToken     = "CONTROL_PLANE_KUBE_BEARER_TOKEN"
	envKubeBearerTokenFile = "CONTROL_PLANE_KUBE_BEARER_TOKEN_FILE"
	envAuditBucket         = "CONTROL_PLANE_AUDIT_BUCKET"
	envAuditPrefix         = "CONTROL_PLANE_AUDIT_PREFIX"
	envGCSEndpoint         = "CONTROL_PLANE_GCS_ENDPOINT"
	envGCSBearerToken      = "CONTROL_PLANE_GCS_BEARER_TOKEN"
)

type controlPlaneConfig struct {
	ListenAddr       string
	StorePath        string
	TargetNamespace  string
	DeployerMode     string
	KubectlBinary    string
	KubeContext      string
	AuditBucket      string
	AuditPrefix      string
	GCSEndpoint      string
	GCSBearerToken   string
	DeploymentPolicy deploymentPolicy
}

func loadControlPlaneConfig() (controlPlaneConfig, error) {
	listenAddr := strings.TrimSpace(os.Getenv("CONTROL_PLANE_LISTEN_ADDR"))
	if listenAddr == "" {
		listenAddr = defaultListenAddr
	}

	namespace := strings.TrimSpace(os.Getenv(envTargetNamespace))
	if namespace == "" {
		namespace = defaultNamespace
	}

	mode := strings.ToLower(strings.TrimSpace(os.Getenv(envDeployerMode)))
	if mode == "" {
		mode = defaultDeployerMode
	}

	kubeContext := strings.TrimSpace(os.Getenv(envKubeContext))
	if kubeContext == "" {
		kubeContext = strings.TrimSpace(os.Getenv("CONTROL_PLANE_KUBECTL_CONTEXT"))
	}

	policy, err := loadDeploymentPolicyFromEnv(os.Getenv)
	if err != nil {
		return controlPlaneConfig{}, err
	}

	return controlPlaneConfig{
		ListenAddr:       listenAddr,
		StorePath:        os.Getenv("CONTROL_PLANE_STORE_PATH"),
		TargetNamespace:  namespace,
		DeployerMode:     mode,
		KubectlBinary:    os.Getenv("CONTROL_PLANE_KUBECTL_BIN"),
		KubeContext:      kubeContext,
		AuditBucket:      strings.TrimSpace(os.Getenv(envAuditBucket)),
		AuditPrefix:      strings.TrimSpace(os.Getenv(envAuditPrefix)),
		GCSEndpoint:      strings.TrimSpace(os.Getenv(envGCSEndpoint)),
		GCSBearerToken:   strings.TrimSpace(os.Getenv(envGCSBearerToken)),
		DeploymentPolicy: policy,
	}, nil
}

func newManifestDeployerFromConfig(cfg controlPlaneConfig) (manifestDeployer, error) {
	switch cfg.DeployerMode {
	case "client-go":
		return newLazyClientGoDeployer(cfg.KubeContext), nil
	case "kubectl":
		return newKubectlDeployer(cfg.KubectlBinary, cfg.KubeContext), nil
	case "disabled", "none":
		return nil, nil
	default:
		return nil, fmt.Errorf("unsupported %s %q; expected client-go, kubectl, or disabled", envDeployerMode, cfg.DeployerMode)
	}
}

func buildKubernetesRESTConfig(kubeContext string) (*rest.Config, error) {
	if apiServer := strings.TrimSpace(os.Getenv(envKubeAPIServer)); apiServer != "" {
		config := &rest.Config{
			Host: apiServer,
			TLSClientConfig: rest.TLSClientConfig{
				CAFile: strings.TrimSpace(os.Getenv(envKubeCAFile)),
			},
			BearerToken:     strings.TrimSpace(os.Getenv(envKubeBearerToken)),
			BearerTokenFile: strings.TrimSpace(os.Getenv(envKubeBearerTokenFile)),
		}

		if caData := strings.TrimSpace(os.Getenv(envKubeCAData)); caData != "" {
			decoded, err := base64.StdEncoding.DecodeString(caData)
			if err != nil {
				return nil, fmt.Errorf("decode %s: %w", envKubeCAData, err)
			}
			config.TLSClientConfig.CAData = decoded
		}

		return config, nil
	}

	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if kubeconfig := strings.TrimSpace(os.Getenv(envKubeconfig)); kubeconfig != "" {
		loadingRules.ExplicitPath = kubeconfig
	} else if home, err := os.UserHomeDir(); err == nil {
		loadingRules.ExplicitPath = filepath.Join(home, ".kube", "config")
	}

	overrides := &clientcmd.ConfigOverrides{CurrentContext: kubeContext}
	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("load Kubernetes client config: %w", err)
	}
	return config, nil
}
