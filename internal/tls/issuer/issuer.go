// Package issuer issues leaf TLS certificates on demand, using the local CA
// and validating requests against the domain policy.
package issuer

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"nameport/internal/tls/ca"
	"nameport/internal/tls/policy"
)

// DefaultValidFor is the default leaf certificate lifetime.
const DefaultValidFor = 24 * time.Hour

// renewBefore is how far before expiry a cached certificate is considered
// stale and will be reissued.
const renewBefore = 1 * time.Hour

// IssueRequest describes a leaf certificate to create.
type IssueRequest struct {
	DNSNames []string
	IPs      []net.IP
	ValidFor time.Duration // default: 24 hours
}

// CachedCert holds a leaf certificate and its private key, ready for serving.
type CachedCert struct {
	CertPEM []byte
	KeyPEM  []byte
	Cert    *tls.Certificate // parsed, ready for TLS serving
	Expiry  time.Time
}

// Issuer creates and caches leaf certificates signed by the local CA.
type Issuer struct {
	ca     *ca.CA
	policy *policy.Policy
	cache  map[string]*CachedCert
	mu     sync.RWMutex
}

// NewIssuer returns an Issuer backed by the given CA and domain policy.
func NewIssuer(c *ca.CA, p *policy.Policy) *Issuer {
	return &Issuer{
		ca:     c,
		policy: p,
		cache:  make(map[string]*CachedCert),
	}
}

// Issue creates a new leaf certificate with an ECDSA P-256 key, validates all
// requested domains against the policy, and caches the result keyed by the
// primary (first) DNS name.
func (i *Issuer) Issue(req IssueRequest) (*CachedCert, error) {
	if len(req.DNSNames) == 0 && len(req.IPs) == 0 {
		return nil, errors.New("issuer: at least one DNS name or IP address is required")
	}

	// Validate every DNS name against the policy.
	for _, name := range req.DNSNames {
		if strings.HasPrefix(name, "*.") {
			if err := i.policy.ValidateWildcard(name); err != nil {
				return nil, fmt.Errorf("issuer: %w", err)
			}
		} else {
			if err := i.policy.ValidateDomain(name); err != nil {
				return nil, fmt.Errorf("issuer: %w", err)
			}
		}
	}

	// Generate ECDSA P-256 leaf key.
	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("issuer: generate key: %w", err)
	}

	validFor := req.ValidFor
	if validFor == 0 {
		validFor = DefaultValidFor
	}

	now := time.Now()
	notAfter := now.Add(validFor)

	// Build certificate template (SAN-only; CN is for display only).
	template := &x509.Certificate{
		Subject: pkix.Name{},
		DNSNames:    req.DNSNames,
		IPAddresses: req.IPs,
		NotBefore:   now,
		NotAfter:    notAfter,
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	if len(req.DNSNames) > 0 {
		template.Subject.CommonName = req.DNSNames[0]
	}

	// Sign via the CA (returns PEM).
	certPEM, err := i.ca.SignCertificate(template, &ecKey.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("issuer: sign: %w", err)
	}

	// Decode PEM to get the DER bytes and parsed certificate.
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return nil, errors.New("issuer: failed to decode signed certificate PEM")
	}
	leafDER := block.Bytes

	leafCert, err := x509.ParseCertificate(leafDER)
	if err != nil {
		return nil, fmt.Errorf("issuer: parse leaf cert: %w", err)
	}

	// Encode the private key to PEM.
	keyDER, err := x509.MarshalECPrivateKey(ecKey)
	if err != nil {
		return nil, fmt.Errorf("issuer: marshal key: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: keyDER,
	})

	// Build the tls.Certificate with the intermediate in the chain.
	tlsCert := tls.Certificate{
		Certificate: [][]byte{leafDER, i.ca.InterCert.Raw},
		PrivateKey:  ecKey,
		Leaf:        leafCert,
	}

	cached := &CachedCert{
		CertPEM: certPEM,
		KeyPEM:  keyPEM,
		Cert:    &tlsCert,
		Expiry:  notAfter,
	}

	// Cache by primary DNS name.
	if len(req.DNSNames) > 0 {
		i.mu.Lock()
		i.cache[req.DNSNames[0]] = cached
		i.mu.Unlock()
	}

	return cached, nil
}

// GetCertificate implements the tls.Config.GetCertificate callback. It looks
// up a cached certificate for the requested server name, reissues if the cert
// is within one hour of expiry, or issues a fresh one if none is cached.
func (i *Issuer) GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	serverName := hello.ServerName
	if serverName == "" {
		return nil, errors.New("issuer: no server name in ClientHello")
	}

	// Validate the domain against policy before doing anything.
	if err := i.policy.ValidateDomain(serverName); err != nil {
		return nil, fmt.Errorf("issuer: %w", err)
	}

	// Check cache.
	i.mu.RLock()
	cached, ok := i.cache[serverName]
	i.mu.RUnlock()

	if ok && time.Now().Before(cached.Expiry.Add(-renewBefore)) {
		return cached.Cert, nil
	}

	// Issue (or reissue) a certificate.
	cc, err := i.Issue(IssueRequest{
		DNSNames: []string{serverName},
	})
	if err != nil {
		return nil, err
	}

	return cc.Cert, nil
}
