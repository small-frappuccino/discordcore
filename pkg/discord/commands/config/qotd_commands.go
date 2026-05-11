package config

import (
	"errors"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	"github.com/small-frappuccino/discordcore/pkg/files"
	qotdservice "github.com/small-frappuccino/discordcore/pkg/qotd"
)

const (
	qotdGetSubCommandName = "qotd_get"
	qotdEnabledSubCommandName = "qotd_enabled"
	qotdChannelSubCommandName = "qotd_channel"
	qotdScheduleSubCommandName = "qotd_schedule"
	qotdEnabledOptionName     = "enabled"
	qotdChannelOptionName     = "channel"
	qotdScheduleHourOptionName = "hour"
	qotdScheduleMinuteOptionName = "minute"
)

type QOTDGetSubCommand struct {
	configManager *files.ConfigManager
	now           func() time.Time
}

func NewQOTDGetSubCommand(configManager *files.ConfigManager) *QOTDGetSubCommand {
	return &QOTDGetSubCommand{configManager: configManager, now: qotdConfigClock(nil)}
}

func NewQOTDGetSubCommandWithClock(configManager *files.ConfigManager, now func() time.Time) *QOTDGetSubCommand {
	return &QOTDGetSubCommand{configManager: configManager, now: qotdConfigClock(now)}
}

func (c *QOTDGetSubCommand) Name() string { return qotdGetSubCommandName }

func (c *QOTDGetSubCommand) Description() string {
	return "Show current QOTD configuration for the active deck"
}

func (c *QOTDGetSubCommand) Options() []*discordgo.ApplicationCommandOption { return nil }

func (c *QOTDGetSubCommand) RequiresGuild() bool       { return true }
func (c *QOTDGetSubCommand) RequiresPermissions() bool { return true }

func (c *QOTDGetSubCommand) Handle(ctx *core.Context) error {
	if err := core.RequiresGuildConfig(ctx); err != nil {
		return err
	}

	locale := ctx.Locale()
	settings := files.DashboardQOTDConfig(ctx.GuildConfig.QOTD)
	deck, _ := settings.ActiveDeck()
	deckLabel := strings.TrimSpace(deck.Name)
	if deckLabel == "" {
		deckLabel = strings.TrimSpace(deck.ID)
	}
	deckCount := len(settings.Decks)

	lines := []string{tc(locale, cfgMsgHeader)}
	if deckLabel != "" {
		if deckCount > 1 {
			lines = append(lines, tc(locale, cfgMsgActiveDeckMulti, deckLabel, deckCount))
		} else {
			lines = append(lines, tc(locale, cfgMsgActiveDeckSingle, deckLabel))
		}
	}
	lines = append(lines, tc(locale, cfgMsgEnabledLine, deck.Enabled))
	lines = append(lines, tc(locale, cfgMsgChannelLine, emptyToDash(deck.ChannelID)))
	lines = append(lines, tc(locale, cfgMsgScheduleLine, formatQOTDScheduleWithLocalPreview(settings.Schedule, c.now())))

	builder := configCommandCurrentStateResponseBuilder(ctx.Session).
		WithEmbed().
		WithTitle(tc(locale, cfgMsgEmbedTitle))

	return builder.Info(ctx.Interaction, strings.Join(lines, "\n"))
}

type QOTDEnabledSubCommand struct {
	configManager *files.ConfigManager
	now           func() time.Time
}

func NewQOTDEnabledSubCommand(configManager *files.ConfigManager, now func() time.Time) *QOTDEnabledSubCommand {
	return &QOTDEnabledSubCommand{configManager: configManager, now: qotdConfigClock(now)}
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
	locale := ctx.Locale()
	extractor := core.NewOptionExtractor(core.GetSubCommandOptions(ctx.Interaction))
	enabled := extractor.Bool(qotdEnabledOptionName)

	updatedDeck, err := updateActiveQOTDDeck(ctx, c.configManager, c.now, func(deck *files.QOTDDeckConfig) error {
		if enabled && strings.TrimSpace(deck.ChannelID) == "" {
			return qotdConfigDetailedCommandError(tc(locale, cfgMsgErrNoChannel))
		}
		deck.Enabled = enabled
		return nil
	})
	if err != nil {
		return err
	}

	state := tc(locale, cfgMsgStateDisabled)
	if updatedDeck.Enabled {
		state = tc(locale, cfgMsgStateEnabled)
	}

	return qotdConfigShortConfirmationResponseBuilder(ctx.Session).
		Success(ctx.Interaction, tc(locale, cfgMsgPublishingState, state, updatedDeck.Name))
}

type QOTDChannelSubCommand struct {
	configManager *files.ConfigManager
	now           func() time.Time
}

func NewQOTDChannelSubCommand(configManager *files.ConfigManager, now func() time.Time) *QOTDChannelSubCommand {
	return &QOTDChannelSubCommand{configManager: configManager, now: qotdConfigClock(now)}
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
	locale := ctx.Locale()
	channelID := channelOptionID(ctx.Session, core.GetSubCommandOptions(ctx.Interaction), qotdChannelOptionName)
	if channelID == "" {
		return qotdConfigDetailedCommandError(tc(locale, cfgMsgErrSetChannelFirst))
	}

	updatedDeck, err := updateActiveQOTDDeck(ctx, c.configManager, c.now, func(deck *files.QOTDDeckConfig) error {
		deck.ChannelID = channelID
		return nil
	})
	if err != nil {
		return err
	}

	state := tc(locale, cfgMsgStateDisabled)
	if updatedDeck.Enabled {
		state = tc(locale, cfgMsgStateEnabled)
	}

	return qotdConfigShortConfirmationResponseBuilder(ctx.Session).
		Success(ctx.Interaction, tc(locale, cfgMsgChannelSet, updatedDeck.Name, channelID, state))
}

