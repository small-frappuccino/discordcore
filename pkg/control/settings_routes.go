package control

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"sort"
	"strings"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
)

var (
	errGuildRegistrationRequired    = errors.New("guild registration required")
	errGuildRegistrationUnavailable = errors.New("guild registration unavailable")
)

type registerGuildRequest struct {
	GuildID string `json:"guild_id"`
}

func (s *Server) handleSettingsRoutes(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.authorizeRequest(w, r)
	if !ok {
		return
	}

	if s.configManager == nil {
		http.Error(w, "config manager unavailable", http.StatusInternalServerError)
		return
	}

	switch normalizeSettingsRoutePath(r.URL.Path) {
	case "/v1/settings":
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleSettingsOverviewGet(w, r, auth)
		return
	case "/v1/settings/catalog":
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"status":  "ok",
			"catalog": buildSettingsCatalog(),
		})
		return
	case "/v1/settings/global":
		switch r.Method {
		case http.MethodGet:
			s.handleGlobalSettingsGet(w, r)
		case http.MethodPut:
			s.handleGlobalSettingsPut(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
		return
	case "/v1/settings/guilds":
		switch r.Method {
		case http.MethodGet:
			s.handleGuildRegistryGet(w, r, auth)
		case http.MethodPost:
			s.handleGuildRegistrationPost(w, r, auth)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
		return
	default:
		http.NotFound(w, r)
		return
	}
}

func (s *Server) handleSettingsOverviewGet(w http.ResponseWriter, r *http.Request, auth requestAuthorization) {
	cfg := s.configManager.SnapshotConfig()
	registrySources, allowedGuilds, err := s.resolveGuildRegistrySources(r, auth)
	if err != nil {
		status := statusForManageableGuildsError(err)
		message := "failed to resolve configured guild access"
		if status == http.StatusUnauthorized {
			message = "oauth session requires re-authentication"
		}
		http.Error(w, message, status)
		return
	}

	registry := buildGuildRegistryWorkspace(cfg, registrySources, allowedGuilds)
	writeJSON(w, http.StatusOK, map[string]any{
		"status":    "ok",
		"workspace": buildSettingsOverview(cfg, s.configManager.ConfigPath(), registry, allowedGuilds),
	})
}

func (s *Server) handleGlobalSettingsGet(w http.ResponseWriter, r *http.Request) {
	cfg := s.configManager.SnapshotConfig()
	writeJSON(w, http.StatusOK, map[string]any{
		"status":    "ok",
		"workspace": buildGlobalSettingsWorkspace(cfg),
	})
}

func (s *Server) handleGlobalSettingsPut(w http.ResponseWriter, r *http.Request) {
	var payload updateGlobalSettingsRequest
	if err := decodeJSONBody(w, r, &payload); err != nil {
		return
	}
	if payload.Features == nil && payload.Runtime == nil {
		http.Error(w, "payload must contain at least one settings section", http.StatusBadRequest)
		return
	}

	updated, err := s.configManager.UpdateConfig(func(cfg *files.BotConfig) error {
		if payload.Features != nil {
			cfg.Features = *payload.Features
		}
		if payload.Runtime != nil {
			next, err := files.NormalizeRuntimeConfig(flattenRuntimeSettingsSections(*payload.Runtime))
			if err != nil {
				return err
			}
			cfg.RuntimeConfig = next
		}
		return nil
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to update global settings: %v", err), statusForSettingsMutationError(err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":    "ok",
		"workspace": buildGlobalSettingsWorkspace(updated),
	})
}

func (s *Server) handleGuildRegistryGet(w http.ResponseWriter, r *http.Request, auth requestAuthorization) {
	cfg := s.configManager.SnapshotConfig()
	registrySources, allowedGuilds, err := s.resolveGuildRegistrySources(r, auth)
	if err != nil {
		status := statusForManageableGuildsError(err)
		message := "failed to resolve configured guild access"
		if status == http.StatusUnauthorized {
			message = "oauth session requires re-authentication"
		}
		http.Error(w, message, status)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":    "ok",
		"workspace": buildGuildRegistryWorkspace(cfg, registrySources, allowedGuilds),
		"guilds":    buildConfiguredGuildSummaries(cfg, allowedGuilds),
	})
}

