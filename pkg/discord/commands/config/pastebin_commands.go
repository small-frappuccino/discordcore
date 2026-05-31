package config

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

// PastebinConfigSubCommand implements the /config pastebin command
type PastebinConfigSubCommand struct {
	configManager *files.ConfigManager
}

func NewPastebinConfigSubCommand(configManager *files.ConfigManager) *PastebinConfigSubCommand {
	return &PastebinConfigSubCommand{configManager: configManager}
}

func (c *PastebinConfigSubCommand) Name() string { return "pastebin" }

func (c *PastebinConfigSubCommand) Description() string {
	return "Configure global Pastebin credentials (exclusive to Administrators)"
}

func (c *PastebinConfigSubCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "api_dev_key",
			Description: "Your Pastebin Developer API Key",
			Required:    true,
		},
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "api_user_name",
			Description: "Your Pastebin Username",
			Required:    true,
		},
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "api_user_password",
			Description: "Your Pastebin Password",
			Required:    true,
		},
	}
}

func (c *PastebinConfigSubCommand) RequiresGuild() bool       { return true }
func (c *PastebinConfigSubCommand) RequiresPermissions() bool { return true }

func (c *PastebinConfigSubCommand) Handle(ctx *core.Context) error {
	// Restrict exclusively to members with administrative permissions
	if !isAdministrator(ctx) {
		return core.NewCommandError("You must have administrative permissions (Administrator or Manage Server) to run this command.", true)
	}

	extractor := core.NewOptionExtractor(core.GetSubCommandOptions(ctx.Interaction))
	devKey, err := extractor.StringRequired("api_dev_key")
	if err != nil {
		return fmt.Errorf("PastebinConfigSubCommand.Handle: %w", err)
	}
	userName, err := extractor.StringRequired("api_user_name")
	if err != nil {
		return fmt.Errorf("PastebinConfigSubCommand.Handle: %w", err)
	}
	password, err := extractor.StringRequired("api_user_password")
	if err != nil {
		return fmt.Errorf("PastebinConfigSubCommand.Handle: %w", err)
	}

	// Update global RuntimeConfig
	_, err = c.configManager.UpdateRuntimeConfig(func(rc *files.RuntimeConfig) error {
		rc.PastebinDevKey = files.EncryptedString(devKey)
		rc.PastebinUserName = files.EncryptedString(userName)
		rc.PastebinUserPassword = files.EncryptedString(password)
		return nil
	})
	if err != nil {
		ctx.Logger.Error().Errorf("Failed to save Pastebin credentials: %v", err)
		return core.NewCommandError("Failed to save Pastebin credentials. Please try again.", true)
	}

	return configCommandShortConfirmationResponseBuilder(ctx.Session).Success(
		ctx.Interaction,
		"Successfully saved global Pastebin credentials securely.",
	)
}

func isAdministrator(ctx *core.Context) bool {
	if ctx.Interaction.Member == nil {
		return false
	}
	// Check if owner
	guild, err := ctx.Session.State.Guild(ctx.GuildID)
	if err == nil && guild.OwnerID == ctx.UserID {
		return true
	}
	// Check permissions
	perms := ctx.Interaction.Member.Permissions
	if (perms & discordgo.PermissionAdministrator) != 0 {
		return true
	}
	if (perms & discordgo.PermissionManageGuild) != 0 {
		return true
	}
	return false
}
