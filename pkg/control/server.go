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

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/runtimeapply"
)

const (
	defaultMaxBodyBytes = 64 * 1024
)

// Server exposes operational controls for a running Discordcore instance.
type Server struct {
	addr           string
	configManager  *files.ConfigManager
	runtimeApplier *runtimeapply.Manager
	httpServer     *http.Server
	listener       net.Listener
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
		configManager:  configManager,
		runtimeApplier: runtimeApplier,
	}
	s.httpServer = &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	mux.HandleFunc("/v1/runtime-config", s.handleRuntimeConfig)

	return s
}

// Start opens the control server listening socket.
func (s *Server) Start() error {
	if s == nil {
		return nil
	}

	ln, err := net.Listen("tcp", s.httpServer.Addr)
	if err != nil {
		return fmt.Errorf("bind control server: %w", err)
	}
	s.listener = ln

	log.ApplicationLogger().Info("Control server listening", "addr", s.httpServer.Addr)

	go func() {
		if err := s.httpServer.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.ApplicationLogger().Error("Control server stopped unexpectedly", "err", err)
		}
	}()

	return nil
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
