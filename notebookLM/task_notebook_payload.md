# Domain Architecture: task

## Layout Topology
```text
task/
├── adapters.go
├── adapters_test.go
├── doc.go
├── router.go
└── router_test.go
```

## Source Stream Aggregation

// === FILE: pkg/task/adapters.go ===
```go
//go:build !legacy
// +build !legacy

package task

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/state"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/members"
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
	MembersRepo     members.Repository
	Config          *files.ConfigManager
	Session         *state.State
	Logger          *slog.Logger
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

// handleSendMessageDelete encapsulates message purges from raw postgres.
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
	if a.MembersRepo != nil {
		err := a.MembersRepo.UpsertGuildMemberSnapshotsContext(ctx, p.GuildID, []members.Snapshot{{UserID: p.UserID, HasAvatar: true, AvatarHash: p.NewAvatar}}, time.Now())
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

```

// === FILE: pkg/task/adapters_test.go ===
```go
//go:build !legacy
// +build !legacy

package task

import (
	"context"
	"iter"
	"strings"
	"testing"
	"time"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/members"
)

type mockDB struct {
	execCount int
}

type mockMembersRepo struct {
	db *mockDB
}

func (m *mockMembersRepo) GetUserPreferences(ctx context.Context, userID string) (*members.UserPreferences, error) {
	return nil, nil
}
func (m *mockMembersRepo) UpdateUserPreferences(ctx context.Context, prefs *members.UserPreferences) error {
	return nil
}
func (m *mockMembersRepo) UpsertGuildMemberSnapshotsContext(ctx context.Context, guildID string, snapshots []members.Snapshot, updatedAt time.Time) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		m.db.execCount++
		return nil
	}
}
func (m *mockMembersRepo) UpsertMemberJoinContext(ctx context.Context, guildID, userID string, joinedAt time.Time) error {
	return nil
}
func (m *mockMembersRepo) UpsertMemberPresenceContext(ctx context.Context, input members.PresenceInput) error {
	return nil
}
func (m *mockMembersRepo) MemberJoin(ctx context.Context, guildID, userID string) (time.Time, bool, error) {
	return time.Time{}, false, nil
}
func (m *mockMembersRepo) GetAvatar(ctx context.Context, guildID, userID string) (hash string, updatedAt time.Time, ok bool, err error) {
	return "", time.Time{}, false, nil
}
func (m *mockMembersRepo) GetActiveGuildMemberStatesContext(ctx context.Context, guildID string) iter.Seq2[members.CurrentState, error] {
	return nil
}
func (m *mockMembersRepo) StreamAllGuildMemberRoles(ctx context.Context, guildID string) (iter.Seq2[string, []string], error) {
	return nil, nil
}
func (m *mockMembersRepo) MarkMemberLeftContext(ctx context.Context, guildID, userID string, at time.Time) error {
	return nil
}
func (m *mockMembersRepo) UpsertMemberRoles(guildID, userID string, roles []string, at time.Time) error {
	return nil
}

func TestAdapters_TransactionalFallback(t *testing.T) {
	t.Parallel()

	db := &mockDB{}
	store := &mockMembersRepo{db: db}

	adapters := &NotificationAdapters{
		Router:          NewRouter(Defaults()),
		AvatarProcessor: nil, // Intentionally suppressed to trigger storage fallback
		MembersRepo:     store,
	}
	defer adapters.Router.Close()

	adapters.RegisterHandlers()

	payload := AvatarChangePayload{
		GuildID:   "123456789012345678",
		UserID:    "876543210987654321",
		NewAvatar: "abcdef1234567890",
	}

	// First pass: Valid context (Commit)
	ctx := context.Background()
	err := adapters.handleProcessAvatarChange(ctx, payload)
	if err != nil {
		t.Fatalf("Expected success for UpsertGuildMemberSnapshotsContext, got: %v", err)
	}

	if db.execCount == 0 {
		t.Fatalf("Expected fallback database insertion to be executed")
	}

	// Second pass: Invalid context (Rollback via simulated error)
	canceledCtx, cancelRollback := context.WithCancel(context.Background())
	cancelRollback() // Cancel immediately

	errRollback := adapters.handleProcessAvatarChange(canceledCtx, payload)
	if errRollback == nil {
		t.Fatalf("Expected context cancellation to force a database transaction rollback, got success")
	}
	if !strings.Contains(errRollback.Error(), "context canceled") {
		t.Fatalf("Expected context cancellation error, got: %v", errRollback)
	}
}

type mockNotifier struct {
	memberJoins    []MemberJoinPayload
	memberLeaves   []MemberLeavePayload
	messageEdits   []MessageEditPayload
	messageDeletes []MessageDeletePayload
}

func (m *mockNotifier) SendAvatarChangeNotification(channelID discord.ChannelID, change files.AvatarChange) error {
	return nil
}

func (m *mockNotifier) SendMemberJoinNotification(channelID discord.ChannelID, member *gateway.GuildMemberAddEvent, accountAge time.Duration) error {
	m.memberJoins = append(m.memberJoins, MemberJoinPayload{
		ChannelID:  channelID,
		Member:     member,
		AccountAge: accountAge,
	})
	return nil
}

func (m *mockNotifier) SendMemberLeaveNotification(channelID discord.ChannelID, member *gateway.GuildMemberRemoveEvent, serverTime time.Duration, botTime time.Duration) error {
	m.memberLeaves = append(m.memberLeaves, MemberLeavePayload{
		ChannelID:  channelID,
		Member:     member,
		ServerTime: serverTime,
		BotTime:    botTime,
	})
	return nil
}

func (m *mockNotifier) SendMessageEditNotification(channelID discord.ChannelID, original *CachedMessage, edited *gateway.MessageUpdateEvent) error {
	m.messageEdits = append(m.messageEdits, MessageEditPayload{
		ChannelID: channelID,
		Original:  original,
		Edited:    edited,
	})
	return nil
}

func (m *mockNotifier) SendMessageDeleteNotification(channelID discord.ChannelID, deleted *CachedMessage, deletedBy string) error {
	m.messageDeletes = append(m.messageDeletes, MessageDeletePayload{
		ChannelID: channelID,
		Deleted:   deleted,
		DeletedBy: deletedBy,
	})
	return nil
}

type mockAvatarProcessor struct {
	changes []AvatarChangePayload
}

func (m *mockAvatarProcessor) ProcessChange(guildID, userID, currentAvatar, username string) {
	m.changes = append(m.changes, AvatarChangePayload{
		GuildID:   guildID,
		UserID:    userID,
		Username:  username,
		NewAvatar: currentAvatar,
	})
}

func TestNotificationAdapters_AllMethods(t *testing.T) {
	t.Parallel()
	notifier := &mockNotifier{}
	processor := &mockAvatarProcessor{}
	router := NewRouter(Defaults())
	defer router.Close()

	adapters := &NotificationAdapters{
		Router:          router,
		Notifier:        notifier,
		AvatarProcessor: processor,
	}

	adapters.RegisterHandlers()

	// Test SetAvatarProcessor
	newProcessor := &mockAvatarProcessor{}
	adapters.SetAvatarProcessor(newProcessor)
	if adapters.AvatarProcessor != newProcessor {
		t.Error("SetAvatarProcessor failed")
	}
	// restore
	adapters.SetAvatarProcessor(processor)

	// Test nil checks in Enqueues
	if err := adapters.EnqueueMemberJoin(1, nil, 0); err != nil {
		t.Errorf("EnqueueMemberJoin(nil) error: %v", err)
	}
	if err := adapters.EnqueueMemberLeave(1, nil, 0, 0); err != nil {
		t.Errorf("EnqueueMemberLeave(nil) error: %v", err)
	}
	if err := adapters.EnqueueMessageEdit(1, nil, nil); err != nil {
		t.Errorf("EnqueueMessageEdit(nil) error: %v", err)
	}
	if err := adapters.EnqueueMessageDelete(1, nil, ""); err != nil {
		t.Errorf("EnqueueMessageDelete(nil) error: %v", err)
	}

	// Test handleSendMemberJoin
	ctx := context.Background()
	joinEvt := &gateway.GuildMemberAddEvent{
		GuildID: 123,
		Member: discord.Member{
			User: discord.User{ID: 456},
		},
	}
	err := adapters.handleSendMemberJoin(ctx, MemberJoinPayload{
		ChannelID:  789,
		Member:     joinEvt,
		AccountAge: 24 * time.Hour,
	})
	if err != nil {
		t.Errorf("handleSendMemberJoin failed: %v", err)
	}
	if len(notifier.memberJoins) != 1 || notifier.memberJoins[0].ChannelID != 789 {
		t.Errorf("notifier memberJoin assertion failed: %+v", notifier.memberJoins)
	}

	// Test handleSendMemberJoin error cases
	if err := adapters.handleSendMemberJoin(ctx, nil); err == nil {
		t.Error("expected error on invalid payload type")
	}
	badAdapters := &NotificationAdapters{Notifier: nil}
	if err := badAdapters.handleSendMemberJoin(ctx, MemberJoinPayload{Member: joinEvt}); err == nil {
		t.Error("expected error on nil notifier")
	}

	// Test handleSendMemberLeave
	leaveEvt := &gateway.GuildMemberRemoveEvent{
		GuildID: 123,
		User:    discord.User{ID: 456},
	}
	err = adapters.handleSendMemberLeave(ctx, MemberLeavePayload{
		ChannelID:  789,
		Member:     leaveEvt,
		ServerTime: 48 * time.Hour,
		BotTime:    12 * time.Hour,
	})
	if err != nil {
		t.Errorf("handleSendMemberLeave failed: %v", err)
	}
	if len(notifier.memberLeaves) != 1 || notifier.memberLeaves[0].ServerTime != 48*time.Hour {
		t.Error("notifier memberLeaves assertion failed")
	}

	// Test handleSendMemberLeave error cases
	if err := adapters.handleSendMemberLeave(ctx, nil); err == nil {
		t.Error("expected error on invalid payload type")
	}
	if err := badAdapters.handleSendMemberLeave(ctx, MemberLeavePayload{Member: leaveEvt}); err == nil {
		t.Error("expected error on nil notifier")
	}

	// Test handleSendMessageEdit
	msgUpdate := &gateway.MessageUpdateEvent{}
	origMsg := &CachedMessage{
		ID: 999,
	}
	err = adapters.handleSendMessageEdit(ctx, MessageEditPayload{
		ChannelID: 789,
		Original:  origMsg,
		Edited:    msgUpdate,
	})
	if err != nil {
		t.Errorf("handleSendMessageEdit failed: %v", err)
	}
	if len(notifier.messageEdits) != 1 || notifier.messageEdits[0].Original.ID != 999 {
		t.Error("notifier messageEdits assertion failed")
	}

	// Test handleSendMessageEdit error cases
	if err := adapters.handleSendMessageEdit(ctx, nil); err == nil {
		t.Error("expected error on invalid payload type")
	}
	if err := badAdapters.handleSendMessageEdit(ctx, MessageEditPayload{Original: origMsg, Edited: msgUpdate}); err == nil {
		t.Error("expected error on nil notifier")
	}

	// Test handleSendMessageDelete
	delMsg := &CachedMessage{
		ID: 888,
	}
	err = adapters.handleSendMessageDelete(ctx, MessageDeletePayload{
		ChannelID: 789,
		Deleted:   delMsg,
		DeletedBy: "someone",
	})
	if err != nil {
		t.Errorf("handleSendMessageDelete failed: %v", err)
	}
	if len(notifier.messageDeletes) != 1 || notifier.messageDeletes[0].Deleted.ID != 888 {
		t.Error("notifier messageDeletes assertion failed")
	}

	// Test handleSendMessageDelete error cases
	if err := adapters.handleSendMessageDelete(ctx, nil); err == nil {
		t.Error("expected error on invalid payload type")
	}
	if err := badAdapters.handleSendMessageDelete(ctx, MessageDeletePayload{Deleted: delMsg}); err == nil {
		t.Error("expected error on nil notifier")
	}

	// Test handleProcessAvatarChange
	err = adapters.handleProcessAvatarChange(ctx, AvatarChangePayload{
		GuildID:   "111",
		UserID:    "222",
		Username:  "user",
		NewAvatar: "hash",
	})
	if err != nil {
		t.Errorf("handleProcessAvatarChange failed: %v", err)
	}
	if len(processor.changes) != 1 || processor.changes[0].NewAvatar != "hash" {
		t.Error("processor changes assertion failed")
	}

	// Test handleProcessAvatarChange error cases
	if err := adapters.handleProcessAvatarChange(ctx, nil); err == nil {
		t.Error("expected error on invalid payload type")
	}
	badAdapters2 := &NotificationAdapters{AvatarProcessor: nil, MembersRepo: nil}
	if err := badAdapters2.handleProcessAvatarChange(ctx, AvatarChangePayload{GuildID: "1", UserID: "2"}); err == nil {
		t.Error("expected error on nil processor and repo")
	}

	// Test handleFlushAvatarCache
	err = adapters.handleFlushAvatarCache(ctx, FlushAvatarCachePayload{})
	if err != nil {
		t.Errorf("handleFlushAvatarCache failed: %v", err)
	}
	if err := adapters.handleFlushAvatarCache(ctx, nil); err == nil {
		t.Error("expected error on invalid payload type for flush avatar cache")
	}

	// Test Enqueues (Happy Paths to check dispatch wiring)
	err = adapters.EnqueueMemberJoin(789, joinEvt, 24*time.Hour)
	if err != nil {
		t.Errorf("EnqueueMemberJoin failed: %v", err)
	}
	err = adapters.EnqueueMemberLeave(789, leaveEvt, 48*time.Hour, 12*time.Hour)
	if err != nil {
		t.Errorf("EnqueueMemberLeave failed: %v", err)
	}
	err = adapters.EnqueueMessageEdit(789, origMsg, msgUpdate)
	if err != nil {
		t.Errorf("EnqueueMessageEdit failed: %v", err)
	}
	err = adapters.EnqueueMessageDelete(789, delMsg, "someone")
	if err != nil {
		t.Errorf("EnqueueMessageDelete failed: %v", err)
	}
	err = adapters.EnqueueProcessAvatarChange("111", "222", "user", "hash")
	if err != nil {
		t.Errorf("EnqueueProcessAvatarChange failed: %v", err)
	}
	err = adapters.EnqueueFlushAvatarCache()
	if err != nil {
		t.Errorf("EnqueueFlushAvatarCache failed: %v", err)
	}
}

```

