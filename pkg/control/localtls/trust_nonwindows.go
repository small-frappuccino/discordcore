//go:build !windows

package localtls

import (
	"context"
	"crypto/x509"
	"fmt"
)

func (unsupportedTrustInstaller) EnsureTrusted(context.Context, *x509.Certificate) (TrustResult, error) {
	return TrustResult{}, fmt.Errorf("automatic local tls trust is only supported on windows")
}

func newPlatformTrustInstaller() TrustInstaller {
	return unsupportedTrustInstaller{}
}
