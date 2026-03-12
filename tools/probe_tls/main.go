package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"net/url"
)

func main() {
	proxyUrl, _ := url.Parse("http://localhost:8080")
	
	// Create a transport that uses the proxy
	transport := &http.Transport{
		Proxy: http.ProxyURL(proxyUrl),
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	client := &http.Client{
		Transport: transport,
	}

	// Try to access an HTTPS site through the proxy
	fmt.Println("Attempting to connect to https://google.com through proxy...")
	resp, err := client.Get("https://google.com")
	if err != nil {
		log.Fatalf("Failed to GET: %v", err)
	}
	defer resp.Body.Close()

	fmt.Printf("\nSuccess! Status: %s\n", resp.Status)
	
	if resp.TLS != nil {
		fmt.Println("\n--- TLS Handshake Successful (Inside Tunnel) ---")
		fmt.Printf("Version: %x\n", resp.TLS.Version)
		for i, cert := range resp.TLS.PeerCertificates {
			fmt.Printf("  [%d] Subject: %s\n", i, cert.Subject)
			fmt.Printf("      Issuer: %s\n", i, cert.Issuer)
		}
	}
}