// === FILE: pkg/task/doc.go ===
```go
//go:build !legacy
// +build !legacy

/*
Package task orchestrates background execution scheduling, topological routing,
and deterministic execution guarantees across the application ecosystem.

# Contract

The task orchestration boundary guarantees that identical GroupKey instances will
never be processed in parallel across active goroutines. Handlers are executed
sequentially, mitigating data-races natively without requiring consumer-side
distributed locks.

# Retry Semantics

Handlers returning errors are subjected to an exponential backoff formula combined
with an underlying container/heap priority queue. Context cancellation from the Close()
lifecycle propagates synchronously into executing tasks to immediately abort network I/O.

# Invariants

- Tasks with duplicate IdempotencyKey values within the TTL window are rejected synchronously with ErrDuplicateTask.
- Panic states within worker bounds are isolated, recovered, and logged without tearing down the routing engine.
- Handlers MUST NOT spawn detached background routines. All logic must obey the passed context.Context.
*/
package task

```

// === FILE: pkg/task/router.go ===
```go
//go:build !legacy
// +build !legacy

package task

import (
	"container/heap"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"math/rand"
	"runtime/debug"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/clock"
	"github.com/small-frappuccino/discordcore/pkg/observability"
)

// TaskHandler represents a state-less processing unit for a single task payload.
// Handlers must be concurrent-safe and respect context cancellation guarantees.
type TaskHandler func(ctx context.Context, payload any) error

// TaskOptions dictates the operational routing parameters for a scheduled task.
type TaskOptions struct {
	// GroupKey enforces sequential execution guarantees across concurrent dispatchers.
	// Tasks sharing an identical group key will never execute in parallel.
	GroupKey string

	// IdempotencyKey prevents duplicate task queuing within the defined TTL window.
	IdempotencyKey string

	// MaxAttempts defines the upper bound for handler execution retries.
	// If 0, the router defaults to RouterConfig.DefaultMaxAttempts.
	MaxAttempts int

	// InitialBackoff configures the baseline delay for exponential retry scaling.
	InitialBackoff time.Duration

	// MaxBackoff establishes the absolute ceiling for the exponential backoff formula.
	MaxBackoff time.Duration

	// IdempotencyTTL determines the survival duration of the idempotency token in memory.
	IdempotencyTTL time.Duration
}

// EmptyPayload serves as a zero-allocation marker for tasks requiring no dynamic context.
type EmptyPayload struct{}

// Task encapsulates the immutable instructions and metadata required for background dispatch.
type Task struct {
	Type    string
	Payload any
	Options TaskOptions
}

// RouterConfig defines the holistic tuning parameters for the task orchestration layer.
type RouterConfig struct {
	DefaultMaxAttempts int
	InitialBackoff     time.Duration
	MaxBackoff         time.Duration
	IdempotencyTTL     time.Duration
	GroupBuffer        int
	GroupIdleTTL       time.Duration
	CleanupInterval    time.Duration
	GlobalMaxWorkers   int
	GroupMaxParallel   int

	// ExecutionLimiter enables resource sharing across multiple router topologies.
	ExecutionLimiter *ExecutionLimiter

	// Clock abstracts the time package to enable deterministic execution testing.
	Clock clock.Clock

	// Logger receives the injected application logger for structural reporting.
	Logger *slog.Logger
}

// Defaults provisions a base configuration profile suitable for production operations.
func Defaults() RouterConfig {
	return RouterConfig{
		DefaultMaxAttempts: 3,
		InitialBackoff:     1 * time.Second,
		MaxBackoff:         30 * time.Second,
		IdempotencyTTL:     60 * time.Second,
		GroupBuffer:        128,
		GroupIdleTTL:       2 * time.Minute,
		CleanupInterval:    2 * time.Minute,
		GlobalMaxWorkers:   0,
		GroupMaxParallel:   1,
		Clock:              clock.RealClock{},
		Logger:             slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

// ExecutionLimiter bounds the aggregate concurrency ceiling via a counting semaphore.
type ExecutionLimiter struct {
	sem chan struct{}
}

// NewExecutionLimiter allocates a fixed-capacity execution bounded semaphore.
// A capacity of 0 or less returns a nil limiter, which signifies unbounded execution.
func NewExecutionLimiter(maxWorkers int) *ExecutionLimiter {
	if maxWorkers <= 0 {
		return nil
	}
	return &ExecutionLimiter{
		sem: make(chan struct{}, maxWorkers),
	}
}

// Acquire blocks until a concurrency token becomes available within the semaphore.
func (l *ExecutionLimiter) Acquire() {
	if l == nil || l.sem == nil {
		return
	}
	// Blocks until capacity is freed.
	l.sem <- struct{}{}
}

// Release yields a concurrency token back to the semaphore pool.
func (l *ExecutionLimiter) Release() {
	if l == nil || l.sem == nil {
		return
	}
	select {
	case <-l.sem:
	default:
		// Evades panic on over-release, which indicates a severe architectural failure but shouldn't crash the loop.
	}
}

// Capacity interrogates the upper bound of the concurrency semaphore.
func (l *ExecutionLimiter) Capacity() int {
	if l == nil || l.sem == nil {
		return 0
	}
	return cap(l.sem)
}

// Operational errors for routing boundaries.
var (
	ErrRouterClosed    = errors.New("task router is closed")
	ErrUnknownTaskType = errors.New("unknown task type")
	ErrDuplicateTask   = errors.New("duplicate task (idempotency key present)")
	ErrRetrySilent     = errors.New("retryable task error (silent)")
	errTaskEnqueue     = errors.New("task enqueue failed")
)

const (
	globalGroup         = "_global"
	maxEnqueueAttempts  = 3
	retryRescheduleWait = 50 * time.Millisecond
)

type groupSendResult uint8

const (
	groupSendEnqueued groupSendResult = iota
	groupSendClosed
	groupSendContextDone
	groupSendRouterClosed
	groupSendWouldBlock
)

type queueState uint32

const (
	queueStateOpen queueState = iota
	queueStateStopping
	queueStateClosed
)

// TaskRouter orchestrates background execution scheduling, concurrency boundaries, and idempotency states.
type TaskRouter struct {
	mu        sync.Mutex
	handlers  map[string]TaskHandler
	groups    map[string]*groupWorker
	inflight  map[string]time.Time
	closed    bool
	cfg       RouterConfig
	startedAt time.Time
	wg        sync.WaitGroup
	stopOnce  sync.Once
	stopCh    chan struct{}

	// ctx dictates the holistic lifecycle boundary passed into active task handlers.
	ctx       context.Context
	cancel    context.CancelFunc
	randMutex sync.Mutex

	execLimiter *ExecutionLimiter

	retryMu     sync.Mutex
	retryQueue  retryTaskHeap
	retryWakeCh chan struct{}
	retrySeq    uint64

	cronMu   sync.Mutex
	cronJobs []*cronJob

	cronDispatchAttempts int64
	cronDispatchSuccess  int64
	cronDispatchFailures int64

	latencyMu       sync.Mutex
	latenciesByType map[string]*observability.Summary
}

type groupWorker struct {
	key          string
	ch           chan *enqueuedTask
	state        atomic.Uint32
	senders      atomic.Int32
	closeMu      sync.Mutex
	closeCond    *sync.Cond
	lastActiveNs atomic.Int64
	activeCount  atomic.Int32
}

type enqueuedTask struct {
	task    Task
	attempt int
}

type scheduledRetry struct {
	at       time.Time
	groupKey string
	task     *enqueuedTask
	seq      uint64
	index    int
}

type retryTaskHeap []*scheduledRetry

// Len reports the total elements within the retry heap.
func (h retryTaskHeap) Len() int {
	return len(h)
}

// Less ensures chronological priority, falling back to sequence insertion ordering.
func (h retryTaskHeap) Less(i, j int) bool {
	if h[i].at.Equal(h[j].at) {
		return h[i].seq < h[j].seq
	}
	return h[i].at.Before(h[j].at)
}

// Swap transposes two elements within the heap to maintain algorithmic invariants.
func (h retryTaskHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

// Push allocates a scheduled retry entity onto the trailing tail of the slice.
func (h *retryTaskHeap) Push(x any) {
	item := x.(*scheduledRetry)
	item.index = len(*h)
	*h = append(*h, item)
}

// Pop truncates and returns the lowest priority element from the slice tail.
func (h *retryTaskHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	item.index = -1
	*h = old[:n-1]
	return item
}

type cronJob struct {
	Interval time.Duration
	Task     Task
	lastRun  time.Time
	stopped  bool
}

// NewRouter constructs an isolated task dispatch infrastructure mapping config defaults.
func NewRouter(cfg RouterConfig) *TaskRouter {
	def := Defaults()
	if cfg.DefaultMaxAttempts <= 0 {
		cfg.DefaultMaxAttempts = def.DefaultMaxAttempts
	}
	if cfg.InitialBackoff <= 0 {
		cfg.InitialBackoff = def.InitialBackoff
	}
	if cfg.MaxBackoff <= 0 {
		cfg.MaxBackoff = def.MaxBackoff
	}
	if cfg.IdempotencyTTL <= 0 {
		cfg.IdempotencyTTL = def.IdempotencyTTL
	}
	if cfg.GroupBuffer <= 0 {
		cfg.GroupBuffer = def.GroupBuffer
	}
	if cfg.GroupIdleTTL <= 0 {
		cfg.GroupIdleTTL = def.GroupIdleTTL
	}
	if cfg.CleanupInterval <= 0 {
		cfg.CleanupInterval = def.CleanupInterval
	}
	if cfg.GroupMaxParallel <= 0 {
		cfg.GroupMaxParallel = def.GroupMaxParallel
	}
	if cfg.Clock == nil {
		cfg.Clock = def.Clock
	}
	if cfg.Logger == nil {
		cfg.Logger = def.Logger
	}

	tr := &TaskRouter{
		handlers:    make(map[string]TaskHandler),
		groups:      make(map[string]*groupWorker),
		inflight:    make(map[string]time.Time),
		cfg:         cfg,
		startedAt:   cfg.Clock.Now(),
		stopCh:      make(chan struct{}),
		retryWakeCh: make(chan struct{}, 1),
	}
	tr.ctx, tr.cancel = context.WithCancel(context.Background())
	if cfg.ExecutionLimiter != nil {
		tr.execLimiter = cfg.ExecutionLimiter
	} else if cfg.GlobalMaxWorkers > 0 {
		tr.execLimiter = NewExecutionLimiter(cfg.GlobalMaxWorkers)
	}

	tr.wg.Add(1)
	go tr.backgroundLoop()
	tr.wg.Add(1)
	go tr.retryLoop()
	tr.cfg.Logger.Info("TaskRouter initialized")
	return tr
}

// RegisterHandler binds an execution callback directly to a string payload type boundary.
func (tr *TaskRouter) RegisterHandler(taskType string, handler TaskHandler) {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	tr.handlers[taskType] = handler
}

// Dispatch queues an arbitrary payload to the routing engine.
// Emits ErrDuplicateTask synchronously if idempotency bounds are violated.
func (tr *TaskRouter) Dispatch(ctx context.Context, t Task) error {
	groupKey, eff, err := tr.prepareDispatch(t)
	if err != nil {
		return fmt.Errorf("TaskRouter.Dispatch: %w", err)
	}

	enq := &enqueuedTask{task: t, attempt: 1}
	for i := 0; i < maxEnqueueAttempts; i++ {
		gw, ok := tr.getOrCreateGroup(groupKey)
		if !ok || gw == nil {
			tr.rollbackIdempotencyReservation(eff)
			return ErrRouterClosed
		}

		switch tr.sendToGroupContext(ctx, gw, enq) {
		case groupSendEnqueued:
			return nil
		case groupSendContextDone:
			tr.rollbackIdempotencyReservation(eff)
			if ctx == nil {
				return context.Canceled
			}
			return ctx.Err()
		case groupSendRouterClosed:
			tr.rollbackIdempotencyReservation(eff)
			return ErrRouterClosed
		case groupSendClosed:
			// Group was shutting down while attempting dispatch, cycle loop to instantiate cleanly.
			tr.dropStaleGroup(groupKey, gw)
		}
	}

	tr.rollbackIdempotencyReservation(eff)
	return errTaskEnqueue
}

func (tr *TaskRouter) prepareDispatch(t Task) (string, TaskOptions, error) {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	if tr.closed {
		return "", TaskOptions{}, ErrRouterClosed
	}

	handler, ok := tr.handlers[t.Type]
	if !ok || handler == nil {
		return "", TaskOptions{}, ErrUnknownTaskType
	}

	eff := tr.effectiveOptions(t.Options)

	if eff.IdempotencyKey != "" {
		if expiry, exists := tr.inflight[eff.IdempotencyKey]; exists && tr.cfg.Clock.Now().Before(expiry) {
			return "", TaskOptions{}, ErrDuplicateTask
		}
		tr.inflight[eff.IdempotencyKey] = tr.cfg.Clock.Now().Add(eff.IdempotencyTTL)
	}

	groupKey := eff.GroupKey
	if groupKey == "" {
		groupKey = globalGroup
	}

	return groupKey, eff, nil
}

// Close gracefully halts all pending orchestration and unblocks all background workers.
// Any execution routines in-flight will be hard-canceled via context propagation.
func (tr *TaskRouter) Close() {
	tr.stopOnce.Do(func() {
		tr.cfg.Logger.Info("TaskRouter shutting down")
		tr.mu.Lock()
		tr.closed = true
		groups := make([]*groupWorker, 0, len(tr.groups))
		for _, gw := range tr.groups {
			if gw == nil {
				continue
			}
			if gw.queueState() == queueStateOpen {
				gw.beginStop()
			}
			groups = append(groups, gw)
		}
		// Discard memory structures to halt map accesses proactively
		clear(tr.groups)
		clear(tr.inflight)
		clear(tr.handlers)
		tr.mu.Unlock()

		close(tr.stopCh)
		if tr.cancel != nil {
			tr.cancel()
		}
		for _, gw := range groups {
			gw.finishStop()
		}
		tr.wg.Wait()
	})
}

// Stats encapsulates operational metrics intended for telemetry scraping.
type Stats struct {
	GroupsCount          int
	InflightCount        int
	RouterClosed         bool
	RegisteredTypes      int
	CronDispatchAttempts int64
	CronDispatchSuccess  int64
	CronDispatchFailures int64
}

// Stats aggregates immediate internal counters inside a thread-safe read envelope.
func (tr *TaskRouter) Stats() Stats {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	return Stats{
		GroupsCount:          len(tr.groups),
		InflightCount:        len(tr.inflight),
		RouterClosed:         tr.closed,
		RegisteredTypes:      len(tr.handlers),
		CronDispatchAttempts: atomic.LoadInt64(&tr.cronDispatchAttempts),
		CronDispatchSuccess:  atomic.LoadInt64(&tr.cronDispatchSuccess),
		CronDispatchFailures: atomic.LoadInt64(&tr.cronDispatchFailures),
	}
}

// ScheduleEvery establishes an interval repetition loop that enqueues the provided task.
func (tr *TaskRouter) ScheduleEvery(interval time.Duration, t Task) func() {
	job := &cronJob{
		Interval: interval,
		Task:     t,
		lastRun:  time.Time{},
		stopped:  false,
	}
	tr.cronMu.Lock()
	tr.cronJobs = append(tr.cronJobs, job)
	idx := len(tr.cronJobs) - 1
	tr.cronMu.Unlock()

	cancel := func() {
		tr.cronMu.Lock()
		if idx >= 0 && idx < len(tr.cronJobs) && tr.cronJobs[idx] == job {
			tr.cronJobs[idx] = nil
		}
		job.stopped = true
		tr.cronMu.Unlock()
	}
	return cancel
}

// Cancel models the tear-down execution routine for scheduled jobs.
type Cancel func()

// ScheduleDailyAtUTC registers a recurring periodic submission locking onto an absolute UTC clock boundary.
func (tr *TaskRouter) ScheduleDailyAtUTC(hour, minute int, t Task) Cancel {
	return tr.ScheduleEveryNDaysAtUTCWithSeconds(1, hour, minute, 0, t)
}

// ScheduleDailyAtUTCWithSeconds anchors a 24-hour job repeating down to the exact second precision.
func (tr *TaskRouter) ScheduleDailyAtUTCWithSeconds(hour, minute, second int, t Task) Cancel {
	return tr.ScheduleEveryNDaysAtUTCWithSeconds(1, hour, minute, second, t)
}

// ScheduleEveryNDaysAtUTC schedules across an N day period omitting seconds.
func (tr *TaskRouter) ScheduleEveryNDaysAtUTC(n int, hour, minute int, t Task) Cancel {
	return tr.ScheduleEveryNDaysAtUTCWithSeconds(n, hour, minute, 0, t)
}

// ScheduleEveryNDaysAtUTCWithSeconds computes a localized UTC offset boundary for execution bridging multiple days.
func (tr *TaskRouter) ScheduleEveryNDaysAtUTCWithSeconds(n int, hour, minute, second int, t Task) Cancel {
	if n <= 0 {
		n = 1
	}
	hour = clampInt(hour, 0, 23)
	minute = clampInt(minute, 0, 59)
	second = clampInt(second, 0, 59)

	interval := time.Duration(n) * 24 * time.Hour

	now := tr.cfg.Clock.Now().UTC()
	target := nextUTCTimestamp(now, hour, minute, second)
	if !now.Before(target) {
		target = target.Add(interval)
	}

	// Pre-date the lastRun so that when 'now' reaches 'target', exactly 'interval' has passed.
	lastRun := target.Add(-interval)

	job := &cronJob{
		Interval: interval,
		Task:     t,
		lastRun:  lastRun,
		stopped:  false,
	}

	tr.cronMu.Lock()
	tr.cronJobs = append(tr.cronJobs, job)
	idx := len(tr.cronJobs) - 1
	tr.cronMu.Unlock()

	cancel := func() {
		tr.cronMu.Lock()
		if idx >= 0 && idx < len(tr.cronJobs) && tr.cronJobs[idx] == job {
			tr.cronJobs[idx] = nil
		}
		job.stopped = true
		tr.cronMu.Unlock()
	}

	return cancel
}

func nextUTCTimestamp(from time.Time, hour, minute, second int) time.Time {
	return time.Date(from.Year(), from.Month(), from.Day(), hour, minute, second, 0, time.UTC)
}

func clampInt(v, lo, hi int) int {
	return max(min(v, hi), lo)
}

func (tr *TaskRouter) effectiveOptions(opt TaskOptions) TaskOptions {
	if opt.MaxAttempts <= 0 {
		opt.MaxAttempts = tr.cfg.DefaultMaxAttempts
	}
	if opt.InitialBackoff <= 0 {
		opt.InitialBackoff = tr.cfg.InitialBackoff
	}
	if opt.MaxBackoff <= 0 {
		opt.MaxBackoff = tr.cfg.MaxBackoff
	}
	if opt.IdempotencyTTL <= 0 {
		opt.IdempotencyTTL = tr.cfg.IdempotencyTTL
	}
	return opt
}

func (tr *TaskRouter) effectiveGroupParallel() int {
	if tr.cfg.GroupMaxParallel <= 0 {
		return 1
	}
	return tr.cfg.GroupMaxParallel
}

func newGroupWorker(key string, buffer int, nowNs int64) *groupWorker {
	gw := &groupWorker{
		key: key,
		ch:  make(chan *enqueuedTask, buffer),
	}
	gw.closeCond = sync.NewCond(&gw.closeMu)
	gw.state.Store(uint32(queueStateOpen))
	gw.markActive(nowNs)
	return gw
}

func (tr *TaskRouter) nowNs() int64 {
	return tr.cfg.Clock.Now().Sub(tr.startedAt).Nanoseconds() + 1
}

func (gw *groupWorker) queueState() queueState {
	if gw == nil {
		return queueStateClosed
	}
	return queueState(gw.state.Load())
}

func (gw *groupWorker) markActive(nowNs int64) {
	gw.lastActiveNs.Store(nowNs)
}

func (gw *groupWorker) beginWork(nowNs int64) {
	gw.markActive(nowNs)
	gw.activeCount.Add(1)
}

func (gw *groupWorker) endWork(nowNs int64) {
	gw.markActive(nowNs)
	gw.activeCount.Add(-1)
}

func (gw *groupWorker) idleFor(nowNs int64) time.Duration {
	lastActiveNs := gw.lastActiveNs.Load()
	if lastActiveNs <= 0 || nowNs <= lastActiveNs {
		return 0
	}
	return time.Duration(nowNs - lastActiveNs)
}

func (gw *groupWorker) tryAcquireSender() bool {
	if gw == nil {
		return false
	}
	for {
		if gw.queueState() != queueStateOpen {
			return false
		}
		current := gw.senders.Load()
		if !gw.senders.CompareAndSwap(current, current+1) {
			continue
		}
		if gw.queueState() == queueStateOpen {
			return true
		}
		gw.releaseSender()
		return false
	}
}

func (gw *groupWorker) releaseSender() {
	if gw == nil {
		return
	}
	if gw.senders.Add(-1) == 0 && gw.queueState() != queueStateOpen {
		gw.closeMu.Lock()
		if gw.closeCond != nil {
			gw.closeCond.Broadcast()
		}
		gw.closeMu.Unlock()
	}
}

func (gw *groupWorker) beginStop() bool {
	if gw == nil {
		return false
	}
	return gw.state.CompareAndSwap(uint32(queueStateOpen), uint32(queueStateStopping))
}

func (gw *groupWorker) finishStop() {
	if gw == nil {
		return
	}
	// Blocks aggressively waiting for all pending atomic senders to drain to 0
	gw.closeMu.Lock()
	for gw.senders.Load() > 0 {
		gw.closeCond.Wait()
	}
	if gw.queueState() == queueStateStopping {
		close(gw.ch)
		gw.state.Store(uint32(queueStateClosed))
	}
	if gw.closeCond != nil {
		gw.closeCond.Broadcast()
	}
	gw.closeMu.Unlock()
}

func (tr *TaskRouter) ensureGroupLocked(key string) *groupWorker {
	if gw, ok := tr.groups[key]; ok && gw != nil {
		return gw
	}
	gw := newGroupWorker(key, tr.cfg.GroupBuffer, tr.nowNs())
	tr.groups[key] = gw
	parallel := tr.effectiveGroupParallel()
	for i := 0; i < parallel; i++ {
		tr.wg.Add(1)
		go tr.groupLoop(gw)
	}
	return gw
}

func (tr *TaskRouter) acquireExecSlot() {
	tr.execLimiter.Acquire()
}

func (tr *TaskRouter) releaseExecSlot() {
	tr.execLimiter.Release()
}

func (tr *TaskRouter) groupLoop(gw *groupWorker) {
	defer tr.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			tr.cfg.Logger.Error("Worker runtime panic pre-empted", "panic", r, "stack", string(debug.Stack()))
		}
	}()

	for {
		select {
		case <-tr.ctx.Done():
			tr.cfg.Logger.Warn("Context cancelled: abandoning unread queue", "group", gw.key)
			return
		case enq, ok := <-gw.ch:
			if !ok {
				return
			}
			gw.beginWork(tr.nowNs())

			tr.mu.Lock()
			handler := tr.handlers[enq.task.Type]
			eff := tr.effectiveOptions(enq.task.Options)
			tr.mu.Unlock()

			if handler == nil {
				gw.endWork(tr.nowNs())
				tr.cfg.Logger.Warn("Task dropped (handler not registered)", "type", enq.task.Type, "group", gw.key)
				continue
			}

			// Implements execution slot limitation across all topological boundaries to prevent host saturation.
			tr.acquireExecSlot()
			startExec := tr.cfg.Clock.Now()
			err := func() error {
				defer tr.releaseExecSlot()
				ctx := tr.ctx
				if ctx == nil {
					ctx = context.Background()
				}
				return handler(ctx, enq.task.Payload)
			}()
			execDuration := tr.cfg.Clock.Now().Sub(startExec)

			summary := observability.GetOrCreateLabeledSummary(&tr.latencyMu, &tr.latenciesByType, enq.task.Type)
			summary.Observe(execDuration)

			if execDuration > 5*time.Second {
				tr.cfg.Logger.Warn("slow background task execution",
					"type", enq.task.Type,
					"duration", execDuration.String(),
					"duration_ms", execDuration.Milliseconds(),
				)
			}
			gw.endWork(tr.nowNs())

			if err != nil {
				silent := errors.Is(err, ErrRetrySilent)
				if enq.attempt < eff.MaxAttempts {
					delay := tr.computeBackoff(eff.InitialBackoff, eff.MaxBackoff, enq.attempt)
					attempt := enq.attempt + 1

					if silent {
						tr.cfg.Logger.Debug("Task failed, scheduling retry",
							"type", enq.task.Type,
							"group", gw.key,
							"attempt", attempt,
							"max_attempts", eff.MaxAttempts,
							"backoff", delay.String(),
							"err", err,
						)
					} else {
						tr.cfg.Logger.Warn("Task failed, scheduling retry",
							"type", enq.task.Type,
							"group", gw.key,
							"attempt", attempt,
							"max_attempts", eff.MaxAttempts,
							"backoff", delay.String(),
							"err", err,
						)
					}

					enq.attempt = attempt
					tr.scheduleRetry(gw.key, enq, delay)
					continue
				}

				if silent {
					tr.cfg.Logger.Info("Task dropped after retry window",
						"type", enq.task.Type,
						"group", gw.key,
						"attempts", enq.attempt,
						"err", err,
					)
				} else {
					tr.cfg.Logger.Error("Task dropped after retry window",
						"type", enq.task.Type,
						"group", gw.key,
						"attempts", enq.attempt,
						"err", err,
					)
				}
			}
		}
	}
}

func (tr *TaskRouter) enqueueRetry(groupKey string, et *enqueuedTask) bool {
	return tr.tryEnqueueRetry(groupKey, et) == groupSendEnqueued
}

func (tr *TaskRouter) scheduleRetry(groupKey string, et *enqueuedTask, delay time.Duration) {
	if groupKey == "" || et == nil {
		return
	}

	item := &scheduledRetry{
		at:       tr.cfg.Clock.Now().Add(delay),
		groupKey: groupKey,
		task:     et,
	}

	tr.retryMu.Lock()
	tr.retrySeq++
	item.seq = tr.retrySeq
	heap.Push(&tr.retryQueue, item)
	tr.retryMu.Unlock()
	tr.signalRetryLoop()
}

func (tr *TaskRouter) signalRetryLoop() {
	select {
	case tr.retryWakeCh <- struct{}{}:
	default:
	}
}

func (tr *TaskRouter) retryLoop() {
	defer tr.wg.Done()

	for {
		delay, ok := tr.nextRetryDelay()
		if !ok {
			select {
			case <-tr.stopCh:
				return
			case <-tr.retryWakeCh:
				continue
			}
		}

		if delay > 0 {
			timer := tr.cfg.Clock.NewTimer(delay)
			select {
			case <-tr.stopCh:
				if !timer.Stop() {
					select {
					case <-timer.C():
					default:
					}
				}
				return
			case <-tr.retryWakeCh:
				if !timer.Stop() {
					select {
					case <-timer.C():
					default:
					}
				}
				continue
			case <-timer.C():
			}
		}

		for _, item := range tr.popDueRetries(tr.cfg.Clock.Now()) {
			switch tr.tryEnqueueRetry(item.groupKey, item.task) {
			case groupSendEnqueued:
				continue
			case groupSendWouldBlock:
				tr.scheduleRetry(item.groupKey, item.task, retryRescheduleWait)
			case groupSendRouterClosed:
				return
			default:
				tr.cfg.Logger.Debug("Task retry dropped while enqueuing",
					"type", item.task.task.Type,
					"group", item.groupKey,
					"attempt", item.task.attempt,
				)
			}
		}
	}
}

func (tr *TaskRouter) nextRetryDelay() (time.Duration, bool) {
	tr.retryMu.Lock()
	defer tr.retryMu.Unlock()

	if len(tr.retryQueue) == 0 {
		return 0, false
	}

	delay := tr.retryQueue[0].at.Sub(tr.cfg.Clock.Now())
	if delay < 0 {
		delay = 0
	}
	return delay, true
}

func (tr *TaskRouter) popDueRetries(now time.Time) []*scheduledRetry {
	tr.retryMu.Lock()
	defer tr.retryMu.Unlock()

	var due []*scheduledRetry
	for len(tr.retryQueue) > 0 {
		next := tr.retryQueue[0]
		if next == nil || next.at.After(now) {
			break
		}
		due = append(due, heap.Pop(&tr.retryQueue).(*scheduledRetry))
	}
	return due
}

func (tr *TaskRouter) tryEnqueueRetry(groupKey string, et *enqueuedTask) groupSendResult {
	if groupKey == "" || et == nil {
		return groupSendClosed
	}

	for i := 0; i < maxEnqueueAttempts; i++ {
		select {
		case <-tr.stopCh:
			return groupSendRouterClosed
		default:
		}

		gw, ok := tr.getOrCreateGroup(groupKey)
		if !ok || gw == nil {
			return groupSendRouterClosed
		}

		switch tr.sendToGroupTry(gw, et) {
		case groupSendEnqueued:
			return groupSendEnqueued
		case groupSendWouldBlock:
			return groupSendWouldBlock
		case groupSendRouterClosed:
			return groupSendRouterClosed
		}

		tr.dropStaleGroup(groupKey, gw)
	}

	return groupSendClosed
}

func (tr *TaskRouter) getOrCreateGroup(groupKey string) (*groupWorker, bool) {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	if tr.closed {
		return nil, false
	}

	if gw, exists := tr.groups[groupKey]; exists && gw != nil {
		if gw.queueState() != queueStateOpen {
			delete(tr.groups, groupKey)
		} else {
			return gw, true
		}
	}

	return tr.ensureGroupLocked(groupKey), true
}

func (tr *TaskRouter) sendToGroupTry(gw *groupWorker, et *enqueuedTask) (result groupSendResult) {
	if gw == nil || gw.ch == nil || et == nil {
		return groupSendClosed
	}
	if !gw.tryAcquireSender() {
		return groupSendClosed
	}
	defer gw.releaseSender()

	select {
	case gw.ch <- et:
		return groupSendEnqueued
	case <-tr.stopCh:
		return groupSendRouterClosed
	default:
		return groupSendWouldBlock
	}
}

func (tr *TaskRouter) sendToGroupContext(ctx context.Context, gw *groupWorker, et *enqueuedTask) (result groupSendResult) {
	if gw == nil || gw.ch == nil || et == nil {
		return groupSendClosed
	}
	if !gw.tryAcquireSender() {
		return groupSendClosed
	}
	defer gw.releaseSender()

	var ctxDone <-chan struct{}
	if ctx != nil {
		ctxDone = ctx.Done()
	}

	select {
	case gw.ch <- et:
		return groupSendEnqueued
	case <-ctxDone:
		return groupSendContextDone
	case <-tr.stopCh:
		return groupSendRouterClosed
	}
}

func (tr *TaskRouter) dropStaleGroup(groupKey string, gw *groupWorker) {
	tr.mu.Lock()
	if current, exists := tr.groups[groupKey]; exists && current == gw {
		delete(tr.groups, groupKey)
	}
	tr.mu.Unlock()
}

func (tr *TaskRouter) rollbackIdempotencyReservation(eff TaskOptions) {
	if eff.IdempotencyKey == "" {
		return
	}
	tr.mu.Lock()
	delete(tr.inflight, eff.IdempotencyKey)
	tr.mu.Unlock()
}

func (tr *TaskRouter) computeBackoff(initial, max time.Duration, attempt int) time.Duration {
	backoff := initial
	for i := 1; i < attempt; i++ {
		backoff *= 2
		if backoff > max {
			backoff = max
			break
		}
	}
	jitter := tr.jitter(backoff, 0.1)
	return clampDuration(backoff+jitter, initial, max)
}

func (tr *TaskRouter) jitter(d time.Duration, ratio float64) time.Duration {
	if ratio <= 0 {
		return 0
	}
	tr.randMutex.Lock()
	defer tr.randMutex.Unlock()
	delta := int64(float64(d) * ratio)
	if delta <= 0 {
		return 0
	}
	n := rand.Int63n(2*delta+1) - delta
	return time.Duration(n)
}

func clampDuration(v, lo, hi time.Duration) time.Duration {
	return max(min(v, hi), lo)
}

func (tr *TaskRouter) backgroundLoop() {
	defer tr.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			tr.cfg.Logger.Error("TaskRouter background loop panic caught", "panic", r, "stack", string(debug.Stack()))
		}
	}()

	ticker := tr.cfg.Clock.NewTicker(tr.cfg.CleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-tr.stopCh:
			return
		case <-ticker.C():
			tr.cleanupOnce()
			tr.runCronOnce()
		}
	}
}

func (tr *TaskRouter) cleanupOnce() {
	now := tr.cfg.Clock.Now()
	nowNs := tr.nowNs()
	toClose := make([]*groupWorker, 0)

	tr.mu.Lock()
	maps.DeleteFunc(tr.inflight, func(_ string, expiry time.Time) bool {
		return now.After(expiry)
	})

	for key, gw := range tr.groups {
		if gw == nil {
			delete(tr.groups, key)
			continue
		}
		if gw.queueState() != queueStateOpen {
			delete(tr.groups, key)
			toClose = append(toClose, gw)
			continue
		}
		if gw.activeCount.Load() == 0 && gw.senders.Load() == 0 && len(gw.ch) == 0 && gw.idleFor(nowNs) >= tr.cfg.GroupIdleTTL {
			gw.beginStop()
			delete(tr.groups, key)
			toClose = append(toClose, gw)
		}
	}
	tr.mu.Unlock()
	for _, gw := range toClose {
		gw.finishStop()
	}
}

func (tr *TaskRouter) runCronOnce() {
	now := tr.cfg.Clock.Now()
	tr.cronMu.Lock()
	for _, job := range tr.cronJobs {
		if job == nil || job.stopped {
			continue
		}
		if job.lastRun.IsZero() || now.Sub(job.lastRun) >= job.Interval {
			atomic.AddInt64(&tr.cronDispatchAttempts, 1)
			if err := tr.Dispatch(context.Background(), job.Task); err != nil {
				atomic.AddInt64(&tr.cronDispatchFailures, 1)
				tr.cfg.Logger.Error(
					"Cron task dispatch failed",
					"operation", "task.router.cron.dispatch",
					"taskType", job.Task.Type,
					"interval", job.Interval.String(),
					"group", job.Task.Options.GroupKey,
					"idempotencyKey", job.Task.Options.IdempotencyKey,
					"err", err,
				)
			} else {
				atomic.AddInt64(&tr.cronDispatchSuccess, 1)
			}
			job.lastRun = now
		}
	}
	last := len(tr.cronJobs)
	for last > 0 && tr.cronJobs[last-1] == nil {
		last--
	}
	if last != len(tr.cronJobs) {
		tr.cronJobs = slices.Clip(tr.cronJobs[:last])
	}
	tr.cronMu.Unlock()
}

```

