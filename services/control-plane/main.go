package main

import (
	"log"
	"net/http"
	"time"
)

const defaultListenAddr = ":8080"
const defaultNamespace = "service-launchpad-dev"

func main() {
	cfg := loadControlPlaneConfig()

	store, err := newServiceStore(cfg.StorePath)
	if err != nil {
		log.Fatalf("control-plane store setup failed: %v", err)
	}

	deployer, err := newManifestDeployerFromConfig(cfg)
	if err != nil {
		log.Fatalf("control-plane deployer setup failed: %v", err)
	}

	server := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           newAPIServer(store, cfg.TargetNamespace, deployer).routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("control-plane listening on %s", cfg.ListenAddr)
	log.Printf("control-plane target namespace %s", cfg.TargetNamespace)
	log.Printf("control-plane deployer mode %s", cfg.DeployerMode)
	if cfg.StorePath != "" {
		log.Printf("control-plane persistence enabled at %s", cfg.StorePath)
	}
	if cfg.KubeContext != "" {
		log.Printf("control-plane Kubernetes context set to %s", cfg.KubeContext)
	}
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("control-plane server failed: %v", err)
	}
}
