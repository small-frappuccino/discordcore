package control

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/discord/cache"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/moderation"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/monitoring"
	"github.com/small-frappuccino/discordcore/pkg/qotd"
	"github.com/small-frappuccino/discordcore/pkg/runtimeapply"
	"github.com/small-frappuccino/discordcore/pkg/storage"
	"github.com/small-frappuccino/discordgo"
)

// CacheSnapshotResolver returns the primary UnifiedCache the control server
// should snapshot for /v1/health/cache. The resolver is called per request so
// it sees whichever cache the runtime layer has installed; returning nil keeps
// the route in its 503 "cache observability not wired" state without panic.
type CacheSnapshotResolver func() *cache.UnifiedCache

// MonitoringMetricsResolver returns the monitoring observability sink the
// control server should snapshot for /v1/health/monitoring. Mirrors
// CacheSnapshotResolver — monitoring is constructed per bot runtime, so the
// resolver is called per request and may return nil while the runtime is
// still booting; the route surfaces 503 in that window.
type MonitoringMetricsResolver func() monitoring.Metrics

const (
	defaultMaxBodyBytes          = 64 * 1024
	defaultSyncTimeout           = 20 * time.Second
	defaultAccessibleGuildsQuery = 20 * time.Second
	defaultAccessibleGuildsCache = 45 * time.Second
)

// ErrControlServerBind defines err control server bind.
var ErrControlServerBind = errors.New("control server bind failed")

type botGuildIDsProvider func(context.Context) ([]string, error)
type botGuildBindingsProvider func(context.Context) ([]BotGuildBinding, error)
type guildRegistrationFunc func(context.Context, string) error
type discordSessionResolver func(string) (*discordgo.Session, error)

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

// healthSources holds the observability sinks the /v1/health/* routes read.
// Each is wired post-construction via the Server.Set* methods and may be nil
// while the corresponding subsystem is still booting, in which case the route
// surfaces 503 rather than panicking.
type healthSources struct {
	moderationMetrics        moderation.Metrics
	cacheSnapshotResolve     CacheSnapshotResolver
	cacheSnapshotStore       *storage.Store
	monitoringMetricsResolve MonitoringMetricsResolver
}

// Server exposes operational controls for a running Discordcore instance.
type Server struct {
	addr                 string
	startedAt            time.Time
	authBearerToken      string
	tlsCertFile          string
	tlsKeyFile           string
	configManager        *files.ConfigManager
	knownBotInstanceIDs  []string
	qotdService          *qotd.Service
	health               healthSources
	guildRegistration    guildRegistrationFunc
	discordSession       discordSessionResolver
	discordOAuth         *discordOAuthProvider
	publicOrigin         controlPublicOrigin
	runtimeApplier       *runtimeapply.Manager
	botGuildSource       *botGuildBindingSource
	accessibleGuildCache *accessibleGuildCache
	featureControlSvc    *featureControlService
	discordOAuthSvc      *discordOAuthControlService
	httpServer           *http.Server
	listener             net.Listener
	guildEventBroker     *guildEventBroker
	logger               *slog.Logger
	storage              *storage.Store
}

