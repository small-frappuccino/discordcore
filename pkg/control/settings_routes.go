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
	errGuildDiscoveryRequired       = errors.New("guild discovery required")
)

type registerGuildRequest struct {
	GuildID       string `json:"guild_id"`
	BotInstanceID string `json:"bot_instance_id,omitempty"`
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
		if !s.authorizeGlobalControlAccess(w, r, auth, guildAccessLevelRead) {
			return
		}
		s.handleSettingsOverviewGet(w, r, auth)
		return
	case "/v1/settings/catalog":
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !s.authorizeGlobalControlAccess(w, r, auth, guildAccessLevelRead) {
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
			if !s.authorizeGlobalControlAccess(w, r, auth, guildAccessLevelRead) {
				return
			}
			s.handleGlobalSettingsGet(w, r)
		case http.MethodPut:
			if !s.authorizeGlobalControlAccess(w, r, auth, guildAccessLevelWrite) {
				return
			}
			s.handleGlobalSettingsPut(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
		return
	case "/v1/settings/guilds":
		switch r.Method {
		case http.MethodGet:
			if !s.authorizeGlobalControlAccess(w, r, auth, guildAccessLevelRead) {
				return
			}
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

	registry := buildGuildRegistryWorkspace(cfg, registrySources, allowedGuilds, s.defaultBotInstanceID)
	writeJSON(w, http.StatusOK, map[string]any{
		"status":    "ok",
		"workspace": buildSettingsOverview(cfg, s.configManager.ConfigPath(), registry, allowedGuilds, s.defaultBotInstanceID),
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
		"workspace": buildGuildRegistryWorkspace(cfg, registrySources, allowedGuilds, s.defaultBotInstanceID),
		"guilds":    buildConfiguredGuildSummaries(cfg, allowedGuilds, s.defaultBotInstanceID),
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
	if !s.authorizeGuildControlAccess(w, r, auth, guildID, guildAccessLevelWrite) {
		return
	}

	availableBotInstanceIDs, err := s.resolveAvailableBotInstanceIDsForGuild(r.Context(), auth, guildID)
	if err != nil {
		status := statusForManageableGuildsError(err)
		http.Error(w, fmt.Sprintf("failed to resolve guild bot instances: %v", err), status)
		return
	}
	botInstanceID, err := selectGuildBotInstanceID(payload.BotInstanceID, availableBotInstanceIDs)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	current := s.configManager.SnapshotConfig()
	if guild, ok := findGuildSettings(current, guildID); ok {
		logSettingsRegistrationResult(auth, guildID, "control.settings.guild_registry.register.skip", nil)
		writeJSON(w, http.StatusOK, map[string]any{
			"status":    "ok",
			"guild_id":  guildID,
			"created":   false,
			"workspace": buildGuildSettingsWorkspaceWithBindings(current, guild, availableBotInstanceIDs, s.defaultBotInstanceID),
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
	if err := s.guildRegistration(r.Context(), guildID, botInstanceID); err != nil {
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
		"workspace": buildGuildSettingsWorkspaceWithBindings(updated, guild, availableBotInstanceIDs, s.defaultBotInstanceID),
	})
}

func (s *Server) handleGuildSettingsGet(w http.ResponseWriter, r *http.Request, guildID string) {
	cfg := s.configManager.SnapshotConfig()
	guild, ok := findGuildSettings(cfg, guildID)
	if !ok {
		http.Error(w, fmt.Sprintf("guild settings not found for %s", guildID), http.StatusNotFound)
		return
	}
	availableBotInstanceIDs, err := s.resolveAvailableBotInstanceIDsForGuild(r.Context(), requestAuthorization{mode: requestAuthModeBearer}, guildID)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to resolve guild bot instances: %v", err), statusForManageableGuildsError(err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":    "ok",
		"workspace": buildGuildSettingsWorkspaceWithBindings(cfg, guild, availableBotInstanceIDs, s.defaultBotInstanceID),
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
	var (
		availableBotInstanceIDs []string
		nextBotInstanceID       string
		invalidateAccessCache   bool
	)
	if payload.BotInstanceID != nil {
		available, err := s.resolveAvailableBotInstanceIDsForGuild(r.Context(), requestAuthorization{mode: requestAuthModeBearer}, guildID)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to resolve guild bot instances: %v", err), statusForManageableGuildsError(err))
			return
		}
		selectedBotInstanceID, err := selectGuildBotInstanceID(*payload.BotInstanceID, available)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		availableBotInstanceIDs = available
		nextBotInstanceID = selectedBotInstanceID
	}

	updated, err := s.configManager.UpdateConfig(func(cfg *files.BotConfig) error {
		guild, ok := findGuildSettingsMutable(cfg, guildID)
		if !ok {
			return fmt.Errorf("%w: register this guild first (guild_id=%s)", errGuildRegistrationRequired, guildID)
		}
		if payload.BotInstanceID != nil {
			guild.BotInstanceID = nextBotInstanceID
		}
		if payload.Features != nil {
			guild.Features = *payload.Features
		}
		if payload.Channels != nil {
			guild.Channels = *payload.Channels
		}
		if payload.Roles != nil {
			invalidateAccessCache = dashboardAccessRolesChanged(guild.Roles, *payload.Roles)
			guild.Roles = *payload.Roles
		}
		if payload.Stats != nil {
			guild.Stats = *payload.Stats
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
	if invalidateAccessCache {
		s.invalidateAccessibleGuildsCache()
	}

	guild, ok := findGuildSettings(updated, guildID)
	if !ok {
		http.Error(w, fmt.Sprintf("updated guild settings not found for %s", guildID), http.StatusInternalServerError)
		return
	}
	if payload.BotInstanceID == nil {
		availableBotInstanceIDs, err = s.resolveAvailableBotInstanceIDsForGuild(r.Context(), requestAuthorization{mode: requestAuthModeBearer}, guildID)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to resolve guild bot instances: %v", err), statusForManageableGuildsError(err))
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":    "ok",
		"workspace": buildGuildSettingsWorkspaceWithBindings(updated, guild, availableBotInstanceIDs, s.defaultBotInstanceID),
	})
}

func (s *Server) handleGuildSettingsDelete(w http.ResponseWriter, r *http.Request, guildID string) {
	if _, err := s.resolveAvailableBotInstanceIDsForGuild(r.Context(), requestAuthorization{mode: requestAuthModeBearer}, guildID); err != nil {
		http.Error(w, fmt.Sprintf("failed to resolve guild bot instances: %v", err), statusForManageableGuildsError(err))
		return
	}
	invalidateAccessCache := false
	_, err := s.configManager.UpdateConfig(func(cfg *files.BotConfig) error {
		for idx := range cfg.Guilds {
			if cfg.Guilds[idx].GuildID != guildID {
				continue
			}
			invalidateAccessCache = dashboardAccessRolesConfigured(cfg.Guilds[idx].Roles)
			cfg.Guilds = slices.Delete(cfg.Guilds, idx, idx+1)
			return nil
		}
		return fmt.Errorf("%w: guild_id=%s", files.ErrGuildConfigNotFound, guildID)
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to delete guild settings: %v", err), statusForSettingsMutationError(err))
		return
	}
	if invalidateAccessCache {
		s.invalidateAccessibleGuildsCache()
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
		ctx, cancel := context.WithTimeout(r.Context(), defaultAccessibleGuildsQuery)
		defer cancel()

		bindings, err := s.resolveBotGuildBindings(ctx)
		if err != nil {
			return nil, nil, err
		}
		return guildRegistrySourcesFromBindings(bindings), nil, nil
	case requestAuthModeDiscordOAuthSession:
		ctx, cancel := context.WithTimeout(r.Context(), defaultAccessibleGuildsQuery)
		defer cancel()

		accessible, err := s.oauthControl().resolveAccessibleGuilds(ctx, auth.oauthSession)
		if err != nil {
			return nil, nil, err
		}
		bindings, err := s.resolveBotGuildBindings(ctx)
		if err != nil {
			return nil, nil, err
		}
		botInstanceIDsByGuild := groupBotInstanceIDsByGuild(bindings)

		out := make(map[string]struct{}, len(accessible))
		sources := make([]guildRegistrySource, 0, len(accessible))
		for _, guild := range accessible {
			guildID := strings.TrimSpace(guild.ID)
			if guildID == "" {
				continue
			}
			out[guildID] = struct{}{}
			sources = append(sources, guildRegistrySource{
				GuildID:                 guildID,
				Name:                    strings.TrimSpace(guild.Name),
				Icon:                    strings.TrimSpace(guild.Icon),
				Owner:                   guild.Owner,
				Permissions:             guild.Permissions,
				AvailableBotInstanceIDs: botInstanceIDsByGuild[guildID],
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
	return payload.BotInstanceID == nil &&
		payload.Features == nil &&
		payload.Channels == nil &&
		payload.Roles == nil &&
		payload.Stats == nil &&
		payload.Cache == nil &&
		payload.UserPrune == nil &&
		payload.PartnerBoard == nil &&
		payload.Runtime == nil
}

func dashboardAccessRolesChanged(before, after files.RolesConfig) bool {
	return !slices.Equal(normalizeDashboardAccessRoleIDs(before.DashboardRead), normalizeDashboardAccessRoleIDs(after.DashboardRead)) ||
		!slices.Equal(normalizeDashboardAccessRoleIDs(before.DashboardWrite), normalizeDashboardAccessRoleIDs(after.DashboardWrite))
}

func dashboardAccessRolesConfigured(roles files.RolesConfig) bool {
	return len(normalizeDashboardAccessRoleIDs(roles.DashboardRead)) > 0 ||
		len(normalizeDashboardAccessRoleIDs(roles.DashboardWrite)) > 0
}

func normalizeDashboardAccessRoleIDs(roleIDs []string) []string {
	if len(roleIDs) == 0 {
		return nil
	}

	out := make([]string, 0, len(roleIDs))
	for _, roleID := range roleIDs {
		roleID = strings.TrimSpace(roleID)
		if roleID == "" {
			continue
		}
		out = append(out, roleID)
	}
	if len(out) == 0 {
		return nil
	}

	sort.Strings(out)
	return slices.Compact(out)
}

func (s *Server) resolveAvailableBotInstanceIDsForGuild(
	ctx context.Context,
	auth requestAuthorization,
	guildID string,
) ([]string, error) {
	guildID = strings.TrimSpace(guildID)
	if guildID == "" {
		return nil, fmt.Errorf("guild_id is required")
	}

	bindings, err := s.resolveBotGuildBindings(ctx)
	if err != nil {
		return nil, err
	}
	if !guildIDPresentInBindings(bindings, guildID) {
		return nil, fmt.Errorf("%w: guild_id=%s", errGuildDiscoveryRequired, guildID)
	}
	available := groupBotInstanceIDsByGuild(bindings)[guildID]
	if auth.mode == requestAuthModeDiscordOAuthSession {
		accessible, resolveErr := s.oauthControl().resolveAccessibleGuilds(ctx, auth.oauthSession)
		if resolveErr != nil {
			return nil, resolveErr
		}
		allowed := false
		for _, guild := range accessible {
			if strings.TrimSpace(guild.ID) == guildID {
				allowed = true
				break
			}
		}
		if !allowed {
			return nil, fmt.Errorf("guild %s is not accessible by the authenticated user", guildID)
		}
	}
	return available, nil
}

func guildIDPresentInBindings(bindings []BotGuildBinding, guildID string) bool {
	guildID = strings.TrimSpace(guildID)
	if guildID == "" {
		return false
	}
	for _, binding := range bindings {
		if strings.TrimSpace(binding.GuildID) == guildID {
			return true
		}
	}
	return false
}

func selectGuildBotInstanceID(requested string, availableBotInstanceIDs []string) (string, error) {
	availableBotInstanceIDs = normalizeBotInstanceIDs(availableBotInstanceIDs)
	requested = strings.TrimSpace(requested)

	switch {
	case requested == "" && len(availableBotInstanceIDs) == 0:
		return "", nil
	case requested == "" && len(availableBotInstanceIDs) == 1:
		return availableBotInstanceIDs[0], nil
	case requested == "":
		return "", fmt.Errorf("bot_instance_id is required when multiple bot instances are available")
	}

	if len(availableBotInstanceIDs) == 0 {
		return requested, nil
	}
	for _, candidate := range availableBotInstanceIDs {
		if candidate == requested {
			return requested, nil
		}
	}
	return "", fmt.Errorf("bot_instance_id %q is not available for this guild", requested)
}

func groupBotInstanceIDsByGuild(bindings []BotGuildBinding) map[string][]string {
	out := make(map[string][]string)
	seen := make(map[string]map[string]struct{})
	for _, binding := range bindings {
		guildID := strings.TrimSpace(binding.GuildID)
		botInstanceID := strings.TrimSpace(binding.BotInstanceID)
		if guildID == "" || botInstanceID == "" {
			continue
		}
		if _, ok := seen[guildID]; !ok {
			seen[guildID] = make(map[string]struct{})
		}
		if _, ok := seen[guildID][botInstanceID]; ok {
			continue
		}
		seen[guildID][botInstanceID] = struct{}{}
		out[guildID] = append(out[guildID], botInstanceID)
	}
	for guildID := range out {
		sort.Strings(out[guildID])
	}
	return out
}

func guildRegistrySourcesFromBindings(bindings []BotGuildBinding) []guildRegistrySource {
	if len(bindings) == 0 {
		return nil
	}

	availableByGuild := groupBotInstanceIDsByGuild(bindings)
	seen := make(map[string]struct{}, len(bindings))
	sources := make([]guildRegistrySource, 0, len(bindings))
	for _, binding := range bindings {
		guildID := strings.TrimSpace(binding.GuildID)
		if guildID == "" {
			continue
		}
		if _, ok := seen[guildID]; ok {
			continue
		}
		seen[guildID] = struct{}{}
		sources = append(sources, guildRegistrySource{
			GuildID:                 guildID,
			AvailableBotInstanceIDs: slices.Clone(availableByGuild[guildID]),
		})
	}
	sort.Slice(sources, func(i, j int) bool {
		return sources[i].GuildID < sources[j].GuildID
	})
	return sources
}

func normalizeBotInstanceIDs(ids []string) []string {
	if len(ids) == 0 {
		return nil
	}
	out := make([]string, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		trimmed := strings.TrimSpace(id)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	sort.Strings(out)
	return out
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
