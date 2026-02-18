//go:build darwin

package trust

import (
	"fmt"
	"os"
	"os/exec"
)

const (
	securityBin    = "/usr/bin/security"
	systemKeychain = "/Library/Keychains/System.keychain"
)

type darwinTrustor struct{}

func newPlatformTrustor() Trustor {
	return &darwinTrustor{}
}

// Install adds the root CA PEM to the macOS System Keychain as a trusted root
// certificate. This requires elevated (sudo) privileges.
func (d *darwinTrustor) Install(rootCertPEM []byte) error {
	if len(rootCertPEM) == 0 {
		return fmt.Errorf("trust: empty certificate data")
	}

	// Validate the PEM before touching the system.
	if _, err := parsePEMCertificate(rootCertPEM); err != nil {
		return err
	}

	tmpFile, err := writeTempPEM(rootCertPEM)
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile)

	// security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain <file>
	cmd := exec.Command(securityBin, "add-trusted-cert",
		"-d",
		"-r", "trustRoot",
		"-k", systemKeychain,
		tmpFile,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("trust: add-trusted-cert failed: %w\noutput: %s", err, string(output))
	}
	return nil
}

// Uninstall removes the root CA from the macOS System Keychain.
func (d *darwinTrustor) Uninstall() error {
	// security delete-certificate -c "nameport Root CA" -t /Library/Keychains/System.keychain
	cmd := exec.Command(securityBin, "delete-certificate",
		"-c", certCommonName,
		"-t",
		systemKeychain,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("trust: delete-certificate failed: %w\noutput: %s", err, string(output))
	}
	return nil
}

// IsInstalled checks whether the root CA is already present in the System Keychain.
func (d *darwinTrustor) IsInstalled(rootCertPEM []byte) bool {
	cmd := exec.Command(securityBin, "find-certificate",
		"-c", certCommonName,
		systemKeychain,
	)
	return cmd.Run() == nil
}

// NeedsElevation reports that macOS trust operations always require sudo.
func (d *darwinTrustor) NeedsElevation() bool {
	return true
}

// writeTempPEM writes PEM data to a temporary file and returns the path.
// The caller is responsible for removing the file.
func writeTempPEM(pemData []byte) (string, error) {
	f, err := os.CreateTemp("", "nameport-ca-*.pem")
	if err != nil {
		return "", fmt.Errorf("trust: create temp file: %w", err)
	}
	if _, err := f.Write(pemData); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", fmt.Errorf("trust: write temp file: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(f.Name())
		return "", fmt.Errorf("trust: close temp file: %w", err)
	}
	return f.Name(), nil
}
