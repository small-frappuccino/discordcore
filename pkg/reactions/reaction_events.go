package reactions

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/discord/perf"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/logpolicy"
	"github.com/small-frappuccino/discordcore/pkg/monitoring"
	"github.com/small-frappuccino/discordcore/pkg/storage"
)

// ReactionEventService listens to reaction add events, enforces configured
// reaction blocks, and records per-day reaction metrics when enabled.
// The implementation avoids extra API calls by preferring data present in the
// event or in the session state. It only falls back to REST if strictly
// necessary.
type ReactionEventService struct {
	adapter       ReactionAdapter
	configManager *files.ConfigManager
	botInstanceID string
	defaultBotID  string
	store         *storage.Store
	activity      *monitoring.RuntimeActivity
	lifecycle     monitoring.ServiceLifecycle

	logger *slog.Logger
}

// NewReactionEventService creates a new ReactionEventService.
func NewReactionEventService(adapter ReactionAdapter, configManager *files.ConfigManager, store *storage.Store, logger *slog.Logger) *ReactionEventService {
	return NewReactionEventServiceForBot(adapter, configManager, store, "", "", logger)
}

// NewReactionEventServiceForBot creates a ReactionEventService scoped to one bot instance.
func NewReactionEventServiceForBot(
	adapter ReactionAdapter,
	configManager *files.ConfigManager,
	store *storage.Store,
	botInstanceID string,
	defaultBotInstanceID string,
	logger *slog.Logger,
) *ReactionEventService {
	return &ReactionEventService{
		adapter:       adapter,
		configManager: configManager,
		botInstanceID: files.NormalizeBotInstanceID(botInstanceID),
		defaultBotID:  files.NormalizeBotInstanceID(defaultBotInstanceID),
		store:         store,
		logger:        logger,
		activity: monitoring.NewRuntimeActivity(store, monitoring.RuntimeActivityOptions{
			RunErr:        monitoring.RunErrWithTimeoutContext,
			EventTimeout:  monitoring.DependencyTimeout,
			BotInstanceID: files.NormalizeBotInstanceID(botInstanceID),
			Warn:          slog.Warn,
		}),
		lifecycle: monitoring.NewServiceLifecycle("reaction event service"),
	}
}

// Start registers the service lifecycle. The caller is responsible for wiring Discord events.
func (rs *ReactionEventService) Start(ctx context.Context) error {
	if _, err := rs.lifecycle.Start(ctx); err != nil {
		return fmt.Errorf("ReactionEventService.Start: %w", err)
	}
	rs.logger.Info("Reaction event service started")
	return nil
}

// Stop stops the service lifecycle.
func (rs *ReactionEventService) Stop(ctx context.Context) error {
	if err := rs.lifecycle.Cancel(); err != nil {
		return fmt.Errorf("ReactionEventService.Stop: %w", err)
	}
	if err := rs.lifecycle.Wait(ctx); err != nil {
		return fmt.Errorf("ReactionEventService.Stop: %w", err)
	}
	rs.logger.Info("Reaction event service stopped")
	return nil
}

// IsRunning indicates whether the service is active.
func (rs *ReactionEventService) IsRunning() bool {
	return rs.lifecycle.IsRunning()
}

// HandleReactionAdd processes MessageReactionAdd events, removing blocked
// reactions first and then recording daily metrics when enabled.
func (rs *ReactionEventService) HandleReactionAdd(ctx context.Context, e *MessageReactionAdd) {
	if e == nil {
		return
	}
	if err := ctx.Err(); err != nil {
		return
	}

	guildID := e.GuildID
	if guildID == "" {
		if id, err := rs.adapter.GetGuildIDForChannel(e.ChannelID); err == nil && id != "" {
			guildID = id
		}
	}

	// If still unknown, skip (likely a DM or insufficient cache state)
	if guildID == "" {
		rs.logger.Debug("ReactionAdd: guildID missing; skipping metrics", "channelID", e.ChannelID, "userID", e.UserID)
		return
	}
	if !rs.handlesGuild(guildID) {
		return
	}

	done := perf.StartGatewayEvent(
		"message_reaction_add",
		slog.String("guildID", guildID),
		slog.String("channelID", e.ChannelID),
		slog.String("userID", e.UserID),
	)
	defer done()

	// Mark that we processed an event (best effort).
	rs.markEvent(ctx)

	blocked, err := rs.enforceBlockedReaction(e, guildID)
	if err != nil {
		rs.logger.Warn("ReactionAdd: failed to enforce blocked reaction", "guildID", guildID, "channelID", e.ChannelID, "messageID", e.MessageID, "userID", e.UserID, "err", err)
	}
	if blocked {
		return
	}

	// Skip metrics when no store is available.
	if rs.store == nil {
		return
	}

	emit := logpolicy.ShouldEmitLogEvent(nil, rs.configManager, logpolicy.LogEventReactionMetric, guildID)
	if !emit.Enabled {
		rs.logger.Debug("ReactionAdd: metrics suppressed by policy", "guildID", guildID, "channelID", e.ChannelID, "userID", e.UserID, "reason", emit.Reason)
		return
	}

	// Increment per-day reaction count for the reactor.
	if err := monitoring.RunErrWithTimeoutContext(ctx, monitoring.DependencyTimeout, func(runCtx context.Context) error {
		return rs.store.IncrementDailyReactionCountContext(runCtx, guildID, e.ChannelID, e.UserID, time.Now().UTC())
	}); err != nil {
		rs.logger.Error("Failed to increment daily reaction count", "guildID", guildID, "channelID", e.ChannelID, "userID", e.UserID, "err", err)
		return
	}

	rs.logger.Info("Reaction recorded for daily metrics", "guildID", guildID, "channelID", e.ChannelID, "userID", e.UserID, "emoji", emojiName(e.Emoji))
}

