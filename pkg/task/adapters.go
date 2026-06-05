package task

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/storage"
)

// Task type identifiers for notifications and avatar pipeline
// MemberNotificationSender defines methods for sending member-related notifications.
type MemberNotificationSender interface {
	SendAvatarChangeNotification(channelID string, change files.AvatarChange) error
	SendMemberJoinNotification(channelID string, member *discordgo.GuildMemberAdd, accountAge time.Duration) error
	SendMemberLeaveNotification(channelID string, member *discordgo.GuildMemberRemove, serverTime time.Duration, botTime time.Duration) error
}

// MessageNotificationSender defines methods for sending message-related notifications.
type MessageNotificationSender interface {
	SendMessageEditNotification(channelID string, original *CachedMessage, edited *discordgo.MessageUpdate) error
	SendMessageDeleteNotification(channelID string, deleted *CachedMessage, deletedBy string) error
}

// ModerationNotificationSender defines methods for sending automod-related notifications.
type ModerationNotificationSender interface {
	SendAutomodActionNotification(channelID string, event *discordgo.AutoModerationActionExecution) error
}

// NotificationSender defines dependency-free methods for sending notifications.
type NotificationSender interface {
	MemberNotificationSender
	MessageNotificationSender
	ModerationNotificationSender
}

// AvatarProcessor defines the logic for processing avatar changes.
type AvatarProcessor interface {
	ProcessChange(guildID, userID, currentAvatar, username string)
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

// TaskTypeFlushAvatarCache defines task type flush avatar cache.
// TaskTypeProcessAvatarChange defines task type process avatar change.
// TaskTypeSendAutomodAction defines task type send automod action.
// TaskTypeSendMessageDelete defines task type send message delete.
// TaskTypeSendMessageEdit defines task type send message edit.
// TaskTypeSendMemberLeave defines task type send member leave.
// TaskTypeSendMemberJoin defines task type send member join.
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

// FlushAvatarCachePayload holds information for flushing the avatar cache.
type FlushAvatarCachePayload struct{}

// AvatarChangePayload holds information to process an avatar change. Username is
// optional; the handler looks it up when empty.
type AvatarChangePayload struct {
	GuildID   string
	UserID    string
	Username  string // optional; handler may look up if empty
	NewAvatar string
}

// NotificationAdapters wires NotificationSender and AvatarCache to the TaskRouter.
type NotificationAdapters struct {
	Router          *TaskRouter
	Notifier        NotificationSender
	AvatarProcessor AvatarProcessor
	Store           *storage.Store
	Config          *files.ConfigManager
	Session         *discordgo.Session
}

// SetAvatarProcessor sets the processor for avatar change tasks.
func (a *NotificationAdapters) SetAvatarProcessor(p AvatarProcessor) {
	a.AvatarProcessor = p
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

// EnqueueAutomodAction enqueues an automod action notification using the
// default idempotency key (computed without a gateway sequence number). Kept
// for callers that do not have access to the raw *discordgo.Event envelope.
func (a *NotificationAdapters) EnqueueAutomodAction(channelID string, event *discordgo.AutoModerationActionExecution) error {
	return a.EnqueueAutomodActionWithKey(channelID, event, AutomodIdempotencyKey(event))
}

// EnqueueAutomodActionWithKey enqueues an automod action notification with an
// explicit idempotency key. The key is typically computed by the caller via
// AutomodIdempotencyKey, which prefers per-violation identifiers (MessageID,
// MatchedContent, MatchedKeyword) over per-action ones so the multiple
// AUTO_MODERATION_ACTION_EXECUTION events Discord fires for a single rule
// trigger collapse to one notification. An empty key disables router-level
// dedup for this event.
func (a *NotificationAdapters) EnqueueAutomodActionWithKey(channelID string, event *discordgo.AutoModerationActionExecution, idempotencyKey string) error {
	if event == nil {
		return nil
	}
	return a.Router.Dispatch(context.Background(), Task{
		Type: TaskTypeSendAutomodAction,
		Payload: AutomodActionPayload{
			ChannelID: channelID,
			Event:     event,
		},
		Options: TaskOptions{
			GroupKey:       event.GuildID,
			IdempotencyKey: idempotencyKey,
			IdempotencyTTL: 10 * time.Second,
			MaxAttempts:    3,
			InitialBackoff: 1 * time.Second,
			MaxBackoff:     10 * time.Second,
		},
	})
}

// automodCoalesceBucketSec is the time-bucket width (seconds) for the
// last-resort idempotency key fallback. Wide enough that the per-action
// gateway events Discord fires for one violation (typically arriving
// milliseconds apart) land in the same bucket and dedup, narrow enough
// that genuinely-distinct violations on the same (guild, rule, user)
// tuple stay independent.
const automodCoalesceBucketSec = 3

// AutomodIdempotencyKey computes the dedupe key for an AutoMod action event.
//
// Discord emits AUTO_MODERATION_ACTION_EXECUTION once per *action* configured
// on a triggered rule (Block Message, Send Alert Message, Timeout, Block
// Member Interactions). A single rule trigger therefore produces multiple
// gateway events whose only stable shared identifiers are the per-violation
// fields — MessageID for message triggers, MatchedContent/MatchedKeyword for
// member-profile triggers. Keying on those collapses the per-action stream to
// one notification, matching Discord's own chat-side "one notice per
// violation" behavior. The gateway sequence number and the per-action
// AlertSystemMessageID are intentionally NOT used here because they differ
// across the actions of a single violation and would defeat the coalescing.
//
// Precedence: msg → content → keyword+second-bucket → (guild, rule, user)
// tbucket fallback. The tbucket fallback always returns a non-empty key so
// router-level dedup remains active even if a future trigger type carries
// no per-violation payload identifiers.
//
// Exported so the synchronous fallback path in pkg/discord/logging can
// compute the same key the router would and apply its own dedup before
// sending.
func AutomodIdempotencyKey(event *discordgo.AutoModerationActionExecution) string {
	return automodIdempotencyKeyAt(event, time.Now())
}

func automodIdempotencyKeyAt(event *discordgo.AutoModerationActionExecution, now time.Time) string {
	if event == nil {
		return ""
	}
	if messageID := strings.TrimSpace(event.MessageID); messageID != "" {
		// MessageID is set on every action event for a message-triggered
		// violation (the blocked message), so all per-action events for one
		// violation share it.
		return fmt.Sprintf("automod:%s:%s:%s:msg:%s", event.GuildID, event.RuleID, event.UserID, messageID)
	}
	if content := strings.TrimSpace(event.MatchedContent); content != "" {
		// MatchedContent is the offending substring — shared across the
		// per-action events of one member-profile violation. Re-deliveries
		// carry the same content; distinct profile updates carry different
		// content. No time bucket needed.
		digest := sha256.Sum256([]byte(content))
		return fmt.Sprintf("automod:%s:%s:%s:content:%s", event.GuildID, event.RuleID, event.UserID, hex.EncodeToString(digest[:8]))
	}
	if keyword := strings.TrimSpace(event.MatchedKeyword); keyword != "" {
		// MatchedKeyword is the rule's configured keyword — shared across
		// distinct events of the same rule. Bucket by second so re-deliveries
		// within the same second collide while distinct events seconds apart
		// stay independent, instead of falsely deduping for the full TTL.
		digest := sha256.Sum256([]byte(keyword))
		return fmt.Sprintf("automod:%s:%s:%s:keyword:%s:t%d", event.GuildID, event.RuleID, event.UserID, hex.EncodeToString(digest[:8]), now.Unix())
	}
	// Defensive coalescing fallback. Triggered when no per-violation
	// identifier is present — a shape Discord does not emit today on the
	// rules in use, but worth keying anyway so a future trigger type's
	// per-action stream still collapses to one embed per violation.
	// AlertSystemMessageID is intentionally NOT used as a tiebreaker here
	// because it is per-action; mixing it in would split a violation across
	// its block + alert events.
	return fmt.Sprintf("automod:%s:%s:%s:tbucket:%d", event.GuildID, event.RuleID, event.UserID, now.Unix()/automodCoalesceBucketSec)
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
		Payload: FlushAvatarCachePayload{},
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
		return fmt.Errorf("NotificationAdapters.handleSendMemberJoin: %w", err)
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
		return fmt.Errorf("NotificationAdapters.handleSendMemberLeave: %w", err)
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
		return fmt.Errorf("NotificationAdapters.handleSendMessageEdit: %w", err)
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
		return fmt.Errorf("NotificationAdapters.handleSendMessageDelete: %w", err)
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
	p, ok := payload.(AvatarChangePayload)
	if !ok || p.GuildID == "" || p.UserID == "" {
		return fmt.Errorf("invalid payload for %s", TaskTypeProcessAvatarChange)
	}

	if a.AvatarProcessor != nil {
		a.AvatarProcessor.ProcessChange(p.GuildID, p.UserID, p.NewAvatar, p.Username)
		return nil
	}

	// Fallback to minimal persistence if no processor is available (should not happen in production)
	if a.Store != nil {
		_, _, err := a.Store.UpsertAvatar(p.GuildID, p.UserID, p.NewAvatar, time.Now())
		return err
	}

	return fmt.Errorf("avatar processor not initialized")
}

func (a *NotificationAdapters) handleFlushAvatarCache(ctx context.Context, payload any) error {
	_, ok := payload.(FlushAvatarCachePayload)
	if !ok {
		return fmt.Errorf("invalid payload type for avatar cache flush")
	}
	// No-op when using database-backed avatar persistence
	return nil
}
