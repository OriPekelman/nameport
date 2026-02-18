//go:build linux

package trust

import (
	"fmt"
	"os"
	"os/exec"
)

// Debian/Ubuntu paths.
const (
	debianCertDir  = "/usr/local/share/ca-certificates"
	debianCertFile = "localhost-magic.crt"
	debianUpdate   = "update-ca-certificates"
)

// Fedora/RHEL/Arch paths.
const (
	fedoraCertDir  = "/etc/pki/ca-trust/source/anchors"
	fedoraCertFile = "localhost-magic.pem"
	fedoraUpdate   = "update-ca-trust"
)

// distroFamily represents the detected Linux distribution family.
type distroFamily int

const (
	distroUnknown distroFamily = iota
	distroDebian
	distroFedora
)

type linuxTrustor struct {
	family distroFamily
}

func newPlatformTrustor() Trustor {
	return &linuxTrustor{
		family: detectDistro(),
	}
}

// detectDistro determines which distro family we are on by checking for the
// presence of known certificate update tools.
func detectDistro() distroFamily {
	if _, err := exec.LookPath(debianUpdate); err == nil {
		return distroDebian
	}
	if _, err := exec.LookPath(fedoraUpdate); err == nil {
		return distroFedora
	}
	return distroUnknown
}

// Install adds the root CA PEM to the system trust store and updates the
// certificate cache. Requires root privileges.
func (l *linuxTrustor) Install(rootCertPEM []byte) error {
	if len(rootCertPEM) == 0 {
		return fmt.Errorf("trust: empty certificate data")
	}

	// Validate the PEM.
	if _, err := parsePEMCertificate(rootCertPEM); err != nil {
		return err
	}

	certPath, updateCmd, err := l.paths()
	if err != nil {
		return err
	}

	// Write certificate file.
	if err := os.MkdirAll(certDir(certPath), 0755); err != nil {
		return fmt.Errorf("trust: create cert dir: %w", err)
	}
	if err := os.WriteFile(certPath, rootCertPEM, 0644); err != nil {
		return fmt.Errorf("trust: write cert file: %w", err)
	}

	// Update certificate store.
	cmd := exec.Command(updateCmd)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Try to clean up on failure.
		os.Remove(certPath)
		return fmt.Errorf("trust: %s failed: %w\noutput: %s", updateCmd, err, string(output))
	}
	return nil
}

// Uninstall removes the root CA from the system trust store and updates the
// certificate cache.
func (l *linuxTrustor) Uninstall() error {
	certPath, updateCmd, err := l.paths()
	if err != nil {
		return err
	}

	if err := os.Remove(certPath); err != nil {
		if os.IsNotExist(err) {
			return nil // Already removed.
		}
		return fmt.Errorf("trust: remove cert file: %w", err)
	}

	cmd := exec.Command(updateCmd)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("trust: %s failed: %w\noutput: %s", updateCmd, err, string(output))
	}
	return nil
}

// IsInstalled checks whether the certificate file exists in the expected
// trust store location.
func (l *linuxTrustor) IsInstalled(rootCertPEM []byte) bool {
	certPath, _, err := l.paths()
	if err != nil {
		return false
	}

	existing, err := os.ReadFile(certPath)
	if err != nil {
		return false
	}

	// Compare the stored PEM with what was provided.
	return string(existing) == string(rootCertPEM)
}

// NeedsElevation reports that Linux trust store operations require root.
func (l *linuxTrustor) NeedsElevation() bool {
	return true
}

// paths returns the certificate file path and update command for the detected
// distro family.
func (l *linuxTrustor) paths() (certPath string, updateCmd string, err error) {
	switch l.family {
	case distroDebian:
		return debianCertDir + "/" + debianCertFile, debianUpdate, nil
	case distroFedora:
		return fedoraCertDir + "/" + fedoraCertFile, fedoraUpdate, nil
	default:
		return "", "", fmt.Errorf("trust: unsupported Linux distribution: neither %s nor %s found in PATH", debianUpdate, fedoraUpdate)
	}
}

// certDir returns the directory portion of the cert path.
func certDir(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[:i]
		}
	}
	return "."
}
