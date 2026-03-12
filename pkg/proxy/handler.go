package proxy

import (
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
	"unsafe"

	"github.com/nobodyprox/nobodyprox/pkg/cert"
	"github.com/nobodyprox/nobodyprox/pkg/filter"
)

type Proxy struct {
	CA            *cert.CA
	Filter        *filter.Engine
	FilterDomains []string
}

func (p *Proxy) shouldFilter(host string) bool {
	if len(p.FilterDomains) == 0 {
		return true
	}
	for _, domain := range p.FilterDomains {
		if strings.HasSuffix(host, domain) {
			return true
		}
	}
	return false
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	reqID := fmt.Sprintf("%x", (uintptr)(unsafe.Pointer(r)) ^ (uintptr)(time.Now().UnixNano()))[:8]

	if r.Method == http.MethodConnect {
		p.handleConnect(w, r, reqID)
		return
	}

	p.handleHTTP(w, r, reqID)
}

func (p *Proxy) handleConnect(w http.ResponseWriter, r *http.Request, reqID string) {
	host, _, err := net.SplitHostPort(r.Host)
	if err != nil {
		host = r.Host
	}

	// Domain-specific bypass optimization
	if !p.shouldFilter(host) {
		log.Printf("[%s][Proxy] Bypassing MITM for %s (Domain not in filter list)", reqID, host)
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
		defer clientConn.Close()

		_, err = clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
		if err != nil {
			return
		}

		remoteConn, err := net.Dial("tcp", r.Host)
		if err != nil {
			log.Printf("[%s][Proxy] Failed to connect to %s: %v", reqID, r.Host, err)
			return
		}
		defer remoteConn.Close()

		errChan := make(chan error, 2)
		go func() {
			_, err := io.Copy(remoteConn, clientConn)
			errChan <- err
		}()
		go func() {
			_, err := io.Copy(clientConn, remoteConn)
			errChan <- err
		}()
		<-errChan
		return
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
		MinVersion:   tls.VersionTLS12,
	}

	tlsConn := tls.Server(clientConn, tlsConfig)
	if err := tlsConn.Handshake(); err != nil {
		log.Printf("[%s][Proxy] TLS handshake failed for %s: %v", reqID, host, err)
		tlsConn.Close()
		return
	}
	defer tlsConn.Close()

	// Connect to the remote server
	remoteConn, err := tls.Dial("tcp", r.Host, &tls.Config{
		InsecureSkipVerify: false,
	})
	if err != nil {
		log.Printf("[%s][Proxy] Failed to connect to remote server %s: %v", reqID, r.Host, err)
		return
	}
	defer remoteConn.Close()

	// Intercept the HTTP requests within the TLS connection
	errChan := make(chan error, 2)
	go func() {
		errChan <- p.pipeWithRedaction(remoteConn, tlsConn, reqID)
	}()
	go func() {
		errChan <- p.pipeWithRedaction(tlsConn, remoteConn, reqID)
	}()

	<-errChan
}

// pipeWithRedaction copies data from src to dst while redacting sensitive information
func (p *Proxy) pipeWithRedaction(dst io.Writer, src io.Reader, reqID string) error {
	buf := make([]byte, 32*1024)
	for {
		nr, err := src.Read(buf)
		if nr > 0 {
			redacted := p.Filter.RedactBytes(buf[0:nr], "TUNNEL", reqID)
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

func (p *Proxy) handleHTTP(w http.ResponseWriter, r *http.Request, reqID string) {
	// Clear RequestURI as it's not allowed in client requests
	r.RequestURI = ""

	// Ensure the URL has a scheme and host (for RoundTrip)
	if r.URL.Scheme == "" {
		r.URL.Scheme = "http"
	}
	if r.URL.Host == "" {
		r.URL.Host = r.Host
	}

	// Domain-specific bypass optimization
	if !p.shouldFilter(r.URL.Host) {
		log.Printf("[%s][Proxy] Bypassing redaction for %s", reqID, r.URL.Host)
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
		return
	}

	// Disable compression so we can redact the response body easily
	r.Header.Set("Accept-Encoding", "identity")

	// Read and redact request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	r.Body.Close()

	redactedBody := p.Filter.RedactBytes(body, "REQ", reqID)
	r.Body = io.NopCloser(strings.NewReader(string(redactedBody)))
	r.ContentLength = int64(len(redactedBody))
	r.Header.Del("Content-Length")

	resp, err := http.DefaultTransport.RoundTrip(r)
	if err != nil {
		log.Printf("[%s][Proxy] RoundTrip error for %s: %v", reqID, r.URL, err)
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer resp.Body.Close()

	// Redact response body first so we know the final size
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[%s][Proxy] Error reading response body: %v", reqID, err)
		return
	}
	redactedRespBody := p.Filter.RedactBytes(respBody, "RES", reqID)

	// Copy response headers
	for k, vv := range resp.Header {
		if isHopByHop(k) || strings.EqualFold(k, "Content-Length") {
			continue
		}
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}

	// Set the correct content length for the redacted body
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(redactedRespBody)))
	w.WriteHeader(resp.StatusCode)
	w.Write(redactedRespBody)
}

func isHopByHop(header string) bool {
	hopByHop := []string{
		"Connection",
		"Keep-Alive",
		"Proxy-Authenticate",
		"Proxy-Authorization",
		"Te",
		"Trailers",
		"Transfer-Encoding",
		"Upgrade",
	}
	for _, h := range hopByHop {
		if strings.EqualFold(header, h) {
			return true
		}
	}
	return false
}
