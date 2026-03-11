package localtls

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

const (
	defaultCARelativePath     = "local-ca-cert.pem"
	defaultCAKeyRelativePath  = "local-ca-key.pem"
	defaultCertRelativePath   = "control-cert.pem"
	defaultKeyRelativePath    = "control-key.pem"
	defaultCARotationWindow   = 30 * 24 * time.Hour
	defaultCertRotationWindow = 14 * 24 * time.Hour
	defaultCAValidity         = 5 * 365 * 24 * time.Hour
	defaultCertValidity       = 397 * 24 * time.Hour
)

type TrustResult struct {
	Trusted   bool
	Installed bool
	Store     string
}

type ReadyResult struct {
	CertFile    string
	KeyFile     string
	Fingerprint string
	Trust       TrustResult
}

type TrustInstaller interface {
	EnsureTrusted(context.Context, *x509.Certificate) (TrustResult, error)
}

type Config struct {
	Directory       string
	CommonName      string
	DNSNames        []string
	IPAddresses     []net.IP
	Organization    string
	CACommonName    string
	AutoTrust       bool
	TrustInstaller  TrustInstaller
	Now             func() time.Time
	CARotationAfter time.Duration
	CertRotateAfter time.Duration
}

func EnsureReady(ctx context.Context, cfg Config) (ReadyResult, error) {
	now := cfg.now()
	if err := cfg.validate(); err != nil {
		return ReadyResult{}, err
	}
	if err := os.MkdirAll(cfg.Directory, 0o755); err != nil {
		return ReadyResult{}, fmt.Errorf("create local tls directory: %w", err)
	}

	caCertPath := filepath.Join(cfg.Directory, defaultCARelativePath)
	caKeyPath := filepath.Join(cfg.Directory, defaultCAKeyRelativePath)
	serverCertPath := filepath.Join(cfg.Directory, defaultCertRelativePath)
	serverKeyPath := filepath.Join(cfg.Directory, defaultKeyRelativePath)

	caPair, caRotated, err := ensureCAPair(caCertPath, caKeyPath, cfg, now)
	if err != nil {
		return ReadyResult{}, fmt.Errorf("ensure local tls ca: %w", err)
	}

	serverPair, err := ensureServerPair(serverCertPath, serverKeyPath, cfg, caPair, caRotated, now)
	if err != nil {
		return ReadyResult{}, fmt.Errorf("ensure local tls certificate: %w", err)
	}

	trustResult := TrustResult{}
	if cfg.AutoTrust {
		installer := cfg.trustInstaller()
		if installer == nil {
			return ReadyResult{}, fmt.Errorf("auto-trust requested but no trust installer is available")
		}
		trustResult, err = installer.EnsureTrusted(ctx, caPair.cert)
		if err != nil {
			return ReadyResult{}, fmt.Errorf("trust local tls ca: %w", err)
		}
	}

	return ReadyResult{
		CertFile:    serverCertPath,
		KeyFile:     serverKeyPath,
		Fingerprint: certificateFingerprint(serverPair.cert.Raw),
		Trust:       trustResult,
	}, nil
}

type certificatePair struct {
	cert *x509.Certificate
	key  *rsa.PrivateKey
}

type invalidMaterialError struct {
	err error
}

func (e invalidMaterialError) Error() string {
	return e.err.Error()
}

func (e invalidMaterialError) Unwrap() error {
	return e.err
}

func ensureCAPair(certPath string, keyPath string, cfg Config, now time.Time) (certificatePair, bool, error) {
	if pair, err := loadCertificatePair(certPath, keyPath); err == nil {
		if err := validateCAPair(pair, cfg, now); err == nil {
			return pair, false, nil
		}
	} else if !os.IsNotExist(err) && !isInvalidMaterialError(err) {
		return certificatePair{}, false, err
	}

	pair, err := generateCAPair(cfg, now)
	if err != nil {
		return certificatePair{}, false, err
	}
	if err := persistCertificatePair(certPath, keyPath, pair); err != nil {
		return certificatePair{}, false, err
	}
	return pair, true, nil
}

func ensureServerPair(certPath string, keyPath string, cfg Config, ca certificatePair, forceRotate bool, now time.Time) (certificatePair, error) {
	if !forceRotate {
		if pair, err := loadCertificatePair(certPath, keyPath); err == nil {
			if err := validateServerPair(pair, ca.cert, cfg, now); err == nil {
				return pair, nil
			}
		} else if !os.IsNotExist(err) && !isInvalidMaterialError(err) {
			return certificatePair{}, err
		}
	}

	pair, err := generateServerPair(cfg, ca, now)
	if err != nil {
		return certificatePair{}, err
	}
	if err := persistCertificatePair(certPath, keyPath, pair); err != nil {
		return certificatePair{}, err
	}
	return pair, nil
}

