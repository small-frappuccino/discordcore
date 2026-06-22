package members

import "time"

// Snapshot represents the persisted snapshot for one guild member.
type Snapshot struct {
	UserID     string
	AvatarHash string
	HasAvatar  bool
	Roles      []string
	HasRoles   bool
	JoinedAt   time.Time
	IsBot      bool
	HasBot     bool
}

// CurrentState is the persisted current membership state for a user.
type CurrentState struct {
	UserID     string
	JoinedAt   time.Time
	LastSeenAt time.Time
	LeftAt     time.Time
	Active     bool
	IsBot      bool
	HasBot     bool
	Roles      []string
}

// UserPreferences represents user-specific settings.
type UserPreferences struct {
	UserID   string `json:"user_id"`
	Theme    string `json:"theme"`
	Timezone string `json:"timezone"`
}

// PresenceInput describes a member presence upsert payload.
type PresenceInput struct {
	GuildID  string
	UserID   string
	JoinedAt time.Time
	SeenAt   time.Time
	IsBot    bool
}
