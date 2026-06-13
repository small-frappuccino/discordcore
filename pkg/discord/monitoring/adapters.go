package monitoring

import (
	"context"
	"iter"
	"time"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/state"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/logpolicy"
	"github.com/small-frappuccino/discordcore/pkg/monitoring"
	"github.com/small-frappuccino/discordcore/pkg/notifications"
)

// SessionDataProvider implements monitoring.DataProvider using arikawa state.State.
type SessionDataProvider struct {
	state *state.State
}

func NewSessionDataProvider(st *state.State) *SessionDataProvider {
	return &SessionDataProvider{state: st}
}

func (p *SessionDataProvider) GetMember(ctx context.Context, guildID, userID string) (*monitoring.Member, error) {
	gID, err := discord.ParseSnowflake(guildID)
	if err != nil {
		return nil, err
	}
	uID, err := discord.ParseSnowflake(userID)
	if err != nil {
		return nil, err
	}
	m, err := p.state.Member(discord.GuildID(gID), discord.UserID(uID))
	if err != nil {
		return nil, err
	}
	return p.mapMember(m, guildID), nil
}

func (p *SessionDataProvider) BotUserID() string {
	u, err := p.state.Me()
	if err == nil {
		return u.ID.String()
	}
	return ""
}

func (p *SessionDataProvider) GetRole(ctx context.Context, guildID, roleID string) (*monitoring.Role, error) {
	gID, err := discord.ParseSnowflake(guildID)
	if err != nil {
		return nil, err
	}
	rID, err := discord.ParseSnowflake(roleID)
	if err != nil {
		return nil, err
	}
	r, err := p.state.Role(discord.GuildID(gID), discord.RoleID(rID))
	if err != nil {
		return nil, err
	}
	return &monitoring.Role{
		ID:          r.ID.String(),
		GuildID:     guildID,
		Managed:     r.Managed,
		Permissions: int64(r.Permissions),
	}, nil
}

func (p *SessionDataProvider) GetGuildRoles(ctx context.Context, guildID string) ([]*monitoring.Role, error) {
	gID, err := discord.ParseSnowflake(guildID)
	if err != nil {
		return nil, err
	}
	roles, err := p.state.Roles(discord.GuildID(gID))
	if err != nil {
		return nil, err
	}
	var res []*monitoring.Role
	for _, r := range roles {
		res = append(res, &monitoring.Role{
			ID:          r.ID.String(),
			GuildID:     guildID,
			Managed:     r.Managed,
			Permissions: int64(r.Permissions),
		})
	}
	return res, nil
}

func (p *SessionDataProvider) GetGuild(ctx context.Context, guildID string) (*monitoring.Guild, error) {
	gID, err := discord.ParseSnowflake(guildID)
	if err != nil {
		return nil, err
	}
	g, err := p.state.Guild(discord.GuildID(gID))
	if err != nil {
		return nil, err
	}
	return &monitoring.Guild{
		ID:      g.ID.String(),
		Name:    g.Name,
		OwnerID: g.OwnerID.String(),
	}, nil
}

func (p *SessionDataProvider) GetGuildAuditLog(ctx context.Context, guildID string, actionType int, limit int) (*monitoring.AuditLog, error) {
	gID, err := discord.ParseSnowflake(guildID)
	if err != nil {
		return nil, err
	}
	al, err := p.state.AuditLog(discord.GuildID(gID), api.AuditLogData{
		ActionType: discord.AuditLogEvent(actionType),
		Limit:      uint(limit),
	})
	if err != nil {
		return nil, err
	}
	var res monitoring.AuditLog
	for _, e := range al.Entries {
		var changes []monitoring.AuditLogChange
		for _, c := range e.Changes {
			changes = append(changes, monitoring.AuditLogChange{
				Key:      string(c.Key),
				OldValue: c.OldValue,
				NewValue: c.NewValue,
			})
		}
		res.Entries = append(res.Entries, monitoring.AuditLogEntry{
			ID:         e.ID.String(),
			UserID:     e.UserID.String(),
			ActionType: int(e.ActionType),
			TargetID:   e.TargetID.String(),
			Changes:    changes,
		})
	}
	return &res, nil
}

