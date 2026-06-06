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
	"github.com/small-frappuccino/discordcore/pkg/discord/commands"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

const (
	defaultLocalHTTPSControlAddr    = "127.0.0.1:8443"
	defaultLocalHTTPSPublicOrigin   = "https://localhost:8443"
	controlPublicOriginEnv          = "ALICE_CONTROL_PUBLIC_ORIGIN"
	defaultLocalTLSCommonName       = "localhost"
	defaultLocalTLSOrganizationName = "Small Frappuccino"
)

var errControlLocalTLSUnavailable = errors.New("control local tls unavailable")

// RunProfile selects which runtime a process drives: the primary main runtime
// or the QOTD-specialized runtime. See the RunProfile* constants.
type RunProfile string

// RunProfileDiscordQOTD defines run profile discord qotd.
// RunProfileDiscordMain defines run profile discord main.
const (
	RunProfileDiscordMain RunProfile = "discordmain"
	RunProfileDiscordQOTD RunProfile = "discordqotd"
)

// RunOptions is the full configuration for a runtime process: which profile it
// drives, which bot instances and domains it hosts, and how its optional control
// plane is exposed. The zero value is not runnable; Profile must be set.
type RunOptions struct {
	// Profile identifies the runtime profile driving this process, such as the
	// primary main runtime or the qotd-specialized runtime.
	Profile RunProfile
	// Control configures the optional local control plane served by this process.
	Control ControlOptions
	// BotCatalog is the set of bot instances whose tokens are hosted locally.
	BotCatalog []BotInstanceDefinition
	// DefaultOwnerBotInstanceID is the fallback owner used by legacy guild bindings.
	DefaultOwnerBotInstanceID string
	// KnownBotInstanceIDs are valid owner IDs referenced by shared config even when
	// this process does not host their tokens locally.
	KnownBotInstanceIDs []string
	// CommandCatalogRegistrars optionally override the default slash command
	// catalog composition for handlers started by this process.
	CommandCatalogRegistrars []commands.CommandCatalogRegistrar
	// DisableControl skips starting the local control plane for this process.
	DisableControl bool
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
	case string(RunProfileDiscordQOTD):
		return RunProfileDiscordQOTD
	default:
		return ""
	}
}

func defaultLocalHTTPSPublicOriginForProfile(profile RunProfile) string {
	switch normalizeRunProfile(profile) {
	case RunProfileDiscordMain:
		return "https://discordmain.localhost:8443"
	case RunProfileDiscordQOTD:
		return "https://discordqotd.localhost:8443"
	default:
		return defaultLocalHTTPSPublicOrigin
	}
}

func defaultLocalTLSCommonNameForProfile(profile RunProfile) string {
	switch normalizeRunProfile(profile) {
	case RunProfileDiscordMain:
		return "discordmain.localhost"
	case RunProfileDiscordQOTD:
		return "discordqotd.localhost"
	default:
		return defaultLocalTLSCommonName
	}
}

func resolveControlRuntime(ctx context.Context, opts RunOptions) (resolvedControlRuntime, error) {
	profile := normalizeRunProfile(opts.Profile)
	bindAddr := strings.TrimSpace(opts.Control.BindAddr)
	publicOrigin := strings.TrimSpace(files.EnvString(controlPublicOriginEnv, opts.Control.PublicOrigin))
	if opts.Control.LocalHTTPS.Enabled {
		if bindAddr == "" {
			bindAddr = defaultLocalHTTPSControlAddr
		}
		if publicOrigin == "" {
			publicOrigin = defaultLocalHTTPSPublicOriginForProfile(profile)
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
		ready, readyErr := prepareManagedLocalTLS(ctx, profile, publicOrigin, opts.Control.LocalHTTPS.AutoTrust)
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

func prepareManagedLocalTLS(ctx context.Context, profile RunProfile, publicOrigin string, autoTrust bool) (localtls.ReadyResult, error) {
	hostName, ipAddresses, err := localTLSSANs(profile, publicOrigin)
	if err != nil {
		return localtls.ReadyResult{}, fmt.Errorf("resolve local tls sans: %w", err)
	}

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
		return defaultLocalTLSCommonNameForProfile(profile), []net.IP{net.ParseIP("127.0.0.1")}, nil
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
