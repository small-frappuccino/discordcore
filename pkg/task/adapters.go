package task

import (
	"context"
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/storage"
)

// Task type identifiers for notifications and avatar pipeline
// NotificationSender defines dependency-free methods for sending notifications.
type NotificationSender interface {
	SendAvatarChangeNotification(channelID string, change files.AvatarChange) error
	SendMemberJoinNotification(channelID string, member *discordgo.GuildMemberAdd, accountAge time.Duration) error
	SendMemberLeaveNotification(channelID string, member *discordgo.GuildMemberRemove, serverTime time.Duration, botTime time.Duration) error
	SendMessageEditNotification(channelID string, original *CachedMessage, edited *discordgo.MessageUpdate) error
	SendMessageDeleteNotification(channelID string, deleted *CachedMessage, deletedBy string) error
	SendAutomodActionNotification(channelID string, event *discordgo.AutoModerationActionExecution) error
}

// CachedMessage is a minimal snapshot of a Discord message used for notifications.
type CachedMessage struct {
	ID        string
	Content   string
	Author    *discordgo.User
	ChannelID string
	GuildID   string
	Timestamp time.Time
}

const (
	TaskTypeSendMemberJoin    = "notifications.member_join"
	TaskTypeSendMemberLeave   = "notifications.member_leave"
	TaskTypeSendMessageEdit   = "notifications.message_edit"
	TaskTypeSendMessageDelete = "notifications.message_delete"
	TaskTypeSendAutomodAction = "notifications.automod_action"

	TaskTypeProcessAvatarChange = "avatar.process_change"
	TaskTypeFlushAvatarCache    = "avatar.flush_cache"
)

// MemberJoinPayload holds information for a member join notification task.
type MemberJoinPayload struct {
	ChannelID  string
	Member     *discordgo.GuildMemberAdd
	AccountAge time.Duration
}

// MemberLeavePayload holds information for a member leave notification task.
type MemberLeavePayload struct {
	ChannelID  string
	Member     *discordgo.GuildMemberRemove
	ServerTime time.Duration
	BotTime    time.Duration
}

// MessageEditPayload holds information for a message edit notification task.
type MessageEditPayload struct {
	ChannelID string
	Original  *CachedMessage
	Edited    *discordgo.MessageUpdate
}

// MessageDeletePayload holds information for a message delete notification task.
type MessageDeletePayload struct {
	ChannelID string
	Deleted   *CachedMessage
	DeletedBy string
}

// AutomodActionPayload holds information for an automod action notification task.
type AutomodActionPayload struct {
	ChannelID string
	Event     *discordgo.AutoModerationActionExecution
}

// AvatarChangePayload holds information to process an avatar change.
type AvatarChangePayload struct {
	GuildID   string
	UserID    string
	Username  string // optional; handler may look up if empty
	NewAvatar string
}

// NotificationAdapters wires NotificationSender and AvatarCache to the TaskRouter.
type NotificationAdapters struct {
	Router   *TaskRouter
	Notifier NotificationSender
	Store    *storage.Store
	Config   *files.ConfigManager
	Session  *discordgo.Session
}

// NewNotificationAdapters creates adapters and registers task handlers.
func NewNotificationAdapters(
	router *TaskRouter,
	session *discordgo.Session,
	cfg *files.ConfigManager,
	store *storage.Store,
	notifier NotificationSender,
) *NotificationAdapters {
	ad := &NotificationAdapters{
		Router:   router,
		Notifier: notifier,
		Store:    store,
		Config:   cfg,
		Session:  session,
	}
	ad.RegisterHandlers()
	return ad
}

// RegisterHandlers registers all handlers for the supported task types.
func (a *NotificationAdapters) RegisterHandlers() {
	a.Router.RegisterHandler(TaskTypeSendMemberJoin, a.handleSendMemberJoin)
	a.Router.RegisterHandler(TaskTypeSendMemberLeave, a.handleSendMemberLeave)
	a.Router.RegisterHandler(TaskTypeSendMessageEdit, a.handleSendMessageEdit)
	a.Router.RegisterHandler(TaskTypeSendMessageDelete, a.handleSendMessageDelete)
	a.Router.RegisterHandler(TaskTypeSendAutomodAction, a.handleSendAutomodAction)

	a.Router.RegisterHandler(TaskTypeProcessAvatarChange, a.handleProcessAvatarChange)
	a.Router.RegisterHandler(TaskTypeFlushAvatarCache, a.handleFlushAvatarCache)
}

// ---- Producer convenience methods ----