func (p *SessionDataProvider) EditGuildRolePermissions(ctx context.Context, guildID, roleID string, permissions int64) error {
	gID, err := discord.ParseSnowflake(guildID)
	if err != nil {
		return err
	}
	rID, err := discord.ParseSnowflake(roleID)
	if err != nil {
		return err
	}
	pArikawa := discord.Permissions(permissions)
	_, err = p.state.ModifyRole(discord.GuildID(gID), discord.RoleID(rID), api.ModifyRoleData{
		Permissions: &pArikawa,
	})
	return err
}

func (p *SessionDataProvider) StreamGuildMembers(ctx context.Context, guildID string) iter.Seq2[*monitoring.Member, error] {
	return func(yield func(*monitoring.Member, error) bool) {
		gID, err := discord.ParseSnowflake(guildID)
		if err != nil {
			yield(nil, err)
			return
		}
		var lastID discord.UserID
		for {
			members, err := p.state.MembersAfter(discord.GuildID(gID), lastID, 1000)
			if err != nil {
				yield(nil, err)
				return
			}
			if len(members) == 0 {
				return
			}
			for _, m := range members {
				if !yield(p.mapMember(&m, guildID), nil) {
					return
				}
				lastID = m.User.ID
			}
		}
	}
}

func (p *SessionDataProvider) AddGuildMemberRole(ctx context.Context, guildID, userID, roleID string) error {
	gID, err := discord.ParseSnowflake(guildID)
	if err != nil {
		return err
	}
	uID, err := discord.ParseSnowflake(userID)
	if err != nil {
		return err
	}
	rID, err := discord.ParseSnowflake(roleID)
	if err != nil {
		return err
	}
	return p.state.AddRole(discord.GuildID(gID), discord.UserID(uID), discord.RoleID(rID), api.AddRoleData{})
}

func (p *SessionDataProvider) RemoveGuildMemberRole(ctx context.Context, guildID, userID, roleID string) error {
	gID, err := discord.ParseSnowflake(guildID)
	if err != nil {
		return err
	}
	uID, err := discord.ParseSnowflake(userID)
	if err != nil {
		return err
	}
	rID, err := discord.ParseSnowflake(roleID)
	if err != nil {
		return err
	}
	return p.state.RemoveRole(discord.GuildID(gID), discord.UserID(uID), discord.RoleID(rID), api.AuditLogReason(""))
}

func (p *SessionDataProvider) GetChannelMessages(ctx context.Context, channelID string, limit int, beforeID string) ([]*monitoring.Message, error) {
	cID, err := discord.ParseSnowflake(channelID)
	if err != nil {
		return nil, err
	}
	var bID discord.MessageID
	if beforeID != "" {
		b, err := discord.ParseSnowflake(beforeID)
		if err != nil {
			return nil, err
		}
		bID = discord.MessageID(b)
	}
	msgs, err := p.state.MessagesBefore(discord.ChannelID(cID), bID, uint(limit))
	if err != nil {
		return nil, err
	}
	var res []*monitoring.Message
	for _, m := range msgs {
		res = append(res, &monitoring.Message{
			ID:        m.ID.String(),
			ChannelID: m.ChannelID.String(),
			Content:   m.Content,
			AuthorID:  m.Author.ID.String(),
			Type:      int(m.Type),
			Timestamp: m.Timestamp.Time(),
		})
	}
	return res, nil
}

func (p *SessionDataProvider) mapMember(m *discord.Member, guildID string) *monitoring.Member {
	if m == nil {
		return nil
	}
	roles := make([]string, len(m.RoleIDs))
	for i, r := range m.RoleIDs {
		roles[i] = r.String()
	}
	return &monitoring.Member{
		UserID:     m.User.ID.String(),
		Username:   m.User.Username,
		GuildID:    guildID,
		IsBot:      m.User.Bot,
		AvatarHash: m.User.Avatar,
		JoinedAt:   m.Joined.Time(),
		Roles:      roles,
	}
}

// SessionNotifier implements monitoring.Notifier
type SessionNotifier struct {
	notifier *notifications.NotificationSender
}

