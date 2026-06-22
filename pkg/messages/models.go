package messages

import "time"

type Record struct {
	GuildID        string
	MessageID      string
	ChannelID      string
	AuthorID       string
	AuthorUsername string
	AuthorAvatar   string
	Content        string
	CachedAt       time.Time
	ExpiresAt      time.Time
	HasExpiry      bool
}

type DeleteKey struct {
	GuildID   string
	MessageID string
}

type Version struct {
	GuildID     string
	MessageID   string
	ChannelID   string
	AuthorID    string
	Version     int
	EventType   string
	Content     string
	Attachments int
	Embeds      int
	Stickers    int
	CreatedAt   time.Time
}

type DailyCountDelta struct {
	GuildID     string
	ChannelID   string
	UserID      string
	Day         time.Time
	MessageType string
	Count       int
}
