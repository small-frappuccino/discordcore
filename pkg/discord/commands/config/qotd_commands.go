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
	qotdScheduleSubCommandName = "qotd_schedule"
	qotdEnabledOptionName     = "enabled"
	qotdChannelOptionName     = "channel"
	qotdScheduleHourOptionName = "hour"
	qotdScheduleMinuteOptionName = "minute"
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
			return qotdConfigDetailedCommandError("QOTD publishing couldn't be turned on yet because this deck still has no channel. This reply stays private so that can be fixed first.")
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

	return qotdConfigShortConfirmationResponseBuilder(ctx.Session).
		Success(ctx.Interaction, fmt.Sprintf("QOTD publishing is now %s for deck `%s`.", state, updatedDeck.Name))
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
		return qotdConfigDetailedCommandError("This change needs a channel before it can be applied, so this reply stays private.")
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

	return qotdConfigShortConfirmationResponseBuilder(ctx.Session).
		Success(ctx.Interaction, fmt.Sprintf("QOTD posts for deck `%s` will now go to <#%s>. Publishing stays %s.", updatedDeck.Name, channelID, state))
}

type QOTDScheduleSubCommand struct {
	configManager *files.ConfigManager
}

func NewQOTDScheduleSubCommand(configManager *files.ConfigManager) *QOTDScheduleSubCommand {
	return &QOTDScheduleSubCommand{configManager: configManager}
}

func (c *QOTDScheduleSubCommand) Name() string { return qotdScheduleSubCommandName }
func (c *QOTDScheduleSubCommand) Description() string {
	return "Set the QOTD publish schedule in UTC"
}
func (c *QOTDScheduleSubCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionInteger,
			Name:        qotdScheduleHourOptionName,
			Description: "UTC hour for scheduled QOTD publishing (0-23)",
			Required:    true,
		},
		{
			Type:        discordgo.ApplicationCommandOptionInteger,
			Name:        qotdScheduleMinuteOptionName,
			Description: "UTC minute for scheduled QOTD publishing (0-59)",
			Required:    true,
		},
	}
}
func (c *QOTDScheduleSubCommand) RequiresGuild() bool       { return true }
func (c *QOTDScheduleSubCommand) RequiresPermissions() bool { return true }
func (c *QOTDScheduleSubCommand) Handle(ctx *core.Context) error {
	extractor := core.NewOptionExtractor(core.GetSubCommandOptions(ctx.Interaction))
	hourUTC := int(extractor.Int(qotdScheduleHourOptionName))
	minuteUTC := int(extractor.Int(qotdScheduleMinuteOptionName))

	updatedConfig, err := updateQOTDConfig(ctx, c.configManager, func(cfg *files.QOTDConfig) error {
		cfg.Schedule = files.QOTDPublishScheduleConfig{
			HourUTC:   &hourUTC,
			MinuteUTC: &minuteUTC,
		}
		return nil
	})
	if err != nil {
		return err
	}

	return qotdConfigShortConfirmationResponseBuilder(ctx.Session).
		Success(ctx.Interaction, fmt.Sprintf("QOTD for the active deck will now post at %s UTC.", formatQOTDSchedule(updatedConfig.Schedule)))
}

func updateQOTDConfig(
	ctx *core.Context,
	configManager *files.ConfigManager,
	mutate func(*files.QOTDConfig) error,
) (files.QOTDConfig, error) {
	if err := core.RequiresGuildConfig(ctx); err != nil {
		return files.QOTDConfig{}, err
	}

	var updatedConfig files.QOTDConfig
	err := core.SafeGuildAccess(ctx, func(guildConfig *files.GuildConfig) error {
		next := files.DashboardQOTDConfig(guildConfig.QOTD)
		if err := mutate(&next); err != nil {
			return err
		}

		normalized, err := files.NormalizeQOTDConfig(next)
		if err != nil {
			return translateQOTDConfigError(err)
		}
		guildConfig.QOTD = normalized
		updatedConfig = files.DashboardQOTDConfig(normalized)
		return nil
	})
	if err != nil {
		return files.QOTDConfig{}, err
	}

	persister := core.NewConfigPersister(configManager)
	if err := persister.Save(ctx.GuildConfig); err != nil {
		ctx.Logger.Error().Errorf("Failed to save QOTD config: %v", err)
		return files.QOTDConfig{}, qotdConfigDetailedCommandError("That change couldn't be saved. This reply stays private so it can be adjusted and retried without extra channel noise.")
	}

	return updatedConfig, nil
}

func updateActiveQOTDDeck(
	ctx *core.Context,
	configManager *files.ConfigManager,
	mutate func(*files.QOTDDeckConfig) error,
) (files.QOTDDeckConfig, error) {
	updatedConfig, err := updateQOTDConfig(ctx, configManager, func(cfg *files.QOTDConfig) error {
		deckIndex := activeQOTDDeckIndex(*cfg)
		if deckIndex < 0 {
			return qotdConfigDetailedCommandError("The QOTD setup for this server couldn't be loaded, so this reply stays private.")
		}
		return mutate(&cfg.Decks[deckIndex])
	})
	if err != nil {
		return files.QOTDDeckConfig{}, err
	}
	deckIndex := activeQOTDDeckIndex(updatedConfig)
	if deckIndex < 0 {
		return files.QOTDDeckConfig{}, qotdConfigDetailedCommandError("The QOTD setup for this server couldn't be loaded, so this reply stays private.")
	}
	return updatedConfig.Decks[deckIndex], nil
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
			message = "That QOTD setup couldn't be applied because part of the configuration is invalid. This reply stays private."
		}
		if message == "schedule.hour_utc and schedule.minute_utc are required when enabled" {
			message = "QOTD publishing couldn't be turned on yet because the schedule is incomplete. This reply stays private so the setup can be finished first."
		}
		return qotdConfigDetailedCommandError(message)
	}
	return err
}