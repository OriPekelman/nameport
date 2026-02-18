package issuer

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/tls"
	"crypto/x509"
	"net"
	"testing"
	"time"

	"nameport/internal/tls/ca"
	"nameport/internal/tls/policy"
)

// newTestCA creates an initialised CA in a temporary directory.
func newTestCA(t *testing.T) *ca.CA {
	t.Helper()
	dir := t.TempDir()
	c, err := ca.NewCA(dir)
	if err != nil {
		t.Fatalf("NewCA: %v", err)
	}
	if err := c.Init(); err != nil {
		t.Fatalf("CA.Init: %v", err)
	}
	return c
}

func TestIssue_CorrectSANs(t *testing.T) {
	c := newTestCA(t)
	p := policy.NewPolicy()
	iss := NewIssuer(c, p)

	cc, err := iss.Issue(IssueRequest{
		DNSNames: []string{"myapp.localhost", "api.myapp.localhost"},
		IPs:      []net.IP{net.IPv4(127, 0, 0, 1)},
	})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	leaf := cc.Cert.Leaf
	if len(leaf.DNSNames) != 2 {
		t.Fatalf("expected 2 DNS names, got %d", len(leaf.DNSNames))
	}
	if leaf.DNSNames[0] != "myapp.localhost" {
		t.Errorf("DNS[0] = %q, want %q", leaf.DNSNames[0], "myapp.localhost")
	}
	if leaf.DNSNames[1] != "api.myapp.localhost" {
		t.Errorf("DNS[1] = %q, want %q", leaf.DNSNames[1], "api.myapp.localhost")
	}
	if len(leaf.IPAddresses) != 1 || !leaf.IPAddresses[0].Equal(net.IPv4(127, 0, 0, 1)) {
		t.Errorf("unexpected IP SANs: %v", leaf.IPAddresses)
	}

	// CN should be set for display.
	if leaf.Subject.CommonName != "myapp.localhost" {
		t.Errorf("CommonName = %q, want %q", leaf.Subject.CommonName, "myapp.localhost")
	}
}

func TestIssue_ECDSAP256Key(t *testing.T) {
	c := newTestCA(t)
	p := policy.NewPolicy()
	iss := NewIssuer(c, p)

	cc, err := iss.Issue(IssueRequest{
		DNSNames: []string{"app.localhost"},
	})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	ecKey, ok := cc.Cert.PrivateKey.(*ecdsa.PrivateKey)
	if !ok {
		t.Fatalf("private key is %T, want *ecdsa.PrivateKey", cc.Cert.PrivateKey)
	}
	if ecKey.Curve != elliptic.P256() {
		t.Errorf("curve = %v, want P-256", ecKey.Curve.Params().Name)
	}

	// Also check the public key algorithm in the certificate.
	if cc.Cert.Leaf.PublicKeyAlgorithm != x509.ECDSA {
		t.Errorf("PublicKeyAlgorithm = %v, want ECDSA", cc.Cert.Leaf.PublicKeyAlgorithm)
	}
}

func TestIssue_KeyUsage(t *testing.T) {
	c := newTestCA(t)
	p := policy.NewPolicy()
	iss := NewIssuer(c, p)

	cc, err := iss.Issue(IssueRequest{
		DNSNames: []string{"app.localhost"},
	})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	leaf := cc.Cert.Leaf
	if leaf.KeyUsage&x509.KeyUsageDigitalSignature == 0 {
		t.Error("KeyUsage missing DigitalSignature")
	}
	found := false
	for _, eku := range leaf.ExtKeyUsage {
		if eku == x509.ExtKeyUsageServerAuth {
			found = true
		}
	}
	if !found {
		t.Error("ExtKeyUsage missing ServerAuth")
	}
}

func TestIssue_ChainIncludesIntermediate(t *testing.T) {
	c := newTestCA(t)
	p := policy.NewPolicy()
	iss := NewIssuer(c, p)

	cc, err := iss.Issue(IssueRequest{
		DNSNames: []string{"app.localhost"},
	})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	// tls.Certificate.Certificate should have [leaf, intermediate].
	if len(cc.Cert.Certificate) != 2 {
		t.Fatalf("chain length = %d, want 2", len(cc.Cert.Certificate))
	}

	interCert, err := x509.ParseCertificate(cc.Cert.Certificate[1])
	if err != nil {
		t.Fatalf("parse intermediate from chain: %v", err)
	}
	if !interCert.IsCA {
		t.Error("second cert in chain is not a CA")
	}
}

func TestIssue_CacheHit(t *testing.T) {
	c := newTestCA(t)
	p := policy.NewPolicy()
	iss := NewIssuer(c, p)

	cc1, err := iss.Issue(IssueRequest{
		DNSNames: []string{"cached.localhost"},
	})
	if err != nil {
		t.Fatalf("Issue 1: %v", err)
	}

	// A second Issue call with the same primary name should overwrite the
	// cache entry (Issue always creates a new cert). But GetCertificate
	// should return the cached one.
	hello := &tls.ClientHelloInfo{ServerName: "cached.localhost"}
	cert, err := iss.GetCertificate(hello)
	if err != nil {
		t.Fatalf("GetCertificate: %v", err)
	}

	// Should be the exact same object from the cache.
	if cert != cc1.Cert {
		t.Error("GetCertificate did not return cached cert")
	}
}

