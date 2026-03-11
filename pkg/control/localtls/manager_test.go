package localtls

import (
	"context"
	"crypto/x509"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type fakeTrustInstaller struct {
	calls     int
	result    TrustResult
	lastCert  *x509.Certificate
	returnErr error
}

func (f *fakeTrustInstaller) EnsureTrusted(_ context.Context, cert *x509.Certificate) (TrustResult, error) {
	f.calls++
	f.lastCert = cert
	if f.returnErr != nil {
		return TrustResult{}, f.returnErr
	}
	return f.result, nil
}

func TestEnsureReadyCreatesMaterialsAndTrusts(t *testing.T) {
	dir := t.TempDir()
	truster := &fakeTrustInstaller{result: TrustResult{Trusted: true, Installed: true, Store: "test"}}
	now := time.Date(2026, time.March, 11, 12, 0, 0, 0, time.UTC)

	result, err := EnsureReady(context.Background(), Config{
		Directory:      dir,
		CommonName:     "alice.localhost",
		DNSNames:       []string{"localhost"},
		IPAddresses:    []net.IP{net.ParseIP("127.0.0.1")},
		AutoTrust:      true,
		TrustInstaller: truster,
		Now:            func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("ensure ready: %v", err)
	}
	if result.CertFile == "" || result.KeyFile == "" || result.Fingerprint == "" {
		t.Fatalf("unexpected ready result: %+v", result)
	}
	if truster.calls != 1 || truster.lastCert == nil {
		t.Fatalf("expected trust installer to be called once, got %+v", truster)
	}
	if _, err := os.Stat(result.CertFile); err != nil {
		t.Fatalf("stat cert file: %v", err)
	}
	if _, err := os.Stat(result.KeyFile); err != nil {
		t.Fatalf("stat key file: %v", err)
	}
}

func TestEnsureReadyReusesExistingMaterials(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, time.March, 11, 12, 0, 0, 0, time.UTC)

	first, err := EnsureReady(context.Background(), Config{
		Directory:   dir,
		CommonName:  "alice.localhost",
		DNSNames:    []string{"localhost"},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
		Now:         func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("first ensure ready: %v", err)
	}
	second, err := EnsureReady(context.Background(), Config{
		Directory:   dir,
		CommonName:  "alice.localhost",
		DNSNames:    []string{"localhost"},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
		Now:         func() time.Time { return now.Add(24 * time.Hour) },
	})
	if err != nil {
		t.Fatalf("second ensure ready: %v", err)
	}
	if first.Fingerprint != second.Fingerprint {
		t.Fatalf("expected certificate fingerprint to be reused, first=%q second=%q", first.Fingerprint, second.Fingerprint)
	}
}

func TestEnsureReadyRotatesServerCertificateWhenSANSChange(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, time.March, 11, 12, 0, 0, 0, time.UTC)

	first, err := EnsureReady(context.Background(), Config{
		Directory:  dir,
		CommonName: "alice.localhost",
		DNSNames:   []string{"localhost"},
		Now:        func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("first ensure ready: %v", err)
	}

	second, err := EnsureReady(context.Background(), Config{
		Directory:  dir,
		CommonName: "alice.localhost",
		DNSNames:   []string{"localhost", "api.alice.localhost"},
		Now:        func() time.Time { return now.Add(24 * time.Hour) },
	})
	if err != nil {
		t.Fatalf("second ensure ready: %v", err)
	}
	if first.Fingerprint == second.Fingerprint {
		t.Fatalf("expected server certificate to rotate after SAN change, fingerprint=%q", first.Fingerprint)
	}
}

func TestEnsureReadyErrorsOnCorruptKey(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, time.March, 11, 12, 0, 0, 0, time.UTC)
	if _, err := EnsureReady(context.Background(), Config{
		Directory:  dir,
		CommonName: "alice.localhost",
		DNSNames:   []string{"localhost"},
		Now:        func() time.Time { return now },
	}); err != nil {
		t.Fatalf("initial ensure ready: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, defaultKeyRelativePath), []byte("not-a-key"), 0o600); err != nil {
		t.Fatalf("corrupt key file: %v", err)
	}

	if _, err := EnsureReady(context.Background(), Config{
		Directory:  dir,
		CommonName: "alice.localhost",
		DNSNames:   []string{"localhost"},
		Now:        func() time.Time { return now.Add(24 * time.Hour) },
	}); err != nil {
		t.Fatalf("expected corrupt key to trigger rotation, got error: %v", err)
	}
}
