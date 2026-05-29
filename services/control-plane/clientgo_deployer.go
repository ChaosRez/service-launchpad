package main

import (
	"context"
	"encoding/json"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/restmapper"
)

const clientGoFieldManager = "service-launchpad-control-plane"

type clientGoDeployer struct {
	client      dynamic.Interface
	mapper      meta.RESTMapper
	kubeContext string
}

func newClientGoDeployer(client dynamic.Interface, discoveryClient discovery.DiscoveryInterface) manifestDeployer {
	return &clientGoDeployer{
		client: client,
		mapper: restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(discoveryClient)),
	}
}

func newLazyClientGoDeployer(kubeContext string) manifestDeployer {
	return &clientGoDeployer{kubeContext: kubeContext}
}

func (d *clientGoDeployer) Apply(ctx context.Context, bundle manifestBundle) (applyResult, error) {
	if err := d.ensureClient(); err != nil {
		return applyResult{Deployer: "client-go"}, err
	}

	objects, err := bundleKubernetesObjects(bundle)
	if err != nil {
		return applyResult{Deployer: "client-go"}, err
	}

	applied := make([]string, 0, len(objects))
	for _, object := range objects {
		patch, err := json.Marshal(object.Object)
		if err != nil {
			return applyResult{Deployer: "client-go", Applied: applied}, fmt.Errorf("marshal %s/%s: %w", object.GetKind(), object.GetName(), err)
		}

		mapping, err := d.mapper.RESTMapping(object.GroupVersionKind().GroupKind(), object.GroupVersionKind().Version)
		if err != nil {
			return applyResult{Deployer: "client-go", Applied: applied}, fmt.Errorf("resolve REST mapping for %s: %w", object.GroupVersionKind().String(), err)
		}

		namespaceableResource := d.client.Resource(mapping.Resource)
		var resource dynamic.ResourceInterface
		resource = namespaceableResource
		if object.GetNamespace() != "" {
			resource = namespaceableResource.Namespace(object.GetNamespace())
		}

		if _, err := resource.Patch(ctx, object.GetName(), types.ApplyPatchType, patch, metav1.PatchOptions{
			FieldManager: clientGoFieldManager,
			Force:        ptrTo(true),
		}); err != nil {
			return applyResult{Deployer: "client-go", Applied: applied}, fmt.Errorf("apply %s %s: %w", object.GetKind(), object.GetName(), err)
		}

		applied = append(applied, fmt.Sprintf("%s/%s", object.GetKind(), object.GetName()))
	}

	return applyResult{
		Deployer: "client-go",
		Applied:  applied,
	}, nil
}

func (d *clientGoDeployer) ensureClient() error {
	if d.client != nil && d.mapper != nil {
		return nil
	}

	restConfig, err := buildKubernetesRESTConfig(d.kubeContext)
	if err != nil {
		return err
	}

	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("create dynamic Kubernetes client: %w", err)
	}

	discoveryClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("create Kubernetes discovery client: %w", err)
	}

	d.client = dynamicClient
	d.mapper = restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(discoveryClient.Discovery()))
	return nil
}

func bundleKubernetesObjects(bundle manifestBundle) ([]*unstructured.Unstructured, error) {
	manifests := []map[string]any{bundle.NamespaceManifest}
	if bundle.ConfigMap != nil {
		manifests = append(manifests, bundle.ConfigMap)
	}
	manifests = append(manifests, bundle.Deployment, bundle.Service)
	if bundle.HPA != nil {
		manifests = append(manifests, bundle.HPA)
	}

	objects := make([]*unstructured.Unstructured, 0, len(manifests))
	for _, manifest := range manifests {
		object := &unstructured.Unstructured{Object: manifest}
		if object.GetAPIVersion() == "" || object.GetKind() == "" || object.GetName() == "" {
			return nil, fmt.Errorf("manifest missing apiVersion, kind, or metadata.name")
		}
		objects = append(objects, object)
	}
	return objects, nil
}

func clientGoResourceIntent(bundle manifestBundle) ([]schema.GroupVersionKind, error) {
	objects, err := bundleKubernetesObjects(bundle)
	if err != nil {
		return nil, err
	}

	intent := make([]schema.GroupVersionKind, 0, len(objects))
	for _, object := range objects {
		intent = append(intent, object.GroupVersionKind())
	}
	return intent, nil
}

func ptrTo[T any](value T) *T {
	return &value
}