func loadCertificatePair(certPath string, keyPath string) (certificatePair, error) {
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return certificatePair{}, err
	}
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return certificatePair{}, err
	}

	cert, err := decodeCertificatePEM(certPEM)
	if err != nil {
		return certificatePair{}, invalidMaterialError{err: fmt.Errorf("decode certificate %s: %w", certPath, err)}
	}
	key, err := decodeRSAPrivateKeyPEM(keyPEM)
	if err != nil {
		return certificatePair{}, invalidMaterialError{err: fmt.Errorf("decode key %s: %w", keyPath, err)}
	}
	return certificatePair{cert: cert, key: key}, nil
}

func persistCertificatePair(certPath string, keyPath string, pair certificatePair) error {
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: pair.cert.Raw,
	})
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(pair.key),
	})

	if err := os.WriteFile(certPath, certPEM, 0o600); err != nil {
		return fmt.Errorf("write certificate %s: %w", certPath, err)
	}
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		return fmt.Errorf("write key %s: %w", keyPath, err)
	}
	return nil
}

func validateCAPair(pair certificatePair, cfg Config, now time.Time) error {
	if pair.cert == nil || pair.key == nil {
		return fmt.Errorf("ca pair is incomplete")
	}
	if !pair.cert.IsCA {
		return fmt.Errorf("ca certificate is not marked as a certificate authority")
	}
	if err := validateCertificateWindow(pair.cert, now, cfg.caRotationWindow()); err != nil {
		return err
	}
	if !publicKeysMatch(pair.cert.PublicKey, &pair.key.PublicKey) {
		return fmt.Errorf("ca certificate and key do not match")
	}
	return nil
}

func validateServerPair(pair certificatePair, ca *x509.Certificate, cfg Config, now time.Time) error {
	if pair.cert == nil || pair.key == nil {
		return fmt.Errorf("server pair is incomplete")
	}
	if err := validateCertificateWindow(pair.cert, now, cfg.certRotationWindow()); err != nil {
		return err
	}
	if !publicKeysMatch(pair.cert.PublicKey, &pair.key.PublicKey) {
		return fmt.Errorf("server certificate and key do not match")
	}
	if ca == nil {
		return fmt.Errorf("ca certificate is required")
	}
	pool := x509.NewCertPool()
	pool.AddCert(ca)
	if _, err := pair.cert.Verify(x509.VerifyOptions{
		Roots:     pool,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}); err != nil {
		return fmt.Errorf("verify server certificate against ca: %w", err)
	}
	for _, dnsName := range cfg.normalizedDNSNames() {
		if err := pair.cert.VerifyHostname(dnsName); err != nil {
			return fmt.Errorf("server certificate missing dns san %q", dnsName)
		}
	}
	for _, ip := range cfg.normalizedIPAddresses() {
		if !slices.ContainsFunc(pair.cert.IPAddresses, func(existing net.IP) bool {
			return existing.Equal(ip)
		}) {
			return fmt.Errorf("server certificate missing ip san %q", ip.String())
		}
	}
	return nil
}

func generateCAPair(cfg Config, now time.Time) (certificatePair, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return certificatePair{}, fmt.Errorf("generate ca key: %w", err)
	}
	notBefore := now.Add(-time.Hour)
	template := &x509.Certificate{
		SerialNumber: randomSerialNumber(),
		Subject: pkix.Name{
			CommonName:   cfg.caCommonName(),
			Organization: []string{cfg.organization()},
		},
		NotBefore:             notBefore,
		NotAfter:              notBefore.Add(defaultCAValidity),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLenZero:        true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return certificatePair{}, fmt.Errorf("create ca certificate: %w", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return certificatePair{}, fmt.Errorf("parse generated ca certificate: %w", err)
	}
	return certificatePair{cert: cert, key: key}, nil
}

func generateServerPair(cfg Config, ca certificatePair, now time.Time) (certificatePair, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return certificatePair{}, fmt.Errorf("generate server key: %w", err)
	}
	notBefore := now.Add(-time.Hour)
	template := &x509.Certificate{
		SerialNumber: randomSerialNumber(),
		Subject: pkix.Name{
			CommonName:   cfg.commonName(),
			Organization: []string{cfg.organization()},
		},
		NotBefore:             notBefore,
		NotAfter:              notBefore.Add(defaultCertValidity),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              cfg.normalizedDNSNames(),
		IPAddresses:           cfg.normalizedIPAddresses(),
	}
	der, err := x509.CreateCertificate(rand.Reader, template, ca.cert, &key.PublicKey, ca.key)
	if err != nil {
		return certificatePair{}, fmt.Errorf("create server certificate: %w", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return certificatePair{}, fmt.Errorf("parse generated server certificate: %w", err)
	}
	return certificatePair{cert: cert, key: key}, nil
}

