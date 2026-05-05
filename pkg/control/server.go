package control

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/partners"
	"github.com/small-frappuccino/discordcore/pkg/qotd"
	"github.com/small-frappuccino/discordcore/pkg/runtimeapply"
)

const (
	defaultMaxBodyBytes          = 64 * 1024
	defaultSyncTimeout           = 20 * time.Second
	defaultAccessibleGuildsQuery = 20 * time.Second
	defaultAccessibleGuildsCache = 45 * time.Second
)

var ErrControlServerBind = errors.New("control server bind failed")

type botGuildIDsProvider func(context.Context) ([]string, error)
type botGuildBindingsProvider func(context.Context) ([]BotGuildBinding, error)
type guildRegistrationFunc func(context.Context, string, string) error
type discordSessionResolver func(string) (*discordgo.Session, error)
type discordSessionDomainResolver func(string, string) (*discordgo.Session, error)

// BotGuildBinding associates a guild visible to the control plane with a bot
// instance identifier from the host runtime catalog.
type BotGuildBinding struct {
	GuildID       string
	BotInstanceID string
}

type requestAuthMode string

const (
	requestAuthModeBearer              requestAuthMode = "bearer"
	requestAuthModeDiscordOAuthSession requestAuthMode = "discord_oauth_session"
)

type requestAuthorization struct {
	mode         requestAuthMode
	oauthSession discordOAuthSession
}

// Server exposes operational controls for a running Discordcore instance.
type Server struct {
	addr                 string
	authBearerToken      string
	tlsCertFile          string
	tlsKeyFile           string
	configManager        *files.ConfigManager
	knownBotInstanceIDs  []string
	partnerBoardService  partners.BoardService
	partnerBoardSyncer   partners.GuildSyncExecutor
	qotdService          *qotd.Service
	guildRegistration    guildRegistrationFunc
	discordSession       discordSessionDomainResolver
	defaultBotInstanceID string
	discordOAuth         *discordOAuthProvider
	publicOrigin         controlPublicOrigin
	runtimeApplier       *runtimeapply.Manager
	botGuildSource       *botGuildBindingSource
	accessibleGuildCache *accessibleGuildCache
	guildAccessEvaluator *guildAccessEvaluator
	guildAccessResolver  *accessibleGuildResolver
	featureBuilder       *featureWorkspaceBuilder
	featureApplier       *featureMutationApplier
	featureControlSvc    *featureControlService
	discordOAuthSvc      *discordOAuthControlService
	httpServer           *http.Server
	listener             net.Listener
}

