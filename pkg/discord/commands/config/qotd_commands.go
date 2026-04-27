package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

const (
	qotdEnabledSubCommandName = "qotd_enabled"
	qotdChannelSubCommandName = "qotd_channel"
	qotdEnabledOptionName     = "enabled"
	qotdChannelOptionName     = "channel"
)

type QOTDEnabledSubCommand struct {
	configManager *files.ConfigManager
}

func NewQOTDEnabledSubCommand(configManager *files.ConfigManager) *QOTDEnabledSubCommand {
	return &QOTDEnabledSubCommand{configManager: configManager}
}

func (c *QOTDEnabledSubCommand) Name() string { return qotdEnabledSubCommandName }

func (c *QOTDEnabledSubCommand) Description() string {
	return "Enable or disable QOTD publishing for the active deck"
}

func (c *QOTDEnabledSubCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{{
		Type:        discordgo.ApplicationCommandOptionBoolean,
		Name:        qotdEnabledOptionName,
		Description: "Whether QOTD publishing should be enabled",
		Required:    true,
	}}
}

func (c *QOTDEnabledSubCommand) RequiresGuild() bool       { return true }
func (c *QOTDEnabledSubCommand) RequiresPermissions() bool { return true }

func (c *QOTDEnabledSubCommand) Handle(ctx *core.Context) error {
	extractor := core.NewOptionExtractor(core.GetSubCommandOptions(ctx.Interaction))
	enabled := extractor.Bool(qotdEnabledOptionName)

	updatedDeck, err := updateActiveQOTDDeck(ctx, c.configManager, func(deck *files.QOTDDeckConfig) error {
		if enabled && strings.TrimSpace(deck.ChannelID) == "" {
			return core.NewCommandError("Set a QOTD channel before enabling publishing", true)
		}
		deck.Enabled = enabled
		return nil
	})
	if err != nil {
		return err
	}

	state := "disabled"
	if updatedDeck.Enabled {
		state = "enabled"
	}

	return core.NewResponseBuilder(ctx.Session).
		Ephemeral().
		Success(ctx.Interaction, fmt.Sprintf("QOTD is now %s for deck `%s`.", state, updatedDeck.Name))
}

type QOTDChannelSubCommand struct {
	configManager *files.ConfigManager
}

func NewQOTDChannelSubCommand(configManager *files.ConfigManager) *QOTDChannelSubCommand {
	return &QOTDChannelSubCommand{configManager: configManager}
}

func (c *QOTDChannelSubCommand) Name() string { return qotdChannelSubCommandName }

func (c *QOTDChannelSubCommand) Description() string {
	return "Set the QOTD delivery channel for the active deck"
}

func (c *QOTDChannelSubCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{{
		Type:         discordgo.ApplicationCommandOptionChannel,
		Name:         qotdChannelOptionName,
		Description:  "Existing text channel that receives the daily QOTD post",
		Required:     true,
		ChannelTypes: []discordgo.ChannelType{discordgo.ChannelTypeGuildText},
	}}
}

func (c *QOTDChannelSubCommand) RequiresGuild() bool       { return true }
func (c *QOTDChannelSubCommand) RequiresPermissions() bool { return true }

func (c *QOTDChannelSubCommand) Handle(ctx *core.Context) error {
	channelID := channelOptionID(ctx.Session, core.GetSubCommandOptions(ctx.Interaction), qotdChannelOptionName)
	if channelID == "" {
		return core.NewCommandError("Channel is required", true)
	}

	updatedDeck, err := updateActiveQOTDDeck(ctx, c.configManager, func(deck *files.QOTDDeckConfig) error {
		deck.ChannelID = channelID
		return nil
	})
	if err != nil {
		return err
	}

	state := "disabled"
	if updatedDeck.Enabled {
		state = "enabled"
	}

	return core.NewResponseBuilder(ctx.Session).
		Ephemeral().
		Success(ctx.Interaction, fmt.Sprintf("QOTD channel set to <#%s> for deck `%s`. Publishing remains %s.", channelID, updatedDeck.Name, state))
}

func updateActiveQOTDDeck(
	ctx *core.Context,
	configManager *files.ConfigManager,
	mutate func(*files.QOTDDeckConfig) error,
) (files.QOTDDeckConfig, error) {
	if err := core.RequiresGuildConfig(ctx); err != nil {
		return files.QOTDDeckConfig{}, err
	}

	var updatedDeck files.QOTDDeckConfig
	err := core.SafeGuildAccess(ctx, func(guildConfig *files.GuildConfig) error {
		next := files.DashboardQOTDConfig(guildConfig.QOTD)
		deckIndex := activeQOTDDeckIndex(next)
		if deckIndex < 0 {
			return core.NewCommandError("QOTD configuration is unavailable", true)
		}
		if err := mutate(&next.Decks[deckIndex]); err != nil {
			return err
		}

		normalized, err := files.NormalizeQOTDConfig(next)
		if err != nil {
			return translateQOTDConfigError(err)
		}
		guildConfig.QOTD = normalized

		display := files.DashboardQOTDConfig(normalized)
		deckIndex = activeQOTDDeckIndex(display)
		if deckIndex >= 0 {
			updatedDeck = display.Decks[deckIndex]
		}
		return nil
	})
	if err != nil {
		return files.QOTDDeckConfig{}, err
	}

	persister := core.NewConfigPersister(configManager)
	if err := persister.Save(ctx.GuildConfig); err != nil {
		ctx.Logger.Error().Errorf("Failed to save QOTD config: %v", err)
		return files.QOTDDeckConfig{}, core.NewCommandError("Failed to save configuration", true)
	}

	return updatedDeck, nil
}

func activeQOTDDeckIndex(cfg files.QOTDConfig) int {
	activeDeckID := strings.TrimSpace(cfg.ActiveDeckID)
	for idx := range cfg.Decks {
		if strings.TrimSpace(cfg.Decks[idx].ID) == activeDeckID {
			return idx
		}
	}
	if len(cfg.Decks) == 0 {
		return -1
	}
	return 0
}

func channelOptionID(session *discordgo.Session, options []*discordgo.ApplicationCommandInteractionDataOption, name string) string {
	for _, option := range options {
		if option == nil || option.Name != name {
			continue
		}
		if channel := option.ChannelValue(session); channel != nil {
			return strings.TrimSpace(channel.ID)
		}
		if value, ok := option.Value.(string); ok {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func translateQOTDConfigError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, files.ErrInvalidQOTDInput) {
		message := strings.TrimSpace(strings.TrimPrefix(err.Error(), files.ErrInvalidQOTDInput.Error()+":"))
		if message == "" {
			message = "Invalid QOTD configuration"
		}
		return core.NewCommandError(message, true)
	}
	return err
}