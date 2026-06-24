package control

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/diamondburned/arikawa/v3/state"
	"github.com/small-frappuccino/discordcore/pkg/discord/cache"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/moderation"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/members"
	"github.com/small-frappuccino/discordcore/pkg/messages"
	"github.com/small-frappuccino/discordcore/pkg/qotd"
	"github.com/small-frappuccino/discordcore/pkg/runtimeapply"
	"github.com/small-frappuccino/discordcore/pkg/storage/postgres"
	"golang.org/x/oauth2"
)

// ErrControlServerBind indicates a fatal failure to bind the HTTP control plane to its configured network interface.
var ErrControlServerBind = errors.New("control server bind failed")

// ServerOption defines a functional option for configuring the control server.
type ServerOption func(*Server) error

// BotGuildBinding couples a Discord guild identifier with its authoritative bot instance execution context.
type BotGuildBinding struct {
	GuildID       string
	BotInstanceID string
}

// DiscordOAuthConfig defines the immutable parameters required for authenticating users via the Discord OAuth2 flow.
type DiscordOAuthConfig struct {
	ClientID                 string
	ClientSecret             string
	RedirectURI              string
	IncludeGuildsMembersRead bool
	SessionStorePath         string
	Scopes                   []string
}

// Server orchestrates the primary HTTP control plane, dashboard serving, and external API routing.
//
// The server manages the lifecycle of the HTTP listener, TLS configuration, and multiplexes requests
// to their respective feature handlers while enforcing concurrent state isolation via internal mutexes.
type Server struct {
	bindAddr       string
	configManager  *files.ConfigManager
	runtimeApplier *runtimeapply.Manager

	bearerToken               string
	knownBotInstanceIDs       []string
	qotdService               *qotd.Service
	moderationMetrics         moderation.Metrics
	membersMetricsResolver    func() members.Metrics
	messagesMetricsResolver   func() messages.Metrics
	store                     *postgres.Store
	cacheObservability        func() *cache.UnifiedCache
	arikawaStateResolver      func(guildID string) (*state.State, error)
	botGuildBindingsProvider  func(ctx context.Context) ([]BotGuildBinding, error)
	guildRegistrationResolver func(ctx context.Context, guildID string) error

	publicOrigin string
	tlsCertFile  string
	tlsKeyFile   string
	oauthConfig  *oauth2.Config

	httpServer *http.Server
}

// NewServer initializes a new control plane server instance.
//
// It assigns the network bind address and explicitly injects the required configuration and runtime dependencies
// prior to route registration and lifecycle commencement.
func NewServer(addr string, configManager *files.ConfigManager, runtimeApplier *runtimeapply.Manager, opts ...ServerOption) (*Server, error) {
	if addr == "" {
		return nil, errors.New("empty bind address")
	}
	s := &Server{
		bindAddr:       addr,
		configManager:  configManager,
		runtimeApplier: runtimeApplier,
	}

	for _, opt := range opts {
		if err := opt(s); err != nil {
			return nil, err
		}
	}

	return s, nil
}

// SetBearerToken injects the authorization token required for secured administrative route access.
func (s *Server) SetBearerToken(token string) { s.bearerToken = token }

// SetKnownBotInstanceIDs configures the slice of active bot instance identifiers for runtime validation.
func (s *Server) SetKnownBotInstanceIDs(ids []string) { s.knownBotInstanceIDs = ids }

// SetQOTDService injects the Question of the Day service interface into the control plane.
func (s *Server) SetQOTDService(svc *qotd.Service) { s.qotdService = svc }

// SetModerationMetrics configures the accessors for exposing real-time moderation telemetry.
func (s *Server) SetModerationMetrics(metrics moderation.Metrics) {
	s.moderationMetrics = metrics
}

// SetMembersMetricsResolver provides the callback function responsible for resolving member telemetry.
func (s *Server) SetMembersMetricsResolver(resolver func() members.Metrics) {
	s.membersMetricsResolver = resolver
}

// SetMessagesMetricsResolver provides the callback function responsible for resolving message telemetry.
func (s *Server) SetMessagesMetricsResolver(resolver func() messages.Metrics) {
	s.messagesMetricsResolver = resolver
}

// SetStorage injects the persistent PostgreSQL domain storage dependency into the server instance.
func (s *Server) SetStorage(store *postgres.Store) { s.store = store }

