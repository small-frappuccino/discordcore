package control

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/diamondburned/arikawa/v3/state"
	"github.com/small-frappuccino/discordcore/pkg/discord/cache"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/members"
	"github.com/small-frappuccino/discordcore/pkg/messages"
	"github.com/small-frappuccino/discordcore/pkg/runtimeapply"
	"github.com/small-frappuccino/discordcore/pkg/storage"
	"golang.org/x/oauth2"
)

var ErrControlServerBind = errors.New("control server bind failed")

type BotGuildBinding struct {
	GuildID       string
	BotInstanceID string
}

type DiscordOAuthConfig struct {
	ClientID                 string
	ClientSecret             string
	RedirectURI              string
	IncludeGuildsMembersRead bool
	SessionStorePath         string
	Scopes                   []string
}

type Server struct {
	bindAddr       string
	configManager  *files.ConfigManager
	runtimeApplier *runtimeapply.Manager

	bearerToken               string
	knownBotInstanceIDs       []string
	qotdService               any
	moderationMetrics         interface{} // Use specific type if known
	membersMetricsResolver    func() members.Metrics
	messagesMetricsResolver   func() messages.Metrics
	store                     *storage.Store
	cacheObservability        func() *cache.UnifiedCache
	arikawaStateResolver      func(guildID string) (*state.State, error)
	botGuildBindingsProvider  func(ctx context.Context) ([]BotGuildBinding, error)
	guildRegistrationResolver func(ctx context.Context, guildID string) error

	publicOrigin string
	tlsCertFile  string
	tlsKeyFile   string
	oauthConfig  *oauth2.Config

	httpServer *http.Server
	mu         sync.RWMutex
}

func NewServer(addr string, configManager *files.ConfigManager, runtimeApplier *runtimeapply.Manager) *Server {
	if addr == "" {
		return nil
	}
	return &Server{
		bindAddr:       addr,
		configManager:  configManager,
		runtimeApplier: runtimeApplier,
	}
}

func (s *Server) SetBearerToken(token string)              { s.bearerToken = token }
func (s *Server) SetKnownBotInstanceIDs(ids []string)      { s.knownBotInstanceIDs = ids }
func (s *Server) SetQOTDService(svc any)                   { s.qotdService = svc }
func (s *Server) SetModerationMetrics(metrics interface{}) { s.moderationMetrics = metrics }
func (s *Server) SetMembersMetricsResolver(resolver func() members.Metrics) {
	s.membersMetricsResolver = resolver
}
func (s *Server) SetMessagesMetricsResolver(resolver func() messages.Metrics) {
	s.messagesMetricsResolver = resolver
}
func (s *Server) SetStorage(store *storage.Store) { s.store = store }
func (s *Server) SetCacheObservability(resolver func() *cache.UnifiedCache, store *storage.Store) {
	s.cacheObservability = resolver
}
func (s *Server) SetArikawaStateResolver(resolver func(guildID string) (*state.State, error)) {
	s.arikawaStateResolver = resolver
}
func (s *Server) SetBotGuildBindingsProvider(provider func(ctx context.Context) ([]BotGuildBinding, error)) {
	s.botGuildBindingsProvider = provider
}
func (s *Server) SetGuildRegistrationResolver(resolver func(ctx context.Context, guildID string) error) {
	s.guildRegistrationResolver = resolver
}

func (s *Server) SetPublicOrigin(origin string) error {
	s.publicOrigin = origin
	return nil
}

func (s *Server) SetTLSCertificates(certFile, keyFile string) error {
	s.tlsCertFile = certFile
	s.tlsKeyFile = keyFile
	return nil
}

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

func (s *Server) Start() error {
	slog.Info("Architectural state transition: Initializing primary HTTP control plane", slog.String("bind_addr", s.bindAddr))

	mux := http.NewServeMux()
	s.registerRoutes(mux)

	s.httpServer = &http.Server{
		Addr:    s.bindAddr,
		Handler: mux,
	}

	go func() {
		var err error
		if s.tlsCertFile != "" && s.tlsKeyFile != "" {
			err = s.httpServer.ListenAndServeTLS(s.tlsCertFile, s.tlsKeyFile)
		} else {
			err = s.httpServer.ListenAndServe()
		}
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.EmitBlockingError("http server failure", err, "")
		}
	}()
	return nil
}

func (s *Server) Stop(ctx context.Context) error {
	if s.httpServer == nil {
		slog.Debug("Granular inspection: Stop invoked on uninitialized HTTP control plane")
		return nil
	}

	slog.Info("Architectural state transition: Commencing graceful shutdown of HTTP control plane")

	// Enforce 5s limit
	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return s.httpServer.Shutdown(shutdownCtx)
}

func (s *Server) BroadcastGuildEvent(guildID string, botPresent bool) {}