// EnqueueMemberJoin enqueues a member join notification.
func (a *NotificationAdapters) EnqueueMemberJoin(channelID string, member *discordgo.GuildMemberAdd, accountAge time.Duration) error {
	if member == nil || member.User == nil {
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
			GroupKey:       member.GuildID, // serialize per guild
			IdempotencyKey: fmt.Sprintf("join:%s:%s", member.GuildID, member.User.ID),
			IdempotencyTTL: 10 * time.Second,
			MaxAttempts:    3,
			InitialBackoff: 1 * time.Second,
			MaxBackoff:     10 * time.Second,
		},
	})
}

// EnqueueMemberLeave enqueues a member leave notification.
func (a *NotificationAdapters) EnqueueMemberLeave(channelID string, member *discordgo.GuildMemberRemove, serverTime time.Duration, botTime time.Duration) error {
	if member == nil || member.User == nil {
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
			GroupKey:       member.GuildID,
			IdempotencyKey: fmt.Sprintf("leave:%s:%s", member.GuildID, member.User.ID),
			IdempotencyTTL: 10 * time.Second,
			MaxAttempts:    3,
			InitialBackoff: 1 * time.Second,
			MaxBackoff:     10 * time.Second,
		},
	})
}

// EnqueueMessageEdit enqueues a message edit notification.
func (a *NotificationAdapters) EnqueueMessageEdit(channelID string, original *CachedMessage, edited *discordgo.MessageUpdate) error {
	if original == nil || edited == nil {
		return nil
	}
	group := original.GuildID
	if group == "" {
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
			GroupKey:       group,
			IdempotencyKey: fmt.Sprintf("edit:%s:%s", group, original.ID),
			IdempotencyTTL: 10 * time.Second,
			MaxAttempts:    3,
			InitialBackoff: 1 * time.Second,
			MaxBackoff:     10 * time.Second,
		},
	})
}

// EnqueueMessageDelete enqueues a message delete notification.
func (a *NotificationAdapters) EnqueueMessageDelete(channelID string, deleted *CachedMessage, deletedBy string) error {
	if deleted == nil {
		return nil
	}
	group := deleted.GuildID
	return a.Router.Dispatch(context.Background(), Task{
		Type: TaskTypeSendMessageDelete,
		Payload: MessageDeletePayload{
			ChannelID: channelID,
			Deleted:   deleted,
			DeletedBy: deletedBy,
		},
		Options: TaskOptions{
			GroupKey:       group,
			IdempotencyKey: fmt.Sprintf("delete:%s:%s", group, deleted.ID),
			IdempotencyTTL: 10 * time.Second,
			MaxAttempts:    3,
			InitialBackoff: 1 * time.Second,
			MaxBackoff:     10 * time.Second,
		},
	})
}

// EnqueueAutomodAction enqueues an automod action notification.
func (a *NotificationAdapters) EnqueueAutomodAction(channelID string, event *discordgo.AutoModerationActionExecution) error {
	if event == nil {
		return nil
	}
	group := event.GuildID
	return a.Router.Dispatch(context.Background(), Task{
		Type: TaskTypeSendAutomodAction,
		Payload: AutomodActionPayload{
			ChannelID: channelID,
			Event:     event,
		},
		Options: TaskOptions{
			GroupKey:       group,
			IdempotencyKey: fmt.Sprintf("automod:%s:%s:%s:%s", event.GuildID, event.RuleID, event.UserID, event.MessageID),
			IdempotencyTTL: 10 * time.Second,
			MaxAttempts:    3,
			InitialBackoff: 1 * time.Second,
			MaxBackoff:     10 * time.Second,
		},
	})
}

// EnqueueProcessAvatarChange enqueues processing of an avatar change.
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
			GroupKey:       guildID + ":" + userID, // serialize per user in guild
			IdempotencyKey: fmt.Sprintf("avatar:%s:%s:%s", guildID, userID, newAvatar),
			IdempotencyTTL: 60 * time.Second,
			MaxAttempts:    3,
			InitialBackoff: 2 * time.Second,
			MaxBackoff:     20 * time.Second,
		},
	})
}

// EnqueueFlushAvatarCache enqueues a flush of the avatar cache to disk.
func (a *NotificationAdapters) EnqueueFlushAvatarCache() error {
	return a.Router.Dispatch(context.Background(), Task{
		Type:    TaskTypeFlushAvatarCache,
		Payload: struct{}{},
		Options: TaskOptions{
			GroupKey:       "avatar_cache",
			IdempotencyKey: fmt.Sprintf("avatar_flush:%d", time.Now().Unix()/5), // coalesce every 5s
			IdempotencyTTL: 5 * time.Second,
			MaxAttempts:    2,
		},
	})
}

