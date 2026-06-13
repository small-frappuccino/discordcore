package messages

import "time"

// User is an agnostic representation of a user.
type User struct {
	ID       string
	Username string
	Avatar   string
	Bot      bool
}

// CachedMessage stores message data for comparison.
type CachedMessage struct {
	ID        string
	Content   string
	Author    *User
	ChannelID string
	GuildID   string
	Timestamp time.Time
}
