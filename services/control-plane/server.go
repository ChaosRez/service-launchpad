package main

// routing, handlers, JSON/error helpers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"
)

type apiServer struct {
	store     *serviceStore
	namespace string
	deployer  manifestDeployer
	metrics   *controlPlaneMetrics
	audit     auditRecorder
}

func newAPIServer(store *serviceStore, namespace string, deployer manifestDeployer, auditRecorders ...auditRecorder) *apiServer {
	var audit auditRecorder
	if len(auditRecorders) > 0 {
		audit = auditRecorders[0]
	}

	return &apiServer{
		store:     store,
		namespace: namespace,
		deployer:  deployer,
		metrics:   newControlPlaneMetrics(store),
		audit:     audit,
	}
}

func (a *apiServer) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", a.handleHealth)
	mux.HandleFunc("/ready", a.handleReady)
	mux.HandleFunc("/metrics", a.metrics.handleMetrics)
	mux.HandleFunc("/services", a.handleServices)
	mux.HandleFunc("/services/", a.handleServiceByName)
	return loggingMiddleware(mux)
}

func (a *apiServer) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}

func (a *apiServer) handleReady(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":             "ready",
		"namespace":          a.namespace,
		"managedServices":    a.store.count(),
		"deploymentEnabled":  a.deployer != nil,
		"metricsEnabled":     a.metrics != nil,
		"persistenceEnabled": a.store.storePath != "",
		"auditEnabled":       a.audit != nil,
	})
}

func (a *apiServer) handleServices(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{
			"services": a.store.list(),
		})
	case http.MethodPost:
		var def serviceDefinition
		if err := json.NewDecoder(r.Body).Decode(&def); err != nil {
			a.metrics.recordServiceRegistration("failure")
			writeError(w, http.StatusBadRequest, "invalid JSON request body")
			return
		}

		if err := validateServiceDefinition(def); err != nil {
			a.metrics.recordServiceRegistration("failure")
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		created, err := a.store.create(def)
		if err != nil {
			a.metrics.recordServiceRegistration("failure")
			if strings.Contains(err.Error(), "already exists") {
				writeError(w, http.StatusConflict, err.Error()) // 409 Conflict instead of silent overwrite
				return
			}
			log.Printf("failed to create service definition: %v", err)
			writeError(w, http.StatusInternalServerError, "failed to persist service definition")
			return
		}

		a.metrics.recordServiceRegistration("success")
		writeJSON(w, http.StatusCreated, map[string]any{
			"service": created,
		})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (a *apiServer) handleServiceByName(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/services/")
	path = strings.Trim(path, "/")
	if path == "" {
		writeError(w, http.StatusNotFound, "service not found")
		return
	}

	parts := strings.Split(path, "/")
	name := parts[0]
	if name == "" {
		writeError(w, http.StatusNotFound, "service not found")
		return
	}

	service, ok := a.store.get(name)
	if !ok {
		writeError(w, http.StatusNotFound, "service not found")
		return
	}

	switch {
	case len(parts) == 1 && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{
			"service": service,
		})
		return
	case len(parts) == 2 && parts[1] == "manifests" && r.Method == http.MethodGet:
		bundle := renderManifestBundle(service, a.namespace)
		writeJSON(w, http.StatusOK, map[string]any{
			"service":   service,
			"manifests": bundle,
		})
		return
	case len(parts) == 2 && parts[1] == "deploy" && r.Method == http.MethodPost:
		a.handleServiceDeploy(w, r, service)
		return
	case len(parts) >= 1:
		if r.Method != http.MethodGet && !(len(parts) == 2 && parts[1] == "deploy" && r.Method == http.MethodPost) {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
	}

	writeError(w, http.StatusNotFound, "service not found")
}

func (a *apiServer) handleServiceDeploy(w http.ResponseWriter, r *http.Request, service serviceDefinition) {
	if a.deployer == nil {
		a.metrics.recordDeployment("failure", 0)
		writeError(w, http.StatusNotImplemented, "deployment is not configured")
		return
	}

	bundle := renderManifestBundle(service, a.namespace)
	start := time.Now()
	result, err := a.deployer.Apply(r.Context(), bundle)
	duration := time.Since(start)
	if err != nil {
		a.metrics.recordDeployment("failure", duration)
		log.Printf("failed to apply manifests for %s: %v", service.Name, err)
		response := map[string]any{
			"error":     "failed to apply manifests",
			"service":   service.Name,
			"namespace": a.namespace,
			"details":   err.Error(),
		}
		if auditResult, auditErr := a.recordDeploymentAudit(r.Context(), service, bundle, result, "failure", duration, err); auditErr != nil {
			log.Printf("failed to write deployment audit record for %s: %v", service.Name, auditErr)
		} else if auditResult.Object != "" {
			response["audit"] = auditResult
		}
		writeJSON(w, http.StatusBadGateway, response)
		return
	}

	a.metrics.recordDeployment("success", duration)
	response := map[string]any{
		"status":    "applied",
		"service":   service,
		"namespace": a.namespace,
		"result":    result,
		"manifests": bundle,
	}
	if auditResult, auditErr := a.recordDeploymentAudit(r.Context(), service, bundle, result, "success", duration, nil); auditErr != nil {
		log.Printf("failed to write deployment audit record for %s: %v", service.Name, auditErr)
	} else if auditResult.Object != "" {
		response["audit"] = auditResult
	}
	writeJSON(w, http.StatusOK, response)
}

func (a *apiServer) recordDeploymentAudit(ctx context.Context, service serviceDefinition, bundle manifestBundle, result applyResult, status string, duration time.Duration, deployErr error) (auditResult, error) {
	if a.audit == nil {
		return auditResult{}, nil
	}

	record := deploymentAuditRecord{
		RecordedAt:           time.Now().UTC(),
		Service:              service,
		Namespace:            a.namespace,
		Status:               status,
		DurationMilliseconds: duration.Milliseconds(),
		Result:               result,
		Manifests:            bundle,
	}
	if deployErr != nil {
		record.Error = deployErr.Error()
	}
	return a.audit.RecordDeployment(ctx, record)
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start).Round(time.Millisecond))
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("failed to encode response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{
		"error": message,
	})
}
