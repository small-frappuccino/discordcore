package roles

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordgo"
)

// rolePanelComponentHandler resolves a button click into either a role
// add or a role remove on the invoking member. The button's customID
// carries the role ID; the handler looks the role up in the persisted
// panel config to confirm it is still managed by us before mutating
// guild membership.
type rolePanelComponentHandler struct {
	configManager *files.ConfigManager
	// memberLookup returns whether the invoking member currently owns
	// roleID. Defaults to a discordgo-backed implementation; the field
	// is exposed so tests can substitute a deterministic source.
	memberLookup func(s *discordgo.Session, i *discordgo.InteractionCreate, roleID string) (bool, error)
	// addRole / removeRole mirror the discordgo session methods so unit
	// tests can assert without hitting a live session.
	addRole    func(s *discordgo.Session, guildID, userID, roleID string) error
	removeRole func(s *discordgo.Session, guildID, userID, roleID string) error
}

func newRolePanelComponentHandler(configManager *files.ConfigManager) *rolePanelComponentHandler {
	return &rolePanelComponentHandler{
		configManager: configManager,
		memberLookup:  defaultRolePanelMemberHasRole,
		addRole:       defaultRolePanelAddRole,
		removeRole:    defaultRolePanelRemoveRole,
	}
}

// HandleComponent handles component.
func (h *rolePanelComponentHandler) HandleComponent(ctx *core.Context) error {
	if ctx == nil || ctx.Interaction == nil {
		return nil
	}
	if h == nil || h.configManager == nil {
		return rolePanelToggleEphemeralError(ctx, "Role panels are unavailable right now.")
	}

	guildID := ctx.GuildID
	if guildID == "" {
		return rolePanelToggleEphemeralError(ctx, "Role panel buttons only work inside a server.")
	}

	if cfg := ctx.Config.Config(); cfg != nil {
		if enabled, _ := cfg.ResolveFeatures(guildID).Lookup(rolePanelFeatureID); !enabled {
			return rolePanelToggleEphemeralError(ctx, "Role panels are disabled for this server.")
		}
	}

	roleID := rolePanelButtonRoleIDFromCustomID(ctx.RouteKey.CustomID)
	if roleID == "" {
		return rolePanelToggleEphemeralError(ctx, "This button is no longer recognized. Ask a moderator to repost the panel.")
	}

	if _, _, err := h.configManager.RolePanelButtonByRoleID(guildID, roleID); err != nil {
		if errors.Is(err, files.ErrRolePanelButtonNotFound) {
			return rolePanelToggleEphemeralError(ctx, "This button is no longer linked to a configured role. Ask a moderator to repost the panel.")
		}
		slog.Error("Role panel button lookup failed",
			"guildID", guildID,
			"roleID", roleID,
			"err", err,
		)
		return rolePanelToggleEphemeralError(ctx, "Could not load the role assignment configuration. Try again in a moment.")
	}

	userID := rolePanelInteractionUserID(ctx.Interaction)
	if userID == "" {
		return rolePanelToggleEphemeralError(ctx, "Could not identify your account on this click.")
	}

	hasRole, err := h.memberLookup(ctx.Session, ctx.Interaction, roleID)
	if err != nil {
		slog.Error("Role panel member lookup failed",
			"guildID", guildID,
			"userID", userID,
			"roleID", roleID,
			"err", err,
		)
		return rolePanelToggleEphemeralError(ctx, "Could not read your current roles. Try again in a moment.")
	}

	if hasRole {
		if err := h.removeRole(ctx.Session, guildID, userID, roleID); err != nil {
			slog.Error("Role panel role removal failed",
				"guildID", guildID,
				"userID", userID,
				"roleID", roleID,
				"err", err,
			)
			return rolePanelToggleEphemeralError(ctx, fmt.Sprintf("Could not remove <@&%s>. Discord said: %v", roleID, err))
		}
		return rolePanelToggleEphemeralSuccess(ctx, fmt.Sprintf("Removed <@&%s>.", roleID))
	}

	if err := h.addRole(ctx.Session, guildID, userID, roleID); err != nil {
		slog.Error("Role panel role addition failed",
			"guildID", guildID,
			"userID", userID,
			"roleID", roleID,
			"err", err,
		)
		return rolePanelToggleEphemeralError(ctx, fmt.Sprintf("Could not assign <@&%s>. Discord said: %v", roleID, err))
	}
	return rolePanelToggleEphemeralSuccess(ctx, fmt.Sprintf("Assigned <@&%s>.", roleID))
}

