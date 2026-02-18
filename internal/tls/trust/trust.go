// Package trust manages installation and removal of the nameport root
// CA certificate in the operating system's trust store.
package trust

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
)

// certCommonName is the CN used to identify our root CA in the trust store.
const certCommonName = "nameport Root CA"

// Trustor manages OS trust store operations for the root CA certificate.
type Trustor interface {
	// Install adds the root CA PEM to the OS trust store. May require elevated privileges.
	Install(rootCertPEM []byte) error
	// Uninstall removes the root CA from the OS trust store.
	Uninstall() error
	// IsInstalled checks if the CA is already trusted.
	IsInstalled(rootCertPEM []byte) bool
	// NeedsElevation reports whether Install/Uninstall require sudo.
	NeedsElevation() bool
}

// NewPlatformTrustor returns a Trustor appropriate for the current operating
// system. On unsupported platforms it returns a no-op implementation that
// reports errors.
func NewPlatformTrustor() Trustor {
	return newPlatformTrustor()
}

// parsePEMCertificate decodes a PEM-encoded certificate and returns the parsed
// x509 certificate. It is a shared helper used by platform implementations.
func parsePEMCertificate(pemData []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, errors.New("trust: no PEM block found in certificate data")
	}
	if block.Type != "CERTIFICATE" {
		return nil, fmt.Errorf("trust: unexpected PEM block type %q, expected CERTIFICATE", block.Type)
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("trust: parse certificate: %w", err)
	}
	return cert, nil
}
