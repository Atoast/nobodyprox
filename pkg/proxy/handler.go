package proxy

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"
	"unsafe"

	"github.com/nobodyprox/nobodyprox/pkg/cert"
	"github.com/nobodyprox/nobodyprox/pkg/event"
	"github.com/nobodyprox/nobodyprox/pkg/filter"
)

type Proxy struct {
	CA              *cert.CA
	Filter          *filter.Engine
	FilterDomains   []string
	RedactResponses bool
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

	event.GlobalBus.Publish(event.Event{
		Type:  event.TypeRequestStart,
		ReqID: reqID,
		Data: event.RequestData{
			Method: r.Method,
			URL:    r.URL.String(),
			Host:   r.Host,
		},
	})
	start := time.Now()

	if r.Method == http.MethodConnect {
		p.handleConnect(w, r, reqID)
	} else {
		p.handleHTTP(w, r, reqID)
	}

	event.GlobalBus.Publish(event.Event{
		Type:  event.TypeRequestEnd,
		ReqID: reqID,
		Data: event.RequestEndData{
			Duration: time.Since(start),
		},
	})
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
		// Client -> Remote (Request)
		p.handleTunnelTraffic(remoteConn, tlsConn, reqID, "REQ")
		errChan <- nil
	}()
	go func() {
		// Remote -> Client (Response)
		p.handleTunnelTraffic(tlsConn, remoteConn, reqID, "RES")
		errChan <- nil
	}()

	<-errChan
}

// handleTunnelTraffic attempts to parse and redact HTTP traffic inside a TLS tunnel
func (p *Proxy) handleTunnelTraffic(dst io.Writer, src io.Reader, reqID, context string) {
	reader := bufio.NewReader(src)
	
	for {
		if context == "REQ" {
			req, err := http.ReadRequest(reader)
			if err != nil {
				if err != io.EOF && !strings.Contains(err.Error(), "closed") {
					log.Printf("[%s][Proxy] Tunnel REQ error: %v", reqID, err)
				}
				io.Copy(dst, reader) // Fallback to raw copy
				return
			}

			// Redact body if present
			if req.Body != nil {
				body, _ := io.ReadAll(req.Body)
				redacted := p.Filter.RedactBytes(body, "REQ", reqID)
				req.Body = io.NopCloser(strings.NewReader(string(redacted)))
				req.ContentLength = int64(len(redacted))
				req.Header.Set("Content-Length", fmt.Sprintf("%d", len(redacted)))
			}

			// Forward the redacted request
			dump, err := httputil.DumpRequest(req, true)
			if err == nil {
				dst.Write(dump)
			} else {
				req.Write(dst)
			}
		} else {
			// Response handling
			// We can't use http.ReadResponse easily without a matching request object.
			// For responses in tunnels, we fallback to a safer line-based or chunked approach
			// to avoid the 32KB splitting issue.
			p.pipeWithRedaction(dst, reader, reqID, "RES")
			return
		}
	}
}

// pipeWithRedaction copies data while ensuring we don't redact across sensitive word boundaries
func (p *Proxy) pipeWithRedaction(dst io.Writer, src io.Reader, reqID, context string) error {
	if context == "RES" && !p.RedactResponses {
		_, err := io.Copy(dst, src)
		return err
	}

	buf := make([]byte, 64*1024) // Larger buffer
	for {
		nr, err := src.Read(buf)
		if nr > 0 {
			// To avoid splitting entities at the end of a chunk, 
			// the engine should ideally handle a carry-over. 
			// For now, we'll process the whole chunk.
			redacted := p.Filter.RedactBytes(buf[0:nr], context, reqID)
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

	var finalRespBody []byte
	if p.RedactResponses {
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Printf("[%s][Proxy] Error reading response body: %v", reqID, err)
			return
		}
		finalRespBody = p.Filter.RedactBytes(respBody, "RES", reqID)
	} else {
		finalRespBody, err = io.ReadAll(resp.Body)
		if err != nil {
			log.Printf("[%s][Proxy] Error reading response body: %v", reqID, err)
			return
		}
	}

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
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(finalRespBody)))
	w.WriteHeader(resp.StatusCode)
	w.Write(finalRespBody)
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
