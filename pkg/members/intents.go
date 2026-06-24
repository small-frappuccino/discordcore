package members

import "time"

// MemberJoinIntent represents a user joining a guild.
type MemberJoinIntent struct {
	GuildID    string
	UserID     string
	Username   string
	Bot        bool
	AvatarHash string
	RoleIDs    []string
	JoinedAt   time.Time
}

// MemberLeaveIntent represents a user leaving a guild.
type MemberLeaveIntent struct {
	GuildID    string
	UserID     string
	Username   string
	Bot        bool
	AvatarHash string
}

// RoleUpdateIntent represents a role update for a member.
type RoleUpdateIntent struct {
	GuildID      string
	UserID       string
	Username     string
	Bot          bool
	AddedRoles   []string
	RemovedRoles []string
}

// AvatarUpdateIntent represents a change in the user's avatar.
type AvatarUpdateIntent struct {
	GuildID       string
	UserID        string
	Username      string
	Bot           bool
	OldAvatarHash string
	NewAvatarHash string
}

// ModerationActionIntent represents an action applied to a member.
type ModerationActionIntent struct {
	GuildID        string
	ActionType     string
	TargetUserID   string
	TargetUsername string
	TargetBot      bool
	Reason         string
	ModeratorID    string
}

// MemberUpdateIntent represents a raw member update event for ingestion.
type MemberUpdateIntent struct {
	GuildID    string
	UserID     string
	Username   string
	Bot        bool
	RoleIDs    []string
	AvatarHash string
	OldRoleIDs []string
	OldAvatar  string
}
