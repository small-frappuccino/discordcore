package notifications

import "time"

// User represents a basic user.
type User struct {
	ID       string
	Username string
	Avatar   string
}

// MemberJoin represents a member join event.
type MemberJoin struct {
	User *User
}

// MemberLeave represents a member leave event.
type MemberLeave struct {
	User *User
}

// MessageUpdate represents a message update event.
type MessageUpdate struct {
	ID      string
	Content string
}

// Embed represents a message embed.
type Embed struct {
	Title       string
	Description string
	Color       int
	Timestamp   string
	Author      *EmbedAuthor
	Thumbnail   *EmbedThumbnail
	Fields      []*EmbedField
	Footer      *EmbedFooter
}

// EmbedAuthor represents an embed author.
type EmbedAuthor struct {
	Name    string
	IconURL string
}

// EmbedThumbnail represents an embed thumbnail.
type EmbedThumbnail struct {
	URL string
}

// EmbedField represents an embed field.
type EmbedField struct {
	Name   string
	Value  string
	Inline bool
}

// EmbedFooter represents an embed footer.
type EmbedFooter struct {
	Text string
}

// NotificationPublisher is the interface for sending notification embeds to Discord.
type NotificationPublisher interface {
	SendEmbed(channelID string, embed *Embed) error
}

// CachedMessage is a snapshot of a message used for notifications.
type CachedMessage struct {
	ID        string
	Content   string
	Author    *User
	ChannelID string
	GuildID   string
	Timestamp time.Time
}
