package control

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/discord/messageupdate"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/partners"
	"github.com/small-frappuccino/discordcore/pkg/runtimeapply"
)

const (
	defaultMaxBodyBytes          = 64 * 1024
	defaultSyncTimeout           = 20 * time.Second
	defaultManageableGuildsQuery = 20 * time.Second
)

var ErrControlServerBind = errors.New("control server bind failed")

type botGuildIDsProvider func(context.Context) ([]string, error)
type guildRegistrationFunc func(context.Context, string) error

type requestAuthMode string

const (
	requestAuthModeUnknown             requestAuthMode = ""
	requestAuthModeBearer              requestAuthMode = "bearer"
	requestAuthModeDiscordOAuthSession requestAuthMode = "discord_oauth_session"
)

type requestAuthorization struct {
	mode         requestAuthMode
	oauthSession discordOAuthSession
}

// Server exposes operational controls for a running Discordcore instance.
type Server struct {
	addr                string
	authBearerToken     string
	tlsCertFile         string
	tlsKeyFile          string
	configManager       *files.ConfigManager
	partnerBoardService partners.BoardService
	partnerBoardSyncer  partners.GuildSyncExecutor
	botGuildIDsProvider botGuildIDsProvider
	guildRegistration   guildRegistrationFunc
	discordOAuth        *discordOAuthProvider
	runtimeApplier      *runtimeapply.Manager
	httpServer          *http.Server
	listener            net.Listener
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
	s.httpServer = &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	mux.HandleFunc("/auth/discord/login", s.handleDiscordOAuthLogin)
	mux.HandleFunc("/auth/discord/callback", s.handleDiscordOAuthCallback)
	mux.HandleFunc("/auth/me", s.handleDiscordOAuthMe)
	mux.HandleFunc("/auth/logout", s.handleDiscordOAuthLogout)
	mux.HandleFunc("/auth/guilds/manageable", s.handleDiscordOAuthManageableGuilds)
	mux.HandleFunc("/v1/settings", s.handleSettingsRoutes)
	mux.HandleFunc("/v1/settings/", s.handleSettingsRoutes)
	mux.HandleFunc("/v1/runtime-config", s.handleRuntimeConfig)
	mux.HandleFunc("/v1/guilds/", s.handleGuildConfigRoutes)
	mux.Handle("/", newLandingHandler())
	mux.HandleFunc("/dashboard", func(w http.ResponseWriter, r *http.Request) {
		if !s.hasAuthenticatedDashboardSession(r) {
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}
		http.Redirect(w, r, dashboardRoutePrefix, http.StatusFound)
	})
	mux.Handle(dashboardRoutePrefix, newProtectedEmbeddedDashboardHandler(s))

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
	s.botGuildIDsProvider = provider
}

