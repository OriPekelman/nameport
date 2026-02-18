// Package ca implements a two-tier certificate authority with a long-lived
// root and a shorter-lived intermediate, both using ECDSA P-256 keys for
// maximum browser compatibility.
package ca

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

// CA holds the root and intermediate certificate authority material.
type CA struct {
	RootCert  *x509.Certificate
	RootKey   crypto.PrivateKey
	InterCert *x509.Certificate
	InterKey  crypto.PrivateKey
	StorePath string
}

// NewCA returns a CA backed by the given store directory. If certificates
// already exist on disk they are loaded; otherwise the CA is returned
// uninitialised and Init must be called.
func NewCA(storePath string) (*CA, error) {
	ca := &CA{StorePath: storePath}

	if err := os.MkdirAll(storePath, 0700); err != nil {
		return nil, fmt.Errorf("ca: create store dir: %w", err)
	}

	rootCertPath := filepath.Join(storePath, "root_ca.pem")
	rootKeyPath := filepath.Join(storePath, "root_ca.key")
	interCertPath := filepath.Join(storePath, "intermediate.pem")
	interKeyPath := filepath.Join(storePath, "intermediate.key")

	// Try to load existing material.
	rootCertPEM, errRC := os.ReadFile(rootCertPath)
	rootKeyPEM, errRK := os.ReadFile(rootKeyPath)
	interCertPEM, errIC := os.ReadFile(interCertPath)
	interKeyPEM, errIK := os.ReadFile(interKeyPath)

	if errRC != nil || errRK != nil || errIC != nil || errIK != nil {
		// Not all files present – return uninitialised.
		return ca, nil
	}

	var err error
	ca.RootCert, ca.RootKey, err = parseCertAndKey(rootCertPEM, rootKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("ca: load root: %w", err)
	}
	ca.InterCert, ca.InterKey, err = parseCertAndKey(interCertPEM, interKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("ca: load intermediate: %w", err)
	}

	return ca, nil
}

// IsInitialized reports whether both root and intermediate material is loaded.
func (ca *CA) IsInitialized() bool {
	return ca.RootCert != nil && ca.RootKey != nil &&
		ca.InterCert != nil && ca.InterKey != nil
}

// Init generates a new root CA and intermediate CA, writing all material to
// StorePath. It is an error to call Init on an already-initialised CA.
func (ca *CA) Init() error {
	if ca.IsInitialized() {
		return errors.New("ca: already initialised")
	}

	// --- Root CA (ECDSA P-256) ---
	rootPriv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("ca: generate root key: %w", err)
	}

	rootSerial, err := randomSerial()
	if err != nil {
		return err
	}

	now := time.Now()
	rootTemplate := &x509.Certificate{
		SerialNumber: rootSerial,
		Subject: pkix.Name{
			CommonName: "nameport Root CA",
		},
		NotBefore:             now,
		NotAfter:              now.Add(10 * 365 * 24 * time.Hour), // ~10 years
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	rootDER, err := x509.CreateCertificate(rand.Reader, rootTemplate, rootTemplate, &rootPriv.PublicKey, rootPriv)
	if err != nil {
		return fmt.Errorf("ca: create root cert: %w", err)
	}

	rootCert, err := x509.ParseCertificate(rootDER)
	if err != nil {
		return fmt.Errorf("ca: parse root cert: %w", err)
	}

	// --- Intermediate CA (ECDSA P-256) ---
	interPriv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("ca: generate intermediate key: %w", err)
	}

	interSerial, err := randomSerial()
	if err != nil {
		return err
	}

	interTemplate := &x509.Certificate{
		SerialNumber: interSerial,
		Subject: pkix.Name{
			CommonName: "nameport Intermediate CA",
		},
		NotBefore:             now,
		NotAfter:              now.Add(365 * 24 * time.Hour), // 1 year
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            0,
		MaxPathLenZero:        true,
	}

	interDER, err := x509.CreateCertificate(rand.Reader, interTemplate, rootCert, &interPriv.PublicKey, rootPriv)
	if err != nil {
		return fmt.Errorf("ca: create intermediate cert: %w", err)
	}

	interCert, err := x509.ParseCertificate(interDER)
	if err != nil {
		return fmt.Errorf("ca: parse intermediate cert: %w", err)
	}

	// --- Persist ---
	if err := ca.persist(rootCert, rootPriv, interCert, interPriv); err != nil {
		return err
	}

	ca.RootCert = rootCert
	ca.RootKey = rootPriv
	ca.InterCert = interCert
	ca.InterKey = interPriv

	return nil
}

