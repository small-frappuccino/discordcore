package logging

import (
	"errors"
	"log/slog"

	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/storage"
	"github.com/small-frappuccino/discordgo"
)

func TestMessageEventService_ProcessMessageUpdateQueuesAsyncPersistence(t *testing.T) {
	const (
		guildID      = "g-message-writer-update"
		channelID    = "c-message-writer-update"
		logChannelID = "c-message-writer-update-log"
		userID       = "u-message-writer-update"
		messageID    = "m-message-writer-update"
	)

	store, db := newLoggingStore(t, "message-writer-update.db")
	cfgMgr := newMessageWriterConfigManager(t, guildID, files.ChannelsConfig{
		MessageEdit: logChannelID,
	})

	session := newMessageWriterTestSession(t, guildID, logChannelID)
	session.Identify.Intents = discordgo.IntentsGuildMessages

	service := NewMessageEventService(session, cfgMgr, NewNotificationSender(session, slog.Default()), store, slog.Default())
	service.cacheEnabled = true
	service.versioningEnabled = true
	service.cacheTTL = 24 * time.Hour

	metrics := NewInMemoryMessageWriterMetrics()
	writer := newMessageCreateWriter(store, metrics, slog.Default())
	writer.flushInterval = time.Hour
	service.messageCreateWriter = writer
	writer.Start()
	defer func() {
		if service.messageCreateWriter != nil {
			if err := service.messageCreateWriter.Stop(context.Background()); err != nil {
				t.Fatalf("stop message create writer: %v", err)
			}
			service.messageCreateWriter = nil
		}
	}()

	service.persistMessageCreate(guildID, &discordgo.MessageCreate{
		Message: &discordgo.Message{
			ID:        messageID,
			GuildID:   guildID,
			ChannelID: channelID,
			Content:   "before",
			Author: &discordgo.User{
				ID:       userID,
				Username: "before-user",
			},
		},
	})

	cachedBeforeFlush, err := store.GetMessage(guildID, messageID)
	if err != nil {
		t.Fatalf("get pending message before update: %v", err)
	}
	if cachedBeforeFlush != nil {
		t.Fatalf("expected pending create to stay out of store before flush, got %+v", cachedBeforeFlush)
	}

	if err := service.processMessageUpdate(context.Background(), session, &discordgo.MessageUpdate{
		Message: &discordgo.Message{
			ID:        messageID,
			GuildID:   guildID,
			ChannelID: channelID,
			Content:   "after",
			Author: &discordgo.User{
				ID:       userID,
				Username: "before-user",
			},
		},
	}, false); err != nil {
		t.Fatalf("process update: %v", err)
	}

	cachedAfterUpdate, err := store.GetMessage(guildID, messageID)
	if err != nil {
		t.Fatalf("get updated message before writer drain: %v", err)
	}
	if cachedAfterUpdate != nil {
		t.Fatalf("expected async update to stay out of store before writer drain, got %+v", cachedAfterUpdate)
	}
	if pending := service.lookupCachedMessage(context.Background(), guildID, messageID, false); pending == nil || pending.Content != "after" {
		t.Fatalf("expected pending cache to expose updated content before drain, got %+v", pending)
	}

	if err := writer.Stop(context.Background()); err != nil {
		t.Fatalf("stop message create writer: %v", err)
	}

	cachedAfterDrain, err := store.GetMessage(guildID, messageID)
	if err != nil {
		t.Fatalf("get updated message after writer drain: %v", err)
	}
	if cachedAfterDrain == nil || cachedAfterDrain.Content != "after" {
		t.Fatalf("expected async create flush not to overwrite edited content, got %+v", cachedAfterDrain)
	}

	waitForDailyMessageMetricCount(t, db, guildID, channelID, userID, time.Now().UTC(), 1)
	if got := messageHistoryCount(t, db, guildID, messageID, "create"); got != 1 {
		t.Fatalf("expected one create history row after writer drain, got %d", got)
	}
	if got := messageHistoryCount(t, db, guildID, messageID, "edit"); got != 1 {
		t.Fatalf("expected one edit history row, got %d", got)
	}

	snap := metrics.Snapshot()
	if got := snap.Enqueue.UpsertsTotal; got < 2 {
		t.Fatalf("expected message writer to record >=2 enqueued upserts, got %d (snapshot=%+v)", got, snap)
	}
	if got := snap.Flush.FlushedByOp[MessageWriterFlushOpVersions]; got < 2 {
		t.Fatalf("expected message writer to flush >=2 create+edit versions, got %d (snapshot=%+v)", got, snap)
	}
	service.messageCreateWriter = nil
}