func (s *Server) handleGuildRegistrationPost(w http.ResponseWriter, r *http.Request, auth requestAuthorization) {
	var payload registerGuildRequest
	if err := decodeJSONBody(w, r, &payload); err != nil {
		return
	}

	guildID := strings.TrimSpace(payload.GuildID)
	if guildID == "" {
		http.Error(w, "guild_id is required", http.StatusBadRequest)
		return
	}
	if !s.authorizeGuildAccess(w, r, auth, guildID) {
		return
	}

	current := s.configManager.SnapshotConfig()
	if guild, ok := findGuildSettings(current, guildID); ok {
		logSettingsRegistrationResult(auth, guildID, "control.settings.guild_registry.register.skip", nil)
		writeJSON(w, http.StatusOK, map[string]any{
			"status":    "ok",
			"guild_id":  guildID,
			"created":   false,
			"workspace": buildGuildSettingsWorkspace(current, guild),
		})
		return
	}
	if s.guildRegistration == nil {
		err := fmt.Errorf("%w: bootstrap is not configured for guild_id=%s", errGuildRegistrationUnavailable, guildID)
		logSettingsRegistrationResult(auth, guildID, "control.settings.guild_registry.register.unavailable", err)
		http.Error(w, fmt.Sprintf("failed to register guild settings: %v", err), statusForSettingsMutationError(err))
		return
	}

	logSettingsRegistrationAttempt(auth, guildID)
	if err := s.guildRegistration(r.Context(), guildID); err != nil {
		logSettingsRegistrationResult(auth, guildID, "control.settings.guild_registry.register.failed", err)
		http.Error(w, fmt.Sprintf("failed to register guild settings: %v", err), statusForSettingsMutationError(err))
		return
	}

	updated := s.configManager.SnapshotConfig()
	guild, ok := findGuildSettings(updated, guildID)
	if !ok {
		err := fmt.Errorf("registered guild settings not found for %s", guildID)
		logSettingsRegistrationResult(auth, guildID, "control.settings.guild_registry.register.missing", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	logSettingsRegistrationResult(auth, guildID, "control.settings.guild_registry.register.success", nil)
	writeJSON(w, http.StatusCreated, map[string]any{
		"status":    "ok",
		"guild_id":  guildID,
		"created":   true,
		"workspace": buildGuildSettingsWorkspace(updated, guild),
	})
}

func (s *Server) handleGuildSettingsGet(w http.ResponseWriter, r *http.Request, guildID string) {
	cfg := s.configManager.SnapshotConfig()
	guild, ok := findGuildSettings(cfg, guildID)
	if !ok {
		http.Error(w, fmt.Sprintf("guild settings not found for %s", guildID), http.StatusNotFound)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":    "ok",
		"workspace": buildGuildSettingsWorkspace(cfg, guild),
	})
}

func (s *Server) handleGuildSettingsPut(w http.ResponseWriter, r *http.Request, guildID string) {
	var payload updateGuildSettingsRequest
	if err := decodeJSONBody(w, r, &payload); err != nil {
		return
	}
	if guildPayloadEmpty(payload) {
		http.Error(w, "payload must contain at least one settings section", http.StatusBadRequest)
		return
	}

	updated, err := s.configManager.UpdateConfig(func(cfg *files.BotConfig) error {
		guild, ok := findGuildSettingsMutable(cfg, guildID)
		if !ok {
			return fmt.Errorf("%w: register this guild first (guild_id=%s)", errGuildRegistrationRequired, guildID)
		}
		if payload.Features != nil {
			guild.Features = *payload.Features
		}
		if payload.Channels != nil {
			guild.Channels = *payload.Channels
		}
		if payload.Roles != nil {
			guild.Roles = *payload.Roles
		}
		if payload.Stats != nil {
			guild.Stats = *payload.Stats
		}
		if payload.Moderation != nil {
			next, err := files.NormalizeGuildModerationConfig(
				payload.Moderation.Rulesets,
				payload.Moderation.LooseRules,
				payload.Moderation.Blocklist,
			)
			if err != nil {
				return err
			}
			guild.Rulesets = next.Rulesets
			guild.LooseLists = next.LooseRules
			guild.Blocklist = next.Blocklist
		}
		if payload.Cache != nil {
			guild.RolesCacheTTL = payload.Cache.RolesCacheTTL
			guild.MemberCacheTTL = payload.Cache.MemberCacheTTL
			guild.GuildCacheTTL = payload.Cache.GuildCacheTTL
			guild.ChannelCacheTTL = payload.Cache.ChannelCacheTTL
		}
		if payload.UserPrune != nil {
			guild.UserPrune = *payload.UserPrune
		}
		if payload.PartnerBoard != nil {
			next, err := files.NormalizePartnerBoardConfig(*payload.PartnerBoard)
			if err != nil {
				return err
			}
			guild.PartnerBoard = next
		}
		if payload.Runtime != nil {
			next, err := files.NormalizeRuntimeConfig(flattenRuntimeSettingsSections(*payload.Runtime))
			if err != nil {
				return err
			}
			guild.RuntimeConfig = next
		}
		return nil
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to update guild settings: %v", err), statusForSettingsMutationError(err))
		return
	}

	guild, ok := findGuildSettings(updated, guildID)
	if !ok {
		http.Error(w, fmt.Sprintf("updated guild settings not found for %s", guildID), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":    "ok",
		"workspace": buildGuildSettingsWorkspace(updated, guild),
	})
}

func (s *Server) handleGuildSettingsDelete(w http.ResponseWriter, r *http.Request, guildID string) {
	_, err := s.configManager.UpdateConfig(func(cfg *files.BotConfig) error {
		for idx := range cfg.Guilds {
			if cfg.Guilds[idx].GuildID != guildID {
				continue
			}
			cfg.Guilds = slices.Delete(cfg.Guilds, idx, idx+1)
			return nil
		}
		return fmt.Errorf("%w: guild_id=%s", files.ErrGuildConfigNotFound, guildID)
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to delete guild settings: %v", err), statusForSettingsMutationError(err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"guild_id": guildID,
		"deleted":  true,
	})
}

func (s *Server) resolveGuildRegistrySources(
	r *http.Request,
	auth requestAuthorization,
) ([]guildRegistrySource, map[string]struct{}, error) {
	switch auth.mode {
	case requestAuthModeBearer:
		ctx, cancel := context.WithTimeout(r.Context(), defaultManageableGuildsQuery)
		defer cancel()

		botGuildSet, err := s.resolveBotGuildIDSet(ctx)
		if err != nil {
			if errors.Is(err, errBotGuildIDsProviderUnavailable) {
				return nil, nil, nil
			}
			return nil, nil, err
		}

		ids := make([]string, 0, len(botGuildSet))
		for guildID := range botGuildSet {
			ids = append(ids, guildID)
		}
		sort.Strings(ids)

		sources := make([]guildRegistrySource, 0, len(ids))
		for _, guildID := range ids {
			sources = append(sources, guildRegistrySource{GuildID: guildID})
		}
		return sources, nil, nil
	case requestAuthModeDiscordOAuthSession:
		ctx, cancel := context.WithTimeout(r.Context(), defaultManageableGuildsQuery)
		defer cancel()

		manageable, err := s.resolveManageableGuilds(ctx, s.discordOAuth, auth.oauthSession)
		if err != nil {
			return nil, nil, err
		}

		out := make(map[string]struct{}, len(manageable))
		sources := make([]guildRegistrySource, 0, len(manageable))
		for _, guild := range manageable {
			guildID := strings.TrimSpace(guild.ID)
			if guildID == "" {
				continue
			}
			out[guildID] = struct{}{}
			sources = append(sources, guildRegistrySource{
				GuildID:     guildID,
				Name:        strings.TrimSpace(guild.Name),
				Icon:        strings.TrimSpace(guild.Icon),
				Owner:       guild.Owner,
				Permissions: guild.Permissions,
			})
		}
		return sources, out, nil
	default:
		return nil, nil, fmt.Errorf("missing authorization")
	}
}

func normalizeSettingsRoutePath(path string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(path), "/")
	if trimmed == "" {
		return "/"
	}
	return trimmed
}

func guildPayloadEmpty(payload updateGuildSettingsRequest) bool {
	return payload.Features == nil &&
		payload.Channels == nil &&
		payload.Roles == nil &&
		payload.Stats == nil &&
		payload.Moderation == nil &&
		payload.Cache == nil &&
		payload.UserPrune == nil &&
		payload.PartnerBoard == nil &&
		payload.Runtime == nil
}

func statusForSettingsMutationError(err error) int {
	if err == nil {
		return http.StatusInternalServerError
	}

	var validationErr files.ValidationError
	switch {
	case errors.Is(err, errGuildRegistrationRequired):
		return http.StatusConflict
	case errors.Is(err, errGuildRegistrationUnavailable):
		return http.StatusServiceUnavailable
	case errors.Is(err, files.ErrGuildConfigNotFound):
		return http.StatusNotFound
	case errors.Is(err, files.ErrGuildBootstrapPrerequisite):
		return http.StatusConflict
	case errors.Is(err, files.ErrGuildBootstrapDiscordFetch):
		return http.StatusBadGateway
	case errors.Is(err, files.ErrInvalidPartnerBoardInput):
		return http.StatusBadRequest
	case errors.Is(err, files.ErrWebhookEmbedUpdateAlreadyExists):
		return http.StatusConflict
	case errors.As(err, &validationErr):
		return http.StatusBadRequest
	case strings.Contains(err.Error(), files.ErrValidationFailed):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

func logSettingsRegistrationAttempt(auth requestAuthorization, guildID string) {
	fields := []any{
		"operation", "control.settings.guild_registry.register",
		"guildID", guildID,
	}
	if userID := settingsRequestUserID(auth); userID != "" {
		fields = append(fields, "userID", userID)
	}
	log.ApplicationLogger().Info("Registering guild settings", fields...)
}

func logSettingsRegistrationResult(auth requestAuthorization, guildID, operation string, err error) {
	fields := []any{
		"operation", operation,
		"guildID", guildID,
	}
	if userID := settingsRequestUserID(auth); userID != "" {
		fields = append(fields, "userID", userID)
	}
	if err != nil {
		fields = append(fields, "err", err)
		log.ApplicationLogger().Error("Guild settings registration failed", fields...)
		return
	}
	log.ApplicationLogger().Info("Guild settings registration completed", fields...)
}

func settingsRequestUserID(auth requestAuthorization) string {
	if auth.mode != requestAuthModeDiscordOAuthSession {
		return ""
	}
	return strings.TrimSpace(auth.oauthSession.User.ID)
}
