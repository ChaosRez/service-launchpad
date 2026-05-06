package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"
)

type apiServer struct {
	store     *serviceStore
	namespace string
}

func newAPIServer(store *serviceStore, namespace string) *apiServer {
	return &apiServer{
		store:     store,
		namespace: namespace,
	}
}

func (a *apiServer) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", a.handleHealth)
	mux.HandleFunc("/services", a.handleServices)
	mux.HandleFunc("/services/", a.handleServiceByName)
	return loggingMiddleware(mux)
}

func (a *apiServer) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
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
			writeError(w, http.StatusBadRequest, "invalid JSON request body")
			return
		}

		if err := validateServiceDefinition(def); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		created, err := a.store.create(def)
		if err != nil {
			if strings.Contains(err.Error(), "already exists") {
				writeError(w, http.StatusConflict, err.Error()) // 409 Conflict instead of silent overwrite
				return
			}
			log.Printf("failed to create service definition: %v", err)
			writeError(w, http.StatusInternalServerError, "failed to persist service definition")
			return
		}

		writeJSON(w, http.StatusCreated, map[string]any{
			"service": created,
		})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (a *apiServer) handleServiceByName(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

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

	if len(parts) == 2 && parts[1] == "manifests" {
		bundle := renderManifestBundle(service, a.namespace)
		writeJSON(w, http.StatusOK, map[string]any{
			"service":   service,
			"manifests": bundle,
		})
		return
	}
	if len(parts) > 1 {
		writeError(w, http.StatusNotFound, "service not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"service": service,
	})
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