func isInteractiveEphemeralDisabled(ctx *core.Context) bool {
	if ctx == nil {
		return false
	}
	if ctx.GuildConfig != nil {
		return ctx.GuildConfig.RuntimeConfig.DisableInteractiveEphemeral
	}
	if ctx.Config != nil && ctx.GuildID != "" {
		if gc := ctx.Config.GuildConfig(ctx.GuildID); gc != nil {
			return gc.RuntimeConfig.DisableInteractiveEphemeral
		}
	}
	return false
}

func rolePanelToggleEphemeralError(ctx *core.Context, message string) error {
	if isInteractiveEphemeralDisabled(ctx) {
		return ctx.Session.InteractionRespond(ctx.Interaction.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredMessageUpdate,
		})
	}
	rm := rolePanelToggleResponseBuilder(ctx).WithContext(ctx).Build()
	if err := rm.Custom(ctx.Interaction, message, nil); err != nil {
		slog.Error("Role panel toggle response failed", "err", err)
		return fmt.Errorf("rolePanelToggleEphemeralError: %w", err)
	}
	return nil
}

func rolePanelToggleEphemeralSuccess(ctx *core.Context, message string) error {
	if isInteractiveEphemeralDisabled(ctx) {
		return ctx.Session.InteractionRespond(ctx.Interaction.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredMessageUpdate,
		})
	}
	rm := rolePanelToggleResponseBuilder(ctx).WithContext(ctx).Build()
	return rm.Custom(ctx.Interaction, message, nil)
}

func rolePanelInteractionUserID(i *discordgo.InteractionCreate) string {
	if i == nil {
		return ""
	}
	if i.Member != nil && i.Member.User != nil {
		return i.Member.User.ID
	}
	if i.User != nil {
		return i.User.ID
	}
	return ""
}

// defaultRolePanelMemberHasRole always fetches the member fresh from
// Discord instead of trusting the interaction's Member.Roles snapshot.
// The snapshot is captured when Discord sends the interaction and does
// not reflect role mutations the bot has performed in between two
// quick clicks; trusting it caused a previously-known drift where a
// fast second click could re-Add a role the bot had just assigned
// (the click would echo "Assigned" instead of "Removed"). The fresh
// GuildMember call adds one Discord round-trip per click but matches
// the freshness expectation the user has when they click the button.
func defaultRolePanelMemberHasRole(s *discordgo.Session, i *discordgo.InteractionCreate, roleID string) (bool, error) {
	if i == nil || roleID == "" {
		return false, nil
	}
	if s == nil {
		return false, errors.New("discord session is nil")
	}
	if i.GuildID == "" {
		return false, errors.New("interaction has no guild context")
	}
	userID := rolePanelInteractionUserID(i)
	if userID == "" {
		return false, errors.New("interaction has no user context")
	}
	member, err := s.GuildMember(i.GuildID, userID)
	if err != nil {
		return false, fmt.Errorf("defaultRolePanelMemberHasRole: %w", err)
	}
	for _, r := range member.Roles {
		if r == roleID {
			return true, nil
		}
	}
	return false, nil
}

func defaultRolePanelAddRole(s *discordgo.Session, guildID, userID, roleID string) error {
	if s == nil {
		return errors.New("session is nil")
	}
	return s.GuildMemberRoleAdd(guildID, userID, roleID)
}

func defaultRolePanelRemoveRole(s *discordgo.Session, guildID, userID, roleID string) error {
	if s == nil {
		return errors.New("session is nil")
	}
	return s.GuildMemberRoleRemove(guildID, userID, roleID)
}
