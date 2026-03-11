//go:build windows

package localtls

import (
	"context"
	"crypto/x509"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	x509ASNEncoding         = 0x00000001
	pkcs7ASNEncoding        = 0x00010000
	certStoreAddUseExisting = 2
)

var (
	crypt32DLL                = windows.NewLazySystemDLL("crypt32.dll")
	procCertAddEncodedToStore = crypt32DLL.NewProc("CertAddEncodedCertificateToStore")
)

type windowsTrustInstaller struct{}

func newPlatformTrustInstaller() TrustInstaller {
	return windowsTrustInstaller{}
}

func (windowsTrustInstaller) EnsureTrusted(_ context.Context, cert *x509.Certificate) (TrustResult, error) {
	if cert == nil {
		return TrustResult{}, fmt.Errorf("ca certificate is required")
	}
	storeName, err := windows.UTF16PtrFromString("ROOT")
	if err != nil {
		return TrustResult{}, fmt.Errorf("encode windows root store name: %w", err)
	}
	store, err := windows.CertOpenSystemStore(0, storeName)
	if err != nil {
		return TrustResult{}, fmt.Errorf("open windows root certificate store: %w", err)
	}
	defer windows.CertCloseStore(store, 0)

	if trusted, err := certificateExistsInStore(store, cert.Raw); err != nil {
		return TrustResult{}, fmt.Errorf("check windows root certificate store: %w", err)
	} else if trusted {
		return TrustResult{Trusted: true, Installed: false, Store: "CurrentUser\\Root"}, nil
	}

	success, _, addErr := procCertAddEncodedToStore.Call(
		uintptr(store),
		uintptr(x509ASNEncoding|pkcs7ASNEncoding),
		uintptr(unsafe.Pointer(&cert.Raw[0])),
		uintptr(len(cert.Raw)),
		uintptr(certStoreAddUseExisting),
		0,
	)
	if success == 0 {
		return TrustResult{}, fmt.Errorf("install certificate in windows root store: %w", addErr)
	}
	return TrustResult{Trusted: true, Installed: true, Store: "CurrentUser\\Root"}, nil
}

func certificateExistsInStore(store windows.Handle, raw []byte) (bool, error) {
	var previous *windows.CertContext
	for {
		current, err := windows.CertEnumCertificatesInStore(store, previous)
		if err != nil {
			if err == windows.Errno(windows.CRYPT_E_NOT_FOUND) {
				return false, nil
			}
			return false, err
		}
		if current == nil {
			return false, nil
		}
		previous = current

		if current.EncodedCert == nil || current.Length == 0 {
			continue
		}
		encoded := unsafe.Slice(current.EncodedCert, current.Length)
		if len(encoded) == len(raw) && equalBytes(encoded, raw) {
			_ = windows.CertFreeCertificateContext(previous)
			return true, nil
		}
	}
}

func equalBytes(left []byte, right []byte) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