func TestMessageEventService_ProcessMessageDeleteQueuesAsyncPersistenceWhenDeleteOnLogEnabled(t *testing.T) {
	const (
		guildID      = "g-message-writer-delete"
		channelID    = "c-message-writer-delete"
		logChannelID = "c-message-writer-delete-log"
		userID       = "u-message-writer-delete"
		messageID    = "m-message-writer-delete"
	)

	store, db := newLoggingStore(t, "message-writer-delete.db")
	deleteOnLog := true
	cfgMgr := newMessageWriterConfigManager(t, guildID, files.ChannelsConfig{
		MessageDelete: logChannelID,
	}, func(cfg *files.GuildConfig) {
		cfg.Features.MessageCache.DeleteOnLog = &deleteOnLog
	})

	session := newMessageWriterTestSession(t, guildID, logChannelID)
	session.Identify.Intents = discordgo.IntentsGuildMessages

	service := NewMessageEventService(session, cfgMgr, NewNotificationSender(session, slog.Default()), store, slog.Default())
	service.cacheEnabled = true
	service.versioningEnabled = true
	service.cacheTTL = 24 * time.Hour
	service.deleteOnLog = true

	writer := newMessageCreateWriter(store, nil, slog.Default())
	writer.flushInterval = time.Hour
	service.messageCreateWriter = writer
	writer.Start()
	defer func() {
		if service.messageCreateWriter != nil {
			if err := service.messageCreateWriter.Stop(context.Background()); err != nil {
				t.Fatalf("stop message create writer: %v", err)
			}
			service.messageCreateWriter = nil
		}
	}()

	service.persistMessageCreate(guildID, &discordgo.MessageCreate{
		Message: &discordgo.Message{
			ID:        messageID,
			GuildID:   guildID,
			ChannelID: channelID,
			Content:   "before-delete",
			Author: &discordgo.User{
				ID:       userID,
				Username: "delete-user",
			},
		},
	})

	if err := service.processMessageDelete(context.Background(), session, &discordgo.MessageDelete{
		Message: &discordgo.Message{
			ID:        messageID,
			GuildID:   guildID,
			ChannelID: channelID,
		},
	}, false); err != nil {
		t.Fatalf("process delete: %v", err)
	}
	if pending := service.lookupCachedMessage(context.Background(), guildID, messageID, false); pending != nil {
		t.Fatalf("expected pending delete to hide message before drain, got %+v", pending)
	}

	if err := writer.Stop(context.Background()); err != nil {
		t.Fatalf("stop message create writer: %v", err)
	}

	cachedAfterDelete, err := store.GetMessage(guildID, messageID)
	if err != nil {
		t.Fatalf("get message after delete drain: %v", err)
	}
	if cachedAfterDelete != nil {
		t.Fatalf("expected delete-on-log flow to prevent stale create flush, got %+v", cachedAfterDelete)
	}

	waitForDailyMessageMetricCount(t, db, guildID, channelID, userID, time.Now().UTC(), 1)
	if got := messageHistoryCount(t, db, guildID, messageID, "create"); got != 1 {
		t.Fatalf("expected one create history row after writer drain, got %d", got)
	}
	if got := messageHistoryCount(t, db, guildID, messageID, "delete"); got != 1 {
		t.Fatalf("expected one delete history row, got %d", got)
	}
	service.messageCreateWriter = nil
}

