package reactions

// MessageReactionAdd represents an agnostic event for a reaction being added to a message.
type MessageReactionAdd struct {
	UserID    string
	MessageID string
	ChannelID string
	GuildID   string
	Emoji     Emoji
}

// Emoji represents a generalized emoji.
type Emoji struct {
	ID       string
	Name     string
	Animated bool
}

// ReactionAdapter defines the external Discord-specific methods required by the reaction service.
type ReactionAdapter interface {
	// GetGuildIDForChannel returns the GuildID for the given ChannelID. If not found or if a DM, it returns an empty string or an error.
	GetGuildIDForChannel(channelID string) (string, error)

	// GetMessageAuthorID resolves the author ID of a message, returning (authorID, found, error).
	GetMessageAuthorID(channelID, messageID string) (string, bool, error)

	// RemoveReaction removes a user's reaction from a message.
	RemoveReaction(channelID, messageID, emojiID, userID string) error
}