// NewServer returns nil if addr is empty.
func NewServer(addr string, configManager *files.ConfigManager, runtimeApplier *runtimeapply.Manager) *Server {
	addr = strings.TrimSpace(addr)
	if addr == "" || configManager == nil {
		return nil
	}

	mux := http.NewServeMux()
	s := &Server{
		addr:                addr,
		configManager:       configManager,
		partnerBoardService: partners.NewBoardApplicationService(configManager, nil),
		runtimeApplier:      runtimeApplier,
	}
	discordSessions := func(guildID string) (*discordgo.Session, error) {
		return s.discordSessionForGuild(guildID)
	}
	providerSource := func() *discordOAuthProvider {
		return s.discordOAuth
	}
	s.botGuildSource = newBotGuildBindingSource()
	s.accessibleGuildCache = newAccessibleGuildCache(defaultAccessibleGuildsCache, time.Now)
	s.guildAccessEvaluator = newGuildAccessEvaluator(configManager, discordSessions)
	s.guildAccessResolver = newAccessibleGuildResolver(providerSource, s.botGuildSource, s.accessibleGuildCache, s.guildAccessEvaluator)
	s.featureBuilder = newFeatureWorkspaceBuilder(configManager, discordSessions)
	s.featureApplier = newFeatureMutationApplier(configManager)
	s.featureControlSvc = newFeatureControlService(s.featureBuilder, s.featureApplier)
	s.discordOAuthSvc = newDiscordOAuthControlService(
		providerSource,
		s.guildAccessResolver,
		s.publicDashboardURL,
		s.publicDiscordOAuthLoginURL,
	)
	s.httpServer = &http.Server{
		Addr:              addr,
		Handler:           s.wrapCanonicalPublicOrigin(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}
	s.registerHTTPRoutes(mux)

	return s
}

// SetPartnerBoardService overrides the board application service used by partner-board routes.
func (s *Server) SetPartnerBoardService(service partners.BoardService) {
	if s == nil || service == nil {
		return
	}
	s.partnerBoardService = service
}

// SetPartnerBoardSyncExecutor overrides sync execution used by /partner-board/sync endpoint.
func (s *Server) SetPartnerBoardSyncExecutor(executor partners.GuildSyncExecutor) {
	if s == nil || executor == nil {
		return
	}
	s.partnerBoardSyncer = executor
}

// SetQOTDService overrides the QOTD application service used by QOTD routes.
func (s *Server) SetQOTDService(service *qotd.Service) {
	if s == nil || service == nil {
		return
	}
	s.qotdService = service
}

// SetBearerToken configures bearer token authentication for control routes.
func (s *Server) SetBearerToken(token string) {
	if s == nil {
		return
	}
	s.authBearerToken = strings.TrimSpace(token)
}

// SetTLSCertificates configures optional TLS for control server listeners.
func (s *Server) SetTLSCertificates(certFile, keyFile string) error {
	if s == nil {
		return nil
	}

	certFile = strings.TrimSpace(certFile)
	keyFile = strings.TrimSpace(keyFile)
	if certFile == "" && keyFile == "" {
		s.tlsCertFile = ""
		s.tlsKeyFile = ""
		return nil
	}
	if certFile == "" || keyFile == "" {
		return fmt.Errorf("configure control tls: both cert and key files are required")
	}

	s.tlsCertFile = certFile
	s.tlsKeyFile = keyFile
	return nil
}

// SetBotGuildIDsProvider sets a provider used by OAuth manageable-guild endpoints.
func (s *Server) SetBotGuildIDsProvider(provider func(context.Context) ([]string, error)) {
	if s == nil || provider == nil {
		return
	}
	if s.botGuildSource != nil {
		s.botGuildSource.SetIDsProvider(provider)
	}
}

// SetBotGuildBindingsProvider sets a guild -> bot binding provider used by the
// settings registry and OAuth manageable-guild endpoints.
func (s *Server) SetBotGuildBindingsProvider(provider func(context.Context) ([]BotGuildBinding, error)) {
	if s == nil || provider == nil {
		return
	}
	if s.botGuildSource != nil {
		s.botGuildSource.SetBindingsProvider(provider)
	}
}

// SetKnownBotInstanceIDs configures bot instance identifiers that may be used
// for domain-level routing overrides even when this process does not host their
// Discord session locally.
func (s *Server) SetKnownBotInstanceIDs(ids []string) {
	if s == nil {
		return
	}
	s.knownBotInstanceIDs = normalizeBotInstanceIDs(ids)
}

// SetDiscordSessionProvider exposes a fallback Discord session for readiness inspection.
func (s *Server) SetDiscordSessionProvider(provider func() *discordgo.Session) {
	if s == nil || provider == nil {
		return
	}
	s.discordSession = func(string, string) (*discordgo.Session, error) {
		return provider(), nil
	}
}

// SetDiscordSessionResolver exposes a guild-aware Discord session resolver for
// readiness inspection.
func (s *Server) SetDiscordSessionResolver(resolver func(string) (*discordgo.Session, error)) {
	if s == nil || resolver == nil {
		return
	}
	s.discordSession = func(guildID, _ string) (*discordgo.Session, error) {
		return resolver(guildID)
	}
}

// SetDiscordSessionResolverForDomain exposes a guild+domain-aware Discord
// session resolver for control routes that need specialized bot ownership.
func (s *Server) SetDiscordSessionResolverForDomain(resolver func(string, string) (*discordgo.Session, error)) {
	if s == nil || resolver == nil {
		return
	}
	s.discordSession = resolver
}

// SetGuildRegistrationFunc configures Discord-aware guild bootstrap used by
// the settings registry endpoints.
func (s *Server) SetGuildRegistrationFunc(fn func(context.Context, string) error) {
	if s == nil || fn == nil {
		return
	}
	s.guildRegistration = func(ctx context.Context, guildID, _ string) error {
		return fn(ctx, guildID)
	}
}

// SetGuildRegistrationResolver configures Discord-aware guild bootstrap with an
// explicit bot instance choice.
func (s *Server) SetGuildRegistrationResolver(fn func(context.Context, string, string) error) {
	if s == nil || fn == nil {
		return
	}
	s.guildRegistration = fn
}

// SetDefaultBotInstanceID configures the fallback bot instance used for legacy
// guild configs that do not yet have an explicit binding.
func (s *Server) SetDefaultBotInstanceID(botInstanceID string) {
	if s == nil {
		return
	}
	s.defaultBotInstanceID = strings.TrimSpace(botInstanceID)
}

// Start opens the control server listening socket.
func (s *Server) Start() error {
	if s == nil {
		return nil
	}
	if (s.tlsCertFile == "") != (s.tlsKeyFile == "") {
		return fmt.Errorf("start control server: both TLS cert and key files are required")
	}
	tlsEnabled := s.tlsCertFile != "" && s.tlsKeyFile != ""

	ln, err := net.Listen("tcp", s.httpServer.Addr)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrControlServerBind, err)
	}
	s.listener = ln

	listenAddr, dashboardURL := controlServerListenAddrAndDashboardURL(ln.Addr(), tlsEnabled, s.publicControlOrigin())
	log.ApplicationLogger().Info("Control server listening", "addr", listenAddr, "tls", tlsEnabled)
	if dashboardURL != "" {
		log.ApplicationLogger().Info("Control dashboard available", "url", dashboardURL)
	}

	go func() {
		var err error
		if tlsEnabled {
			err = s.httpServer.ServeTLS(ln, s.tlsCertFile, s.tlsKeyFile)
		} else {
			err = s.httpServer.Serve(ln)
		}
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.ApplicationLogger().Error("Control server stopped unexpectedly", "err", err)
		}
	}()

	return nil
}

