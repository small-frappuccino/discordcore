package roles

import (
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/utils/json/option"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/legacycore"
	rolesvc "github.com/small-frappuccino/discordcore/pkg/discord/roles"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

type rolePanelComponentHandler struct {
	configManager *files.ConfigManager
	memberLookup  func(ctx *legacycore.ArikawaContext, roleID string) (bool, error)
	addRole       func(ctx *legacycore.ArikawaContext, guildID, userID, roleID string) error
	removeRole    func(ctx *legacycore.ArikawaContext, guildID, userID, roleID string) error
}

func newRolePanelComponentHandler(configManager *files.ConfigManager) *rolePanelComponentHandler {
	return &rolePanelComponentHandler{
		configManager: configManager,
		memberLookup:  defaultRolePanelMemberHasRoleArikawa,
		addRole:       defaultRolePanelAddRoleArikawa,
		removeRole:    defaultRolePanelRemoveRoleArikawa,
	}
}

func (h *rolePanelComponentHandler) HandleComponent(ctx *legacycore.ArikawaContext) error {
	if ctx == nil || ctx.Interaction == nil {
		return nil
	}
	if h == nil || h.configManager == nil {
		return rolePanelToggleEphemeralError(ctx, "Role panels are unavailable right now.")
	}

	guildID := ctx.GuildID
	if !guildID.IsValid() {
		return rolePanelToggleEphemeralError(ctx, "Role panel buttons only work inside a server.")
	}

	if err := ensureRolePanelEnabled(ctx); err != nil {
		return rolePanelToggleEphemeralError(ctx, "Role panels are disabled for this server.")
	}

	data, ok := ctx.Interaction.Data.(interface{ ID() discord.ComponentID })
	if !ok {
		return rolePanelToggleEphemeralError(ctx, "Invalid component data.")
	}

	roleIDStr := rolesvc.RolePanelButtonRoleIDFromCustomID(string(data.ID()))
	if roleIDStr == "" {
		return rolePanelToggleEphemeralError(ctx, "This button is no longer recognized. Ask a moderator to repost the panel.")
	}

	if _, _, err := h.configManager.RolePanelButtonByRoleID(guildID.String(), roleIDStr); err != nil {
		if errors.Is(err, files.ErrRolePanelButtonNotFound) {
			// Operational annotation: If the configuration was deleted but the Discord message
			// remains active, we intercept the toggle and notify the user safely.
			return rolePanelToggleEphemeralError(ctx, "This button is no longer linked to a configured role. Ask a moderator to repost the panel.")
		}
		slog.Error("Blocking structural failure restricted to operational scope",
			slog.String("req_id", guildID.String()),
			slog.String("stack_trace", string(debug.Stack())),
			slog.Int("fail_id", 500),
			slog.String("error", fmt.Sprintf("button lookup failed for role %s: %v", roleIDStr, err)),
		)
		return rolePanelToggleEphemeralError(ctx, "Could not load the role assignment configuration. Try again in a moment.")
	}

	userID := ctx.UserID
	if !userID.IsValid() {
		return rolePanelToggleEphemeralError(ctx, "Could not identify your account on this click.")
	}

	hasRole, err := h.memberLookup(ctx, roleIDStr)
	if err != nil {
		slog.Error("Blocking structural failure restricted to operational scope",
			slog.String("req_id", guildID.String()),
			slog.String("stack_trace", string(debug.Stack())),
			slog.Int("fail_id", 500),
			slog.String("error", fmt.Sprintf("member lookup failed for user %s: %v", userID, err)),
		)
		return rolePanelToggleEphemeralError(ctx, "Could not read your current roles. Try again in a moment.")
	}

	if hasRole {
		if err := h.removeRole(ctx, guildID.String(), userID.String(), roleIDStr); err != nil {
			slog.Error("Blocking structural failure restricted to operational scope",
				slog.String("req_id", guildID.String()),
				slog.String("stack_trace", string(debug.Stack())),
				slog.Int("fail_id", 500),
				slog.String("error", fmt.Sprintf("role removal failed for user %s: %v", userID, err)),
			)
			// Operational annotation: We bubble up the underlying Discord API failure to the user
			// to provide actionable context (e.g., missing bot permissions).
			return rolePanelToggleEphemeralError(ctx, fmt.Sprintf("Could not remove <@&%s>. Discord said: %v", roleIDStr, err))
		}
		return rolePanelToggleEphemeralSuccess(ctx, fmt.Sprintf("Removed <@&%s>.", roleIDStr))
	}

	if err := h.addRole(ctx, guildID.String(), userID.String(), roleIDStr); err != nil {
		slog.Error("Blocking structural failure restricted to operational scope",
			slog.String("req_id", guildID.String()),
			slog.String("stack_trace", string(debug.Stack())),
			slog.Int("fail_id", 500),
			slog.String("error", fmt.Sprintf("role addition failed for user %s: %v", userID, err)),
		)
		return rolePanelToggleEphemeralError(ctx, fmt.Sprintf("Could not assign <@&%s>. Discord said: %v", roleIDStr, err))
	}
	return rolePanelToggleEphemeralSuccess(ctx, fmt.Sprintf("Assigned <@&%s>.", roleIDStr))
}

func rolePanelToggleEphemeralError(ctx *legacycore.ArikawaContext, message string) error {
	return ctx.Respond(api.InteractionResponseData{
		Content: option.NewNullableString(message),
		Flags:   discord.EphemeralMessage,
	})
}

func rolePanelToggleEphemeralSuccess(ctx *legacycore.ArikawaContext, message string) error {
	return ctx.Respond(api.InteractionResponseData{
		Content: option.NewNullableString(message),
		Flags:   discord.EphemeralMessage,
	})
}

func defaultRolePanelMemberHasRoleArikawa(ctx *legacycore.ArikawaContext, roleIDStr string) (bool, error) {
	if ctx == nil || roleIDStr == "" {
		return false, nil
	}
	if ctx.Client == nil {
		return false, errors.New("discord client is nil")
	}
	if !ctx.GuildID.IsValid() {
		return false, errors.New("interaction has no guild context")
	}
	if !ctx.UserID.IsValid() {
		return false, errors.New("interaction has no user context")
	}
	member, err := ctx.Client.Member(ctx.GuildID, ctx.UserID)
	if err != nil {
		return false, fmt.Errorf("defaultRolePanelMemberHasRoleArikawa: %w", err)
	}
	rID, err := discord.ParseSnowflake(roleIDStr)
	if err != nil {
		return false, err
	}
	targetRole := discord.RoleID(rID)
	for _, r := range member.RoleIDs {
		if r == targetRole {
			return true, nil
		}
	}
	return false, nil
}

func defaultRolePanelAddRoleArikawa(ctx *legacycore.ArikawaContext, guildIDStr, userIDStr, roleIDStr string) error {
	if ctx.Client == nil {
		return errors.New("client is nil")
	}
	gID, _ := discord.ParseSnowflake(guildIDStr)
	uID, _ := discord.ParseSnowflake(userIDStr)
	rID, _ := discord.ParseSnowflake(roleIDStr)
	return ctx.Client.AddRole(discord.GuildID(gID), discord.UserID(uID), discord.RoleID(rID), api.AddRoleData{AuditLogReason: "Role Panel self-assign"})
}

func defaultRolePanelRemoveRoleArikawa(ctx *legacycore.ArikawaContext, guildIDStr, userIDStr, roleIDStr string) error {
	if ctx.Client == nil {
		return errors.New("client is nil")
	}
	gID, _ := discord.ParseSnowflake(guildIDStr)
	uID, _ := discord.ParseSnowflake(userIDStr)
	rID, _ := discord.ParseSnowflake(roleIDStr)
	return ctx.Client.RemoveRole(discord.GuildID(gID), discord.UserID(uID), discord.RoleID(rID), api.AuditLogReason("Role Panel self-assign"))
}
