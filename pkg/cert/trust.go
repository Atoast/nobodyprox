package cert

// TrustManager defines platform-specific actions for certificate trust
type TrustManager interface {
	IsTrusted(commonName string) bool
	InstallTrust(certPath string) error
}
