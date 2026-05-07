package main

import (
	"log"
	"net/http"
	"os"
	"time"
)

const defaultListenAddr = ":8080"
const defaultNamespace = "service-launchpad-dev"

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
		Handler:           newAPIServer(store, defaultNamespace).routes(),
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