type QOTDScheduleSubCommand struct {
	configManager *files.ConfigManager
	now           func() time.Time
}

func NewQOTDScheduleSubCommand(configManager *files.ConfigManager, now func() time.Time) *QOTDScheduleSubCommand {
	return &QOTDScheduleSubCommand{configManager: configManager, now: qotdConfigClock(now)}
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
	locale := ctx.Locale()
	extractor := core.NewOptionExtractor(core.GetSubCommandOptions(ctx.Interaction))
	hourUTC := int(extractor.Int(qotdScheduleHourOptionName))
	minuteUTC := int(extractor.Int(qotdScheduleMinuteOptionName))

	updatedConfig, err := updateQOTDConfig(ctx, c.configManager, c.now, func(cfg *files.QOTDConfig) error {
		cfg.Schedule = files.QOTDPublishScheduleConfig{
			HourUTC:   &hourUTC,
			MinuteUTC: &minuteUTC,
		}
		return nil
	})
	if err != nil {
		return err
	}

	deckLabel := activeDeckDisplayLabel(locale, updatedConfig)
	return qotdConfigShortConfirmationResponseBuilder(ctx.Session).
		Success(ctx.Interaction, tc(locale, cfgMsgScheduleSet, deckLabel, formatQOTDScheduleWithLocalPreview(updatedConfig.Schedule, c.now())))
}

func updateQOTDConfig(
	ctx *core.Context,
	configManager *files.ConfigManager,
	now func() time.Time,
	mutate func(*files.QOTDConfig) error,
) (files.QOTDConfig, error) {
	if err := core.RequiresGuildConfig(ctx); err != nil {
		return files.QOTDConfig{}, err
	}

	locale := ctx.Locale()
	var updatedConfig files.QOTDConfig
	err := core.SafeGuildAccess(ctx, func(guildConfig *files.GuildConfig) error {
		current := files.DashboardQOTDConfig(guildConfig.QOTD)
		next := files.CloneQOTDConfig(current)
		if err := mutate(&next); err != nil {
			return err
		}

		normalized, err := qotdservice.PrepareSettingsUpdate(current, next, now())
		if err != nil {
			return translateQOTDConfigError(locale, err)
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
		return files.QOTDConfig{}, qotdConfigDetailedCommandError(tc(locale, cfgMsgErrSaveFailed))
	}

	return updatedConfig, nil
}

func updateActiveQOTDDeck(
	ctx *core.Context,
	configManager *files.ConfigManager,
	now func() time.Time,
	mutate func(*files.QOTDDeckConfig) error,
) (files.QOTDDeckConfig, error) {
	locale := ctx.Locale()
	updatedConfig, err := updateQOTDConfig(ctx, configManager, now, func(cfg *files.QOTDConfig) error {
		deckIndex := activeQOTDDeckIndex(*cfg)
		if deckIndex < 0 {
			return qotdConfigDetailedCommandError(tc(locale, cfgMsgErrSetupNotLoaded))
		}
		return mutate(&cfg.Decks[deckIndex])
	})
	if err != nil {
		return files.QOTDDeckConfig{}, err
	}
	deckIndex := activeQOTDDeckIndex(updatedConfig)
	if deckIndex < 0 {
		return files.QOTDDeckConfig{}, qotdConfigDetailedCommandError(tc(locale, cfgMsgErrSetupNotLoaded))
	}
	return updatedConfig.Decks[deckIndex], nil
}

func qotdConfigClock(now func() time.Time) func() time.Time {
	if now == nil {
		return func() time.Time {
			return time.Now().UTC()
		}
	}
	return func() time.Time {
		return now().UTC()
	}
}

// activeDeckDisplayLabel picks the most user-friendly identifier for the
// active deck — the human-readable name when set, otherwise the deck ID,
// finally a sentinel so messages never trail with empty backticks.
func activeDeckDisplayLabel(locale discordgo.Locale, cfg files.QOTDConfig) string {
	deck, _ := cfg.ActiveDeck()
	label := strings.TrimSpace(deck.Name)
	if label == "" {
		label = strings.TrimSpace(deck.ID)
	}
	if label == "" {
		return tc(locale, cfgMsgDeckDefault)
	}
	return label
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

func translateQOTDConfigError(locale discordgo.Locale, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, files.ErrInvalidQOTDInput) {
		message := strings.TrimSpace(strings.TrimPrefix(err.Error(), files.ErrInvalidQOTDInput.Error()+":"))
		if message == "" {
			message = tc(locale, cfgMsgErrInvalidInput)
		}
		if message == "schedule.hour_utc and schedule.minute_utc are required when enabled" {
			message = tc(locale, cfgMsgErrIncompleteSchedule)
		}
		return qotdConfigDetailedCommandError(message)
	}
	return err
}