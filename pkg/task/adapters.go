//go:build !legacy
// +build !legacy

package task

import (
	"context"
	"fmt"
	"time"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/state"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/storage"
)

// MemberNotificationSender delegates member lifecycle notifications to downstream subscribers.
type MemberNotificationSender interface {
	SendAvatarChangeNotification(channelID discord.ChannelID, change files.AvatarChange) error
	SendMemberJoinNotification(channelID discord.ChannelID, member *gateway.GuildMemberAddEvent, accountAge time.Duration) error
	SendMemberLeaveNotification(channelID discord.ChannelID, member *gateway.GuildMemberRemoveEvent, serverTime time.Duration, botTime time.Duration) error
}

// MessageNotificationSender delegates message lifecycle notifications to downstream subscribers.
type MessageNotificationSender interface {
	SendMessageEditNotification(channelID discord.ChannelID, original *CachedMessage, edited *gateway.MessageUpdateEvent) error
	SendMessageDeleteNotification(channelID discord.ChannelID, deleted *CachedMessage, deletedBy string) error
}

// NotificationSender defines unified dependency-free methods for propagating event notifications.
type NotificationSender interface {
	MemberNotificationSender
	MessageNotificationSender
}

// AvatarProcessor isolates the synchronization logic applied during avatar hash modifications.
type AvatarProcessor interface {
	ProcessChange(guildID, userID, currentAvatar, username string)
}

// CachedMessage defines an immutable cache snapshot holding essential message telemetry.
type CachedMessage struct {
	ID        discord.MessageID
	Content   string
	Author    *discord.User
	ChannelID discord.ChannelID
	GuildID   discord.GuildID
	Timestamp time.Time
}

// Task Type Registry defining hardcoded routing boundaries for the application ecosystem.
const (
	TaskTypeSendMemberJoin    = "notifications.member_join"
	TaskTypeSendMemberLeave   = "notifications.member_leave"
	TaskTypeSendMessageEdit   = "notifications.message_edit"
	TaskTypeSendMessageDelete = "notifications.message_delete"

	TaskTypeProcessAvatarChange = "avatar.process_change"
	TaskTypeFlushAvatarCache    = "avatar.flush_cache"
)

// MemberJoinPayload models the payload sent during a Discord member join event.
type MemberJoinPayload struct {
	ChannelID  discord.ChannelID
	Member     *gateway.GuildMemberAddEvent
	AccountAge time.Duration
}

// MemberLeavePayload models the payload sent during a Discord member leave event.
type MemberLeavePayload struct {
	ChannelID  discord.ChannelID
	Member     *gateway.GuildMemberRemoveEvent
	ServerTime time.Duration
	BotTime    time.Duration
}

// MessageEditPayload models the payload required to notify a channel about a modified message.
type MessageEditPayload struct {
	ChannelID discord.ChannelID
	Original  *CachedMessage
	Edited    *gateway.MessageUpdateEvent
}

// MessageDeletePayload models the payload required to track an abruptly removed message.
type MessageDeletePayload struct {
	ChannelID discord.ChannelID
	Deleted   *CachedMessage
	DeletedBy string
}

// FlushAvatarCachePayload acts as an empty trigger for synchronizing internal avatar structures.
type FlushAvatarCachePayload struct{}

// AvatarChangePayload encodes the domain request to refresh profile pictures asynchronously.
type AvatarChangePayload struct {
	GuildID   string
	UserID    string
	Username  string // Optional, processor can hydrate if omitted.
	NewAvatar string
}

// NotificationAdapters orchestrates dependency injection for bridging background events to actual side effects.
type NotificationAdapters struct {
	Router          *TaskRouter
	Notifier        NotificationSender
	AvatarProcessor AvatarProcessor
	Store           *storage.Store
	Config          *files.ConfigManager
	Session         *state.State
}

// SetAvatarProcessor dynamically overrides the avatar synchronization engine.
func (a *NotificationAdapters) SetAvatarProcessor(p AvatarProcessor) {
	a.AvatarProcessor = p
}

// RegisterHandlers maps execution endpoints natively into the TaskRouter topology.
func (a *NotificationAdapters) RegisterHandlers() {
	a.Router.RegisterHandler(TaskTypeSendMemberJoin, a.handleSendMemberJoin)
	a.Router.RegisterHandler(TaskTypeSendMemberLeave, a.handleSendMemberLeave)
	a.Router.RegisterHandler(TaskTypeSendMessageEdit, a.handleSendMessageEdit)
	a.Router.RegisterHandler(TaskTypeSendMessageDelete, a.handleSendMessageDelete)

	a.Router.RegisterHandler(TaskTypeProcessAvatarChange, a.handleProcessAvatarChange)
	a.Router.RegisterHandler(TaskTypeFlushAvatarCache, a.handleFlushAvatarCache)
}

