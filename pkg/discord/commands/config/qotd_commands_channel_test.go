package config

import (
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/files"
)

func TestQOTDConfigChannelCommandSetsActiveDeckChannel(t *testing.T) {
	const (
		guildID = "guild-1"
		ownerID = "owner-1"
	)

	harness := newConfigCommandTestHarness(t, guildID, ownerID)

	resp := harness.runSlash(t, qotdChannelSubCommandName,
		channelOpt(qotdChannelOptionName, "123456789012345678"),
	)

	assertPublicContains(t, resp, "QOTD posts for deck")
	assertActiveQOTDDeckState(t, harness.cm, guildID, "123456789012345678", false, files.QOTDPublishScheduleConfig{})
}