package ca

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewCA_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	c, err := NewCA(dir)
	if err != nil {
		t.Fatalf("NewCA: %v", err)
	}
	if c.IsInitialized() {
		t.Fatal("expected uninitialised CA on empty dir")
	}
}

func TestInit(t *testing.T) {
	dir := t.TempDir()
	c, err := NewCA(dir)
	if err != nil {
		t.Fatalf("NewCA: %v", err)
	}

	if err := c.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}

	if !c.IsInitialized() {
		t.Fatal("expected initialised CA after Init")
	}

	// Root certificate checks.
	if c.RootCert.Subject.CommonName != "localhost-magic Root CA" {
		t.Errorf("root CN = %q, want %q", c.RootCert.Subject.CommonName, "localhost-magic Root CA")
	}
	if !c.RootCert.IsCA {
		t.Error("root cert is not CA")
	}
	if c.RootCert.KeyUsage&x509.KeyUsageCertSign == 0 {
		t.Error("root missing KeyUsageCertSign")
	}
	if c.RootCert.KeyUsage&x509.KeyUsageCRLSign == 0 {
		t.Error("root missing KeyUsageCRLSign")
	}
	// Root should be valid for ~10 years.
	if c.RootCert.NotAfter.Before(time.Now().Add(9 * 365 * 24 * time.Hour)) {
		t.Error("root cert expires too soon")
	}

	// Intermediate certificate checks.
	if c.InterCert.Subject.CommonName != "localhost-magic Intermediate CA" {
		t.Errorf("inter CN = %q", c.InterCert.Subject.CommonName)
	}
	if !c.InterCert.IsCA {
		t.Error("intermediate cert is not CA")
	}
	if c.InterCert.MaxPathLen != 0 || !c.InterCert.MaxPathLenZero {
		t.Errorf("intermediate MaxPathLen = %d, MaxPathLenZero = %v", c.InterCert.MaxPathLen, c.InterCert.MaxPathLenZero)
	}
	// Intermediate should be valid for ~1 year.
	if c.InterCert.NotAfter.Before(time.Now().Add(364 * 24 * time.Hour)) {
		t.Error("intermediate cert expires too soon")
	}
	if c.InterCert.NotAfter.After(time.Now().Add(366 * 24 * time.Hour)) {
		t.Error("intermediate cert expires too late")
	}

	// Verify intermediate is signed by root.
	roots := x509.NewCertPool()
	roots.AddCert(c.RootCert)
	if _, err := c.InterCert.Verify(x509.VerifyOptions{
		Roots: roots,
	}); err != nil {
		t.Fatalf("intermediate verification failed: %v", err)
	}
}

func TestInit_AlreadyInitialized(t *testing.T) {
	dir := t.TempDir()
	c, _ := NewCA(dir)
	if err := c.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := c.Init(); err == nil {
		t.Fatal("expected error on double Init")
	}
}

func TestPersistenceAndReload(t *testing.T) {
	dir := t.TempDir()
	c, _ := NewCA(dir)
	if err := c.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}

	origRootSerial := c.RootCert.SerialNumber
	origInterSerial := c.InterCert.SerialNumber

	// Reload from disk.
	c2, err := NewCA(dir)
	if err != nil {
		t.Fatalf("NewCA reload: %v", err)
	}
	if !c2.IsInitialized() {
		t.Fatal("reloaded CA not initialised")
	}
	if c2.RootCert.SerialNumber.Cmp(origRootSerial) != 0 {
		t.Error("root serial mismatch after reload")
	}
	if c2.InterCert.SerialNumber.Cmp(origInterSerial) != 0 {
		t.Error("intermediate serial mismatch after reload")
	}
}

func TestFilePermissions(t *testing.T) {
	dir := t.TempDir()
	c, _ := NewCA(dir)
	if err := c.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}

	for _, name := range []string{"root_ca.key", "intermediate.key"} {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("stat %s: %v", name, err)
		}
		perm := info.Mode().Perm()
		if perm != 0600 {
			t.Errorf("%s has perm %o, want 0600", name, perm)
		}
	}
}

func TestRootCertPEM_InterCertPEM(t *testing.T) {
	dir := t.TempDir()
	c, _ := NewCA(dir)

	// Before init, should return nil.
	if c.RootCertPEM() != nil {
		t.Error("RootCertPEM should be nil before Init")
	}
	if c.InterCertPEM() != nil {
		t.Error("InterCertPEM should be nil before Init")
	}

	if err := c.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}

	rootPEM := c.RootCertPEM()
	block, _ := pem.Decode(rootPEM)
	if block == nil || block.Type != "CERTIFICATE" {
		t.Fatal("invalid root PEM")
	}

	interPEM := c.InterCertPEM()
	block, _ = pem.Decode(interPEM)
	if block == nil || block.Type != "CERTIFICATE" {
		t.Fatal("invalid intermediate PEM")
	}
}