func TestMessageEventService_WriterDrainKeepsCreateEditDeleteVersionsContiguous(t *testing.T) {
	const (
		guildID     = "g-message-writer-sequence"
		channelID   = "c-message-writer-sequence"
		editLogID   = "c-message-writer-sequence-edit"
		deleteLogID = "c-message-writer-sequence-delete"
		userID      = "u-message-writer-sequence"
		messageID   = "m-message-writer-sequence"
	)

	store, db := newLoggingStore(t, "message-writer-sequence.db")
	deleteOnLog := true
	cfgMgr := newMessageWriterConfigManager(t, guildID, files.ChannelsConfig{
		MessageEdit:   editLogID,
		MessageDelete: deleteLogID,
	}, func(cfg *files.GuildConfig) {
		cfg.Features.MessageCache.DeleteOnLog = &deleteOnLog
	})

	session := newDiscordSessionWithAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && (r.URL.Path == fmt.Sprintf("/channels/%s/messages", editLogID) || r.URL.Path == fmt.Sprintf("/channels/%s/messages", deleteLogID)):
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "log-message"})
		case r.Method == http.MethodGet && r.URL.Path == fmt.Sprintf("/guilds/%s/audit-logs", guildID):
			_ = json.NewEncoder(w).Encode(map[string]any{"audit_log_entries": []any{}})
		default:
			_, _ = w.Write([]byte(`{}`))
		}
	})
	session.Identify.Intents = discordgo.IntentsGuildMessages

	service := NewMessageEventService(session, cfgMgr, NewNotificationSender(session, slog.Default()), store, slog.Default())
	service.cacheEnabled = true
	service.versioningEnabled = true
	service.cacheTTL = 24 * time.Hour
	service.deleteOnLog = true

	writer := newMessageCreateWriter(store, nil, slog.Default())
	writer.flushInterval = time.Hour
	service.messageCreateWriter = writer
	writer.Start()
	defer func() {
		if service.messageCreateWriter != nil {
			if err := service.messageCreateWriter.Stop(context.Background()); err != nil {
				t.Fatalf("stop message create writer: %v", err)
			}
			service.messageCreateWriter = nil
		}
	}()

	service.persistMessageCreate(guildID, &discordgo.MessageCreate{
		Message: &discordgo.Message{
			ID:        messageID,
			GuildID:   guildID,
			ChannelID: channelID,
			Content:   "before",
			Author: &discordgo.User{
				ID:       userID,
				Username: "writer-sequence-user",
			},
		},
	})

	if err := service.processMessageUpdate(context.Background(), session, &discordgo.MessageUpdate{
		Message: &discordgo.Message{
			ID:        messageID,
			GuildID:   guildID,
			ChannelID: channelID,
			Content:   "after",
			Author: &discordgo.User{
				ID:       userID,
				Username: "writer-sequence-user",
			},
		},
	}, false); err != nil {
		t.Fatalf("process update: %v", err)
	}

	if err := service.processMessageDelete(context.Background(), session, &discordgo.MessageDelete{
		Message: &discordgo.Message{
			ID:        messageID,
			GuildID:   guildID,
			ChannelID: channelID,
		},
	}, false); err != nil {
		t.Fatalf("process delete: %v", err)
	}

	if err := writer.Stop(context.Background()); err != nil {
		t.Fatalf("stop message create writer: %v", err)
	}

	rows, err := db.Query(context.Background(), `SELECT version, event_type FROM messages_history WHERE guild_id = $1 AND message_id = $2 ORDER BY version`, guildID, messageID)
	if err != nil {
		t.Fatalf("query history: %v", err)
	}
	defer rows.Close()

	var got []string
	for rows.Next() {
		var version int
		var eventType string
		if err := rows.Scan(&version, &eventType); err != nil {
			t.Fatalf("scan history row: %v", err)
		}
		got = append(got, fmt.Sprintf("%d:%s", version, eventType))
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate history rows: %v", err)
	}
	if want := []string{"1:create", "2:edit", "3:delete"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected version sequence: got=%v want=%v", got, want)
	}
	service.messageCreateWriter = nil
}