func controlServerListenAddrAndDashboardURL(addr net.Addr, tlsEnabled bool, publicOrigin controlPublicOrigin) (string, string) {
	if addr == nil {
		return "", ""
	}

	listenAddr := strings.TrimSpace(addr.String())
	if listenAddr == "" {
		return "", ""
	}

	host, port, err := net.SplitHostPort(listenAddr)
	if err != nil {
		return listenAddr, ""
	}

	scheme := "http"
	if tlsEnabled {
		scheme = "https"
	}

	if publicOrigin.valid() {
		return listenAddr, publicOrigin.resolve("/")
	}
	return listenAddr, fmt.Sprintf("%s://%s", scheme, net.JoinHostPort(controlServerBrowserHost(host), port))
}

func (s *Server) wrapCanonicalPublicOrigin(next http.Handler) http.Handler {
	if next == nil {
		next = http.NotFoundHandler()
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if target, ok := s.canonicalPublicRedirectURL(r); ok {
			http.Redirect(w, r, target, http.StatusFound)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func controlServerBrowserHost(host string) string {
	trimmedHost := strings.TrimSpace(strings.TrimPrefix(strings.TrimSuffix(host, "]"), "["))
	if trimmedHost == "" {
		return "127.0.0.1"
	}

	ip := net.ParseIP(trimmedHost)
	if ip == nil {
		return trimmedHost
	}
	if ip.IsUnspecified() {
		if ip.To4() != nil {
			return "127.0.0.1"
		}
		return "::1"
	}

	return trimmedHost
}

// Stop shuts down the control server.
func (s *Server) Stop(ctx context.Context) error {
	if s == nil || s.httpServer == nil {
		return nil
	}

	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := s.httpServer.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("shutdown control server: %w", err)
	}

	log.ApplicationLogger().Info("Control server stopped", "addr", s.addr)
	return nil
}

func (s *Server) handleRuntimeConfig(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.authorizeRequest(w, r)
	if !ok {
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !s.authorizeGlobalControlAccess(w, r, auth, guildAccessLevelWrite) {
		return
	}

	if s.configManager == nil {
		http.Error(w, "config manager unavailable", http.StatusInternalServerError)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, defaultMaxBodyBytes)
	defer r.Body.Close()

	var patch map[string]json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		http.Error(w, fmt.Sprintf("invalid payload: %v", err), http.StatusBadRequest)
		return
	}
	if len(patch) == 0 {
		http.Error(w, "payload must contain at least one field", http.StatusBadRequest)
		return
	}

	updated, err := s.applyRuntimePatch(patch)
	if err != nil {
		status := http.StatusInternalServerError
		var httpErr *httpError
		if errors.As(err, &httpErr) {
			status = httpErr.code
		}
		http.Error(w, fmt.Sprintf("failed to apply runtime config: %v", err), status)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{
		"status":         "ok",
		"runtime_config": updated,
	}); err != nil {
		log.ApplicationLogger().Error("Failed to encode runtime config response", "err", err)
	}
}

func (s *Server) applyRuntimePatch(patch map[string]json.RawMessage) (files.RuntimeConfig, error) {
	updated, err := s.configManager.UpdateRuntimeConfig(func(rc *files.RuntimeConfig) error {
		for field, raw := range patch {
			setter, ok := runtimeConfigFieldSetters[field]
			if !ok {
				return badRequest(fmt.Errorf("unknown field %q", field))
			}
			if err := setter(rc, raw); err != nil {
				return badRequest(fmt.Errorf("field %s: %w", field, err))
			}
		}
		return nil
	})
	if err != nil {
		return files.RuntimeConfig{}, err
	}

	if s.runtimeApplier != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.runtimeApplier.Apply(ctx, updated); err != nil {
			return updated, fmt.Errorf("runtime apply: %w", err)
		}
	}

	return updated, nil
}

type setterFunc func(*files.RuntimeConfig, json.RawMessage) error

var runtimeConfigFieldSetters = map[string]setterFunc{
	"bot_theme":               stringSetter(func(rc *files.RuntimeConfig, v string) { rc.BotTheme = v }),
	"disable_db_cleanup":      boolSetter(func(rc *files.RuntimeConfig, v bool) { rc.DisableDBCleanup = v }),
	"disable_automod_logs":    boolSetter(func(rc *files.RuntimeConfig, v bool) { rc.DisableAutomodLogs = v }),
	"disable_message_logs":    boolSetter(func(rc *files.RuntimeConfig, v bool) { rc.DisableMessageLogs = v }),
	"disable_entry_exit_logs": boolSetter(func(rc *files.RuntimeConfig, v bool) { rc.DisableEntryExitLogs = v }),
	"disable_reaction_logs":   boolSetter(func(rc *files.RuntimeConfig, v bool) { rc.DisableReactionLogs = v }),
	"disable_user_logs":       boolSetter(func(rc *files.RuntimeConfig, v bool) { rc.DisableUserLogs = v }),
	"disable_clean_log":       boolSetter(func(rc *files.RuntimeConfig, v bool) { rc.DisableCleanLog = v }),
	"moderation_logging":      boolSetter(func(rc *files.RuntimeConfig, v bool) { rc.ModerationLogging = boolPtr(v) }),
	// Deprecated (legacy). Accepted for backward compatibility; converted to moderation_logging.
	"moderation_log_mode": stringSetter(func(rc *files.RuntimeConfig, v string) {
		rc.ModerationLogging = boolPtr(strings.ToLower(strings.TrimSpace(v)) != "off")
		rc.ModerationLogMode = v
	}),
	"presence_watch_user_id":  stringSetter(func(rc *files.RuntimeConfig, v string) { rc.PresenceWatchUserID = v }),
	"presence_watch_bot":      boolSetter(func(rc *files.RuntimeConfig, v bool) { rc.PresenceWatchBot = v }),
	"message_cache_ttl_hours": intSetter(func(rc *files.RuntimeConfig, v int) { rc.MessageCacheTTLHours = v }),
	"message_delete_on_log":   boolSetter(func(rc *files.RuntimeConfig, v bool) { rc.MessageDeleteOnLog = v }),
	"message_cache_cleanup":   boolSetter(func(rc *files.RuntimeConfig, v bool) { rc.MessageCacheCleanup = v }),
	"global_max_workers": nonNegativeIntSetter(
		"global_max_workers",
		func(rc *files.RuntimeConfig, v int) { rc.GlobalMaxWorkers = v },
	),
	"backfill_channel_id":                stringSetter(func(rc *files.RuntimeConfig, v string) { rc.BackfillChannelID = v }),
	"backfill_start_day":                 stringSetter(func(rc *files.RuntimeConfig, v string) { rc.BackfillStartDay = v }),
	"backfill_initial_date":              stringSetter(func(rc *files.RuntimeConfig, v string) { rc.BackfillInitialDate = v }),
	"disable_bot_role_perm_mirror":       boolSetter(func(rc *files.RuntimeConfig, v bool) { rc.DisableBotRolePermMirror = v }),
	"bot_role_perm_mirror_actor_role_id": stringSetter(func(rc *files.RuntimeConfig, v string) { rc.BotRolePermMirrorActorRoleID = v }),
}

func stringSetter(assign func(*files.RuntimeConfig, string)) setterFunc {
	return func(rc *files.RuntimeConfig, raw json.RawMessage) error {
		v, err := decodeString(raw)
		if err != nil {
			return err
		}
		assign(rc, v)
		return nil
	}
}

func boolSetter(assign func(*files.RuntimeConfig, bool)) setterFunc {
	return func(rc *files.RuntimeConfig, raw json.RawMessage) error {
		v, err := decodeBool(raw)
		if err != nil {
			return err
		}
		assign(rc, v)
		return nil
	}
}

func intSetter(assign func(*files.RuntimeConfig, int)) setterFunc {
	return func(rc *files.RuntimeConfig, raw json.RawMessage) error {
		v, err := decodeInt(raw)
		if err != nil {
			return err
		}
		assign(rc, v)
		return nil
	}
}

func nonNegativeIntSetter(field string, assign func(*files.RuntimeConfig, int)) setterFunc {
	return func(rc *files.RuntimeConfig, raw json.RawMessage) error {
		v, err := decodeInt(raw)
		if err != nil {
			return err
		}
		if v < 0 {
			return fmt.Errorf("%s must be >= 0", field)
		}
		assign(rc, v)
		return nil
	}
}

func badRequest(err error) error {
	return &httpError{
		code: http.StatusBadRequest,
		err:  err,
	}
}

type httpError struct {
	code int
	err  error
}

func (e *httpError) Error() string { return e.err.Error() }
func (e *httpError) Unwrap() error { return e.err }

func decodeString(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "", fmt.Errorf("empty string value")
	}
	if bytes.Equal(raw, []byte("null")) {
		return "", nil
	}

	var v string
	if err := json.Unmarshal(raw, &v); err != nil {
		return "", err
	}
	return v, nil
}

func decodeBool(raw json.RawMessage) (bool, error) {
	if len(raw) == 0 {
		return false, fmt.Errorf("empty bool value")
	}
	if bytes.Equal(raw, []byte("null")) {
		return false, nil
	}

	var v bool
	if err := json.Unmarshal(raw, &v); err != nil {
		return false, err
	}
	return v, nil
}

func decodeInt(raw json.RawMessage) (int, error) {
	if len(raw) == 0 {
		return 0, fmt.Errorf("empty int value")
	}
	if bytes.Equal(raw, []byte("null")) {
		return 0, nil
	}

	var n json.Number
	if err := json.Unmarshal(raw, &n); err != nil {
		return 0, err
	}
	if i, err := n.Int64(); err == nil {
		return int(i), nil
	}
	f, err := n.Float64()
	if err != nil {
		return 0, err
	}
	return int(f), nil
}

func boolPtr(v bool) *bool {
	return &v
}
