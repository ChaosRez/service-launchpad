package main

import (
	"log"
	"net/http"
	"strings"
	"time"
)

const defaultListenAddr = ":8080"
const defaultNamespace = "service-launchpad-dev"

func main() {
	cfg, err := loadControlPlaneConfig()
	if err != nil {
		log.Fatalf("control-plane config failed: %v", err)
	}

	store, err := newServiceStore(cfg.StorePath)
	if err != nil {
		log.Fatalf("control-plane store setup failed: %v", err)
	}

	deployer, err := newManifestDeployerFromConfig(cfg)
	if err != nil {
		log.Fatalf("control-plane deployer setup failed: %v", err)
	}
	auditRecorder := newGCSAuditRecorder(cfg.AuditBucket, cfg.AuditPrefix, cfg.GCSEndpoint, cfg.GCSBearerToken)

	server := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           newAPIServerWithPolicy(store, cfg.TargetNamespace, deployer, cfg.DeploymentPolicy, auditRecorder).routes(),
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
	if cfg.AuditBucket != "" {
		prefix := cfg.AuditPrefix
		if prefix == "" {
			prefix = defaultAuditPrefix
		}
		log.Printf("control-plane deployment audit enabled for gs://%s/%s", cfg.AuditBucket, prefix)
	}
	log.Printf("control-plane deployment policy image prefixes %s", strings.Join(cfg.DeploymentPolicy.AllowedImagePrefixes, ","))
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("control-plane server failed: %v", err)
	}
}
