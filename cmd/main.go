package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/nobodyprox/nobodyprox/pkg/cert"
	"github.com/nobodyprox/nobodyprox/pkg/config"
	"github.com/nobodyprox/nobodyprox/pkg/filter"
	"github.com/nobodyprox/nobodyprox/pkg/proxy"
)

func main() {
	// 1. Check for 'setup' sub-command before flag parsing
	if len(os.Args) > 1 && os.Args[1] == "setup" {
		handleSetup()
		return
	}

	// 2. Parse Standard Command Line Flags
	watchFlag := flag.Bool("watch", false, "Enable watch mode (logs sensitive data without redacting)")
	flag.Parse()

	// 3. Load Configuration
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if *watchFlag {
		log.Printf("Command line --watch flag detected, enabling WatchMode")
		cfg.WatchMode = true
	}

	// 4. Trust Verification
	tm := cert.NewTrustManager()
	if !tm.IsTrusted("NobodyProx Root CA") {
		log.Println("--------------------------------------------------------------------------------")
		log.Println("[WARNING] Root CA is NOT trusted by your system.")
		log.Println("HTTPS filtering will fail with SSL errors until the CA is trusted.")
		log.Println("Run 'nobodyprox setup' to automate trust and model installation.")
		log.Println("--------------------------------------------------------------------------------")
	}

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
	log.Printf("Privacy Proxy starting on %s (WatchMode: %v)", addr, cfg.WatchMode)
	log.Printf("To use: configure your tool to use HTTP_PROXY=http://localhost%s", addr)

	if err := http.ListenAndServe(addr, p); err != nil {
		log.Fatalf("Proxy server failed: %v", err)
	}
}

func handleSetup() {
	log.Println("=== NobodyProx Interactive Setup ===")
	
	// Load config
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Setup failed: could not load config: %v", err)
	}

	// Bootstrap models
	if err := filter.BootstrapAll(cfg); err != nil {
		log.Fatalf("Setup failed during model bootstrapping: %v", err)
	}

	// Load/Create CA
	ca, err := cert.LoadOrCreateCA("certs")
	if err != nil {
		log.Fatalf("Setup failed during CA creation: %v", err)
	}
	_ = ca // Ensure it's marked as used

	// Trust CA
	log.Println("[Setup] Requesting system trust for Root CA...")
	absCertPath, _ := filepath.Abs(filepath.Join("certs", "ca.crt"))
	
	tm := cert.NewTrustManager()
	if tm.IsTrusted("NobodyProx Root CA") {
		log.Println("[Setup] Root CA is already trusted.")
	} else {
		if err := tm.InstallTrust(absCertPath); err != nil {
			log.Fatalf("Setup failed to install trust: %v", err)
		}
		log.Println("[Setup] Trust installation triggered. Please accept the Windows security prompt.")
	}

	log.Println("\n[SUCCESS] Setup completed successfully!")
	log.Println("You can now run 'nobodyprox' to start the proxy.")
}