func TestMessageEventService_ProcessMessageDeleteSkipsRetryWhenMessageProcessDisabled(t *testing.T) {
	const (
		guildID   = "g-message-delete-no-process"
		channelID = "c-message-delete-no-process"
		messageID = "m-message-delete-no-process"
	)

	store, _ := newLoggingStore(t, "message-delete-no-process.db")
	messageProcess := false
	cfgMgr := newMessageWriterConfigManager(t, guildID, files.ChannelsConfig{
		MessageDelete: "c-message-delete-log",
	}, func(cfg *files.GuildConfig) {
		cfg.Features.Logging.MessageProcess = &messageProcess
	})

	session := newMessageWriterTestSession(t, guildID, "c-message-delete-log")
	session.Identify.Intents = discordgo.IntentsGuildMessages

	service := NewMessageEventService(session, cfgMgr, NewNotificationSender(session, slog.Default()), store, slog.Default())
	service.cacheEnabled = true
	service.versioningEnabled = true

	err := service.processMessageDelete(context.Background(), session, &discordgo.MessageDelete{
		Message: &discordgo.Message{
			ID:        messageID,
			GuildID:   guildID,
			ChannelID: channelID,
		},
	}, false)
	if err != nil {
		t.Fatalf("expected no retry error when message processing is disabled, got %v", err)
	}
}

func TestMessageEventService_ProcessMessageDeleteSkipsRetryForBotMessageInState(t *testing.T) {
	const (
		guildID      = "g-message-delete-bot"
		channelID    = "c-message-delete-bot"
		logChannelID = "c-message-delete-bot-log"
		messageID    = "m-message-delete-bot"
	)

	store, _ := newLoggingStore(t, "message-delete-bot.db")
	cfgMgr := newMessageWriterConfigManager(t, guildID, files.ChannelsConfig{
		MessageDelete: logChannelID,
	})

	session := newMessageWriterTestSession(t, guildID, logChannelID)
	session.Identify.Intents = discordgo.IntentsGuildMessages
	if err := session.State.GuildAdd(&discordgo.Guild{ID: guildID}); err != nil {
		t.Fatalf("add guild to state: %v", err)
	}
	if err := session.State.ChannelAdd(&discordgo.Channel{ID: channelID, GuildID: guildID, Type: discordgo.ChannelTypeGuildText}); err != nil {
		t.Fatalf("add channel to state: %v", err)
	}
	session.State.MaxMessageCount = 10
	session.State.MessageAdd(&discordgo.Message{
		ID:        messageID,
		GuildID:   guildID,
		ChannelID: channelID,
		Author: &discordgo.User{
			ID:  "bot-user",
			Bot: true,
		},
	})

	service := NewMessageEventService(session, cfgMgr, NewNotificationSender(session, slog.Default()), store, slog.Default())
	service.cacheEnabled = true
	service.versioningEnabled = true

	err := service.processMessageDelete(context.Background(), session, &discordgo.MessageDelete{
		Message: &discordgo.Message{
			ID:        messageID,
			GuildID:   guildID,
			ChannelID: channelID,
		},
	}, false)
	if err != nil {
		t.Fatalf("expected no retry error for bot message found in state, got %v", err)
	}
}

func TestMessageCreateWriterEnqueueAfterStopReturnsStopped(t *testing.T) {
	writer := newMessageCreateWriter(nil, nil, slog.Default())
	writer.flushInterval = time.Hour
	writer.Start()

	if err := writer.Stop(context.Background()); err != nil {
		t.Fatalf("stop writer: %v", err)
	}

	err := writer.Enqueue(storage.MessageRecord{
		GuildID:   "guild",
		MessageID: "message",
	}, nil, storage.DailyMessageCountDelta{})
	if !errors.Is(err, errMessageCreateWriterStopped) {
		t.Fatalf("expected stopped error after shutdown, got %v", err)
	}
}

func newMessageWriterConfigManager(t *testing.T, guildID string, channels files.ChannelsConfig, opts ...func(*files.GuildConfig)) *files.ConfigManager {
	t.Helper()

	cfg := files.GuildConfig{
		GuildID:  guildID,
		Channels: channels,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	mgr := files.NewConfigManagerWithStore(&files.MemoryConfigStore{})
	if err := mgr.AddGuildConfig(cfg); err != nil {
		t.Fatalf("add guild config: %v", err)
	}
	return mgr
}

func newMessageWriterTestSession(t *testing.T, guildID, logChannelID string) *discordgo.Session {
	t.Helper()

	return newDiscordSessionWithAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == fmt.Sprintf("/channels/%s/messages", logChannelID):
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "log-message"})
		case r.Method == http.MethodGet && r.URL.Path == fmt.Sprintf("/guilds/%s/audit-logs", guildID):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"audit_log_entries": []any{},
			})
		default:
			_, _ = w.Write([]byte(`{}`))
		}
	})
}
