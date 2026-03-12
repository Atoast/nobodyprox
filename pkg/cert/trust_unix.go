//go:build !windows
// +build !windows

package cert

type UnixTrustManager struct{}

func (m *UnixTrustManager) IsTrusted(commonName string) bool {
	// Not implemented for Unix yet
	return false
}

func (m *UnixTrustManager) InstallTrust(certPath string) error {
	// Manual instructions for Unix for now
	return nil
}

// NewTrustManager returns a stub implementation
func NewTrustManager() TrustManager {
	return &UnixTrustManager{}
}
