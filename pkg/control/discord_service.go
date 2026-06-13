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

// User represents a Discord user.
type User struct {
	ID         string
	Username   string
	GlobalName string
	Bot        bool
}

// Member represents a guild member.
type Member struct {
	User  *User
	Nick  string
	Roles []string
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
	ID       string
	Name     string
	Position int
	Managed  bool
}

// Channel represents a guild channel.
type Channel struct {
	ID       string
	Name     string
	Type     ChannelType
	Position int
}

// Guild represents a Discord guild.
type Guild struct {
	ID       string
	Name     string
	Icon     string
	OwnerID  string
	Roles    []*Role
	Members  []*Member
	Channels []*Channel
}

// DiscordService defines the strictly consumer-side contract for retrieving live
// Discord runtime state, completely decoupling the control plane from external SDK types.
type DiscordService interface {
	Guild(guildID string) (*Guild, error)
	GuildMember(guildID, userID string) (*Member, error)
	GuildMembers(guildID, after string, limit int) ([]*Member, error)
	GuildMembersSearch(guildID, query string, limit int) ([]*Member, error)
}
