package logpolicy

import (
	"fmt"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/state"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

// ShouldEmitLogEventArikawa is the Arikawa equivalent of ShouldEmitLogEvent.
func ShouldEmitLogEventArikawa(st *state.State, configManager *files.ConfigManager, eventType LogEventType, guildID string) EmitDecision {
	capability, ok := logEventCapabilities[eventType]
	if !ok {
		return EmitDecision{EventType: eventType, Enabled: false, Reason: EmitReasonUnknownEvent}
	}

	decision := EmitDecision{
		EventType:  eventType,
		Category:   capability.Category,
		Enabled:    false,
		Reason:     EmitReasonConfigUnavailable,
		Capability: capability,
	}

	if configManager == nil {
		decision.Reason = EmitReasonConfigManagerUnavailable
		return decision
	}
	cfg := configManager.Config()
	if cfg == nil {
		decision.Reason = EmitReasonConfigUnavailable
		return decision
	}
	gcfg := configManager.GuildConfig(guildID)
	if gcfg == nil {
		decision.Reason = EmitReasonGuildConfigMissing
		return decision
	}

	rc := cfg.ResolveRuntimeConfig(guildID)
	features := cfg.ResolveFeatures(guildID)

	if reason, disabled := evaluateEventToggle(eventType, rc, features); disabled {
		decision.Reason = reason
		return decision
	}

	if capability.RequiresChannel {
		channelID, reason, ok := resolveValidatedLogChannelArikawa(st, capability, eventType, guildID, gcfg)
		if channelID != "" {
			decision.ChannelID = channelID
		}
		if !ok {
			decision.Reason = reason
			return decision
		}
	}

	// For Arikawa, we assume intents are correct because we no longer have
	// an easy way to inspect the gateway identify payload post-initialization.
	// If needed, intents validation should be pushed up to the bot bootstrapper.

	decision.Enabled = true
	decision.Reason = EmitReasonEnabled
	return decision
}

func resolveValidatedLogChannelArikawa(st *state.State, capability LogEventCapability, eventType LogEventType, guildID string, gcfg *files.GuildConfig) (string, EmitReason, bool) {
	channelID := resolveLogChannelForGuild(eventType, gcfg)
	if channelID == "" {
		return "", EmitReasonNoChannelConfigured, false
	}
	if capability.ValidateChannelPerms {
		if capability.RequireExclusiveModeration && IsSharedModerationChannel(channelID, gcfg) {
			return channelID, EmitReasonChannelInvalid, false
		}
		botID := ""
		if st != nil {
			if u, err := st.Me(); err == nil && u != nil {
				botID = u.ID.String()
			}
		}
		if err := ValidateModerationLogChannelArikawa(st, guildID, channelID, botID); err != nil {
			return channelID, EmitReasonChannelInvalid, false
		}
	}
	return channelID, EmitReasonEnabled, true
}

// ValidateModerationLogChannelArikawa validates moderation log channel using Arikawa state.
func ValidateModerationLogChannelArikawa(st *state.State, guildID, channelID, botID string) error {
	if st == nil {
		return fmt.Errorf("state is nil")
	}
	if guildID == "" || channelID == "" {
		return fmt.Errorf("missing guildID or channelID")
	}

	chID, err := discord.ParseSnowflake(channelID)
	if err != nil {
		return err
	}
	gID, err := discord.ParseSnowflake(guildID)
	if err != nil {
		return err
	}
	bID, err := discord.ParseSnowflake(botID)
	if err != nil {
		return err
	}

	ch, err := st.Channel(discord.ChannelID(chID))
	if err != nil {
		return fmt.Errorf("channel lookup failed: %w", err)
	}
	if ch.GuildID != discord.GuildID(gID) {
		return fmt.Errorf("channel guild mismatch")
	}
	if ch.Type != discord.GuildText && ch.Type != discord.GuildNews {
		return fmt.Errorf("channel is not a guild text channel")
	}

	if botID == "" {
		return fmt.Errorf("bot identity not available")
	}

	perms, err := st.Permissions(discord.ChannelID(chID), discord.UserID(bID))
	if err != nil {
		return fmt.Errorf("permission check failed: %w", err)
	}

	required := discord.PermissionViewChannel | discord.PermissionSendMessages | discord.PermissionEmbedLinks
	if perms&required != required {
		return fmt.Errorf("missing permissions (need view/send/embed)")
	}
	return nil
}
