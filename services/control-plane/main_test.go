package main

// for validation, duplicate rejection, and JSON persistence reload

import (
	"os"
	"path/filepath"
	"testing"
	"time"
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
