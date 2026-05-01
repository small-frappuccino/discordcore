package config

import (
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

type configCommandTestHarness struct {
	session *discordgo.Session
	rec     *interactionRecorder
	router  *core.CommandRouter
	cm      *files.ConfigManager
	guildID string
	ownerID string
}

func newConfigCommandTestHarness(t *testing.T, guildID, ownerID string) *configCommandTestHarness {
	t.Helper()

	session, rec := newConfigCommandTestSession(t)
	router, cm := newConfigCommandTestRouter(t, session, guildID, ownerID)
	return &configCommandTestHarness{
		session: session,
		rec:     rec,
		router:  router,
		cm:      cm,
		guildID: guildID,
		ownerID: ownerID,
	}
}

func (h *configCommandTestHarness) runSlash(
	t *testing.T,
	subCommand string,
	options ...*discordgo.ApplicationCommandInteractionDataOption,
) discordgo.InteractionResponse {
	t.Helper()

	h.router.HandleInteraction(h.session, newConfigSlashInteraction(h.guildID, h.ownerID, subCommand, options))
	return h.rec.lastResponse(t)
}

func mustUpdateConfig(t *testing.T, cm *files.ConfigManager, fn func(*files.BotConfig)) {
	t.Helper()

	_, err := cm.UpdateConfig(func(cfg *files.BotConfig) error {
		if fn != nil {
			fn(cfg)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("update config: %v", err)
	}
}

func mustSetGuildQOTDConfig(t *testing.T, cm *files.ConfigManager, guildID string, cfg files.QOTDConfig) {
	t.Helper()

	mustUpdateConfig(t, cm, func(config *files.BotConfig) {
		for idx := range config.Guilds {
			if config.Guilds[idx].GuildID != guildID {
				continue
			}
			config.Guilds[idx].QOTD = cfg
		}
	})
}

func buildTestQOTDConfig(enabled bool, channelID string, schedule files.QOTDPublishScheduleConfig) files.QOTDConfig {
	return files.QOTDConfig{
		ActiveDeckID: files.LegacyQOTDDefaultDeckID,
		Schedule:     schedule,
		Decks: []files.QOTDDeckConfig{{
			ID:        files.LegacyQOTDDefaultDeckID,
			Name:      files.LegacyQOTDDefaultDeckName,
			Enabled:   enabled,
			ChannelID: channelID,
		}},
	}
}

func testCommandSchedule() files.QOTDPublishScheduleConfig {
	hourUTC := 12
	minuteUTC := 43
	return files.QOTDPublishScheduleConfig{
		HourUTC:   &hourUTC,
		MinuteUTC: &minuteUTC,
	}
}

func boolOpt(name string, value bool) *discordgo.ApplicationCommandInteractionDataOption {
	return &discordgo.ApplicationCommandInteractionDataOption{
		Name:  name,
		Type:  discordgo.ApplicationCommandOptionBoolean,
		Value: value,
	}
}

func channelOpt(name, channelID string) *discordgo.ApplicationCommandInteractionDataOption {
	return &discordgo.ApplicationCommandInteractionDataOption{
		Name:  name,
		Type:  discordgo.ApplicationCommandOptionChannel,
		Value: channelID,
	}
}

func intOpt(name string, value int64) *discordgo.ApplicationCommandInteractionDataOption {
	return &discordgo.ApplicationCommandInteractionDataOption{
		Name:  name,
		Type:  discordgo.ApplicationCommandOptionInteger,
		Value: float64(value),
	}
}

func assertPublicContains(t *testing.T, resp discordgo.InteractionResponse, want string) {
	t.Helper()

	assertPublicResponse(t, resp)
	if !strings.Contains(resp.Data.Content, want) {
		t.Fatalf("expected public response to contain %q, got %q", want, resp.Data.Content)
	}
}

func assertPublicResponse(t *testing.T, resp discordgo.InteractionResponse) {
	t.Helper()

	if resp.Data.Flags&discordgo.MessageFlagsEphemeral != 0 {
		t.Fatalf("expected public response, got flags=%v content=%q", resp.Data.Flags, resp.Data.Content)
	}
}

func assertActiveQOTDDeckState(
	t *testing.T,
	cm *files.ConfigManager,
	guildID string,
	wantChannel string,
	wantEnabled bool,
	wantSchedule files.QOTDPublishScheduleConfig,
) {
	t.Helper()

	qotdConfig, err := cm.QOTDConfig(guildID)
	if err != nil {
		t.Fatalf("QOTDConfig() failed: %v", err)
	}
	deck, ok := qotdConfig.ActiveDeck()
	if !ok {
		t.Fatalf("expected active deck after update: %+v", qotdConfig)
	}
	if deck.ChannelID != wantChannel || deck.Enabled != wantEnabled {
		t.Fatalf("unexpected active deck state: got channel=%q enabled=%v want channel=%q enabled=%v", deck.ChannelID, deck.Enabled, wantChannel, wantEnabled)
	}
	if !testSchedulesEqual(qotdConfig.Schedule, wantSchedule) {
		t.Fatalf("unexpected qotd schedule: got %+v want %+v", qotdConfig.Schedule, wantSchedule)
	}
}

func testSchedulesEqual(left, right files.QOTDPublishScheduleConfig) bool {
	if (left.HourUTC == nil) != (right.HourUTC == nil) {
		return false
	}
	if left.HourUTC != nil && *left.HourUTC != *right.HourUTC {
		return false
	}
	if (left.MinuteUTC == nil) != (right.MinuteUTC == nil) {
		return false
	}
	if left.MinuteUTC != nil && *left.MinuteUTC != *right.MinuteUTC {
		return false
	}
	return true
}
