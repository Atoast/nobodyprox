//go:build windows
// +build windows

package cert

import (
	"os/exec"
	"strings"
)

type WindowsTrustManager struct{}

func (m *WindowsTrustManager) IsTrusted(commonName string) bool {
	// certutil -user -verifystore Root "Common Name"
	// Exit code 0 if found
	cmd := exec.Command("certutil", "-user", "-verifystore", "Root", commonName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}
	// Also check if output contains the name to be sure
	return strings.Contains(string(output), commonName)
}

func (m *WindowsTrustManager) InstallTrust(certPath string) error {
	// certutil -user -addstore Root "path\to\cert.crt"
	// This will trigger a Windows Security Warning popup
	cmd := exec.Command("certutil", "-user", "-addstore", "Root", certPath)
	return cmd.Run()
}

// NewTrustManager returns a Windows implementation
func NewTrustManager() TrustManager {
	return &WindowsTrustManager{}
}
