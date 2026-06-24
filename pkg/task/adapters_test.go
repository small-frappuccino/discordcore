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
