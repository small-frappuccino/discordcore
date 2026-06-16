package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/small-frappuccino/discordcore/pkg/control"
	"github.com/small-frappuccino/discordcore/pkg/control/localtls"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
)

const (
	defaultLocalHTTPSControlAddr    = "127.0.0.1:8443"
	defaultLocalHTTPSPublicOrigin   = "https://localhost:8443"
	controlPublicOriginEnv          = "DISCORDCORE_CONTROL_PUBLIC_ORIGIN"
	defaultLocalTLSCommonName       = "localhost"
	defaultLocalTLSOrganizationName = "Small Frappuccino"
)

var errControlLocalTLSUnavailable = errors.New("control local tls unavailable")

// RunProfile selects which runtime a process drives: the primary main runtime
// or the QOTD-specialized runtime. See the RunProfile* constants.
type RunProfile string

// RunProfileDiscordMain is the primary runtime profile for the main discord bot process.
const (
	RunProfileDiscordMain RunProfile = "discordmain"
)

// RunOptions is the full configuration for a runtime process: which profile it
// drives, which bot instances and domains it hosts, and how its optional control
// plane is exposed. The zero value is not runnable; Profile must be set.
type RunOptions struct {
	Profile                  RunProfile
	Control                  ControlOptions
	CommandCatalogRegistrars []commands.CommandCatalogRegistrar
	DisableControl           bool
	Logger                   *slog.Logger
}

// ControlOptions configures the local control plane. BindAddr and PublicOrigin
// default to profile-specific values when empty; LocalHTTPS opts into serving
// over TLS on loopback.
type ControlOptions struct {
	BindAddr     string
	PublicOrigin string
	LocalHTTPS   ControlLocalHTTPSOptions
}

// ControlLocalHTTPSOptions enables serving the control plane over local HTTPS.
// AutoTrust additionally installs the generated certificate into the OS trust
// store and is honored only when Enabled is true.
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

func normalizeRunProfile(profile RunProfile) RunProfile {
	switch strings.TrimSpace(string(profile)) {
	case string(RunProfileDiscordMain):
		return RunProfileDiscordMain
	default:
		slog.Debug("Tracking complex conditional branch: Reverting unmapped runtime profile constraint to generic execution flow",
			slog.String("unmapped_profile", string(profile)),
		)
		return ""
	}
}

func defaultLocalHTTPSPublicOriginForProfile(profile RunProfile) string {
	switch normalizeRunProfile(profile) {
	case RunProfileDiscordMain:
		return "https://discordmain.localhost:8443"
	default:
		return defaultLocalHTTPSPublicOrigin
	}
}

func defaultLocalTLSCommonNameForProfile(profile RunProfile) string {
	switch normalizeRunProfile(profile) {
	case RunProfileDiscordMain:
		return "discordmain.localhost"
	default:
		return defaultLocalTLSCommonName
	}
}

