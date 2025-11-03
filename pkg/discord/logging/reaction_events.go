package logging

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/bwmarrin/discordgo"
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
	isRunning     bool

	// handlerCancels keeps unsubscribe functions for all registered handlers
	handlerCancels []func()
}

// NewReactionEventService creates a new ReactionEventService.
func NewReactionEventService(session *discordgo.Session, configManager *files.ConfigManager, store *storage.Store) *ReactionEventService {
	return &ReactionEventService{
		session:        session,
		configManager:  configManager,
		store:          store,
		handlerCancels: make([]func(), 0, 2),
	}
}

// Start registers Discord event handlers for reaction metrics.
func (rs *ReactionEventService) Start() error {
	if rs.isRunning {
		return fmt.Errorf("reaction event service is already running")
	}
	rs.isRunning = true

	// Register only the add handler for now (count "reactions added").
	// If you want to also track removals, add another counter/table or logic accordingly.
	unsubAdd := rs.session.AddHandler(rs.handleReactionAdd)
	rs.handlerCancels = append(rs.handlerCancels, unsubAdd)

	slog.Info("Reaction event service started")
	return nil
}

// Stop unregisters handlers and stops the service.
func (rs *ReactionEventService) Stop() error {
	if !rs.isRunning {
		return fmt.Errorf("reaction event service is not running")
	}
	for _, cancel := range rs.handlerCancels {
		if cancel != nil {
			cancel()
		}
	}
	rs.handlerCancels = nil
	rs.isRunning = false
	slog.Info("Reaction event service stopped")
	return nil
}

// IsRunning indicates whether the service is active.
func (rs *ReactionEventService) IsRunning() bool {
	return rs.isRunning
}

// handleReactionAdd processes MessageReactionAdd events and increments daily metrics.
func (rs *ReactionEventService) handleReactionAdd(s *discordgo.Session, e *discordgo.MessageReactionAdd) {
	if e == nil {
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

	// Skip if no store is available
	if rs.store == nil {
		return
	}

	// Mark that we processed an event (best effort).
	rs.markEvent()

	// Increment per-day reaction count for the reactor.
	if err := rs.store.IncrementDailyReactionCount(guildID, e.ChannelID, e.UserID, time.Now().UTC()); err != nil {
		slog.Error("Failed to increment daily reaction count", "guildID", guildID, "channelID", e.ChannelID, "userID", e.UserID, "err", err)
		return
	}

	slog.Info("Reaction recorded for daily metrics", "guildID", guildID, "channelID", e.ChannelID, "userID", e.UserID, "emoji", emojiName(e.Emoji))
}

// markEvent stores a heartbeat-like "last event" timestamp (best effort).
func (rs *ReactionEventService) markEvent() {
	if rs.store != nil {
		_ = rs.store.SetLastEvent(time.Now())
	}
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
