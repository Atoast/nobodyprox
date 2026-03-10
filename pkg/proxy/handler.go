package proxy

import (
	"crypto/tls"
	"io"
	"log"
	"net"
	"net/http"
	"strings"

	"github.com/nobodyprox/nobodyprox/pkg/cert"
	"github.com/nobodyprox/nobodyprox/pkg/filter"
)

type Proxy struct {
	CA     *cert.CA
	Filter *filter.Engine
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
		InsecureSkipVerify: false,
	})
	if err != nil {
		log.Printf("Failed to connect to remote server %s: %v", r.Host, err)
		return
	}
	defer remoteConn.Close()

	// Intercept the HTTP requests within the TLS connection
	// We wrap the connections to redact data on the fly
	errChan := make(chan error, 2)
	go func() {
		errChan <- p.pipeWithRedaction(remoteConn, tlsConn)
	}()
	go func() {
		errChan <- p.pipeWithRedaction(tlsConn, remoteConn)
	}()

	<-errChan
}

// pipeWithRedaction copies data from src to dst while redacting sensitive information
func (p *Proxy) pipeWithRedaction(dst io.Writer, src io.Reader) error {
	buf := make([]byte, 32*1024)
	for {
		nr, err := src.Read(buf)
		if nr > 0 {
			redacted := p.Filter.RedactBytes(buf[0:nr])
			nw, err := dst.Write(redacted)
			if err != nil {
				return err
			}
			if nw < len(redacted) {
				return io.ErrShortWrite
			}
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

func (p *Proxy) handleHTTP(w http.ResponseWriter, r *http.Request) {
	// Simple HTTP proxying with redirection (non-TLS)
	// Read the body, redact it, and forward it
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	r.Body.Close()

	redactedBody := p.Filter.RedactBytes(body)
	r.Body = io.NopCloser(strings.NewReader(string(redactedBody)))
	r.ContentLength = int64(len(redactedBody))

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

	// Redact response as well
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}
	w.Write(p.Filter.RedactBytes(respBody))
}
