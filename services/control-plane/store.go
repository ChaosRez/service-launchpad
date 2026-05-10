package main

// in-memory + optional JSON persistence

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

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

func (s *serviceStore) count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.services)
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