// NewServer returns nil if addr is empty.
func NewServer(addr string, configManager *files.ConfigManager, runtimeApplier *runtimeapply.Manager) *Server {
	addr = strings.TrimSpace(addr)
	if addr == "" || configManager == nil {
		return nil
	}

	mux := http.NewServeMux()
	s := &Server{
		addr:           addr,
		startedAt:      time.Now().UTC(),
		configManager:  configManager,
		runtimeApplier: runtimeApplier,
	}
	discordSessions := func(guildID string) (*discordgo.Session, error) {
		return s.discordSessionForGuild(guildID)
	}
	providerSource := func() *discordOAuthProvider {
		return s.discordOAuth
	}
	s.botGuildSource = newBotGuildBindingSource()
	s.accessibleGuildCache = newAccessibleGuildCache(defaultAccessibleGuildsCache, time.Now)
	guildAccessEvaluator := newGuildAccessEvaluator(configManager, discordSessions)
	guildAccessResolver := newAccessibleGuildResolver(providerSource, s.botGuildSource, s.accessibleGuildCache, guildAccessEvaluator)
	featureBuilder := newFeatureWorkspaceBuilder(configManager, discordSessions)
	featureApplier := newFeatureMutationApplier(configManager)
	s.featureControlSvc = newFeatureControlService(featureBuilder, featureApplier)
	s.discordOAuthSvc = newDiscordOAuthControlService(
		providerSource,
		guildAccessResolver,
		s.publicDashboardURL,
		s.publicDiscordOAuthLoginURL,
		func() *slog.Logger { return s.log() },
	)
	s.httpServer = &http.Server{
		Addr:              addr,
		Handler:           s.wrapCanonicalPublicOrigin(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}
	s.guildEventBroker = newGuildEventBroker()
	s.registerHTTPRoutes(mux)

	return s
}

// SetLogger injects the slog.Logger.
func (s *Server) SetLogger(logger *slog.Logger) {
	if s == nil {
		return
	}
	s.logger = logger
}

func (s *Server) log() *slog.Logger {
	if s != nil && s.logger != nil {
		return s.logger
	}
	return slog.Default()
}

// SetQOTDService overrides the QOTD application service used by QOTD routes.
func (s *Server) SetQOTDService(service *qotd.Service) {
	if s == nil || service == nil {
		return
	}
	s.qotdService = service
}

// SetModerationMetrics injects the moderation observability sink so the
// /v1/health/moderation route can snapshot it. Mirrors SetQOTDService: nil
// is a no-op, so callers that pass an unwired metrics value leave the
// route in its 503 "not enabled" state instead of panicking.
func (s *Server) SetModerationMetrics(metrics moderation.Metrics) {
	if s == nil || metrics == nil {
		return
	}
	s.health.moderationMetrics = metrics
}

// SetCacheObservability wires the inputs /v1/health/cache scrapes. resolver is
// late-binding because UnifiedCache is constructed per bot runtime, after the
// control server has already started; passing a resolver lets the route see
// the runtime's cache as soon as it exists. store may be nil; without it the
// route still serves in-memory segment counters but the Persisted field stays
// zero. Both nil leaves the route returning 503.
func (s *Server) SetCacheObservability(resolver CacheSnapshotResolver, store *storage.Store) {
	if s == nil {
		return
	}
	s.health.cacheSnapshotResolve = resolver
	s.health.cacheSnapshotStore = store
}

// SetStorage injects the persistent Postgres store for users and metrics routes.
func (s *Server) SetStorage(store *storage.Store) {
	if s == nil || store == nil {
		return
	}
	s.storage = store
}

// SetMonitoringMetricsResolver wires the late-binding accessor /v1/health/monitoring
// uses to obtain the monitoring service's Metrics. Late binding because the
// monitoring service is built per bot runtime — the control server boots
// before any runtime publishes a Metrics value. The resolver may return nil
// (no runtime ready) or NopMetrics (runtime ready but observability disabled);
// the route distinguishes both as 503 with different bodies so operators can
// tell them apart.
func (s *Server) SetMonitoringMetricsResolver(resolver MonitoringMetricsResolver) {
	if s == nil {
		return
	}
	s.health.monitoringMetricsResolve = resolver
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
	s.discordSession = func(string) (*discordgo.Session, error) {
		return provider(), nil
	}
}

// SetDiscordSessionResolver exposes a guild-aware Discord session resolver for
// readiness inspection.
func (s *Server) SetDiscordSessionResolver(resolver func(string) (*discordgo.Session, error)) {
	if s == nil || resolver == nil {
		return
	}
	s.discordSession = func(guildID string) (*discordgo.Session, error) {
		return resolver(guildID)
	}
}

// SetGuildRegistrationFunc configures Discord-aware guild bootstrap used by
// the settings registry endpoints.
func (s *Server) SetGuildRegistrationFunc(fn func(context.Context, string) error) {
	if s == nil || fn == nil {
		return
	}
	s.guildRegistration = func(ctx context.Context, guildID string) error {
		return fn(ctx, guildID)
	}
}

func (s *Server) SetGuildRegistrationResolver(fn func(context.Context, string) error) {
	if s == nil || fn == nil {
		return
	}
	s.guildRegistration = fn
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
	s.log().LogAttrs(context.Background(), slog.LevelInfo, "Control server listening", slog.String("addr", listenAddr), slog.Bool("tls", tlsEnabled))
	if dashboardURL != "" {
		s.log().LogAttrs(context.Background(), slog.LevelInfo, "Control dashboard available", slog.String("url", dashboardURL))
	}

	go func() {
		var err error
		if tlsEnabled {
			err = s.httpServer.ServeTLS(ln, s.tlsCertFile, s.tlsKeyFile)
		} else {
			err = s.httpServer.Serve(ln)
		}
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.log().LogAttrs(context.Background(), slog.LevelError, "Control server stopped unexpectedly", slog.Any("err", err))
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

	s.log().LogAttrs(ctx, slog.LevelInfo, "Control server stopped", slog.String("addr", s.addr))
	return nil
}

// serveHealthRoute consolidates the auth, method-check, JSON-header, and
// encode boilerplate every /v1/health/* snapshot route shares. resolver
// returns the snapshot to encode for a 200 response, or a non-empty reason
// string for 503 Service Unavailable with that body so each subsystem keeps
// its own distinct "wired but inactive" vs "service unavailable" message
// without reimplementing the surrounding auth/method/header policy.
type healthSnapshotResolver interface {
	resolve() (snapshot any, unavailable string)
}

func serveHealthRoute[T healthSnapshotResolver](s *Server, w http.ResponseWriter, r *http.Request, resolver T) {
	auth, ok := s.authorizeRequest(w, r)
	if !ok {
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.authorizeGlobalControlAccess(w, r, auth, guildAccessLevelRead) {
		return
	}

	snapshot, unavailable := resolver.resolve()
	if unavailable != "" {
		http.Error(w, unavailable, http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	// Response status header is already in flight; nothing recoverable.
	if err := json.NewEncoder(w).Encode(snapshot); err != nil {
		s.log().LogAttrs(r.Context(), slog.LevelWarn, "Failed to encode health snapshot response", slog.Any("err", err))
	}
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
		http.Error(w, "invalid payload", http.StatusBadRequest)
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
	if err := json.NewEncoder(w).Encode(RuntimeConfigResponse{
		Status:        "ok",
		RuntimeConfig: updated,
	}); err != nil {
		s.log().LogAttrs(r.Context(), slog.LevelError, "Failed to encode runtime config response", slog.Any("err", err))
	}
}

func (s *Server) applyRuntimePatch(patch map[string]json.RawMessage) (files.RuntimeConfig, error) {
	updated, err := s.configManager.UpdateRuntimeConfig(func(rc *files.RuntimeConfig) error {
		for field, raw := range patch {
			setter, ok := runtimeConfigFieldSetters[field]
			if !ok {
				return badRequest(errUnknownField)
			}
			if err := setter(rc, raw); err != nil {
				return badRequest(errInvalidField)
			}
		}
		return nil
	})
	if err != nil {
		return files.RuntimeConfig{}, fmt.Errorf("Server.applyRuntimePatch: %w", err)
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
			return errNegativeValue
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

// Error errors.
func (e *httpError) Error() string { return e.err.Error() }

// Unwrap unwraps.
func (e *httpError) Unwrap() error { return e.err }

var (
	errEmptyStringValue = errors.New("empty string value")
	errEmptyBoolValue   = errors.New("empty bool value")
	errEmptyIntValue    = errors.New("empty int value")
	errNegativeValue    = errors.New("must be >= 0")
	errUnknownField     = errors.New("unknown field")
	errInvalidField     = errors.New("invalid field")
	errDecodeStringFail = errors.New("decode string failed")
	errDecodeBoolFail   = errors.New("decode bool failed")
	errDecodeIntFail    = errors.New("decode int failed")
)

func decodeString(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "", errEmptyStringValue
	}
	if bytes.Equal(raw, []byte("null")) {
		return "", nil
	}

	var v string
	if err := json.Unmarshal(raw, &v); err != nil {
		return "", errDecodeStringFail
	}
	return v, nil
}

func decodeBool(raw json.RawMessage) (bool, error) {
	if len(raw) == 0 {
		return false, errEmptyBoolValue
	}
	if bytes.Equal(raw, []byte("null")) {
		return false, nil
	}

	var v bool
	if err := json.Unmarshal(raw, &v); err != nil {
		return false, errDecodeBoolFail
	}
	return v, nil
}

func decodeInt(raw json.RawMessage) (int, error) {
	if len(raw) == 0 {
		return 0, errEmptyIntValue
	}
	if bytes.Equal(raw, []byte("null")) {
		return 0, nil
	}

	var n json.Number
	if err := json.Unmarshal(raw, &n); err != nil {
		return 0, errDecodeIntFail
	}
	if i, err := n.Int64(); err == nil {
		return int(i), nil
	}
	f, err := n.Float64()
	if err != nil {
		return 0, errDecodeIntFail
	}
	return int(f), nil
}

func boolPtr(v bool) *bool {
	return &v
}
