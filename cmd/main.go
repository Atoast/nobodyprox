package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/nobodyprox/nobodyprox/pkg/cert"
	"github.com/nobodyprox/nobodyprox/pkg/config"
	"github.com/nobodyprox/nobodyprox/pkg/filter"
	"github.com/nobodyprox/nobodyprox/pkg/proxy"
)

func main() {
	// Load Configuration
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize NER Provider
	var ner filter.NERProvider
	switch cfg.NERProvider {
	case "prose":
		ner, err = filter.NewProseProvider()
		if err != nil {
			log.Fatalf("Failed to initialize Prose provider: %v", err)
		}
	default:
		log.Printf("No NER provider configured or unknown provider: %s", cfg.NERProvider)
	}

	// Initialize Filter Engine
	engine, err := filter.NewEngine(cfg.Rules, ner)
	if err != nil {
		log.Fatalf("Failed to initialize filter engine: %v", err)
	}

	// Initialize CA
	ca, err := cert.LoadOrCreateCA("certs")
	if err != nil {
		log.Fatalf("Failed to initialize CA: %v", err)
	}

	p := &proxy.Proxy{
		CA:     ca,
		Filter: engine,
	}

	addr := fmt.Sprintf(":%d", cfg.ProxyPort)
	log.Printf("Privacy Proxy starting on %s", addr)
	log.Printf("NER Provider: %s", cfg.NERProvider)
	log.Printf("To use: configure your tool to use HTTP_PROXY=http://localhost%s", addr)
	log.Printf("And trust the Root CA at ./certs/ca.crt")

	if err := http.ListenAndServe(addr, p); err != nil {
		log.Fatalf("Proxy server failed: %v", err)
	}
}