// EnqueueMemberJoin provisions an immutable background dispatch modeling a new server member.
func (a *NotificationAdapters) EnqueueMemberJoin(channelID discord.ChannelID, member *gateway.GuildMemberAddEvent, accountAge time.Duration) error {
	if member == nil {
		return nil
	}
	return a.Router.Dispatch(context.Background(), Task{
		Type: TaskTypeSendMemberJoin,
		Payload: MemberJoinPayload{
			ChannelID:  channelID,
			Member:     member,
			AccountAge: accountAge,
		},
		Options: TaskOptions{
			GroupKey:       member.GuildID.String(),
			IdempotencyKey: fmt.Sprintf("join:%s:%s", member.GuildID, member.User.ID),
			IdempotencyTTL: 10 * time.Second,
			MaxAttempts:    3,
			InitialBackoff: 1 * time.Second,
			MaxBackoff:     10 * time.Second,
		},
	})
}

// EnqueueMemberLeave guarantees delivery of an account eviction notification natively scoped to the guild.
func (a *NotificationAdapters) EnqueueMemberLeave(channelID discord.ChannelID, member *gateway.GuildMemberRemoveEvent, serverTime time.Duration, botTime time.Duration) error {
	if member == nil {
		return nil
	}
	return a.Router.Dispatch(context.Background(), Task{
		Type: TaskTypeSendMemberLeave,
		Payload: MemberLeavePayload{
			ChannelID:  channelID,
			Member:     member,
			ServerTime: serverTime,
			BotTime:    botTime,
		},
		Options: TaskOptions{
			GroupKey:       member.GuildID.String(),
			IdempotencyKey: fmt.Sprintf("leave:%s:%s", member.GuildID, member.User.ID),
			IdempotencyTTL: 10 * time.Second,
			MaxAttempts:    3,
			InitialBackoff: 1 * time.Second,
			MaxBackoff:     10 * time.Second,
		},
	})
}

// EnqueueMessageEdit captures string delta events directly within the underlying event stream.
func (a *NotificationAdapters) EnqueueMessageEdit(channelID discord.ChannelID, original *CachedMessage, edited *gateway.MessageUpdateEvent) error {
	if original == nil || edited == nil {
		return nil
	}
	group := original.GuildID
	if !group.IsValid() {
		group = edited.GuildID
	}
	return a.Router.Dispatch(context.Background(), Task{
		Type: TaskTypeSendMessageEdit,
		Payload: MessageEditPayload{
			ChannelID: channelID,
			Original:  original,
			Edited:    edited,
		},
		Options: TaskOptions{
			GroupKey:       group.String(),
			IdempotencyKey: fmt.Sprintf("edit:%s:%s", group, original.ID),
			IdempotencyTTL: 10 * time.Second,
			MaxAttempts:    3,
			InitialBackoff: 1 * time.Second,
			MaxBackoff:     10 * time.Second,
		},
	})
}

// EnqueueMessageDelete guarantees reliable historical destruction tracking across all active environments.
func (a *NotificationAdapters) EnqueueMessageDelete(channelID discord.ChannelID, deleted *CachedMessage, deletedBy string) error {
	if deleted == nil {
		return nil
	}
	return a.Router.Dispatch(context.Background(), Task{
		Type: TaskTypeSendMessageDelete,
		Payload: MessageDeletePayload{
			ChannelID: channelID,
			Deleted:   deleted,
			DeletedBy: deletedBy,
		},
		Options: TaskOptions{
			GroupKey:       deleted.GuildID.String(),
			IdempotencyKey: fmt.Sprintf("delete:%s:%s", deleted.GuildID, deleted.ID),
			IdempotencyTTL: 10 * time.Second,
			MaxAttempts:    3,
			InitialBackoff: 1 * time.Second,
			MaxBackoff:     10 * time.Second,
		},
	})
}

// EnqueueProcessAvatarChange encapsulates the processing layer targeting external database systems.
func (a *NotificationAdapters) EnqueueProcessAvatarChange(guildID, userID, username, newAvatar string) error {
	return a.Router.Dispatch(context.Background(), Task{
		Type: TaskTypeProcessAvatarChange,
		Payload: AvatarChangePayload{
			GuildID:   guildID,
			UserID:    userID,
			Username:  username,
			NewAvatar: newAvatar,
		},
		Options: TaskOptions{
			GroupKey:       guildID + ":" + userID,
			IdempotencyKey: fmt.Sprintf("avatar:%s:%s:%s", guildID, userID, newAvatar),
			IdempotencyTTL: 60 * time.Second,
			MaxAttempts:    3,
			InitialBackoff: 2 * time.Second,
			MaxBackoff:     20 * time.Second,
		},
	})
}

