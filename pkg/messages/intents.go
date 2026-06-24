package messages

import "time"

// CachedMessageData provides a pure representation of the deleted or edited message state.
type CachedMessageData struct {
	ID             string
	Content        string
	AuthorID       string
	AuthorUsername string
	AuthorBot      bool
	ChannelID      string
	GuildID        string
	Timestamp      time.Time
}

// MessageDeleteIntent represents a message being deleted.
type MessageDeleteIntent struct {
	GuildID          string
	ChannelID        string
	MessageID        string
	ExecutorID       string
	ExecutorUsername string
}

// MessageUpdateIntent represents a message being edited.
type MessageUpdateIntent struct {
	GuildID   string
	ChannelID string
	MessageID string
	Content   string
	AuthorID  string
}

// MessageDeleteBulkIntent represents multiple messages being deleted.
type MessageDeleteBulkIntent struct {
	GuildID    string
	ChannelID  string
	MessageIDs []string
}

// MessageCreateIntent represents a message being created.
type MessageCreateIntent struct {
	GuildID        string
	ChannelID      string
	MessageID      string
	Content        string
	AuthorID       string
	AuthorUsername string
	AuthorBot      bool
	Attachments    int
	Embeds         int
	Stickers       int
	Timestamp      time.Time
}

// AuditLogMessageDeleteEntry represents a cached deletion audit log.
type AuditLogMessageDeleteEntry struct {
	EntryID   string
	TargetID  string
	UserID    string
	ChannelID string
}