// SetGuildRegistrationFunc configures Discord-aware guild bootstrap used by
// the settings registry endpoints.
func (s *Server) SetGuildRegistrationFunc(fn func(context.Context, string) error) {
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

	listenAddr, dashboardURL := controlServerListenAddrAndDashboardURL(ln.Addr(), tlsEnabled)
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

func controlServerListenAddrAndDashboardURL(addr net.Addr, tlsEnabled bool) (string, string) {
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

	return listenAddr, fmt.Sprintf("%s://%s", scheme, net.JoinHostPort(controlServerBrowserHost(host), port))
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
	if _, ok := s.authorizeRequest(w, r); !ok {
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
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
	"presence_watch_user_id":             stringSetter(func(rc *files.RuntimeConfig, v string) { rc.PresenceWatchUserID = v }),
	"presence_watch_bot":                 boolSetter(func(rc *files.RuntimeConfig, v bool) { rc.PresenceWatchBot = v }),
	"message_cache_ttl_hours":            intSetter(func(rc *files.RuntimeConfig, v int) { rc.MessageCacheTTLHours = v }),
	"message_delete_on_log":              boolSetter(func(rc *files.RuntimeConfig, v bool) { rc.MessageDeleteOnLog = v }),
	"message_cache_cleanup":              boolSetter(func(rc *files.RuntimeConfig, v bool) { rc.MessageCacheCleanup = v }),
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

func (s *Server) handleGuildConfigRoutes(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.authorizeRequest(w, r)
	if !ok {
		return
	}

	if s.configManager == nil {
		http.Error(w, "config manager unavailable", http.StatusInternalServerError)
		return
	}
	if s.partnerBoardService == nil {
		http.Error(w, "partner board service unavailable", http.StatusInternalServerError)
		return
	}

	guildID, tail, ok := splitGuildRoute(r.URL.Path)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if guildID == "" {
		http.Error(w, "guild_id is required", http.StatusBadRequest)
		return
	}
	if !s.authorizeGuildAccess(w, r, auth, guildID) {
		return
	}

	switch {
	case len(tail) == 1 && tail[0] == "settings":
		switch r.Method {
		case http.MethodGet:
			s.handleGuildSettingsGet(w, r, guildID)
		case http.MethodPut:
			s.handleGuildSettingsPut(w, r, guildID)
		case http.MethodDelete:
			s.handleGuildSettingsDelete(w, r, guildID)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
		return
	case len(tail) == 1 && tail[0] == "partner-board":
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handlePartnerBoardGet(w, r, guildID)
		return
	case len(tail) == 2 && tail[0] == "partner-board" && tail[1] == "target":
		switch r.Method {
		case http.MethodGet:
			s.handlePartnerBoardTargetGet(w, r, guildID)
		case http.MethodPut:
			s.handlePartnerBoardTargetPut(w, r, guildID)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
		return
	case len(tail) == 2 && tail[0] == "partner-board" && tail[1] == "template":
		switch r.Method {
		case http.MethodGet:
			s.handlePartnerBoardTemplateGet(w, r, guildID)
		case http.MethodPut:
			s.handlePartnerBoardTemplatePut(w, r, guildID)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
		return
	case len(tail) == 2 && tail[0] == "partner-board" && tail[1] == "partners":
		switch r.Method {
		case http.MethodGet:
			s.handlePartnerBoardPartnersList(w, r, guildID)
		case http.MethodPost:
			s.handlePartnerBoardPartnersCreate(w, r, guildID)
		case http.MethodPut:
			s.handlePartnerBoardPartnersUpdate(w, r, guildID)
		case http.MethodDelete:
			s.handlePartnerBoardPartnersDelete(w, r, guildID)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
		return
	case len(tail) == 2 && tail[0] == "partner-board" && tail[1] == "sync":
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handlePartnerBoardSyncPost(w, r, guildID)
		return
	default:
		http.NotFound(w, r)
		return
	}
}

func (s *Server) handlePartnerBoardGet(w http.ResponseWriter, r *http.Request, guildID string) {
	board, err := s.partnerBoardService.GetPartnerBoard(guildID)
	if err != nil {
		status := partnerBoardErrorStatus(err)
		http.Error(w, fmt.Sprintf("failed to read partner board: %v", err), status)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":        "ok",
		"guild_id":      guildID,
		"partner_board": board,
	})
}

func (s *Server) handlePartnerBoardTargetGet(w http.ResponseWriter, r *http.Request, guildID string) {
	target, err := s.partnerBoardService.GetPartnerBoardTarget(guildID)
	if err != nil {
		status := partnerBoardErrorStatus(err)
		http.Error(w, fmt.Sprintf("failed to read partner board target: %v", err), status)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"guild_id": guildID,
		"target":   target,
	})
}

func (s *Server) handlePartnerBoardTargetPut(w http.ResponseWriter, r *http.Request, guildID string) {
	var payload files.EmbedUpdateTargetConfig
	if err := decodeJSONBody(w, r, &payload); err != nil {
		return
	}

	if err := s.partnerBoardService.SetPartnerBoardTarget(guildID, payload); err != nil {
		status := partnerBoardErrorStatus(err)
		http.Error(w, fmt.Sprintf("failed to set partner board target: %v", err), status)
		return
	}

	target, err := s.partnerBoardService.GetPartnerBoardTarget(guildID)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to read updated target: %v", err), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"guild_id": guildID,
		"target":   target,
	})
}

func (s *Server) handlePartnerBoardTemplateGet(w http.ResponseWriter, r *http.Request, guildID string) {
	template, err := s.partnerBoardService.GetPartnerBoardTemplate(guildID)
	if err != nil {
		status := partnerBoardErrorStatus(err)
		http.Error(w, fmt.Sprintf("failed to read partner board template: %v", err), status)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"guild_id": guildID,
		"template": template,
	})
}

func (s *Server) handlePartnerBoardTemplatePut(w http.ResponseWriter, r *http.Request, guildID string) {
	var payload files.PartnerBoardTemplateConfig
	if err := decodeJSONBody(w, r, &payload); err != nil {
		return
	}

	if err := s.partnerBoardService.SetPartnerBoardTemplate(guildID, payload); err != nil {
		status := partnerBoardErrorStatus(err)
		http.Error(w, fmt.Sprintf("failed to set partner board template: %v", err), status)
		return
	}

	template, err := s.partnerBoardService.GetPartnerBoardTemplate(guildID)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to read updated template: %v", err), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"guild_id": guildID,
		"template": template,
	})
}