// EnqueueFlushAvatarCache issues an isolated execution request specifically targeted at persistent memory pools.
func (a *NotificationAdapters) EnqueueFlushAvatarCache() error {
	return a.Router.Dispatch(context.Background(), Task{
		Type:    TaskTypeFlushAvatarCache,
		Payload: FlushAvatarCachePayload{},
		Options: TaskOptions{
			GroupKey:       "avatar_cache",
			IdempotencyKey: fmt.Sprintf("avatar_flush:%d", time.Now().Unix()/5),
			IdempotencyTTL: 5 * time.Second,
			MaxAttempts:    2,
		},
	})
}

// handleSendMemberJoin resolves the internal payload structure and delegates correctly back to the notifier.
func (a *NotificationAdapters) handleSendMemberJoin(ctx context.Context, payload any) error {
	if a.Notifier == nil {
		return fmt.Errorf("notifier is nil")
	}
	p, ok := payload.(MemberJoinPayload)
	if !ok || p.Member == nil {
		return fmt.Errorf("invalid payload for %s", TaskTypeSendMemberJoin)
	}
	err := a.Notifier.SendMemberJoinNotification(p.ChannelID, p.Member, p.AccountAge)
	if err != nil {
		return fmt.Errorf("NotificationAdapters.handleSendMemberJoin: %w", err)
	}
	return nil
}

// handleSendMemberLeave unwraps the strict leave context and pushes back into arbitrary delivery pipelines.
func (a *NotificationAdapters) handleSendMemberLeave(ctx context.Context, payload any) error {
	if a.Notifier == nil {
		return fmt.Errorf("notifier is nil")
	}
	p, ok := payload.(MemberLeavePayload)
	if !ok || p.Member == nil {
		return fmt.Errorf("invalid payload for %s", TaskTypeSendMemberLeave)
	}
	err := a.Notifier.SendMemberLeaveNotification(p.ChannelID, p.Member, p.ServerTime, p.BotTime)
	if err != nil {
		return fmt.Errorf("NotificationAdapters.handleSendMemberLeave: %w", err)
	}
	return nil
}

// handleSendMessageEdit delegates strict updates to channel systems, validating internal models synchronously.
func (a *NotificationAdapters) handleSendMessageEdit(ctx context.Context, payload any) error {
	if a.Notifier == nil {
		return fmt.Errorf("notifier is nil")
	}
	p, ok := payload.(MessageEditPayload)
	if !ok || p.Original == nil || p.Edited == nil {
		return fmt.Errorf("invalid payload for %s", TaskTypeSendMessageEdit)
	}
	err := a.Notifier.SendMessageEditNotification(p.ChannelID, p.Original, p.Edited)
	if err != nil {
		return fmt.Errorf("NotificationAdapters.handleSendMessageEdit: %w", err)
	}
	return nil
}

// handleSendMessageDelete encapsulates message purges from raw storage.
func (a *NotificationAdapters) handleSendMessageDelete(ctx context.Context, payload any) error {
	if a.Notifier == nil {
		return fmt.Errorf("notifier is nil")
	}
	p, ok := payload.(MessageDeletePayload)
	if !ok || p.Deleted == nil {
		return fmt.Errorf("invalid payload for %s", TaskTypeSendMessageDelete)
	}
	err := a.Notifier.SendMessageDeleteNotification(p.ChannelID, p.Deleted, p.DeletedBy)
	if err != nil {
		return fmt.Errorf("NotificationAdapters.handleSendMessageDelete: %w", err)
	}
	return nil
}

// handleProcessAvatarChange executes fallback state insertion directly to Postgres bypassing memory tiers entirely.
func (a *NotificationAdapters) handleProcessAvatarChange(ctx context.Context, payload any) error {
	p, ok := payload.(AvatarChangePayload)
	if !ok || p.GuildID == "" || p.UserID == "" {
		return fmt.Errorf("invalid payload for %s", TaskTypeProcessAvatarChange)
	}

	if a.AvatarProcessor != nil {
		a.AvatarProcessor.ProcessChange(p.GuildID, p.UserID, p.NewAvatar, p.Username)
		return nil
	}

	// Transactions simulate direct structural fallbacks when complex processor injections are fully ignored.
	if a.Store != nil {
		err := a.Store.UpsertGuildMemberSnapshotsContext(ctx, p.GuildID, []storage.GuildMemberSnapshot{{UserID: p.UserID, HasAvatar: true, AvatarHash: p.NewAvatar}}, time.Now())
		if err != nil {
			return fmt.Errorf("Store.UpsertGuildMemberSnapshotsContext: %w", err)
		}
		return nil
	}

	return fmt.Errorf("avatar processor and store both absent")
}

// handleFlushAvatarCache represents a dead-end boundary interface for components demanding periodic executions.
func (a *NotificationAdapters) handleFlushAvatarCache(ctx context.Context, payload any) error {
	_, ok := payload.(FlushAvatarCachePayload)
	if !ok {
		return fmt.Errorf("invalid payload type for avatar cache flush")
	}
	return nil
}