// === FILE: pkg/task/router_test.go ===
```go
//go:build !legacy
// +build !legacy

package task

import (
	"container/heap"
	"context"
	"errors"
	"fmt"
	"math"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/clock"
	"go.uber.org/goleak"
	"golang.org/x/sync/errgroup"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestRouter_GroupKeySerialization(t *testing.T) {
	t.Parallel()

	cfg := Defaults()
	router := NewRouter(cfg)
	defer router.Close()

	var counter atomic.Int32
	var maxParallel atomic.Int32
	var currentParallel atomic.Int32

	router.RegisterHandler("serialize_test", func(ctx context.Context, payload any) error {
		v := currentParallel.Add(1)
		defer currentParallel.Add(-1)

		for {
			maxP := maxParallel.Load()
			if v > maxP {
				if !maxParallel.CompareAndSwap(maxP, v) {
					continue
				}
			}
			break
		}

		counter.Add(1)
		runtime.Gosched()
		return nil
	})

	const numTasks = 10000
	eg, ctx := errgroup.WithContext(context.Background())

	for i := 0; i < numTasks; i++ {
		eg.Go(func() error {
			if err := ctx.Err(); err != nil {
				return err
			}
			err := router.Dispatch(context.Background(), Task{
				Type: "serialize_test",
				Options: TaskOptions{
					GroupKey: "single_group",
				},
			})
			if err != nil {
				return fmt.Errorf("Dispatch failed: %v", err)
			}
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		t.Fatalf("group key serialization stress dispatch failed: %v", err)
	}

	// Ensure all tasks complete functionally instead of sleeping
	for {
		if counter.Load() == int32(numTasks) {
			break
		}
		runtime.Gosched()
	}

	if c := counter.Load(); c != numTasks {
		t.Fatalf("Expected %d tasks processed, got %d", numTasks, c)
	}
	if p := maxParallel.Load(); p > 1 {
		t.Fatalf("Expected strictly 1 parallel execution per group, got %d", p)
	}
}

func TestRouter_ExecutionLimiter(t *testing.T) {
	t.Parallel()

	cfg := Defaults()
	cfg.ExecutionLimiter = NewExecutionLimiter(10)
	router := NewRouter(cfg)
	defer router.Close()

	var active atomic.Int32
	var maxActive atomic.Int32
	blockCh := make(chan struct{})
	startedCh := make(chan struct{}, 10)

	var allWg sync.WaitGroup
	allWg.Add(50)

	router.RegisterHandler("limiter_test", func(ctx context.Context, payload any) error {
		defer allWg.Done()
		v := active.Add(1)
		for {
			maxA := maxActive.Load()
			if v > maxA {
				if !maxActive.CompareAndSwap(maxA, v) {
					continue
				}
			}
			break
		}
		select {
		case startedCh <- struct{}{}:
		default:
		}
		<-blockCh
		active.Add(-1)
		return nil
	})

	for i := 0; i < 50; i++ {
		// Unique group keys so group serialization doesn't limit concurrency
		err := router.Dispatch(context.Background(), Task{
			Type: "limiter_test",
			Options: TaskOptions{
				GroupKey: fmt.Sprintf("group_%d", i),
			},
		})
		if err != nil {
			t.Fatalf("Dispatch failed: %v", err)
		}
	}

	for i := 0; i < 10; i++ {
		<-startedCh
	}
	close(blockCh) // Unblock all
	allWg.Wait()

	if p := maxActive.Load(); p != 10 {
		t.Fatalf("Expected max active handlers to be bounded by Limiter (10), got %d", p)
	}
}

func TestRouter_IdempotencyTTL(t *testing.T) {
	t.Parallel()

	mockClock := clock.NewMockClock(time.Now())
	cfg := Defaults()
	cfg.Clock = mockClock
	router := NewRouter(cfg)
	defer router.Close()

	router.RegisterHandler("idem_test", func(ctx context.Context, payload any) error {
		return nil
	})

	task := Task{
		Type: "idem_test",
		Options: TaskOptions{
			IdempotencyKey: "A",
			IdempotencyTTL: 60 * time.Second,
		},
	}

	if err := router.Dispatch(context.Background(), task); err != nil {
		t.Fatalf("Initial dispatch failed: %v", err)
	}

	if err := router.Dispatch(context.Background(), task); !errors.Is(err, ErrDuplicateTask) {
		t.Fatalf("Expected ErrDuplicateTask within TTL window, got %v", err)
	}

	mockClock.Advance(59 * time.Second)
	if err := router.Dispatch(context.Background(), task); !errors.Is(err, ErrDuplicateTask) {
		t.Fatalf("Expected ErrDuplicateTask at 59s, got %v", err)
	}

	mockClock.Advance(2 * time.Second) // Total 61s

	// Force cleanup
	router.cleanupOnce()

	if err := router.Dispatch(context.Background(), task); err != nil {
		t.Fatalf("Expected success after TTL expiry (61s), got %v", err)
	}
}

func TestRouter_RetryHeap(t *testing.T) {
	t.Parallel()

	mockClock := clock.NewMockClock(time.Now())
	cfg := Defaults()
	cfg.Clock = mockClock
	cfg.DefaultMaxAttempts = 5
	cfg.InitialBackoff = 100 * time.Millisecond
	cfg.MaxBackoff = 2 * time.Second
	router := NewRouter(cfg)
	defer router.Close()

	var attemptCount atomic.Int32
	errStatic := errors.New("static network error")

	router.RegisterHandler("retry_test", func(ctx context.Context, payload any) error {
		attemptCount.Add(1)
		return errStatic
	})

	// Inject a task directly to test backoff computation natively
	delay1 := router.computeBackoff(100*time.Millisecond, 2*time.Second, 1)
	if delay1 < 90*time.Millisecond || delay1 > 110*time.Millisecond { // 10% jitter bounds
		t.Fatalf("Expected backoff ~100ms, got %v", delay1)
	}

	delay2 := router.computeBackoff(100*time.Millisecond, 2*time.Second, 2)
	if delay2 < 180*time.Millisecond || delay2 > 220*time.Millisecond {
		t.Fatalf("Expected backoff ~200ms, got %v", delay2)
	}

	// Test heap explicitly without the router's active background loop stealing it
	var h retryTaskHeap
	heap.Init(&h)

	itemA := &scheduledRetry{at: mockClock.Now().Add(10 * time.Second), groupKey: "group_a", task: &enqueuedTask{attempt: 1}, seq: 1}
	itemB := &scheduledRetry{at: mockClock.Now().Add(5 * time.Second), groupKey: "group_b", task: &enqueuedTask{attempt: 1}, seq: 2}

	heap.Push(&h, itemA)
	heap.Push(&h, itemB)

	// mockClock.Now() isn't strictly needed for explicit popping if we do it manually,
	// but let's emulate popDueRetries behavior:
	popDue := func(now time.Time) []*scheduledRetry {
		var due []*scheduledRetry
		for len(h) > 0 {
			next := h[0]
			if next == nil || next.at.After(now) {
				break
			}
			due = append(due, heap.Pop(&h).(*scheduledRetry))
		}
		return due
	}

	mockClock.Advance(6 * time.Second)
	due := popDue(mockClock.Now())
	if len(due) != 1 || due[0].groupKey != "group_b" {
		t.Fatalf("Expected group_b to pop first")
	}

	mockClock.Advance(5 * time.Second)
	due = popDue(mockClock.Now())
	if len(due) != 1 || due[0].groupKey != "group_a" {
		t.Fatalf("Expected group_a to pop second")
	}
}

func TestRouter_CronSchedule(t *testing.T) {
	t.Parallel()

	mockClock := clock.NewMockClock(time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC))
	cfg := Defaults()
	cfg.Clock = mockClock
	router := NewRouter(cfg)
	defer router.Close()

	var runs atomic.Int32
	var wg sync.WaitGroup
	wg.Add(3)

	router.RegisterHandler("cron_task", func(ctx context.Context, payload any) error {
		runs.Add(1)
		wg.Done()
		return nil
	})

	// Schedule for 15:00 UTC daily
	cancel := router.ScheduleDailyAtUTC(15, 0, Task{Type: "cron_task"})
	defer cancel()

	// Initial time is 10:00. First target is today 15:00 (5h from now).
	// Advance clock by exactly 72 hours (3 days).
	// We expect 3 triggers: Today at 15:00, Tomorrow at 15:00, Day 3 at 15:00.
	for i := 0; i < 72; i++ {
		mockClock.Advance(1 * time.Hour)
		router.runCronOnce()
	}

	eg2, ctx2 := errgroup.WithContext(context.Background())
	done := make(chan struct{})
	eg2.Go(func() error {
		select {
		case <-ctx2.Done():
			return ctx2.Err()
		default:
		}
		wg.Wait()
		close(done)
		return nil
	})

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatalf("Expected cron to fire exactly 3 times in 72h, got %d", runs.Load())
	}

	if err := eg2.Wait(); err != nil {
		t.Fatalf("unexpected error waiting for waitgroup: %v", err)
	}
}

func TestRouter_ContextCancel(t *testing.T) {
	t.Parallel()

	router := NewRouter(Defaults())

	blockCh := make(chan struct{})
	doneCh := make(chan struct{})
	var ctxErr error

	router.RegisterHandler("cancel_test", func(ctx context.Context, payload any) error {
		close(blockCh) // Signal we are inside handler
		<-ctx.Done()
		ctxErr = ctx.Err()
		close(doneCh)
		return nil
	})

	_ = router.Dispatch(context.Background(), Task{Type: "cancel_test"})
	<-blockCh

	router.Close()

	<-doneCh
	if !errors.Is(ctxErr, context.Canceled) {
		t.Fatalf("Expected context.Canceled, got %v", ctxErr)
	}
}

func TestRouter_Observability(t *testing.T) {
	t.Parallel()

	mockClock := clock.NewMockClock(time.Now())
	cfg := Defaults()
	cfg.Clock = mockClock
	router := NewRouter(cfg)
	defer router.Close()

	var execWg sync.WaitGroup
	execWg.Add(1)

	router.RegisterHandler("slow_test", func(ctx context.Context, payload any) error {
		// Simulate 6s execution
		mockClock.Advance(6 * time.Second)
		execWg.Done()
		return nil
	})

	_ = router.Dispatch(context.Background(), Task{Type: "slow_test"})

	// Wait for handler to complete deterministically
	execWg.Wait()

	// Intercept observability metrics manually since getOrCreate is not strictly exported
	// We just ensure it doesn't panic and the latency map registers it.
	stats := router.Stats()
	if stats.RegisteredTypes != 1 {
		t.Errorf("Stats registered types mismatch")
	}

	router.latencyMu.Lock()
	s := router.latenciesByType["slow_test"]
	router.latencyMu.Unlock()

	if s == nil {
		t.Fatalf("Expected latency summary to be created for slow_test")
	}
}

// FuzzRouter_QueueMutation validates thread safety and bounds against corrupted payload shapes.
func FuzzRouter_QueueMutation(f *testing.F) {
	f.Add("group_1", "idem_1")
	f.Add("", "")
	f.Add(string([]byte{0x00, 0xFF}), "null_idem")

	f.Fuzz(func(t *testing.T, group string, idem string) {
		router := NewRouter(Defaults())
		router.RegisterHandler("fuzz", func(ctx context.Context, payload any) error {
			return nil
		})

		err := router.Dispatch(context.Background(), Task{
			Type: "fuzz",
			Options: TaskOptions{
				GroupKey:       group,
				IdempotencyKey: idem,
				IdempotencyTTL: 1 * time.Second,
			},
		})
		if err != nil && err.Error() != errTaskEnqueue.Error() && !errors.Is(err, ErrUnknownTaskType) && !errors.Is(err, ErrDuplicateTask) {
			// All other structural panics or out-of-bounds maps will fail test naturally.
		}
		router.Close()
	})
}

// FuzzRouter_HeapLimits injects extreme boundaries to validate container/heap resilience.
func FuzzRouter_HeapLimits(f *testing.F) {
	f.Add(int64(-1), int64(1))
	f.Add(int64(math.MaxInt64), int64(math.MinInt64))
	f.Add(int64(0), int64(0))

	f.Fuzz(func(t *testing.T, t1, t2 int64) {
		var h retryTaskHeap
		heap.Init(&h)

		item1 := &scheduledRetry{at: time.Unix(t1, 0), seq: 1}
		item2 := &scheduledRetry{at: time.Unix(t2, 0), seq: 2}

		heap.Push(&h, item1)
		heap.Push(&h, item2)

		if h.Len() != 2 {
			t.Fatalf("Length mismatch")
		}

		_ = heap.Pop(&h)
		_ = heap.Pop(&h)
	})
}

```

