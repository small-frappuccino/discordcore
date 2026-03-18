package app

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/small-frappuccino/discordcore/pkg/control"
	"github.com/small-frappuccino/discordcore/pkg/control/localtls"
	"github.com/small-frappuccino/discordcore/pkg/util"
)

const (
	defaultLocalHTTPSControlAddr    = "127.0.0.1:8443"
	defaultLocalHTTPSPublicOrigin   = "https://alice.localhost:8443"
	controlPublicOriginEnv          = "ALICE_CONTROL_PUBLIC_ORIGIN"
	defaultLocalTLSCommonName       = "alice.localhost"
	defaultLocalTLSOrganizationName = "Small Frappuccino"
)

var errControlLocalTLSUnavailable = errors.New("control local tls unavailable")

type RunOptions struct {
	Control              ControlOptions
	BotCatalog           []BotInstanceDefinition
	DefaultBotInstanceID string
}

type ControlOptions struct {
	BindAddr     string
	PublicOrigin string
	LocalHTTPS   ControlLocalHTTPSOptions
}

type ControlLocalHTTPSOptions struct {
	Enabled   bool
	AutoTrust bool
}

type resolvedControlRuntime struct {
	bindAddr     string
	publicOrigin string
	oauthConfig  *control.DiscordOAuthConfig
	tlsCertFile  string
	tlsKeyFile   string
}

func resolveControlRuntime(ctx context.Context, opts RunOptions) (resolvedControlRuntime, error) {
	bindAddr := strings.TrimSpace(opts.Control.BindAddr)
	publicOrigin := strings.TrimSpace(util.EnvString(controlPublicOriginEnv, opts.Control.PublicOrigin))
	if opts.Control.LocalHTTPS.Enabled {
		if bindAddr == "" {
			bindAddr = defaultLocalHTTPSControlAddr
		}
		if publicOrigin == "" {
			publicOrigin = defaultLocalHTTPSPublicOrigin
		}
	}
	if bindAddr == "" {
		bindAddr = defaultControlAddr
	}

	tlsCertFile, tlsKeyFile, err := loadControlTLSFilesFromEnv()
	if err != nil {
		return resolvedControlRuntime{}, fmt.Errorf("load control tls config: %w", err)
	}
	if tlsCertFile == "" && tlsKeyFile == "" && opts.Control.LocalHTTPS.Enabled {
		ready, readyErr := prepareManagedLocalTLS(ctx, publicOrigin, opts.Control.LocalHTTPS.AutoTrust)
		if readyErr != nil {
			return resolvedControlRuntime{}, fmt.Errorf("%w: %w", errControlLocalTLSUnavailable, readyErr)
		}
		tlsCertFile = ready.CertFile
		tlsKeyFile = ready.KeyFile
	}

	oauthConfig, err := loadControlDiscordOAuthConfigFromEnv(publicOrigin)
	if err != nil {
		return resolvedControlRuntime{}, fmt.Errorf("load control discord oauth config: %w", err)
	}

	return resolvedControlRuntime{
		bindAddr:     bindAddr,
		publicOrigin: publicOrigin,
		oauthConfig:  oauthConfig,
		tlsCertFile:  tlsCertFile,
		tlsKeyFile:   tlsKeyFile,
	}, nil
}

func prepareManagedLocalTLS(ctx context.Context, publicOrigin string, autoTrust bool) (localtls.ReadyResult, error) {
	hostName, ipAddresses, err := localTLSSANs(publicOrigin)
	if err != nil {
		return localtls.ReadyResult{}, fmt.Errorf("resolve local tls sans: %w", err)
	}

	return localtls.EnsureReady(ctx, localtls.Config{
		Directory:    filepath.Join(util.ApplicationCachesPath, "control", "tls"),
		CommonName:   hostName,
		DNSNames:     []string{hostName, "localhost"},
		IPAddresses:  append(ipAddresses, net.ParseIP("127.0.0.1")),
		Organization: defaultLocalTLSOrganizationName,
		AutoTrust:    autoTrust,
	})
}

func localTLSSANs(publicOrigin string) (string, []net.IP, error) {
	if strings.TrimSpace(publicOrigin) == "" {
		return defaultLocalTLSCommonName, []net.IP{net.ParseIP("127.0.0.1")}, nil
	}
	parsed, err := url.Parse(publicOrigin)
	if err != nil {
		return "", nil, fmt.Errorf("parse public origin: %w", err)
	}
	host := strings.TrimSpace(parsed.Hostname())
	if host == "" {
		return "", nil, fmt.Errorf("public origin %q is missing a hostname", publicOrigin)
	}
	if ip := net.ParseIP(host); ip != nil {
		return host, []net.IP{ip}, nil
	}
	return host, nil, nil
}