func NewSessionNotifier(notifier *notifications.NotificationSender) *SessionNotifier {
	return &SessionNotifier{notifier: notifier}
}

func (n *SessionNotifier) SendRoleUpdateNotification(channelID string, targetUsername, targetID, actorID, added, removed, source string) error {
	if added != "" {
		_ = n.notifier.SendMemberRoleUpdateNotification(notifications.MemberRoleUpdateNotice{
			ChannelID: channelID, ActorID: actorID, TargetID: targetID, TargetUsername: targetUsername,
			RoleName: added, Action: "add",
		})
	}
	if removed != "" {
		_ = n.notifier.SendMemberRoleUpdateNotification(notifications.MemberRoleUpdateNotice{
			ChannelID: channelID, ActorID: actorID, TargetID: targetID, TargetUsername: targetUsername,
			RoleName: removed, Action: "remove",
		})
	}
	return nil
}

func (n *SessionNotifier) SendAvatarChangeNotification(channelID string, userID, username, oldAvatar, newAvatar string) error {
	return n.notifier.SendAvatarChangeNotification(channelID, files.AvatarChange{
		UserID:    userID,
		Username:  username,
		OldAvatar: oldAvatar,
		NewAvatar: newAvatar,
		Timestamp: time.Now(),
	})
}

// DefaultLogPolicyChecker implements monitoring.LogPolicyChecker
type DefaultLogPolicyChecker struct {
	state  *state.State
	config *files.ConfigManager
}

func NewDefaultLogPolicyChecker(st *state.State, config *files.ConfigManager) *DefaultLogPolicyChecker {
	return &DefaultLogPolicyChecker{state: st, config: config}
}

func (c *DefaultLogPolicyChecker) ShouldEmitLogEvent(eventType string, guildID string) monitoring.LogEmitDecision {
	res := logpolicy.ShouldEmitLogEvent(nil, c.config, logpolicy.LogEventType(eventType), guildID)

	if res.Enabled && res.Capability.RequiredIntentsMask != 0 {
		requiredIntents := gateway.Intents(res.Capability.RequiredIntentsMask)
		if !c.state.HasIntents(requiredIntents) {
			res.Enabled = false
			res.Reason = logpolicy.EmitReasonMissingIntent
		}
	}

	if !res.Enabled && res.Reason == logpolicy.EmitReasonChannelInvalid {
		gcfg := c.config.GuildConfig(guildID)
		if gcfg != nil && res.Capability.RequireExclusiveModeration && logpolicy.IsSharedModerationChannel(res.ChannelID, gcfg) {
			// Leave as channel_invalid
		} else {
			if c.validateChannel(guildID, res.ChannelID) {
				res.Enabled = true
				res.Reason = logpolicy.EmitReasonEnabled

				if res.Capability.RequiredIntentsMask != 0 {
					requiredIntents := gateway.Intents(res.Capability.RequiredIntentsMask)
					if !c.state.HasIntents(requiredIntents) {
						res.Enabled = false
						res.Reason = logpolicy.EmitReasonMissingIntent
					}
				}
			}
		}
	}

	return monitoring.LogEmitDecision{
		Enabled:   res.Enabled,
		ChannelID: res.ChannelID,
		Reason:    string(res.Reason),
	}
}

func (c *DefaultLogPolicyChecker) validateChannel(guildID, channelID string) bool {
	_, err := discord.ParseSnowflake(guildID)
	if err != nil {
		return false
	}
	cID, err := discord.ParseSnowflake(channelID)
	if err != nil {
		return false
	}

	me, err := c.state.Me()
	if err != nil {
		return false
	}

	ch, err := c.state.Channel(discord.ChannelID(cID))
	if err != nil {
		return false
	}
	if ch.GuildID.String() != guildID {
		return false
	}
	if ch.Type != discord.GuildText && ch.Type != discord.GuildNews {
		return false
	}

	perms, err := c.state.Permissions(discord.ChannelID(cID), me.ID)
	if err != nil {
		return false
	}

	req := discord.PermissionViewChannel | discord.PermissionSendMessages | discord.PermissionEmbedLinks
	return perms.Has(req)
}
