package automod

import (
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/small-frappuccino/discordcore/pkg/discord/perf"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/logpolicy"
	"github.com/small-frappuccino/discordcore/pkg/task"
	"github.com/small-frappuccino/discordgo"
)

// handleRawEvent decomposes a raw gateway envelope, filters for AutoMod action
// executions, and forwards the typed payload alongside the envelope's gateway
// sequence number to handleAutoModerationAction. The sequence is preserved
// across Discord re-deliveries (including RESUME), so it is the most reliable
// dedup key available to the bot.
func (as *AutomodService) handleRawEvent(s *discordgo.Session, evt *discordgo.Event) {
	if evt == nil || evt.Type != automodActionExecutionEventType {
		return
	}

	e, ok := evt.Struct.(*discordgo.AutoModerationActionExecution)
	if !ok || e == nil {
		// discordgo guarantees registeredInterfaceProviders[evt.Type] populates
		// evt.Struct before dispatch, but documents that the struct may be
		// "partially populated or at default values" if unmarshalling failed
		// (wsapi.go:665-669). Fall back to a fresh unmarshal of RawData so we
		// don't silently drop events when discordgo changes the registered
		// type — small cost, defends the future-bump path.
		fallback := &discordgo.AutoModerationActionExecution{}
		if err := json.Unmarshal(evt.RawData, fallback); err != nil {
			log.ErrorLoggerRaw().Error("Failed to decode automod action execution payload", "type", evt.Type, "seq", evt.Sequence, "err", err)
			return
		}
		e = fallback
	}

	as.handleAutoModerationAction(s, e, evt.Sequence)
}

// handleAutoModerationAction logs native AutoMod events to the configured
// automod log channel. The sequence argument is the gateway sequence number
// from the *Event envelope and is recorded for observability only; the
// idempotency key is derived from per-violation payload fields so the
// multiple action events Discord fires for one trigger coalesce to a single
// log embed.
//
// SEND_ALERT_MESSAGE action events are dropped here as a belt-and-suspenders
// complement to the per-violation key: Discord posts its own native rich
// alert message to the configured alert channel for those, and dropping
// them at the handler guarantees no sibling embed leaks through even if a
// future trigger type's payload shape breaks the key-level coalescing.
func (as *AutomodService) handleAutoModerationAction(s *discordgo.Session, e *discordgo.AutoModerationActionExecution, sequence int64) {
	if e == nil || e.GuildID == "" {
		return
	}

	done := perf.StartGatewayEvent(
		"auto_moderation_action_execution",
		slog.String("guildID", e.GuildID),
		slog.String("channelID", e.ChannelID),
		slog.String("userID", e.UserID),
		slog.String("ruleID", e.RuleID),
		slog.Int64("seq", sequence),
	)
	defer done()

	if int(e.Action.Type) == automodActionSendAlert {
		log.ApplicationLogger().Debug("Dropping SEND_ALERT_MESSAGE automod event; Discord posts its own native alert", "guildID", e.GuildID, "ruleID", e.RuleID, "userID", e.UserID, "seq", sequence)
		return
	}

	if as.configManager == nil {
		return
	}
	guildConfig := as.configManager.GuildConfig(e.GuildID)
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

	emit := logpolicy.ShouldEmitLogEvent(s, as.configManager, logpolicy.LogEventAutomodAction, e.GuildID)
	if !emit.Enabled {
		log.ApplicationLogger().Debug("Automod action notification suppressed by policy", "guildID", e.GuildID, "channelID", e.ChannelID, "userID", e.UserID, "seq", sequence, "reason", emit.Reason)
		return
	}
	logChannelID := emit.ChannelID
	idempotencyKey := task.AutomodIdempotencyKey(e)

	// If adapters are wired, enqueue via TaskRouter for retries/backoff
	if as.adapters != nil {
		if err := as.adapters.EnqueueAutomodActionWithKey(logChannelID, e, idempotencyKey); err != nil {
			if errors.Is(err, task.ErrDuplicateTask) {
				log.ApplicationLogger().Debug("Dropped duplicate automod log task", "guildID", e.GuildID, "channelID", logChannelID, "userID", e.UserID, "ruleID", e.RuleID, "seq", sequence, "messageID", e.MessageID, "alertSystemMessageID", e.AlertSystemMessageID)
				return
			}
			log.ErrorLoggerRaw().Error("Failed to enqueue automod log task; falling back to synchronous send", "guildID", e.GuildID, "channelID", logChannelID, "userID", e.UserID, "seq", sequence, "err", err)
		} else {
			return
		}
	}

	// Synchronous fallback (adapters nil or router enqueue failed). Apply an
	// in-process dedup so Gateway re-deliveries do not duplicate the embed.
	if as.dedupCache.ShouldDedup(idempotencyKey, as.isRunning) {
		log.ApplicationLogger().Debug("Dropped duplicate automod log task on fallback path", "guildID", e.GuildID, "channelID", logChannelID, "userID", e.UserID, "ruleID", e.RuleID, "seq", sequence)
		return
	}

	embed := BuildAutomodEmbed(e)

	if as.notifier != nil {
		if err := as.notifier.Send(logChannelID, embed); err != nil {
			log.ErrorLoggerRaw().Error("Failed to send native automod log message via notifier", "guildID", e.GuildID, "channelID", logChannelID, "userID", e.UserID, "seq", sequence, "err", err)
		}
	} else {
		if _, err := s.ChannelMessageSendEmbed(logChannelID, embed); err != nil {
			log.ErrorLoggerRaw().Error("Failed to send native automod log message", "guildID", e.GuildID, "channelID", logChannelID, "userID", e.UserID, "seq", sequence, "err", err)
		}
	}
}
