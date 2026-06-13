package monitoring

import (
	"context"
	"iter"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/discord/cache"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/logpolicy"
	"github.com/small-frappuccino/discordcore/pkg/monitoring"
	"github.com/small-frappuccino/discordcore/pkg/notifications"
	"github.com/small-frappuccino/discordgo"
)

// SessionDataProvider implements monitoring.DataProvider using discordgo.Session.
type SessionDataProvider struct {
	session      *discordgo.Session
	unifiedCache *cache.UnifiedCache
}

func NewSessionDataProvider(session *discordgo.Session, unifiedCache *cache.UnifiedCache) *SessionDataProvider {
	return &SessionDataProvider{session: session, unifiedCache: unifiedCache}
}

func (p *SessionDataProvider) GetMember(ctx context.Context, guildID, userID string) (*monitoring.Member, error) {
	var m *discordgo.Member
	var err error
	var ok bool
	if p.unifiedCache != nil {
		m, ok = p.unifiedCache.GetMember(guildID, userID)
		if !ok {
			m, err = p.session.GuildMember(guildID, userID)
		}
	} else {
		m, err = p.session.GuildMember(guildID, userID)
	}
	if err != nil {
		return nil, err
	}
	return p.mapMember(m, guildID), nil
}

func (p *SessionDataProvider) BotUserID() string {
	if p.session.State != nil && p.session.State.User != nil {
		return p.session.State.User.ID
	}
	return ""
}

func (p *SessionDataProvider) GetRole(ctx context.Context, guildID, roleID string) (*monitoring.Role, error) {
	roles, err := p.session.GuildRoles(guildID)
	if err != nil {
		return nil, err
	}
	for _, r := range roles {
		if r.ID == roleID {
			return &monitoring.Role{
				ID:          r.ID,
				GuildID:     guildID,
				Managed:     r.Managed,
				Permissions: r.Permissions,
			}, nil
		}
	}
	return nil, nil // Or an error depending on expectations
}

func (p *SessionDataProvider) GetGuildRoles(ctx context.Context, guildID string) ([]*monitoring.Role, error) {
	roles, err := p.session.GuildRoles(guildID)
	if err != nil {
		return nil, err
	}
	var res []*monitoring.Role
	for _, r := range roles {
		res = append(res, &monitoring.Role{
			ID:          r.ID,
			GuildID:     guildID,
			Managed:     r.Managed,
			Permissions: r.Permissions,
		})
	}
	return res, nil
}

func (p *SessionDataProvider) GetGuild(ctx context.Context, guildID string) (*monitoring.Guild, error) {
	g, err := p.session.Guild(guildID)
	if err != nil {
		return nil, err
	}
	return &monitoring.Guild{
		ID:      g.ID,
		Name:    g.Name,
		OwnerID: g.OwnerID,
	}, nil
}

func (p *SessionDataProvider) GetGuildAuditLog(ctx context.Context, guildID string, actionType int, limit int) (*monitoring.AuditLog, error) {
	al, err := p.session.GuildAuditLog(guildID, "", "", actionType, limit)
	if err != nil {
		return nil, err
	}
	var res monitoring.AuditLog
	for _, e := range al.AuditLogEntries {
		var changes []monitoring.AuditLogChange
		for _, c := range e.Changes {
			key := ""
			if c.Key != nil {
				key = string(*c.Key)
			}
			changes = append(changes, monitoring.AuditLogChange{
				Key:      key,
				OldValue: c.OldValue,
				NewValue: c.NewValue,
			})
		}
		var actionType int
		if e.ActionType != nil {
			actionType = int(*e.ActionType)
		}
		res.Entries = append(res.Entries, monitoring.AuditLogEntry{
			ID:         e.ID,
			UserID:     e.UserID,
			ActionType: actionType,
			TargetID:   e.TargetID,
			Changes:    changes,
		})
	}
	return &res, nil
}

func (p *SessionDataProvider) EditGuildRolePermissions(ctx context.Context, guildID, roleID string, permissions int64) error {
	_, err := p.session.GuildRoleEdit(guildID, roleID, &discordgo.RoleParams{Permissions: &permissions})
	return err
}

func (p *SessionDataProvider) StreamGuildMembers(ctx context.Context, guildID string) iter.Seq2[*monitoring.Member, error] {
	return func(yield func(*monitoring.Member, error) bool) {
		lastID := "0"
		for {
			members, err := p.session.GuildMembers(guildID, lastID, 1000)
			if err != nil {
				yield(nil, err)
				return
			}
			if len(members) == 0 {
				return
			}
			for _, m := range members {
				if !yield(p.mapMember(m, guildID), nil) {
					return
				}
				lastID = m.User.ID
			}
		}
	}
}

func (p *SessionDataProvider) AddGuildMemberRole(ctx context.Context, guildID, userID, roleID string) error {
	return p.session.GuildMemberRoleAdd(guildID, userID, roleID)
}

func (p *SessionDataProvider) RemoveGuildMemberRole(ctx context.Context, guildID, userID, roleID string) error {
	return p.session.GuildMemberRoleRemove(guildID, userID, roleID)
}

func (p *SessionDataProvider) GetChannelMessages(ctx context.Context, channelID string, limit int, beforeID string) ([]*monitoring.Message, error) {
	msgs, err := p.session.ChannelMessages(channelID, limit, beforeID, "", "")
	if err != nil {
		return nil, err
	}
	var res []*monitoring.Message
	for _, m := range msgs {
		res = append(res, &monitoring.Message{
			ID:        m.ID,
			ChannelID: m.ChannelID,
			Content:   m.Content,
			AuthorID:  m.Author.ID,
			Type:      int(m.Type),
			Timestamp: m.Timestamp,
		})
	}
	return res, nil
}

func (p *SessionDataProvider) mapMember(m *discordgo.Member, guildID string) *monitoring.Member {
	if m == nil || m.User == nil {
		return nil
	}
	return &monitoring.Member{
		UserID:     m.User.ID,
		Username:   m.User.Username,
		GuildID:    guildID,
		IsBot:      m.User.Bot,
		AvatarHash: m.User.Avatar,
		JoinedAt:   m.JoinedAt,
		Roles:      m.Roles,
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
	session *discordgo.Session
	config  *files.ConfigManager
}

func NewDefaultLogPolicyChecker(session *discordgo.Session, config *files.ConfigManager) *DefaultLogPolicyChecker {
	return &DefaultLogPolicyChecker{session: session, config: config}
}

func (c *DefaultLogPolicyChecker) ShouldEmitLogEvent(eventType string, guildID string) monitoring.LogEmitDecision {
	res := logpolicy.ShouldEmitLogEvent(c.session, c.config, logpolicy.LogEventType(eventType), guildID)
	return monitoring.LogEmitDecision{
		Enabled:   res.Enabled,
		ChannelID: res.ChannelID,
		Reason:    string(res.Reason),
	}
}
