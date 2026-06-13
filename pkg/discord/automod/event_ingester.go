package discordautomod

import (
	"errors"
	"log/slog"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/small-frappuccino/discordcore/pkg/automod"
	"github.com/small-frappuccino/discordcore/pkg/discord/perf"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/logpolicy"
	"github.com/small-frappuccino/discordcore/pkg/task"
)

// handleRawEvent handles the auto moderation action execution event.
func (as *AutomodService) handleRawEvent(e *AutoModerationActionExecutionEvent) {
	if e == nil {
		return
	}

	as.handleAutoModerationAction(e)
}

// handleAutoModerationAction logs native AutoMod events to the configured
// automod log channel.
func (as *AutomodService) handleAutoModerationAction(e *AutoModerationActionExecutionEvent) {
	if e == nil || !e.GuildID.IsValid() {
		return
	}
	guildIDStr := e.GuildID.String()

	done := perf.StartGatewayEvent(
		"auto_moderation_action_execution",
		slog.String("guildID", guildIDStr),
		slog.String("channelID", e.ChannelID.String()),
		slog.String("userID", e.UserID.String()),
		slog.String("ruleID", e.RuleID.String()),
	)
	defer done()

	if int(e.Action.Type) == automodActionSendAlert {
		log.ApplicationLogger().Debug("Dropping SEND_ALERT_MESSAGE automod event; Discord posts its own native alert", "guildID", guildIDStr, "ruleID", e.RuleID.String(), "userID", e.UserID.String())
		return
	}

	if as.configManager == nil {
		return
	}
	guildConfig := as.configManager.GuildConfig(guildIDStr)
	if guildConfig == nil {
		return
	}
	if !guildConfig.BelongsToBotInstance(as.botInstanceID) {
		return
	}
	resolvedID, _ := guildConfig.ResolveFeatureBotInstanceID("moderation", as.defaultBotInstanceID)
	if resolvedID != as.botInstanceID {
		return
	}

	// We pass the Arikawa state so it can check permissions if needed.
	emit := logpolicy.ShouldEmitLogEvent(as.state, as.configManager, logpolicy.LogEventAutomodAction, guildIDStr)
	if !emit.Enabled {
		log.ApplicationLogger().Debug("Automod action notification suppressed by policy", "guildID", guildIDStr, "channelID", e.ChannelID.String(), "userID", e.UserID.String(), "reason", emit.Reason)
		return
	}
	logChannelID := emit.ChannelID

	msgID := ""
	if e.MessageID.IsValid() {
		msgID = e.MessageID.String()
	}
	alertSysMsgID := ""
	if e.AlertSystemMessageID.IsValid() {
		alertSysMsgID = e.AlertSystemMessageID.String()
	}

	domainExec := &automod.ActionExecution{
		GuildID:              guildIDStr,
		ChannelID:            e.ChannelID.String(),
		UserID:               e.UserID.String(),
		RuleID:               e.RuleID.String(),
		ActionType:           int(e.Action.Type),
		TriggerType:          int(e.RuleTriggerType),
		MessageID:            msgID,
		AlertSystemMessageID: alertSysMsgID,
		MatchedKeyword:       e.MatchedKeyword,
		Content:              e.Content,
		MatchedContent:       e.MatchedContent,
	}

	idempotencyKey := task.AutomodIdempotencyKey(domainExec)

	// If adapters are wired, enqueue via TaskRouter for retries/backoff
	if as.adapters != nil {
		if err := as.adapters.EnqueueAutomodActionWithKey(logChannelID, domainExec, idempotencyKey); err != nil {
			if errors.Is(err, task.ErrDuplicateTask) {
				log.ApplicationLogger().Debug("Dropped duplicate automod log task", "guildID", guildIDStr, "channelID", logChannelID, "userID", e.UserID.String(), "ruleID", e.RuleID.String(), "messageID", msgID, "alertSystemMessageID", alertSysMsgID)
				return
			}
			log.ErrorLoggerRaw().Error("Failed to enqueue automod log task; falling back to synchronous send", "guildID", guildIDStr, "channelID", logChannelID, "userID", e.UserID.String(), "err", err)
		} else {
			return
		}
	}

	// Synchronous fallback (adapters nil or router enqueue failed). Apply an
	// in-process dedup so Gateway re-deliveries do not duplicate the embed.
	if as.dedupCache.ShouldDedup(idempotencyKey, as.isRunning) {
		log.ApplicationLogger().Debug("Dropped duplicate automod log task on fallback path", "guildID", guildIDStr, "channelID", logChannelID, "userID", e.UserID.String(), "ruleID", e.RuleID.String())
		return
	}

	domainEmbed := automod.BuildAutomodEmbed(domainExec)

	embed := &discord.Embed{
		Title:       domainEmbed.Title,
		Description: domainEmbed.Description,
		Color:       discord.Color(domainEmbed.Color),
		Timestamp:   discord.NewTimestamp(domainEmbed.Timestamp),
	}
	for _, f := range domainEmbed.Fields {
		embed.Fields = append(embed.Fields, discord.EmbedField{
			Name:   f.Name,
			Value:  f.Value,
			Inline: f.Inline,
		})
	}

	if as.notifier != nil {
		if err := as.notifier.Send(logChannelID, embed); err != nil {
			log.ErrorLoggerRaw().Error("Failed to send native automod log message via notifier", "guildID", guildIDStr, "channelID", logChannelID, "userID", e.UserID.String(), "err", err)
		}
	} else {
		chID, _ := discord.ParseSnowflake(logChannelID)
		if _, err := as.state.SendEmbeds(discord.ChannelID(chID), *embed); err != nil {
			log.ErrorLoggerRaw().Error("Failed to send native automod log message", "guildID", guildIDStr, "channelID", logChannelID, "userID", e.UserID.String(), "err", err)
		}
	}
}