func decodeCertificatePEM(raw []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, fmt.Errorf("missing pem block")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, err
	}
	return cert, nil
}

func decodeRSAPrivateKeyPEM(raw []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, fmt.Errorf("missing pem block")
	}
	switch block.Type {
	case "RSA PRIVATE KEY":
		return x509.ParsePKCS1PrivateKey(block.Bytes)
	case "PRIVATE KEY":
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		rsaKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("private key is not rsa")
		}
		return rsaKey, nil
	default:
		return nil, fmt.Errorf("unsupported private key type %q", block.Type)
	}
}

func validateCertificateWindow(cert *x509.Certificate, now time.Time, rotateAfter time.Duration) error {
	if cert == nil {
		return fmt.Errorf("certificate is nil")
	}
	if now.Before(cert.NotBefore) {
		return fmt.Errorf("certificate is not valid yet")
	}
	if !now.Before(cert.NotAfter) {
		return fmt.Errorf("certificate has expired")
	}
	if rotateAfter > 0 && !now.Before(cert.NotAfter.Add(-rotateAfter)) {
		return fmt.Errorf("certificate is too close to expiration")
	}
	return nil
}

func publicKeysMatch(left any, right any) bool {
	leftKey, ok := left.(*rsa.PublicKey)
	if !ok {
		return false
	}
	rightKey, ok := right.(*rsa.PublicKey)
	if !ok {
		return false
	}
	return leftKey.N.Cmp(rightKey.N) == 0 && leftKey.E == rightKey.E
}

func certificateFingerprint(raw []byte) string {
	sum := sha256.Sum256(raw)
	return strings.ToUpper(hex.EncodeToString(sum[:]))
}

func randomSerialNumber() *big.Int {
	value, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return big.NewInt(time.Now().UnixNano())
	}
	return value
}

func (cfg Config) validate() error {
	if strings.TrimSpace(cfg.Directory) == "" {
		return fmt.Errorf("local tls directory is required")
	}
	if strings.TrimSpace(cfg.commonName()) == "" {
		return fmt.Errorf("local tls common name is required")
	}
	if len(cfg.normalizedDNSNames()) == 0 && len(cfg.normalizedIPAddresses()) == 0 {
		return fmt.Errorf("at least one dns name or ip address is required")
	}
	return nil
}

func (cfg Config) now() time.Time {
	if cfg.Now != nil {
		return cfg.Now().UTC()
	}
	return time.Now().UTC()
}

func (cfg Config) caRotationWindow() time.Duration {
	if cfg.CARotationAfter > 0 {
		return cfg.CARotationAfter
	}
	return defaultCARotationWindow
}

func (cfg Config) certRotationWindow() time.Duration {
	if cfg.CertRotateAfter > 0 {
		return cfg.CertRotateAfter
	}
	return defaultCertRotationWindow
}

func (cfg Config) commonName() string {
	return strings.TrimSpace(cfg.CommonName)
}

func (cfg Config) organization() string {
	value := strings.TrimSpace(cfg.Organization)
	if value == "" {
		return "Small Frappuccino Local TLS"
	}
	return value
}

func (cfg Config) caCommonName() string {
	value := strings.TrimSpace(cfg.CACommonName)
	if value == "" {
		return cfg.organization() + " Root CA"
	}
	return value
}

func (cfg Config) normalizedDNSNames() []string {
	out := make([]string, 0, len(cfg.DNSNames)+1)
	seen := map[string]struct{}{}
	if cn := strings.TrimSpace(cfg.CommonName); cn != "" {
		seen[cn] = struct{}{}
		out = append(out, cn)
	}
	for _, raw := range cfg.DNSNames {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		lower := strings.ToLower(value)
		if _, ok := seen[lower]; ok {
			continue
		}
		seen[lower] = struct{}{}
		out = append(out, value)
	}
	return out
}

func (cfg Config) normalizedIPAddresses() []net.IP {
	out := make([]net.IP, 0, len(cfg.IPAddresses))
	seen := map[string]struct{}{}
	for _, raw := range cfg.IPAddresses {
		if raw == nil {
			continue
		}
		value := append(net.IP(nil), raw...)
		key := value.String()
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	return out
}

func (cfg Config) trustInstaller() TrustInstaller {
	if cfg.TrustInstaller != nil {
		return cfg.TrustInstaller
	}
	return newPlatformTrustInstaller()
}

func isInvalidMaterialError(err error) bool {
	var invalid invalidMaterialError
	return errors.As(err, &invalid)
}
