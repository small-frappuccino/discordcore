package control

// ChannelType represents the type of a Discord channel.
type ChannelType int

const (
	ChannelTypeGuildText          ChannelType = 0
	ChannelTypeGuildVoice         ChannelType = 2
	ChannelTypeGuildCategory      ChannelType = 4
	ChannelTypeGuildNews          ChannelType = 5
	ChannelTypeGuildNewsThread    ChannelType = 10
	ChannelTypeGuildPublicThread  ChannelType = 11
	ChannelTypeGuildPrivateThread ChannelType = 12
	ChannelTypeGuildStageVoice    ChannelType = 13
	ChannelTypeGuildDirectory     ChannelType = 14
	ChannelTypeGuildForum         ChannelType = 15
	ChannelTypeGuildMedia         ChannelType = 16
)

const (
	PermissionAdministrator int64 = 1 << 3
	PermissionManageGuild   int64 = 1 << 5
)

const (
	IntentsGuildMessages = 1 << 9
)

// User represents a Discord user.
type User struct {
	ID         string `json:"id"`
	Username   string `json:"username"`
	GlobalName string `json:"global_name"`
	Bot        bool   `json:"bot"`
	Avatar     string `json:"avatar"`
}

// AvatarURL returns the user's avatar URL.
func (u *User) AvatarURL() string {
	if u.Avatar == "" {
		return ""
	}
	return "https://cdn.discordapp.com/avatars/" + u.ID + "/" + u.Avatar + ".png"
}

// Member represents a guild member.
type Member struct {
	User  *User    `json:"user"`
	Nick  string   `json:"nick"`
	Roles []string `json:"roles"`
}

// DisplayName returns the highest priority display name for the member.
func (m *Member) DisplayName() string {
	if m.Nick != "" {
		return m.Nick
	}
	if m.User != nil {
		if m.User.GlobalName != "" {
			return m.User.GlobalName
		}
		return m.User.Username
	}
	return ""
}

// Role represents a guild role.
type Role struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Position    int    `json:"position"`
	Managed     bool   `json:"managed"`
	Permissions int64  `json:"permissions,string"`
}

// Channel represents a guild channel.
type Channel struct {
	ID       string      `json:"id"`
	Name     string      `json:"name"`
	Type     ChannelType `json:"type"`
	Position int         `json:"position"`
	ParentID string      `json:"parent_id"`
}

// Guild represents a Discord guild.
type Guild struct {
	ID       string     `json:"id"`
	Name     string     `json:"name"`
	Icon     string     `json:"icon"`
	OwnerID  string     `json:"owner_id"`
	Roles    []*Role    `json:"roles"`
	Members  []*Member  `json:"members"`
	Channels []*Channel `json:"channels"`
}

// DiscordService defines the strictly consumer-side contract for retrieving live
// Discord runtime state, completely decoupling the control plane from external SDK types.
type DiscordService interface {
	BotUser() (*User, error)
	Guild(guildID string) (*Guild, error)
	GuildMember(guildID, userID string) (*Member, error)
	GuildMembers(guildID, after string, limit int) ([]*Member, error)
	GuildMembersSearch(guildID, query string, limit int) ([]*Member, error)
	HasIntent(intentMask int) bool
}
