package reactions

import (
	"context"
	"fmt"
	"strings"

	"github.com/small-frappuccino/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/monitoring"
	domain "github.com/small-frappuccino/discordcore/pkg/reactions"
)

// Adapter implements the domain.ReactionAdapter interface using discordgo.
type Adapter struct {
	session *discordgo.Session
}

// NewAdapter creates a new Adapter.
func NewAdapter(session *discordgo.Session) *Adapter {
	return &Adapter{session: session}
}

// GetGuildIDForChannel looks up the Guild ID for a given channel ID.
func (a *Adapter) GetGuildIDForChannel(channelID string) (string, error) {
	if a.session == nil || a.session.State == nil {
		return "", fmt.Errorf("session or state is nil")
	}
	ch, err := a.session.State.Channel(channelID)
	if err != nil {
		return "", err
	}
	return ch.GuildID, nil
}

// GetMessageAuthorID resolves the author ID of a message.
func (a *Adapter) GetMessageAuthorID(channelID, messageID string) (string, bool, error) {
	if a.session == nil || channelID == "" || messageID == "" {
		return "", false, nil
	}
	if a.session.State != nil {
		if message, err := a.session.State.Message(channelID, messageID); err == nil && message != nil && message.Author != nil {
			if authorID := strings.TrimSpace(message.Author.ID); authorID != "" {
				return authorID, true, nil
			}
		}
	}
	message, err := a.session.ChannelMessage(channelID, messageID)
	if err != nil {
		return "", false, fmt.Errorf("load reacted message: %w", err)
	}
	if message == nil || message.Author == nil {
		return "", false, nil
	}
	authorID := strings.TrimSpace(message.Author.ID)
	if authorID == "" {
		return "", false, nil
	}
	return authorID, true, nil
}

// RemoveReaction removes a user's reaction from a message.
func (a *Adapter) RemoveReaction(channelID, messageID, emojiID, userID string) error {
	if a.session == nil {
		return fmt.Errorf("session is nil")
	}
	return a.session.MessageReactionRemove(channelID, messageID, emojiID, userID)
}

// RegisterHandlers wires the domain ReactionEventService to discordgo's event system.
func RegisterHandlers(session *discordgo.Session, service *domain.ReactionEventService, lifecycle *monitoring.ServiceLifecycle) func() {
	if session == nil || service == nil {
		return nil
	}
	unsubAdd := session.AddHandler(monitoring.GuardedHandler(lifecycle, func(ctx context.Context, s *discordgo.Session, e *discordgo.MessageReactionAdd) {
		if e == nil || e.MessageReaction == nil {
			return
		}
		domainEvent := &domain.MessageReactionAdd{
			UserID:    e.UserID,
			MessageID: e.MessageID,
			ChannelID: e.ChannelID,
			GuildID:   e.GuildID,
			Emoji: domain.Emoji{
				ID:       e.Emoji.ID,
				Name:     e.Emoji.Name,
				Animated: e.Emoji.Animated,
			},
		}
		service.HandleReactionAdd(ctx, domainEvent)
	}))
	return unsubAdd
}
