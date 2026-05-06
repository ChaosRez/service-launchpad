package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
)

const defaultListenAddr = ":8080"

var serviceNamePattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)

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
	mu        sync.RWMutex
	services  map[string]serviceDefinition
	storePath string
}

func newServiceStore(storePath string) (*serviceStore, error) {
	store := &serviceStore{
		services:  make(map[string]serviceDefinition),
		storePath: storePath,
	}

	if err := store.load(); err != nil {
		return nil, err
	}

	return store, nil
}

func (s *serviceStore) create(def serviceDefinition) (serviceDefinition, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.services[def.Name]; exists {
		return serviceDefinition{}, fmt.Errorf("service %q already exists", def.Name)
	}

	def.CreatedAt = time.Now().UTC()
	s.services[def.Name] = def

	if err := s.persistLocked(); err != nil {
		delete(s.services, def.Name)
		return serviceDefinition{}, err
	}

	return def, nil
}

func (s *serviceStore) list() []serviceDefinition {
	s.mu.RLock()
	defer s.mu.RUnlock()

	services := make([]serviceDefinition, 0, len(s.services))
	for _, service := range s.services {
		services = append(services, service)
	}

	slices.SortFunc(services, func(a, b serviceDefinition) int {
		return strings.Compare(a.Name, b.Name)
	})

	return services
}

func (s *serviceStore) get(name string) (serviceDefinition, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	service, ok := s.services[name]
	return service, ok
}

func (s *serviceStore) load() error {
	if s.storePath == "" {
		return nil
	}

	data, err := os.ReadFile(s.storePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read store file: %w", err)
	}

	if len(data) == 0 {
		return nil
	}

	var services []serviceDefinition
	if err := json.Unmarshal(data, &services); err != nil {
		return fmt.Errorf("decode store file: %w", err)
	}

	for _, service := range services {
		s.services[service.Name] = service
	}

	return nil
}

func (s *serviceStore) persistLocked() error {
	if s.storePath == "" {
		return nil
	}

	services := make([]serviceDefinition, 0, len(s.services))
	for _, service := range s.services {
		services = append(services, service)
	}

	slices.SortFunc(services, func(a, b serviceDefinition) int {
		return strings.Compare(a.Name, b.Name)
	})

	data, err := json.MarshalIndent(services, "", "  ")
	if err != nil {
		return fmt.Errorf("encode store file: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(s.storePath), 0o755); err != nil {
		return fmt.Errorf("create store directory: %w", err)
	}

	if err := os.WriteFile(s.storePath, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write store file: %w", err)
	}

	return nil
}

type apiServer struct {
	store *serviceStore
}

func newAPIServer(store *serviceStore) *apiServer {
	return &apiServer{
		store: store,
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

func validateServiceDefinition(def serviceDefinition) error {
	if strings.TrimSpace(def.Name) == "" {
		return errors.New("name is required")
	}
	if !serviceNamePattern.MatchString(def.Name) {
		return errors.New("name must be lowercase letters, numbers, or hyphens, and must start and end with an alphanumeric character")
	}
	if strings.TrimSpace(def.Image) == "" {
		return errors.New("image is required")
	}
	if def.Port < 1 || def.Port > 65535 {
		return errors.New("port must be between 1 and 65535")
	}
	if def.Replicas < 1 {
		return errors.New("replicas must be at least 1")
	}
	if !def.Autoscaling.Enabled {
		return nil
	}
	if def.Autoscaling.MinReplicas < 1 {
		return errors.New("autoscaling.minReplicas must be at least 1 when autoscaling is enabled")
	}
	if def.Autoscaling.MaxReplicas < def.Autoscaling.MinReplicas {
		return errors.New("autoscaling.maxReplicas must be greater than or equal to autoscaling.minReplicas")
	}
	if def.Autoscaling.TargetCPUUtilization < 1 || def.Autoscaling.TargetCPUUtilization > 100 {
		return errors.New("autoscaling.targetCpuUtilization must be between 1 and 100 when autoscaling is enabled")
	}

	return nil
}

func main() {
	listenAddr := os.Getenv("CONTROL_PLANE_LISTEN_ADDR")
	if listenAddr == "" {
		listenAddr = defaultListenAddr
	}

	storePath := os.Getenv("CONTROL_PLANE_STORE_PATH") // otherwise, in-memory only
	store, err := newServiceStore(storePath)
	if err != nil {
		log.Fatalf("control-plane store setup failed: %v", err)
	}

	server := &http.Server{
		Addr:              listenAddr,
		Handler:           newAPIServer(store).routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("control-plane listening on %s", listenAddr)
	if storePath != "" {
		log.Printf("control-plane persistence enabled at %s", storePath)
	}
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("control-plane server failed: %v", err)
	}
}
