package proxy

import (
	"crypto/tls"
	"io"
	"log"
	"net"
	"net/http"

	"github.com/nobodyprox/nobodyprox/pkg/cert"
)

type Proxy struct {
	CA *cert.CA
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect {
		p.handleConnect(w, r)
		return
	}

	p.handleHTTP(w, r)
}

func (p *Proxy) handleConnect(w http.ResponseWriter, r *http.Request) {
	host, _, err := net.SplitHostPort(r.Host)
	if err != nil {
		host = r.Host
	}

	// Generate a certificate for the host
	tlsCert, err := p.CA.GenerateCert(host)
	if err != nil {
		http.Error(w, "Failed to generate certificate", http.StatusInternalServerError)
		return
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		return
	}

	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	_, err = clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
	if err != nil {
		clientConn.Close()
		return
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{*tlsCert},
	}

	tlsConn := tls.Server(clientConn, tlsConfig)
	if err := tlsConn.Handshake(); err != nil {
		log.Printf("TLS handshake failed for %s: %v", host, err)
		tlsConn.Close()
		return
	}
	defer tlsConn.Close()

	// Connect to the remote server
	remoteConn, err := tls.Dial("tcp", r.Host, &tls.Config{
		InsecureSkipVerify: false, // In production we should verify, but for now let's be careful
	})
	if err != nil {
		log.Printf("Failed to connect to remote server %s: %v", r.Host, err)
		return
	}
	defer remoteConn.Close()

	// In Phase 1, we just bridge the connections. 
	// In Phase 2, we will wrap these connections to inspect/redact data.
	errChan := make(chan error, 2)
	go func() {
		_, err := io.Copy(remoteConn, tlsConn)
		errChan <- err
	}()
	go func() {
		_, err := io.Copy(tlsConn, remoteConn)
		errChan <- err
	}()

	<-errChan
}

func (p *Proxy) handleHTTP(w http.ResponseWriter, r *http.Request) {
	// Simple HTTP proxying
	resp, err := http.DefaultTransport.RoundTrip(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer resp.Body.Close()

	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}