func (s *Server) handlePartnerBoardPartnersList(w http.ResponseWriter, r *http.Request, guildID string) {
	partners, err := s.partnerBoardService.ListPartners(guildID)
	if err != nil {
		status := partnerBoardErrorStatus(err)
		http.Error(w, fmt.Sprintf("failed to list partners: %v", err), status)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"guild_id": guildID,
		"partners": partners,
	})
}

func (s *Server) handlePartnerBoardPartnersCreate(w http.ResponseWriter, r *http.Request, guildID string) {
	var payload files.PartnerEntryConfig
	if err := decodeJSONBody(w, r, &payload); err != nil {
		return
	}

	if err := s.partnerBoardService.CreatePartner(guildID, payload); err != nil {
		status := partnerBoardErrorStatus(err)
		if errors.Is(err, files.ErrPartnerAlreadyExists) {
			status = http.StatusConflict
		}
		http.Error(w, fmt.Sprintf("failed to create partner: %v", err), status)
		return
	}

	created, err := s.partnerBoardService.GetPartner(guildID, payload.Name)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to read created partner: %v", err), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"status":   "ok",
		"guild_id": guildID,
		"partner":  created,
	})
}

func (s *Server) handlePartnerBoardPartnersUpdate(w http.ResponseWriter, r *http.Request, guildID string) {
	var payload struct {
		CurrentName string                   `json:"current_name"`
		Partner     files.PartnerEntryConfig `json:"partner"`
	}
	if err := decodeJSONBody(w, r, &payload); err != nil {
		return
	}

	if strings.TrimSpace(payload.CurrentName) == "" {
		http.Error(w, "failed to update partner: current_name is required", http.StatusBadRequest)
		return
	}

	if err := s.partnerBoardService.UpdatePartner(guildID, payload.CurrentName, payload.Partner); err != nil {
		status := partnerBoardErrorStatus(err)
		if errors.Is(err, files.ErrPartnerNotFound) {
			status = http.StatusNotFound
		} else if errors.Is(err, files.ErrPartnerAlreadyExists) {
			status = http.StatusConflict
		}
		http.Error(w, fmt.Sprintf("failed to update partner: %v", err), status)
		return
	}

	updated, err := s.partnerBoardService.GetPartner(guildID, payload.Partner.Name)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to read updated partner: %v", err), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"guild_id": guildID,
		"partner":  updated,
	})
}

func (s *Server) handlePartnerBoardPartnersDelete(w http.ResponseWriter, r *http.Request, guildID string) {
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	if name == "" {
		http.Error(w, "failed to delete partner: name query parameter is required", http.StatusBadRequest)
		return
	}

	if err := s.partnerBoardService.DeletePartner(guildID, name); err != nil {
		status := partnerBoardErrorStatus(err)
		if errors.Is(err, files.ErrPartnerNotFound) {
			status = http.StatusNotFound
		}
		http.Error(w, fmt.Sprintf("failed to delete partner: %v", err), status)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"guild_id": guildID,
		"deleted":  strings.TrimSpace(name),
	})
}