func TestIssue_PolicyRejection(t *testing.T) {
	c := newTestCA(t)
	p := policy.NewPolicy()
	iss := NewIssuer(c, p)

	_, err := iss.Issue(IssueRequest{
		DNSNames: []string{"test.com"},
	})
	if err == nil {
		t.Fatal("expected error for public domain, got nil")
	}
}

func TestGetCertificate_PolicyRejection(t *testing.T) {
	c := newTestCA(t)
	p := policy.NewPolicy()
	iss := NewIssuer(c, p)

	hello := &tls.ClientHelloInfo{ServerName: "evil.com"}
	_, err := iss.GetCertificate(hello)
	if err == nil {
		t.Fatal("expected error for public domain, got nil")
	}
}

func TestGetCertificate_EmptyServerName(t *testing.T) {
	c := newTestCA(t)
	p := policy.NewPolicy()
	iss := NewIssuer(c, p)

	hello := &tls.ClientHelloInfo{ServerName: ""}
	_, err := iss.GetCertificate(hello)
	if err == nil {
		t.Fatal("expected error for empty server name, got nil")
	}
}

func TestGetCertificate_Issues(t *testing.T) {
	c := newTestCA(t)
	p := policy.NewPolicy()
	iss := NewIssuer(c, p)

	hello := &tls.ClientHelloInfo{ServerName: "new.localhost"}
	cert, err := iss.GetCertificate(hello)
	if err != nil {
		t.Fatalf("GetCertificate: %v", err)
	}
	if cert == nil {
		t.Fatal("GetCertificate returned nil cert")
	}
	if cert.Leaf.DNSNames[0] != "new.localhost" {
		t.Errorf("leaf DNS[0] = %q, want %q", cert.Leaf.DNSNames[0], "new.localhost")
	}
}

func TestGetCertificate_NearExpiryReissue(t *testing.T) {
	c := newTestCA(t)
	p := policy.NewPolicy()
	iss := NewIssuer(c, p)

	// Issue a cert that expires in 30 minutes (within the 1-hour renewal window).
	cc, err := iss.Issue(IssueRequest{
		DNSNames: []string{"expiring.localhost"},
		ValidFor: 30 * time.Minute,
	})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	originalSerial := cc.Cert.Leaf.SerialNumber

	// GetCertificate should reissue because the cert is within 1h of expiry.
	hello := &tls.ClientHelloInfo{ServerName: "expiring.localhost"}
	cert, err := iss.GetCertificate(hello)
	if err != nil {
		t.Fatalf("GetCertificate: %v", err)
	}

	if cert.Leaf.SerialNumber.Cmp(originalSerial) == 0 {
		t.Error("expected a new cert (different serial), but got the same one")
	}

	// The new cert should have the default 24h validity.
	remaining := time.Until(cert.Leaf.NotAfter)
	if remaining < 23*time.Hour {
		t.Errorf("reissued cert expires in %v, expected ~24h", remaining)
	}
}

func TestIssue_DefaultValidFor(t *testing.T) {
	c := newTestCA(t)
	p := policy.NewPolicy()
	iss := NewIssuer(c, p)

	cc, err := iss.Issue(IssueRequest{
		DNSNames: []string{"default.localhost"},
	})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	// Should default to ~24 hours.
	lifetime := cc.Cert.Leaf.NotAfter.Sub(cc.Cert.Leaf.NotBefore)
	if lifetime < 23*time.Hour || lifetime > 25*time.Hour {
		t.Errorf("default lifetime = %v, expected ~24h", lifetime)
	}
}

func TestIssue_NoDNSNames_OnlyIP(t *testing.T) {
	c := newTestCA(t)
	p := policy.NewPolicy()
	iss := NewIssuer(c, p)

	cc, err := iss.Issue(IssueRequest{
		IPs: []net.IP{net.IPv4(127, 0, 0, 1)},
	})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	if len(cc.Cert.Leaf.IPAddresses) != 1 {
		t.Errorf("expected 1 IP SAN, got %d", len(cc.Cert.Leaf.IPAddresses))
	}
}

func TestIssue_Empty(t *testing.T) {
	c := newTestCA(t)
	p := policy.NewPolicy()
	iss := NewIssuer(c, p)

	_, err := iss.Issue(IssueRequest{})
	if err == nil {
		t.Fatal("expected error for empty request, got nil")
	}
}

func TestIssue_PEMOutputs(t *testing.T) {
	c := newTestCA(t)
	p := policy.NewPolicy()
	iss := NewIssuer(c, p)

	cc, err := iss.Issue(IssueRequest{
		DNSNames: []string{"pem.localhost"},
	})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	// CertPEM should be valid.
	if len(cc.CertPEM) == 0 {
		t.Fatal("CertPEM is empty")
	}
	if len(cc.KeyPEM) == 0 {
		t.Fatal("KeyPEM is empty")
	}

	// Should be parseable via tls.X509KeyPair.
	_, err = tls.X509KeyPair(cc.CertPEM, cc.KeyPEM)
	if err != nil {
		t.Fatalf("X509KeyPair: %v", err)
	}
}
