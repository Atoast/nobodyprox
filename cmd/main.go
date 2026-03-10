package main

import (
	"log"
	"net/http"

	"github.com/nobodyprox/nobodyprox/pkg/cert"
	"github.com/nobodyprox/nobodyprox/pkg/proxy"
)

func main() {
	// Initialize CA
	ca, err := cert.LoadOrCreateCA("certs")
	if err != nil {
		log.Fatalf("Failed to initialize CA: %v", err)
	}

	p := &proxy.Proxy{
		CA: ca,
	}

	addr := ":8080"
	log.Printf("Privacy Proxy starting on %s", addr)
	log.Printf("To use: configure your tool to use HTTP_PROXY=http://localhost:8080")
	log.Printf("And trust the Root CA at ./certs/ca.crt")

	if err := http.ListenAndServe(addr, p); err != nil {
		log.Fatalf("Proxy server failed: %v", err)
	}
}