// ---- Handlers ----

func (a *NotificationAdapters) handleSendMemberJoin(ctx context.Context, payload any) error {
	if a.Notifier == nil {
		return fmt.Errorf("notifier is nil")
	}
	p, ok := payload.(MemberJoinPayload)
	if !ok || p.Member == nil || p.Member.User == nil {
		return fmt.Errorf("invalid payload for %s", TaskTypeSendMemberJoin)
	}
	err := a.Notifier.SendMemberJoinNotification(p.ChannelID, p.Member, p.AccountAge)
	if err != nil {
		return err
	}
	return nil
}

func (a *NotificationAdapters) handleSendMemberLeave(ctx context.Context, payload any) error {
	if a.Notifier == nil {
		return fmt.Errorf("notifier is nil")
	}
	p, ok := payload.(MemberLeavePayload)
	if !ok || p.Member == nil || p.Member.User == nil {
		return fmt.Errorf("invalid payload for %s", TaskTypeSendMemberLeave)
	}
	err := a.Notifier.SendMemberLeaveNotification(p.ChannelID, p.Member, p.ServerTime, p.BotTime)
	if err != nil {
		return err
	}
	return nil
}

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
		return err
	}
	return nil
}

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
		return err
	}
	return nil
}

func (a *NotificationAdapters) handleSendAutomodAction(ctx context.Context, payload any) error {
	if a.Notifier == nil {
		return fmt.Errorf("notifier is nil")
	}
	p, ok := payload.(AutomodActionPayload)
	if !ok || p.Event == nil {
		return fmt.Errorf("invalid payload for %s", TaskTypeSendAutomodAction)
	}
	return a.Notifier.SendAutomodActionNotification(p.ChannelID, p.Event)
}

func (a *NotificationAdapters) handleProcessAvatarChange(ctx context.Context, payload any) error {
	if a.Notifier == nil || a.Store == nil || a.Config == nil {
		return fmt.Errorf("dependencies not initialized")
	}
	p, ok := payload.(AvatarChangePayload)
	if !ok || p.GuildID == "" || p.UserID == "" {
		return fmt.Errorf("invalid payload for %s", TaskTypeProcessAvatarChange)
	}

	// Resolve username if not provided
	username := p.Username
	if username == "" && a.Session != nil {
		if member, err := a.Session.GuildMember(p.GuildID, p.UserID); err == nil && member != nil && member.User != nil {
			if member.User.Username != "" {
				username = member.User.Username
			} else if member.Nick != "" {
				username = member.Nick
			}
		}
	}
	if username == "" {
		username = p.UserID // fallback
	}

	var oldHash string
	if h, _, ok, _ := a.Store.GetAvatar(p.GuildID, p.UserID); ok {
		oldHash = h
	}

	change := files.AvatarChange{
		UserID:    p.UserID,
		Username:  username,
		OldAvatar: oldHash,
		NewAvatar: p.NewAvatar,
		Timestamp: time.Now(),
	}

	// Find destination channel
	gcfg := a.Config.GuildConfig(p.GuildID)
	if gcfg == nil {
		// No configuration; update avatar and exit (avoid retries)
		_, _, _ = a.Store.UpsertAvatar(p.GuildID, p.UserID, p.NewAvatar, time.Now())
		log.ApplicationLogger().Info("No guild config found; skipping avatar notification", "guildID", p.GuildID, "userID", p.UserID)
		return nil
	}

	channelID := gcfg.UserLogChannelID
	if channelID == "" {
		channelID = gcfg.CommandChannelID
	}
	// If there's still no channel, skip notification but update avatar in store
	if channelID == "" {
		log.ApplicationLogger().Info("No log channel configured; skipping avatar notification", "guildID", p.GuildID, "userID", p.UserID)
		_, _, _ = a.Store.UpsertAvatar(p.GuildID, p.UserID, p.NewAvatar, time.Now())
		return nil
	}

	// Send notification
	if err := a.Notifier.SendAvatarChangeNotification(channelID, change); err != nil {
		return err
	}

	// Update current avatar in store
	if _, _, err := a.Store.UpsertAvatar(p.GuildID, p.UserID, p.NewAvatar, time.Now()); err != nil {
		return err
	}

	return nil
}

func (a *NotificationAdapters) handleFlushAvatarCache(ctx context.Context, payload any) error {
	// No-op when using SQLite store
	return nil
}
