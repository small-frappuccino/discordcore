package logging

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/perf"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/storage"
)

// ReactionEventService listens to reaction events and records per-day metrics.
// It increments a daily counter keyed by (guild_id, channel_id, user_id, day)
// where user_id is the reactor (who added the reaction).
// The implementation avoids extra API calls by preferring data present in the event
// or in the session state. It only falls back to REST if strictly necessary.
type ReactionEventService struct {
	session       *discordgo.Session
	configManager *files.ConfigManager
	store         *storage.Store
	activity      *runtimeActivity
	lifecycle     serviceLifecycle

	// handlerCancels keeps unsubscribe functions for all registered handlers
	handlerCancels []func()
}

// NewReactionEventService creates a new ReactionEventService.
func NewReactionEventService(session *discordgo.Session, configManager *files.ConfigManager, store *storage.Store) *ReactionEventService {
	return &ReactionEventService{
		session:       session,
		configManager: configManager,
		store:         store,
		activity: newRuntimeActivity(store, runtimeActivityOptions{
			RunErr:       runErrWithTimeoutContext,
			EventTimeout: loggingDependencyTimeout,
			Warn:         slog.Warn,
		}),
		lifecycle:      newServiceLifecycle("reaction event service"),
		handlerCancels: make([]func(), 0, 2),
	}
}

// Start registers Discord event handlers for reaction metrics.
func (rs *ReactionEventService) Start(ctx context.Context) error {
	if rs.session == nil {
		return fmt.Errorf("reaction event service discord session is nil")
	}
	if _, err := rs.lifecycle.Start(ctx); err != nil {
		return err
	}

	rs.handlerCancels = rs.handlerCancels[:0]

	// Register only the add handler for now (count "reactions added").
	// If you want to also track removals, add another counter/table or logic accordingly.
	unsubAdd := rs.session.AddHandler(func(s *discordgo.Session, e *discordgo.MessageReactionAdd) {
		runCtx, done, ok := rs.lifecycle.Begin()
		if !ok {
			return
		}
		defer done()

		rs.handleReactionAdd(runCtx, s, e)
	})
	rs.handlerCancels = append(rs.handlerCancels, unsubAdd)

	slog.Info("Reaction event service started")
	return nil
}

// Stop unregisters handlers and stops the service.
func (rs *ReactionEventService) Stop(ctx context.Context) error {
	if err := rs.lifecycle.Cancel(); err != nil {
		return err
	}
	for _, cancel := range rs.handlerCancels {
		if cancel != nil {
			cancel()
		}
	}
	rs.handlerCancels = nil

	if err := rs.lifecycle.Wait(ctx); err != nil {
		return err
	}

	slog.Info("Reaction event service stopped")
	return nil
}

// IsRunning indicates whether the service is active.
func (rs *ReactionEventService) IsRunning() bool {
	return rs.lifecycle.IsRunning()
}

// handleReactionAdd processes MessageReactionAdd events and increments daily metrics.
func (rs *ReactionEventService) handleReactionAdd(ctx context.Context, s *discordgo.Session, e *discordgo.MessageReactionAdd) {
	if e == nil {
		return
	}
	if err := ctx.Err(); err != nil {
		return
	}

	// Determine guild ID with minimal API usage.
	guildID := e.GuildID
	if guildID == "" {
		// Try the state cache first
		if s != nil && s.State != nil {
			if ch, _ := s.State.Channel(e.ChannelID); ch != nil {
				guildID = ch.GuildID
			}
		}
	}

	// If still unknown, skip (likely a DM or insufficient cache state)
	if guildID == "" {
		slog.Debug("ReactionAdd: guildID missing; skipping metrics", "channelID", e.ChannelID, "userID", e.UserID)
		return
	}

	done := perf.StartGatewayEvent(
		"message_reaction_add",
		slog.String("guildID", guildID),
		slog.String("channelID", e.ChannelID),
		slog.String("userID", e.UserID),
	)
	defer done()

	// Skip if no store is available
	if rs.store == nil {
		return
	}

	// Mark that we processed an event (best effort).
	rs.markEvent(ctx)

	emit := ShouldEmitLogEvent(rs.session, rs.configManager, LogEventReactionMetric, guildID)
	if !emit.Enabled {
		slog.Debug("ReactionAdd: metrics suppressed by policy", "guildID", guildID, "channelID", e.ChannelID, "userID", e.UserID, "reason", emit.Reason)
		return
	}

	// Increment per-day reaction count for the reactor.
	if err := runErrWithTimeoutContext(ctx, loggingDependencyTimeout, func(runCtx context.Context) error {
		return rs.store.IncrementDailyReactionCountContext(runCtx, guildID, e.ChannelID, e.UserID, time.Now().UTC())
	}); err != nil {
		slog.Error("Failed to increment daily reaction count", "guildID", guildID, "channelID", e.ChannelID, "userID", e.UserID, "err", err)
		return
	}

	slog.Info("Reaction recorded for daily metrics", "guildID", guildID, "channelID", e.ChannelID, "userID", e.UserID, "emoji", emojiName(e.Emoji))
}

// markEvent stores a heartbeat-like "last event" timestamp (best effort).
func (rs *ReactionEventService) markEvent(ctx context.Context) {
	if rs.activity == nil {
		return
	}
	rs.activity.MarkEvent(ctx, "reaction_event_service")
}

// emojiName provides a compact string for logs.
func emojiName(e discordgo.Emoji) string {
	if e.Name != "" {
		return e.Name
	}
	if e.ID != "" {
		return e.ID
	}
	return ""
}
