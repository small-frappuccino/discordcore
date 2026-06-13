package monitoring

import (
	"context"
	"iter"
	"time"
)

// Guild is the agnostic domain representation of a Discord guild.
type Guild struct {
	ID      string
	Name    string
	OwnerID string
}

// Member is the agnostic domain representation of a Discord guild member
// tailored specifically for the monitoring domain's needs.
type Member struct {
	UserID     string
	Username   string
	GuildID    string
	IsBot      bool
	AvatarHash string
	JoinedAt   time.Time
	Roles      []string
}

// Message is the agnostic domain representation of a Discord channel message
// tailored specifically for the monitoring domain's needs.
type Message struct {
	ID        string
	ChannelID string
	Content   string
	AuthorID  string
	Type      int
	Timestamp time.Time
}

// Role is the agnostic domain representation of a Discord guild role
// tailored specifically for the monitoring domain's needs.
type Role struct {
	ID          string
	GuildID     string
	Managed     bool
	Permissions int64
}

// AuditLogEntry represents a single change event in a guild's audit log.
type AuditLogEntry struct {
	ID         string
	UserID     string
	ActionType int
	TargetID   string
	Changes    []AuditLogChange
}

// AuditLogChange represents a mutated property inside an AuditLogEntry.
type AuditLogChange struct {
	Key      string
	OldValue any
	NewValue any
}

// AuditLog represents a batch of audit log entries.
type AuditLog struct {
	Entries []AuditLogEntry
}

// DataProvider is the consumer-side interface that abstracts away Discord API and caching.
// This interface allows the monitoring domain to operate completely unaware of the SDK
// or the cache fallback orchestration.
type DataProvider interface {
	// GetMember retrieves a member, utilizing any underlying caching orchestration.
	GetMember(ctx context.Context, guildID, userID string) (*Member, error)

	// BotUserID returns the user ID of the authenticated bot.
	BotUserID() string

	// GetRole retrieves a role by ID from the guild.
	GetRole(ctx context.Context, guildID, roleID string) (*Role, error)

	// GetGuildRoles retrieves all roles for a guild.
	GetGuildRoles(ctx context.Context, guildID string) ([]*Role, error)

	// GetGuild retrieves a guild by ID.
	GetGuild(ctx context.Context, guildID string) (*Guild, error)

	// GetGuildAuditLog retrieves recent audit log entries for a specific action type.
	GetGuildAuditLog(ctx context.Context, guildID string, actionType int, limit int) (*AuditLog, error)

	// EditGuildRolePermissions patches a role's permissions.
	EditGuildRolePermissions(ctx context.Context, guildID, roleID string, permissions int64) error

	// StreamGuildMembers returns an iterator over all members in a guild, handling API pagination internally.
	StreamGuildMembers(ctx context.Context, guildID string) iter.Seq2[*Member, error]

	// AddGuildMemberRole adds a role to a guild member.
	AddGuildMemberRole(ctx context.Context, guildID, userID, roleID string) error

	// RemoveGuildMemberRole removes a role from a guild member.
	RemoveGuildMemberRole(ctx context.Context, guildID, userID, roleID string) error

	// GetChannelMessages retrieves a batch of channel messages before a specific message ID.
	GetChannelMessages(ctx context.Context, channelID string, limit int, beforeID string) ([]*Message, error)
}

// LogEmitDecision mirrors the logpolicy.LogEmitDecision.
type LogEmitDecision struct {
	Enabled   bool
	ChannelID string
	Reason    string
}

// LogPolicyChecker abstract away pkg/logpolicy.
type LogPolicyChecker interface {
	ShouldEmitLogEvent(eventType string, guildID string) LogEmitDecision
}

// Notifier abstracts away pkg/notifications and task payloads.
type Notifier interface {
	SendRoleUpdateNotification(channelID string, targetUsername, targetID, actorID, added, removed, source string) error
	SendAvatarChangeNotification(channelID string, userID, username, oldAvatar, newAvatar string) error
}
