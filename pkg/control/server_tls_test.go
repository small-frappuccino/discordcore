package control

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/files"
)

func TestSetTLSCertificatesValidation(t *testing.T) {
	t.Parallel()

	srv, _ := newControlTestServer(t)
	if err := srv.SetTLSCertificates("cert.pem", ""); err == nil {
		t.Fatal("expected error when key file is missing")
	}
	if err := srv.SetTLSCertificates("", "key.pem"); err == nil {
		t.Fatal("expected error when cert file is missing")
	}
}

func TestControlServerStartWithTLS(t *testing.T) {
	cm := files.NewConfigManagerWithPath(filepath.Join(t.TempDir(), "settings.json"))
	if err := cm.AddGuildConfig(files.GuildConfig{GuildID: "g1"}); err != nil {
		t.Fatalf("add guild config: %v", err)
	}

	srv := NewServer("127.0.0.1:0", cm, nil)
	if srv == nil {
		t.Fatal("expected non-nil control server")
	}
	srv.SetBearerToken(controlTestAuthToken)

	certFile, keyFile := writeSelfSignedCertificatePair(t)
	if err := srv.SetTLSCertificates(certFile, keyFile); err != nil {
		t.Fatalf("configure tls certificates: %v", err)
	}

	if err := srv.Start(); err != nil {
		t.Fatalf("start control server with tls: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Stop(ctx)
	})

	client := &http.Client{
		Timeout: 2 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				MinVersion:         tls.VersionTLS12,
				InsecureSkipVerify: true, //nolint:gosec // test-only self-signed certificate.
			},
		},
	}

	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("https://%s/v1/guilds/g1/partner-board", srv.listener.Addr().String()), nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+controlTestAuthToken)

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("perform tls request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from tls control endpoint, got %d", resp.StatusCode)
	}
}

func writeSelfSignedCertificatePair(t *testing.T) (certFile string, keyFile string) {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject: pkix.Name{
			CommonName: "127.0.0.1",
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatalf("create self-signed certificate: %v", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDER,
	})
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})

	dir := t.TempDir()
	certFile = filepath.Join(dir, "control-test-cert.pem")
	keyFile = filepath.Join(dir, "control-test-key.pem")

	if err := os.WriteFile(certFile, certPEM, 0o600); err != nil {
		t.Fatalf("write certificate file: %v", err)
	}
	if err := os.WriteFile(keyFile, keyPEM, 0o600); err != nil {
		t.Fatalf("write key file: %v", err)
	}

	if strings.TrimSpace(certFile) == "" || strings.TrimSpace(keyFile) == "" {
		t.Fatal("expected non-empty certificate paths")
	}
	return certFile, keyFile
}