func resolveControlRuntime(ctx context.Context, opts RunOptions) (resolvedControlRuntime, error) {
	profile := normalizeRunProfile(opts.Profile)
	bindAddr := strings.TrimSpace(opts.Control.BindAddr)
	publicOrigin := strings.TrimSpace(files.EnvString(controlPublicOriginEnv, opts.Control.PublicOrigin))

	slog.Info("Architectural state transition: Instantiating resolution pipeline for control plane bindings")

	if opts.Control.LocalHTTPS.Enabled {
		if bindAddr == "" {
			bindAddr = defaultLocalHTTPSControlAddr
		}
		if publicOrigin == "" {
			publicOrigin = defaultLocalHTTPSPublicOriginForProfile(profile)
		}
		slog.Debug("Tracking complex conditional branch: Injecting default local HTTPS topologies into control runtime matrix",
			slog.String("injected_bind_addr", bindAddr),
			slog.String("injected_public_origin", publicOrigin),
		)
	}
	if bindAddr == "" {
		bindAddr = defaultControlAddr
	}

	tlsCertFile, tlsKeyFile, err := loadControlTLSFilesFromEnv()
	if err != nil {
		errWrap := fmt.Errorf("load control tls config: %w", err)
		log.EmitBlockingError("Blocking structural failure: Environmental TLS payload validation rejected", errWrap, log.GenerateRequestID())
		return resolvedControlRuntime{}, errWrap
	}

	if tlsCertFile == "" && tlsKeyFile == "" && opts.Control.LocalHTTPS.Enabled {
		slog.Info("Architectural state transition: Initiating ad-hoc generation of local TLS credentials for control plane binding")
		ready, readyErr := prepareManagedLocalTLS(ctx, profile, publicOrigin, opts.Control.LocalHTTPS.AutoTrust)
		if readyErr != nil {
			errWrap := fmt.Errorf("%w: %w", errControlLocalTLSUnavailable, readyErr)
			log.EmitBlockingError("Blocking structural failure: Aborted generation of self-signed loopback TLS materials", errWrap, log.GenerateRequestID())
			return resolvedControlRuntime{}, errWrap
		}
		tlsCertFile = ready.CertFile
		tlsKeyFile = ready.KeyFile
	}

	oauthConfig, err := loadControlDiscordOAuthConfigFromEnv(publicOrigin)
	if err != nil {
		errWrap := fmt.Errorf("load control discord oauth config: %w", err)
		log.EmitBlockingError("Blocking structural failure: Validation of OAuth credentials against public origin aborted", errWrap, log.GenerateRequestID())
		return resolvedControlRuntime{}, errWrap
	}

	return resolvedControlRuntime{
		bindAddr:     bindAddr,
		publicOrigin: publicOrigin,
		oauthConfig:  oauthConfig,
		tlsCertFile:  tlsCertFile,
		tlsKeyFile:   tlsKeyFile,
	}, nil
}

func prepareManagedLocalTLS(ctx context.Context, profile RunProfile, publicOrigin string, autoTrust bool) (localtls.ReadyResult, error) {
	hostName, ipAddresses, err := localTLSSANs(profile, publicOrigin)
	if err != nil {
		errWrap := fmt.Errorf("resolve local tls sans: %w", err)
		log.EmitBlockingError("Blocking structural failure: Resolution of cryptographic Subject Alternate Names from host parameters failed", errWrap, log.GenerateRequestID())
		return localtls.ReadyResult{}, errWrap
	}

	slog.Debug("Tracking complex conditional branch: Forwarding resolved SAN variables to certificate authority simulation",
		slog.String("host_name", hostName),
		slog.Int("ip_addresses_count", len(ipAddresses)),
	)

	return localtls.EnsureReady(ctx, localtls.Config{
		Directory:    filepath.Join(files.ApplicationCachesPath, "control", "tls"),
		CommonName:   hostName,
		DNSNames:     []string{hostName, "localhost"},
		IPAddresses:  append(ipAddresses, net.ParseIP("127.0.0.1")),
		Organization: defaultLocalTLSOrganizationName,
		AutoTrust:    autoTrust,
	})
}

func localTLSSANs(profile RunProfile, publicOrigin string) (string, []net.IP, error) {
	if strings.TrimSpace(publicOrigin) == "" {
		slog.Debug("Granular inspection: Parsing local TLS Subject Alternate Names skipped, utilizing fallback parameters")
		return defaultLocalTLSCommonNameForProfile(profile), []net.IP{net.ParseIP("127.0.0.1")}, nil
	}
	parsed, err := url.Parse(publicOrigin)
	if err != nil {
		errWrap := fmt.Errorf("parse public origin: %w", err)
		slog.Warn("Mitigated service degradation: URL parsing failed against public origin scalar, aborting SAN computation",
			slog.String("invalid_origin", publicOrigin),
			slog.String("error", errWrap.Error()),
		)
		return "", nil, errWrap
	}
	host := strings.TrimSpace(parsed.Hostname())
	if host == "" {
		errWrap := fmt.Errorf("public origin %q is missing a hostname", publicOrigin)
		slog.Warn("Mitigated service degradation: Valid URL identified but hostname extraction failed, aborting SAN computation",
			slog.String("invalid_origin", publicOrigin),
		)
		return "", nil, errWrap
	}
	if ip := net.ParseIP(host); ip != nil {
		return host, []net.IP{ip}, nil
	}
	return host, nil, nil
}