func (rs *ReactionEventService) enforceBlockedReaction(
	e *MessageReactionAdd,
	guildID string,
) (bool, error) {
	if rs == nil || rs.configManager == nil || rs.adapter == nil || e == nil {
		return false, nil
	}
	guildConfig := rs.configManager.GuildConfig(guildID)
	if guildConfig == nil || guildConfig.ReactionBlocks.IsZero() {
		return false, nil
	}
	targetUserID, found, err := rs.adapter.GetMessageAuthorID(e.ChannelID, e.MessageID)
	if err != nil {
		return false, fmt.Errorf("ReactionEventService.enforceBlockedReaction: %w", err)
	}
	if !found {
		return false, nil
	}
	emoji := reactionBlockEmojiFromDiscord(e.Emoji)
	if emoji.IsZero() || !guildConfig.ReactionBlocks.BlocksEmojiForPair(e.UserID, targetUserID, emoji) {
		return false, nil
	}
	if err := rs.adapter.RemoveReaction(e.ChannelID, e.MessageID, reactionRemovalEmojiID(e.Emoji), e.UserID); err != nil {
		return false, fmt.Errorf("remove blocked reaction: %w", err)
	}
	rs.logger.Info("ReactionAdd: removed blocked reaction", "guildID", guildID, "channelID", e.ChannelID, "messageID", e.MessageID, "userID", e.UserID, "targetUserID", targetUserID, "emoji", emojiName(e.Emoji))
	return true, nil
}

func reactionBlockEmojiFromDiscord(emoji Emoji) files.ReactionBlockEmojiConfig {
	if emoji.ID != "" {
		return files.ReactionBlockEmojiConfig{
			Kind:     files.ReactionBlockEmojiKindCustom,
			Value:    strings.TrimSpace(emoji.ID),
			Name:     strings.TrimSpace(emoji.Name),
			Animated: emoji.Animated,
		}
	}
	if emoji.Name == "" {
		return files.ReactionBlockEmojiConfig{}
	}
	return files.ReactionBlockEmojiConfig{
		Kind:  files.ReactionBlockEmojiKindUnicode,
		Value: strings.TrimSpace(emoji.Name),
	}
}

func reactionRemovalEmojiID(emoji Emoji) string {
	if emoji.ID != "" {
		return strings.TrimSpace(emoji.ID)
	}
	return strings.TrimSpace(emoji.Name)
}

// markEvent stores a heartbeat-like "last event" timestamp (best effort).
func (rs *ReactionEventService) markEvent(ctx context.Context) {
	if rs.activity == nil {
		return
	}
	rs.activity.MarkEvent(ctx, "reaction_event_service")
}

// emojiName provides a compact string for logs.
func emojiName(e Emoji) string {
	if e.Name != "" {
		return e.Name
	}
	if e.ID != "" {
		return e.ID
	}
	return ""
}

func (rs *ReactionEventService) handlesGuild(guildID string) bool {
	if rs == nil || rs.configManager == nil {
		return false
	}
	if files.NormalizeBotInstanceID(rs.botInstanceID) == "" && files.NormalizeBotInstanceID(rs.defaultBotID) == "" {
		return true
	}
	guildID = strings.TrimSpace(guildID)
	if guildID == "" {
		return false
	}
	guild := rs.configManager.GuildConfig(guildID)
	if guild == nil {
		return false
	}
	if !guild.BelongsToBotInstance(rs.botInstanceID) {
		return false
	}
	resolvedID, _ := guild.ResolveFeatureBotInstanceID("moderation", rs.defaultBotID)
	return resolvedID == rs.botInstanceID
}
