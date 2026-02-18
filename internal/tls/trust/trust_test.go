package trust

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"
)

// generateTestCACert creates a self-signed CA certificate for testing.
func generateTestCACert(t *testing.T) []byte {
	t.Helper()

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: certCommonName,
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, pub, priv)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}

	return pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: der,
	})
}

func TestNewPlatformTrustor(t *testing.T) {
	tr := NewPlatformTrustor()
	if tr == nil {
		t.Fatal("NewPlatformTrustor returned nil")
	}
}

func TestTrustorInterface(t *testing.T) {
	// Verify that NewPlatformTrustor returns something that satisfies the
	// Trustor interface at compile time.
	var _ Trustor = NewPlatformTrustor()
}

func TestParsePEMCertificate(t *testing.T) {
	certPEM := generateTestCACert(t)

	t.Run("valid PEM", func(t *testing.T) {
		cert, err := parsePEMCertificate(certPEM)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cert.Subject.CommonName != certCommonName {
			t.Errorf("CN = %q, want %q", cert.Subject.CommonName, certCommonName)
		}
		if !cert.IsCA {
			t.Error("expected IsCA to be true")
		}
	})

	t.Run("empty PEM", func(t *testing.T) {
		_, err := parsePEMCertificate([]byte{})
		if err == nil {
			t.Error("expected error for empty PEM, got nil")
		}
	})

	t.Run("invalid PEM block type", func(t *testing.T) {
		badPEM := pem.EncodeToMemory(&pem.Block{
			Type:  "PRIVATE KEY",
			Bytes: []byte("not a cert"),
		})
		_, err := parsePEMCertificate(badPEM)
		if err == nil {
			t.Error("expected error for wrong PEM type, got nil")
		}
	})

	t.Run("garbage data", func(t *testing.T) {
		_, err := parsePEMCertificate([]byte("not PEM at all"))
		if err == nil {
			t.Error("expected error for garbage data, got nil")
		}
	})
}

func TestNeedsElevation(t *testing.T) {
	tr := NewPlatformTrustor()
	// On darwin and linux this should be true; the test just ensures it
	// does not panic and returns a boolean.
	_ = tr.NeedsElevation()
}

func TestIsInstalledWithoutInstall(t *testing.T) {
	tr := NewPlatformTrustor()
	certPEM := generateTestCACert(t)

	// A freshly generated cert should not be in any trust store.
	// On unsupported platforms IsInstalled always returns false.
	// On macOS/Linux it checks for the well-known CN which should not match
	// our ephemeral test cert (it was never installed).
	if tr.IsInstalled(certPEM) {
		// This is not necessarily wrong (if the real CA is installed),
		// so we just log rather than fail.
		t.Log("IsInstalled returned true; the localhost-magic CA may already be installed on this system")
	}
}

func TestInstallRejectsEmptyPEM(t *testing.T) {
	tr := NewPlatformTrustor()
	err := tr.Install([]byte{})
	if err == nil {
		t.Error("expected error when installing empty PEM, got nil")
	}
}

func TestInstallRejectsInvalidPEM(t *testing.T) {
	tr := NewPlatformTrustor()
	err := tr.Install([]byte("not valid PEM data"))
	if err == nil {
		t.Error("expected error when installing invalid PEM, got nil")
	}
}
