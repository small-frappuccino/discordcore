package config

import (
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

func roleOpt(name, roleID string) *discordgo.ApplicationCommandInteractionDataOption {
	return &discordgo.ApplicationCommandInteractionDataOption{
		Name:  name,
		Type:  discordgo.ApplicationCommandOptionRole,
		Value: roleID,
	}
}

func TestCommandsEnabledSubCommandPersistsFeatureToggle(t *testing.T) {
	const (
		guildID = "guild-1"
		ownerID = "owner-1"
	)

	session, rec := newConfigCommandTestSession(t)
	router, cm := newConfigCommandTestRouter(t, session, guildID, ownerID)
	mustUpdateConfig(t, cm, func(cfg *files.BotConfig) {
		falseValue := false
		cfg.Guilds[0].Features.Services.Commands = &falseValue
	})

	router.HandleInteraction(session, newConfigSlashInteraction(guildID, ownerID, commandsEnabledSubCommandName, []*discordgo.ApplicationCommandInteractionDataOption{
		boolOpt(commandEnabledOptionName, true),
	}))
	resp := rec.lastResponse(t)
	assertPublicResponse(t, resp)
	if !strings.Contains(resp.Data.Content, "now enabled") {
		t.Fatalf("unexpected commands_enabled response: %q", resp.Data.Content)
	}

	snapshot := cm.Config()
	if snapshot == nil || !snapshot.ResolveFeatures(guildID).Services.Commands {
		t.Fatalf("expected commands feature enabled after slash update, got %+v", snapshot.ResolveFeatures(guildID).Services)
	}
}

func TestCommandChannelSubCommandSetsChannel(t *testing.T) {
	const (
		guildID = "guild-1"
		ownerID = "owner-1"
	)

	session, rec := newConfigCommandTestSession(t)
	router, cm := newConfigCommandTestRouter(t, session, guildID, ownerID)

	router.HandleInteraction(session, newConfigSlashInteraction(guildID, ownerID, commandChannelSubCommandName, []*discordgo.ApplicationCommandInteractionDataOption{
		channelOpt(commandChannelOptionName, "987654321098765432"),
	}))
	resp := rec.lastResponse(t)
	assertPublicResponse(t, resp)
	if !strings.Contains(resp.Data.Content, "<#987654321098765432>") {
		t.Fatalf("unexpected command_channel response: %q", resp.Data.Content)
	}

	guild := cm.GuildConfig(guildID)
	if guild == nil || guild.Channels.Commands != "987654321098765432" {
		t.Fatalf("expected command channel persisted, got %+v", guild)
	}
}

func TestAllowedRoleCommandsAddListAndRemove(t *testing.T) {
	const (
		guildID = "guild-1"
		ownerID = "owner-1"
	)

	session, rec := newConfigCommandTestSession(t)
	router, cm := newConfigCommandTestRouter(t, session, guildID, ownerID)

	router.HandleInteraction(session, newConfigSlashInteraction(guildID, ownerID, allowedRoleAddSubCommandName, []*discordgo.ApplicationCommandInteractionDataOption{
		roleOpt(allowedRoleOptionName, "role-123"),
	}))
	addResp := rec.lastResponse(t)
	assertPublicResponse(t, addResp)
	if !strings.Contains(addResp.Data.Content, "role-123") {
		t.Fatalf("unexpected allowed_role_add response: %q", addResp.Data.Content)
	}

	router.HandleInteraction(session, newConfigSlashInteraction(guildID, ownerID, allowedRoleListSubCommandName, nil))
	listResp := rec.lastResponse(t)
	assertEphemeralResponse(t, listResp)
	if !strings.Contains(listResp.Data.Content, "<@&role-123>") {
		t.Fatalf("unexpected allowed_role_list response: %q", listResp.Data.Content)
	}

	router.HandleInteraction(session, newConfigSlashInteraction(guildID, ownerID, allowedRoleRemoveSubCommandName, []*discordgo.ApplicationCommandInteractionDataOption{
		roleOpt(allowedRoleOptionName, "role-123"),
	}))
	removeResp := rec.lastResponse(t)
	assertPublicResponse(t, removeResp)
	if !strings.Contains(removeResp.Data.Content, "role-123") {
		t.Fatalf("unexpected allowed_role_remove response: %q", removeResp.Data.Content)
	}

	guild := cm.GuildConfig(guildID)
	if guild == nil || len(guild.Roles.Allowed) != 0 {
		t.Fatalf("expected allowed roles to be empty after removal, got %+v", guild)
	}
}