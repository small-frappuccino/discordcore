package logging

import (
	"fmt"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/state"
)

type arikawaDiscordAdapter struct {
	st *state.State
}

func (a *arikawaDiscordAdapter) CanLogToChannel(channelIDStr string) (bool, error) {
	channelID, err := discord.ParseSnowflake(channelIDStr)
	if err != nil {
		return false, fmt.Errorf("invalid channel ID: %w", err)
	}

	me, err := a.st.Me()
	if err != nil || me == nil {
		return false, fmt.Errorf("bot identity not available")
	}

	perms, err := a.st.Permissions(discord.ChannelID(channelID), me.ID)
	if err != nil {
		return false, fmt.Errorf("permission check failed: %w", err)
	}

	required := discord.PermissionViewChannel | discord.PermissionSendMessages | discord.PermissionEmbedLinks
	if perms&required != required {
		return false, nil
	}

	return true, nil
}

func (a *arikawaDiscordAdapter) ValidateModerationLogChannel(guildIDStr, channelIDStr string) error {
	channelID, err := discord.ParseSnowflake(channelIDStr)
	if err != nil {
		return fmt.Errorf("invalid channel ID: %w", err)
	}
	guildIDParsed, err := discord.ParseSnowflake(guildIDStr)
	if err != nil {
		return fmt.Errorf("invalid guild ID: %w", err)
	}

	ch, err := a.st.Channel(discord.ChannelID(channelID))
	if err != nil {
		return fmt.Errorf("channel lookup failed: %w", err)
	}

	if ch.GuildID != discord.GuildID(guildIDParsed) {
		return fmt.Errorf("channel guild mismatch")
	}
	if ch.Type != discord.GuildText && ch.Type != discord.GuildNews {
		return fmt.Errorf("channel is not a guild text channel")
	}

	me, err := a.st.Me()
	if err != nil || me == nil {
		return fmt.Errorf("bot identity not available")
	}

	perms, err := a.st.Permissions(discord.ChannelID(channelID), me.ID)
	if err != nil {
		return fmt.Errorf("permission check failed: %w", err)
	}

	required := discord.PermissionViewChannel | discord.PermissionSendMessages | discord.PermissionEmbedLinks
	if perms&required != required {
		return fmt.Errorf("missing permissions (need view/send/embed)")
	}
	return nil
}