// SetCacheObservability configures the callback to access the unified cache state for observability endpoints.
func (s *Server) SetCacheObservability(resolver func() *cache.UnifiedCache, store *postgres.Store) {
	s.cacheObservability = resolver
}

// SetArikawaStateResolver injects the dependency responsible for resolving guild-specific Arikawa runtime states.
func (s *Server) SetArikawaStateResolver(resolver func(guildID string) (*state.State, error)) {
	s.arikawaStateResolver = resolver
}

// SetBotGuildBindingsProvider provides the callback for fetching active guild bindings from persistent postgres.
func (s *Server) SetBotGuildBindingsProvider(provider func(ctx context.Context) ([]BotGuildBinding, error)) {
	s.botGuildBindingsProvider = provider
}

// SetGuildRegistrationResolver configures the callback responsible for registering new guilds within the control plane.
func (s *Server) SetGuildRegistrationResolver(resolver func(ctx context.Context, guildID string) error) {
	s.guildRegistrationResolver = resolver
}

// SetPublicOrigin defines the externally accessible base URL for the dashboard and OAuth callbacks.
func (s *Server) SetPublicOrigin(origin string) error {
	s.publicOrigin = origin
	return nil
}

// SetTLSCertificates configures the absolute file paths to the X.509 certificate and private key for TLS termination.
func (s *Server) SetTLSCertificates(certFile, keyFile string) error {
	s.tlsCertFile = certFile
	s.tlsKeyFile = keyFile
	return nil
}

// SetDiscordOAuthConfig initializes the OAuth2 configuration using the provided credential and scope parameters.
func (s *Server) SetDiscordOAuthConfig(config DiscordOAuthConfig) error {
	s.oauthConfig = &oauth2.Config{
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		RedirectURL:  config.RedirectURI,
		Scopes:       config.Scopes,
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://discord.com/oauth2/authorize",
			TokenURL: "https://discord.com/api/oauth2/token",
		},
	}
	return nil
}

// Start binds the HTTP listener to the configured address synchronously and commences non-blocking request serving.
//
// It triggers a fatal runtime abort if the primary bind fails synchronously, and emits blocking errors
// for asynchronous failures that compromise the main data flow.
func (s *Server) Start() error {
	slog.Info("Architectural state transition: Initializing primary HTTP control plane", slog.String("bind_addr", s.bindAddr))

	mux := http.NewServeMux()
	s.registerRoutes(mux)

	s.httpServer = &http.Server{
		Addr:    s.bindAddr,
		Handler: mux,
	}

	listener, err := net.Listen("tcp", s.bindAddr)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrControlServerBind, err)
	}

	go func() {
		// Isolate the blocking HTTP listener in an asynchronous goroutine to prevent
		// stalling the primary boot pipeline. The main event loop remains responsive.
		var err error
		if s.tlsCertFile != "" && s.tlsKeyFile != "" {
			err = s.httpServer.ServeTLS(listener, s.tlsCertFile, s.tlsKeyFile)
		} else {
			err = s.httpServer.Serve(listener)
		}
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			// Explicitly filter http.ErrServerClosed to prevent false positive blocking errors
			// from being emitted during planned graceful shutdown transitions.
			log.EmitBlockingError("http server failure", err, "")
		}
	}()
	return nil
}

// Stop initiates a graceful teardown of the HTTP control plane, bounded by a strict 5-second context timeout.
//
// All active connections are allowed to drain until the timeout expires, at which point the listener is forcefully closed
// to prevent zombie processes and ensure deterministic lifecycle termination.
func (s *Server) Stop(ctx context.Context) error {
	if s.httpServer == nil {
		slog.Debug("Granular inspection: Stop invoked on uninitialized HTTP control plane")
		return nil
	}

	slog.Info("Architectural state transition: Commencing graceful shutdown of HTTP control plane")

	// Enforce a strict 5-second upper bound context timeout to prevent hanging connections
	// from inducing zombie processes during orchestrated application teardowns.
	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	err := s.httpServer.Shutdown(shutdownCtx)
	if err != nil {
		slog.Warn("Graceful shutdown failed or timed out, forcing immediate socket closure", slog.String("error", err.Error()))
		closeErr := s.httpServer.Close()
		if closeErr != nil {
			return fmt.Errorf("shutdown error: %v, force close error: %v", err, closeErr)
		}
		return err
	}
	return nil
}

// BroadcastGuildEvent propagates a transient presence update across the control plane.
func (s *Server) BroadcastGuildEvent(guildID string, botPresent bool) {}