// RootCertPEM returns the PEM-encoded root certificate.
func (ca *CA) RootCertPEM() []byte {
	if ca.RootCert == nil {
		return nil
	}
	return pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: ca.RootCert.Raw,
	})
}

// InterCertPEM returns the PEM-encoded intermediate certificate.
func (ca *CA) InterCertPEM() []byte {
	if ca.InterCert == nil {
		return nil
	}
	return pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: ca.InterCert.Raw,
	})
}

// RotateIntermediate generates a fresh intermediate CA signed by the existing
// root and persists the new material.
func (ca *CA) RotateIntermediate() error {
	if !ca.IsInitialized() {
		return errors.New("ca: not initialised")
	}

	interPriv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("ca: generate intermediate key: %w", err)
	}

	serial, err := randomSerial()
	if err != nil {
		return err
	}

	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName: "nameport Intermediate CA",
		},
		NotBefore:             now,
		NotAfter:              now.Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            0,
		MaxPathLenZero:        true,
	}

	der, err := x509.CreateCertificate(rand.Reader, template, ca.RootCert, &interPriv.PublicKey, ca.RootKey)
	if err != nil {
		return fmt.Errorf("ca: create intermediate cert: %w", err)
	}

	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return fmt.Errorf("ca: parse intermediate cert: %w", err)
	}

	// Persist only intermediate files (root stays the same).
	if err := writeFileAtomic(filepath.Join(ca.StorePath, "intermediate.pem"), encodeCertPEM(cert), 0644); err != nil {
		return err
	}
	if err := writeFileAtomic(filepath.Join(ca.StorePath, "intermediate.key"), encodeKeyPEM(interPriv), 0600); err != nil {
		return err
	}

	ca.InterCert = cert
	ca.InterKey = interPriv
	return nil
}

// SignCertificate signs the given template using the intermediate CA and
// returns the PEM-encoded certificate. The caller must populate the template
// fields (Subject, SANs, etc.) and supply the leaf public key.
func (ca *CA) SignCertificate(template *x509.Certificate, pub crypto.PublicKey) ([]byte, error) {
	if !ca.IsInitialized() {
		return nil, errors.New("ca: not initialised")
	}

	if template.SerialNumber == nil {
		serial, err := randomSerial()
		if err != nil {
			return nil, err
		}
		template.SerialNumber = serial
	}

	der, err := x509.CreateCertificate(rand.Reader, template, ca.InterCert, pub, ca.InterKey)
	if err != nil {
		return nil, fmt.Errorf("ca: sign certificate: %w", err)
	}

	return pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: der,
	}), nil
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func (ca *CA) persist(rootCert *x509.Certificate, rootKey crypto.PrivateKey, interCert *x509.Certificate, interKey crypto.PrivateKey) error {
	if err := writeFileAtomic(filepath.Join(ca.StorePath, "root_ca.pem"), encodeCertPEM(rootCert), 0644); err != nil {
		return err
	}
	if err := writeFileAtomic(filepath.Join(ca.StorePath, "root_ca.key"), encodeKeyPEM(rootKey), 0600); err != nil {
		return err
	}
	if err := writeFileAtomic(filepath.Join(ca.StorePath, "intermediate.pem"), encodeCertPEM(interCert), 0644); err != nil {
		return err
	}
	if err := writeFileAtomic(filepath.Join(ca.StorePath, "intermediate.key"), encodeKeyPEM(interKey), 0600); err != nil {
		return err
	}
	return nil
}

func encodeCertPEM(cert *x509.Certificate) []byte {
	return pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert.Raw,
	})
}

func encodeKeyPEM(key crypto.PrivateKey) []byte {
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		panic("ca: marshal key: " + err.Error())
	}
	return pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: der,
	})
}

func parseCertAndKey(certPEM, keyPEM []byte) (*x509.Certificate, crypto.PrivateKey, error) {
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return nil, nil, errors.New("ca: no PEM block in certificate")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, nil, err
	}

	kBlock, _ := pem.Decode(keyPEM)
	if kBlock == nil {
		return nil, nil, errors.New("ca: no PEM block in key")
	}
	key, err := x509.ParsePKCS8PrivateKey(kBlock.Bytes)
	if err != nil {
		return nil, nil, err
	}

	return cert, key, nil
}

// writeFileAtomic writes data to a temporary file in the same directory and
// then renames it to the target path, providing atomic-write semantics.
func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("ca: create temp file: %w", err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("ca: write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("ca: close temp file: %w", err)
	}
	if err := os.Chmod(tmpName, mode); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("ca: chmod temp file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("ca: rename temp file: %w", err)
	}
	return nil
}

func randomSerial() (*big.Int, error) {
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("ca: generate serial: %w", err)
	}
	return serial, nil
}