func (s *Server) handlePartnerBoardSyncPost(w http.ResponseWriter, r *http.Request, guildID string) {
	if s.partnerBoardSyncer == nil {
		http.Error(w, "partner board sync unavailable", http.StatusServiceUnavailable)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), defaultSyncTimeout)
	defer cancel()

	if err := s.partnerBoardSyncer.SyncGuild(ctx, guildID); err != nil {
		status := partnerBoardErrorStatus(err)
		http.Error(w, fmt.Sprintf("failed to sync partner board: %v", err), status)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"guild_id": guildID,
		"synced":   true,
	})
}

func partnerBoardErrorStatus(err error) int {
	if err == nil {
		return http.StatusInternalServerError
	}

	switch {
	case errors.Is(err, files.ErrGuildConfigNotFound):
		return http.StatusNotFound
	case errors.Is(err, files.ErrInvalidPartnerBoardInput),
		errors.Is(err, messageupdate.ErrInvalidTarget),
		errors.Is(err, partners.ErrInvalidPartnerBoardEntry),
		errors.Is(err, partners.ErrInvalidPartnerBoardTemplate),
		errors.Is(err, partners.ErrPartnerBoardExceedsEmbedLimit):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

func (s *Server) authorizeRequest(w http.ResponseWriter, r *http.Request) (requestAuthorization, bool) {
	if s == nil {
		http.Error(w, "control server unavailable", http.StatusInternalServerError)
		return requestAuthorization{}, false
	}

	token := strings.TrimSpace(s.authBearerToken)
	oauthConfigured := s.discordOAuth != nil
	if token == "" && !oauthConfigured {
		http.Error(w, "control authentication is not configured", http.StatusServiceUnavailable)
		return requestAuthorization{}, false
	}

	authz := strings.TrimSpace(r.Header.Get("Authorization"))
	if authz != "" {
		if !strings.HasPrefix(authz, "Bearer ") {
			http.Error(w, "invalid authorization scheme", http.StatusUnauthorized)
			return requestAuthorization{}, false
		}
		provided := strings.TrimSpace(strings.TrimPrefix(authz, "Bearer "))
		if provided == "" {
			http.Error(w, "missing bearer token", http.StatusUnauthorized)
			return requestAuthorization{}, false
		}
		if token == "" {
			http.Error(w, "control bearer authentication is not configured", http.StatusServiceUnavailable)
			return requestAuthorization{}, false
		}
		if strings.TrimSpace(r.Header.Get("Origin")) != "" {
			http.Error(w, "bearer authentication is restricted to internal automation", http.StatusForbidden)
			return requestAuthorization{}, false
		}
		if subtle.ConstantTimeCompare([]byte(provided), []byte(token)) != 1 {
			http.Error(w, "forbidden", http.StatusForbidden)
			return requestAuthorization{}, false
		}
		return requestAuthorization{mode: requestAuthModeBearer}, true
	}

	if oauthConfigured {
		if session, err := s.discordOAuth.sessionFromRequest(r); err == nil {
			if err := s.discordOAuth.validateSessionCSRFToken(r, session); err != nil {
				http.Error(w, "forbidden", http.StatusForbidden)
				return requestAuthorization{}, false
			}
			return requestAuthorization{
				mode:         requestAuthModeDiscordOAuthSession,
				oauthSession: session,
			}, true
		}
	}

	http.Error(w, "missing authorization", http.StatusUnauthorized)
	return requestAuthorization{}, false
}

func splitGuildRoute(path string) (string, []string, bool) {
	const prefix = "/v1/guilds/"
	if !strings.HasPrefix(path, prefix) {
		return "", nil, false
	}

	trimmed := strings.Trim(path[len(prefix):], "/")
	if trimmed == "" {
		return "", nil, false
	}
	parts := strings.Split(trimmed, "/")
	if len(parts) == 0 {
		return "", nil, false
	}

	guildID := strings.TrimSpace(parts[0])
	tail := []string{}
	if len(parts) > 1 {
		tail = parts[1:]
	}
	return guildID, tail, true
}

func decodeJSONBody(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, defaultMaxBodyBytes)
	defer r.Body.Close()

	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		http.Error(w, fmt.Sprintf("invalid payload: %v", err), http.StatusBadRequest)
		return err
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, payload map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.ApplicationLogger().Error("Failed to encode control response", "status", status, "err", err)
	}
}
