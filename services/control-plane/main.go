package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const defaultListenAddr = ":8080"

type autoscalingConfig struct {
	Enabled              bool `json:"enabled"`
	MinReplicas          int  `json:"minReplicas"`
	MaxReplicas          int  `json:"maxReplicas"`
	TargetCPUUtilization int  `json:"targetCpuUtilization"`
}

type serviceDefinition struct {
	Name        string            `json:"name"`
	Image       string            `json:"image"`
	Port        int               `json:"port"`
	Replicas    int               `json:"replicas"`
	Autoscaling autoscalingConfig `json:"autoscaling"`
	CreatedAt   time.Time         `json:"createdAt"`
}

type serviceStore struct {
	mu       sync.RWMutex
	services map[string]serviceDefinition
}

func newServiceStore() *serviceStore {
	return &serviceStore{
		services: make(map[string]serviceDefinition),
	}
}

func (s *serviceStore) create(def serviceDefinition) serviceDefinition {
	s.mu.Lock()
	defer s.mu.Unlock()

	def.CreatedAt = time.Now().UTC()
	s.services[def.Name] = def
	return def
}

func (s *serviceStore) list() []serviceDefinition {
	s.mu.RLock()
	defer s.mu.RUnlock()

	services := make([]serviceDefinition, 0, len(s.services))
	for _, service := range s.services {
		services = append(services, service)
	}

	return services
}

func (s *serviceStore) get(name string) (serviceDefinition, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	service, ok := s.services[name]
	return service, ok
}

type apiServer struct {
	store *serviceStore
}

func newAPIServer() *apiServer {
	return &apiServer{
		store: newServiceStore(),
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

		created := a.store.create(def)
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

	name := strings.TrimPrefix(r.URL.Path, "/services/")
	if name == "" || strings.Contains(name, "/") {
		writeError(w, http.StatusNotFound, "service not found")
		return
	}

	service, ok := a.store.get(name)
	if !ok {
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

func main() {
	listenAddr := os.Getenv("CONTROL_PLANE_LISTEN_ADDR")
	if listenAddr == "" {
		listenAddr = defaultListenAddr
	}

	server := &http.Server{
		Addr:              listenAddr,
		Handler:           newAPIServer().routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("control-plane listening on %s", listenAddr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("control-plane server failed: %v", err)
	}
}
