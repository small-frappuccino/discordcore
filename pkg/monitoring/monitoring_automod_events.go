package monitoring

import (
	"context"
	"fmt"
	"time"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/small-frappuccino/discordcore/pkg/logpolicy"
	"github.com/small-frappuccino/discordcore/pkg/task"
	"github.com/small-frappuccino/discordcore/pkg/theme"
	"github.com/small-frappuccino/discordgo"
)

// TaskTypeProcessAutomodEvent is the task type for processing automod events safely.
const TaskTypeProcessAutomodEvent = "monitor.process_automod_event"

// ProcessAutomodEventPayload holds the AutoMod execution trigger.
type ProcessAutomodEventPayload struct {
	GuildID discord.GuildID
	Entry   *discordgo.AutoModerationActionExecution
}

// registerAutomodHandlers wires the automod processing task handlers.
func (ms *MonitoringService) registerAutomodHandlers() {
	if ms.router != nil {
		ms.router.RegisterHandler(TaskTypeProcessAutomodEvent, ms.handleProcessAutomodEvent)
	}
}

// OnAutomodBlock implements automod.Sink.
func (ms *MonitoringService) OnAutomodBlock(ctx context.Context, guildID discord.GuildID, entry *discordgo.AutoModerationActionExecution) {
	if !ms.IsRunning() || !ms.handlesGuild(guildID.String()) {
		return
	}
	if !ms.isFeatureBot(guildID.String(), "moderation") {
		return
	}

	payload := ProcessAutomodEventPayload{
		GuildID: guildID,
		Entry:   entry,
	}

	// Calculate idempotency key like we used to do in adapters, but now localized here.
	var idemKey string
	if entry.MessageID != "" {
		idemKey = fmt.Sprintf("automod_exec:%s:%s", guildID.String(), entry.MessageID)
	} else if entry.MatchedContent != "" {
		idemKey = fmt.Sprintf("automod_exec:%s:%s:content", guildID.String(), entry.UserID)
	} else {
		idemKey = fmt.Sprintf("automod_exec:%s:%s:generic", guildID.String(), entry.UserID)
	}

	_ = ms.router.Dispatch(context.Background(), task.Task{
		Type:    TaskTypeProcessAutomodEvent,
		Payload: payload,
		Options: task.TaskOptions{
			GroupKey:       guildID.String(),
			IdempotencyKey: idemKey,
			IdempotencyTTL: 5 * time.Minute,
			MaxAttempts:    3,
			InitialBackoff: 2 * time.Second,
			MaxBackoff:     15 * time.Second,
		},
	})
}

// handleProcessAutomodEvent is called by the TaskRouter to actually resolve and send the embed.
func (ms *MonitoringService) handleProcessAutomodEvent(ctx context.Context, payload any) error {
	p, ok := payload.(ProcessAutomodEventPayload)
	if !ok {
		return fmt.Errorf("invalid payload for %s", TaskTypeProcessAutomodEvent)
	}

	guildID := p.GuildID.String()
	entry := p.Entry

	emit := logpolicy.ShouldEmitLogEvent(ms.session, ms.configManager, logpolicy.LogEventAutomodAction, guildID)
	if !emit.Enabled {
		ms.logger.Debug("Automod action notification suppressed by policy", "guildID", guildID, "reason", emit.Reason)
		return nil
	}

	desc := "Blocked content detected (AutoMod)."
	if entry.RuleTriggerType != 0 {
		desc = fmt.Sprintf("AutoMod rule **%s** triggered.", entry.RuleID)
	}

	embed := &discord.Embed{
		Title:       "AutoMod • Action Executed",
		Description: desc,
		Color:       discord.Color(theme.AutomodAction()),
		Timestamp:   discord.NewTimestamp(time.Now()),
		Fields: []discord.EmbedField{
			{Name: "User", Value: fmt.Sprintf("<@%s>", entry.UserID), Inline: true},
		},
	}

	if entry.ChannelID != "" {
		embed.Fields = append(embed.Fields, discord.EmbedField{
			Name: "Channel", Value: fmt.Sprintf("<#%s>", entry.ChannelID), Inline: true,
		})
	}

	if entry.MatchedKeyword != "" {
		embed.Fields = append(embed.Fields, discord.EmbedField{
			Name: "Keyword", Value: entry.MatchedKeyword, Inline: true,
		})
	}

	if entry.MatchedContent != "" {
		embed.Fields = append(embed.Fields, discord.EmbedField{
			Name: "Content", Value: entry.MatchedContent, Inline: false,
		})
	}

	_, err := ms.arikawaState.SendMessage(
		discord.ChannelID(mustParseSnowflake(emit.ChannelID)),
		"",
		*embed,
	)
	if err != nil {
		ms.logger.Error("Failed to send automod action notification", "guildID", guildID, "channelID", emit.ChannelID, "err", err)
		return err
	}

	return nil
}

// mustParseSnowflake safely parses a string to discord.Snowflake.
func mustParseSnowflake(s string) discord.Snowflake {
	parsed, _ := discord.ParseSnowflake(s)
	return parsed
}