func TestRotateIntermediate(t *testing.T) {
	dir := t.TempDir()
	c, _ := NewCA(dir)
	if err := c.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}

	origInterSerial := c.InterCert.SerialNumber

	if err := c.RotateIntermediate(); err != nil {
		t.Fatalf("RotateIntermediate: %v", err)
	}

	if c.InterCert.SerialNumber.Cmp(origInterSerial) == 0 {
		t.Error("serial unchanged after rotation")
	}

	// New intermediate should verify against root.
	roots := x509.NewCertPool()
	roots.AddCert(c.RootCert)
	if _, err := c.InterCert.Verify(x509.VerifyOptions{Roots: roots}); err != nil {
		t.Fatalf("rotated intermediate verification failed: %v", err)
	}

	// Reload and verify persistence.
	c2, err := NewCA(dir)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if c2.InterCert.SerialNumber.Cmp(c.InterCert.SerialNumber) != 0 {
		t.Error("rotated intermediate not persisted")
	}
}

func TestRotateIntermediate_NotInitialized(t *testing.T) {
	dir := t.TempDir()
	c, _ := NewCA(dir)
	if err := c.RotateIntermediate(); err == nil {
		t.Fatal("expected error rotating uninitialised CA")
	}
}

func TestSignCertificate(t *testing.T) {
	dir := t.TempDir()
	c, _ := NewCA(dir)
	if err := c.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}

	// Generate a leaf key (ECDSA P-256).
	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate leaf key: %v", err)
	}

	now := time.Now()
	template := &x509.Certificate{
		Subject: pkix.Name{
			CommonName: "myapp.localhost",
		},
		DNSNames:  []string{"myapp.localhost"},
		NotBefore: now,
		NotAfter:  now.Add(24 * time.Hour),
		KeyUsage:  x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
		},
	}

	certPEM, err := c.SignCertificate(template, &leafKey.PublicKey)
	if err != nil {
		t.Fatalf("SignCertificate: %v", err)
	}

	block, _ := pem.Decode(certPEM)
	if block == nil {
		t.Fatal("no PEM block in signed cert")
	}

	leaf, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse leaf cert: %v", err)
	}

	if leaf.Subject.CommonName != "myapp.localhost" {
		t.Errorf("leaf CN = %q", leaf.Subject.CommonName)
	}

	// Verify the full chain: root -> intermediate -> leaf.
	roots := x509.NewCertPool()
	roots.AddCert(c.RootCert)
	inters := x509.NewCertPool()
	inters.AddCert(c.InterCert)

	if _, err := leaf.Verify(x509.VerifyOptions{
		Roots:         roots,
		Intermediates: inters,
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}); err != nil {
		t.Fatalf("leaf chain verification failed: %v", err)
	}
}

func TestSignCertificate_WithSerialNumber(t *testing.T) {
	dir := t.TempDir()
	c, _ := NewCA(dir)
	if err := c.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}

	leafKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)

	customSerial := big.NewInt(42)
	template := &x509.Certificate{
		SerialNumber: customSerial,
		Subject:      pkix.Name{CommonName: "test.localhost"},
		DNSNames:     []string{"test.localhost"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
	}

	certPEM, err := c.SignCertificate(template, &leafKey.PublicKey)
	if err != nil {
		t.Fatalf("SignCertificate: %v", err)
	}

	block, _ := pem.Decode(certPEM)
	leaf, _ := x509.ParseCertificate(block.Bytes)
	if leaf.SerialNumber.Cmp(customSerial) != 0 {
		t.Errorf("serial = %v, want %v", leaf.SerialNumber, customSerial)
	}
}

func TestSignCertificate_NotInitialized(t *testing.T) {
	dir := t.TempDir()
	c, _ := NewCA(dir)
	_, err := c.SignCertificate(&x509.Certificate{}, nil)
	if err == nil {
		t.Fatal("expected error signing with uninitialised CA")
	}
}

func TestNewCA_CreatesStorePath(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "path")
	_, err := NewCA(dir)
	if err != nil {
		t.Fatalf("NewCA: %v", err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if !info.IsDir() {
		t.Error("store path is not a directory")
	}
}

func TestECDSAKeysUsed(t *testing.T) {
	dir := t.TempDir()
	c, _ := NewCA(dir)
	if err := c.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}

	if _, ok := c.RootKey.(*ecdsa.PrivateKey); !ok {
		t.Errorf("root key type = %T, want *ecdsa.PrivateKey", c.RootKey)
	}
	if _, ok := c.InterKey.(*ecdsa.PrivateKey); !ok {
		t.Errorf("intermediate key type = %T, want *ecdsa.PrivateKey", c.InterKey)
	}
}
