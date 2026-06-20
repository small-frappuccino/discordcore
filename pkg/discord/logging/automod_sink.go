package logging

import (
	"context"
	"fmt"
	"time"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/small-frappuccino/discordcore/pkg/automod"
	"github.com/small-frappuccino/discordcore/pkg/discord/embeds"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/logging"
	"github.com/small-frappuccino/discordcore/pkg/theme"
)

// OnAutomodBlock implements automod.Sink for logging automod actions.
func (l *Logger) OnAutomodBlock(ctx context.Context, guildID discord.GuildID, entry *automod.ExecutionEvent) {
	decision, ok := l.checkPolicy(logging.LogEventAutomodAction, guildID.String())
	if !ok {
		return
	}

	channelID, err := discord.ParseSnowflake(decision.ChannelID)
	if err != nil {
		return
	}

	desc := "Blocked content detected (AutoMod)."
	if entry.RuleTriggerType != 0 {
		desc = fmt.Sprintf("AutoMod rule **%s** triggered.", entry.RuleID.String())
	}

	ce := files.CustomEmbedConfig{
		Title:       "AutoMod • Action Executed",
		Description: desc,
		Color:       theme.AutomodAction(),
		Fields: []files.CustomEmbedFieldConfig{
			{Name: "User", Value: fmt.Sprintf("<@%s>", entry.UserID.String()), Inline: true},
		},
	}

	if entry.ChannelID.IsValid() {
		ce.Fields = append(ce.Fields, files.CustomEmbedFieldConfig{
			Name: "Channel", Value: fmt.Sprintf("<#%s>", entry.ChannelID.String()), Inline: true,
		})
	}
	if entry.MatchedKeyword != "" {
		ce.Fields = append(ce.Fields, files.CustomEmbedFieldConfig{
			Name: "Keyword", Value: entry.MatchedKeyword, Inline: true,
		})
	}
	if entry.MatchedContent != "" {
		ce.Fields = append(ce.Fields, files.CustomEmbedFieldConfig{
			Name: "Matched Content", Value: logging.TruncateString(entry.MatchedContent, 1000), Inline: false,
		})
	}

	embed := embeds.Render(ce)
	embed.Timestamp = discord.NewTimestamp(time.Now())

	l.sendEmbed(ctx, discord.ChannelID(channelID), embed, logging.LogEventAutomodAction)
}
