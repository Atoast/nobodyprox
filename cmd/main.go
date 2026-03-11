package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/nobodyprox/nobodyprox/pkg/cert"
	"github.com/nobodyprox/nobodyprox/pkg/config"
	"github.com/nobodyprox/nobodyprox/pkg/filter"
	"github.com/nobodyprox/nobodyprox/pkg/proxy"
)

func main() {
	// Parse Command Line Flags
	watchFlag := flag.Bool("watch", false, "Enable watch mode (logs sensitive data without redacting)")
	flag.Parse()

	// Load Configuration
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Override config with command line flag if set
	if *watchFlag {
		log.Printf("Command line --watch flag detected, enabling WatchMode")
		cfg.WatchMode = true
	}
	log.Printf("Final cfg.WatchMode value: %v", cfg.WatchMode)

	// Initialize NER Provider
	var ner filter.NERProvider
	switch cfg.NERProvider {
	case "prose":
		ner, err = filter.NewProseProvider()
		if err != nil {
			log.Fatalf("Failed to initialize Prose provider: %v", err)
		}
	case "onnx":
		onnxCfg, ok := cfg.ONNXModels[cfg.ActiveModel]
		if !ok {
			log.Printf("Warning: Active model %s not found in onnx_models", cfg.ActiveModel)
		} else {
			onnxProvider, err := filter.NewONNXProvider(
				onnxCfg.ModelPath,
				onnxCfg.VocabPath,
				onnxCfg.ConfigPath,
				cfg.ONNXRuntimeURL,
				onnxCfg.ModelDownloadURL,
				onnxCfg.VocabDownloadURL,
				onnxCfg.ConfigDownloadURL,
				onnxCfg.Labels,
			)
			if err != nil {
				log.Printf("Warning: Failed to initialize ONNX provider: %v", err)
			} else {
				ner = onnxProvider
			}
		}
	default:
		log.Printf("No NER provider configured or unknown provider: %s", cfg.NERProvider)
	}

	// Initialize Filter Engine
	log.Printf("Initializing Engine with WatchMode: %v", cfg.WatchMode)
	engine, err := filter.NewEngine(cfg.Rules, ner, cfg.WatchMode)
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
