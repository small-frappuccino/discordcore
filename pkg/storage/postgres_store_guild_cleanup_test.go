package storage

import (
	"context"
	"testing"
	"time"
)

func TestDeleteGuildDataRemovesGuildScopedRows(t *testing.T) {
	store := newTempStore(t)
	ctx := context.Background()

	allowedGuild := "1390069056530419823"
	removedGuild := "guild-denied"

	seedGuildScopedRows(t, store, allowedGuild)
	seedGuildScopedRows(t, store, removedGuild)

	if err := store.DeleteGuildData(ctx, removedGuild); err != nil {
		t.Fatalf("DeleteGuildData() failed: %v", err)
	}

	assertGuildScopedCounts(t, store, removedGuild, 0)
	assertGuildScopedCounts(t, store, allowedGuild, 1)

	if err := store.DeleteGuildData(ctx, removedGuild); err != nil {
		t.Fatalf("DeleteGuildData() should be idempotent, got %v", err)
	}
	assertGuildScopedCounts(t, store, removedGuild, 0)
}

func seedGuildScopedRows(t *testing.T, store *Store, guildID string) {
	t.Helper()
	ctx := context.Background()
	now := time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC)
	day := now.Format("2006-01-02")

	mustExecGuildSeed(t, store, `INSERT INTO messages (guild_id, message_id, channel_id, author_id, cached_at) VALUES ($1, $2, $3, $4, $5)`, guildID, "message-"+guildID, "channel-"+guildID, "author-"+guildID, now)
	mustExecGuildSeed(t, store, `INSERT INTO member_joins (guild_id, user_id, joined_at, last_seen_at, is_bot) VALUES ($1, $2, $3, $4, $5)`, guildID, "user-"+guildID, now, now, false)
	mustExecGuildSeed(t, store, `INSERT INTO avatars_current (guild_id, user_id, avatar_hash, updated_at) VALUES ($1, $2, $3, $4)`, guildID, "user-"+guildID, "hash", now)
	mustExecGuildSeed(t, store, `INSERT INTO avatars_history (guild_id, user_id, old_hash, new_hash, changed_at) VALUES ($1, $2, $3, $4, $5)`, guildID, "user-"+guildID, "old", "new", now)
	mustExecGuildSeed(t, store, `INSERT INTO messages_history (guild_id, message_id, channel_id, author_id, version, event_type, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7)`, guildID, "message-"+guildID, "channel-"+guildID, "author-"+guildID, 1, "create", now)
	mustExecGuildSeed(t, store, `INSERT INTO guild_meta (guild_id, bot_since, owner_id) VALUES ($1, $2, $3)`, guildID, now, "owner-"+guildID)
	mustExecGuildSeed(t, store, `INSERT INTO moderation_cases (guild_id, last_case_number) VALUES ($1, $2)`, guildID, 1)
	mustExecGuildSeed(t, store, `INSERT INTO moderation_warnings (guild_id, user_id, case_number, moderator_id, reason) VALUES ($1, $2, $3, $4, $5)`, guildID, "user-"+guildID, 1, "mod-"+guildID, "reason")
	mustExecGuildSeed(t, store, `INSERT INTO roles_current (guild_id, user_id, role_id, updated_at) VALUES ($1, $2, $3, $4)`, guildID, "user-"+guildID, "role-"+guildID, now)
	mustExecGuildSeed(t, store, `INSERT INTO daily_message_metrics (guild_id, channel_id, user_id, day, count) VALUES ($1, $2, $3, $4, $5)`, guildID, "channel-"+guildID, "user-"+guildID, day, 1)
	mustExecGuildSeed(t, store, `INSERT INTO daily_reaction_metrics (guild_id, channel_id, user_id, day, count) VALUES ($1, $2, $3, $4, $5)`, guildID, "channel-"+guildID, "user-"+guildID, day, 1)
	mustExecGuildSeed(t, store, `INSERT INTO daily_member_joins (guild_id, user_id, day, count) VALUES ($1, $2, $3, $4)`, guildID, "user-"+guildID, day, 1)
	mustExecGuildSeed(t, store, `INSERT INTO daily_member_leaves (guild_id, user_id, day, count) VALUES ($1, $2, $3, $4)`, guildID, "user-"+guildID, day, 1)
	mustExecGuildSeed(t, store, `INSERT INTO message_version_counters (guild_id, message_id, last_version) VALUES ($1, $2, $3)`, guildID, "message-"+guildID, 1)

	question, err := store.CreateQOTDQuestion(ctx, QOTDQuestionRecord{
		GuildID: guildID,
		DeckID:  "default",
		Body:    "Question for " + guildID,
		Status:  "ready",
	})
	if err != nil {
		t.Fatalf("CreateQOTDQuestion(%s) failed: %v", guildID, err)
	}
	if _, err := store.UpsertQOTDSurface(ctx, QOTDSurfaceRecord{
		GuildID:              guildID,
		DeckID:               "default",
		ChannelID:            "forum-" + guildID,
		QuestionListThreadID: "list-" + guildID,
	}); err != nil {
		t.Fatalf("UpsertQOTDSurface(%s) failed: %v", guildID, err)
	}
	publishDate := time.Date(2026, 4, 27, 0, 0, 0, 0, time.UTC)
	official, err := store.CreateQOTDOfficialPostProvisioning(ctx, QOTDOfficialPostRecord{
		GuildID:              guildID,
		DeckID:               "default",
		DeckNameSnapshot:     "Default",
		QuestionID:           question.ID,
		PublishMode:          "manual",
		PublishDateUTC:       publishDate,
		State:                "current",
		ChannelID:            "forum-" + guildID,
		QuestionTextSnapshot: question.Body,
		GraceUntil:           now.Add(time.Hour),
		ArchiveAt:            now.Add(2 * time.Hour),
	})
	if err != nil {
		t.Fatalf("CreateQOTDOfficialPostProvisioning(%s) failed: %v", guildID, err)
	}
	if _, err := store.CreateQOTDAnswerMessage(ctx, QOTDAnswerMessageRecord{
		GuildID:         guildID,
		OfficialPostID:  official.ID,
		UserID:          "answer-user-" + guildID,
		AnswerChannelID: "answer-" + guildID,
	}); err != nil {
		t.Fatalf("CreateQOTDAnswerMessage(%s) failed: %v", guildID, err)
	}
	mustExecGuildSeed(t, store, `INSERT INTO qotd_thread_archives (guild_id, official_post_id, source_kind, discord_thread_id, archived_at) VALUES ($1, $2, $3, $4, $5)`, guildID, official.ID, "official", "thread-"+guildID, now)
	var threadArchiveID int64
	if err := store.db.QueryRow(`SELECT id FROM qotd_thread_archives WHERE guild_id = $1`, guildID).Scan(&threadArchiveID); err != nil {
		t.Fatalf("query qotd_thread_archives(%s): %v", guildID, err)
	}
	mustExecGuildSeed(t, store, `INSERT INTO qotd_message_archives (thread_archive_id, discord_message_id, created_at) VALUES ($1, $2, $3)`, threadArchiveID, "archived-message-"+guildID, now)
	mustExecGuildSeed(t, store, `INSERT INTO qotd_collected_questions (guild_id, source_channel_id, source_message_id, source_created_at, question_text) VALUES ($1, $2, $3, $4, $5)`, guildID, "source-channel-"+guildID, "source-message-"+guildID, now, "Collected")

	mustExecGuildSeed(t, store, `INSERT INTO persistent_cache (cache_key, cache_type, data, expires_at, cached_at) VALUES ($1, $2, $3, $4, $5)`, guildID, "guild", "{}", now.Add(time.Hour), now)
	mustExecGuildSeed(t, store, `INSERT INTO persistent_cache (cache_key, cache_type, data, expires_at, cached_at) VALUES ($1, $2, $3, $4, $5)`, "roles:"+guildID, "roles", "{}", now.Add(time.Hour), now)
	mustExecGuildSeed(t, store, `INSERT INTO persistent_cache (cache_key, cache_type, data, expires_at, cached_at) VALUES ($1, $2, $3, $4, $5)`, guildID+":user", "member", "{}", now.Add(time.Hour), now)
	mustExecGuildSeed(t, store, `INSERT INTO persistent_cache (cache_key, cache_type, data, expires_at, cached_at) VALUES ($1, $2, $3, $4, $5)`, "member:"+guildID+":user", "member", "{}", now.Add(time.Hour), now)
	mustExecGuildSeed(t, store, `INSERT INTO persistent_cache (cache_key, cache_type, data, expires_at, cached_at) VALUES ($1, $2, $3, $4, $5)`, guildID+":channel", "channel", "{}", now.Add(time.Hour), now)
	mustExecGuildSeed(t, store, `INSERT INTO persistent_cache (cache_key, cache_type, data, expires_at, cached_at) VALUES ($1, $2, $3, $4, $5)`, "channel:"+guildID+":channel", "channel", "{}", now.Add(time.Hour), now)
}

func mustExecGuildSeed(t *testing.T, store *Store, query string, args ...any) {
	t.Helper()
	if _, err := store.db.Exec(query, args...); err != nil {
		t.Fatalf("seed guild scoped row failed: %v", err)
	}
}

func assertGuildScopedCounts(t *testing.T, store *Store, guildID string, want int) {
	t.Helper()
	checks := []struct {
		name  string
		query string
	}{
		{name: "messages", query: `SELECT COUNT(*) FROM messages WHERE guild_id = $1`},
		{name: "member_joins", query: `SELECT COUNT(*) FROM member_joins WHERE guild_id = $1`},
		{name: "avatars_current", query: `SELECT COUNT(*) FROM avatars_current WHERE guild_id = $1`},
		{name: "avatars_history", query: `SELECT COUNT(*) FROM avatars_history WHERE guild_id = $1`},
		{name: "messages_history", query: `SELECT COUNT(*) FROM messages_history WHERE guild_id = $1`},
		{name: "guild_meta", query: `SELECT COUNT(*) FROM guild_meta WHERE guild_id = $1`},
		{name: "moderation_cases", query: `SELECT COUNT(*) FROM moderation_cases WHERE guild_id = $1`},
		{name: "moderation_warnings", query: `SELECT COUNT(*) FROM moderation_warnings WHERE guild_id = $1`},
		{name: "roles_current", query: `SELECT COUNT(*) FROM roles_current WHERE guild_id = $1`},
		{name: "daily_message_metrics", query: `SELECT COUNT(*) FROM daily_message_metrics WHERE guild_id = $1`},
		{name: "daily_reaction_metrics", query: `SELECT COUNT(*) FROM daily_reaction_metrics WHERE guild_id = $1`},
		{name: "daily_member_joins", query: `SELECT COUNT(*) FROM daily_member_joins WHERE guild_id = $1`},
		{name: "daily_member_leaves", query: `SELECT COUNT(*) FROM daily_member_leaves WHERE guild_id = $1`},
		{name: "message_version_counters", query: `SELECT COUNT(*) FROM message_version_counters WHERE guild_id = $1`},
		{name: "qotd_questions", query: `SELECT COUNT(*) FROM qotd_questions WHERE guild_id = $1`},
		{name: "qotd_forum_surfaces", query: `SELECT COUNT(*) FROM qotd_forum_surfaces WHERE guild_id = $1`},
		{name: "qotd_official_posts", query: `SELECT COUNT(*) FROM qotd_official_posts WHERE guild_id = $1`},
		{name: "qotd_thread_archives", query: `SELECT COUNT(*) FROM qotd_thread_archives WHERE guild_id = $1`},
		{name: "qotd_answer_messages", query: `SELECT COUNT(*) FROM qotd_answer_messages WHERE guild_id = $1`},
		{name: "qotd_collected_questions", query: `SELECT COUNT(*) FROM qotd_collected_questions WHERE guild_id = $1`},
		{name: "qotd_message_archives", query: `SELECT COUNT(*) FROM qotd_message_archives ma JOIN qotd_thread_archives ta ON ta.id = ma.thread_archive_id WHERE ta.guild_id = $1`},
		{name: "persistent_cache", query: `SELECT COUNT(*) FROM persistent_cache WHERE cache_key = $1 OR cache_key = $2 OR cache_key = $3 OR cache_key LIKE $4 OR cache_key LIKE $5 OR cache_key LIKE $6 OR cache_key LIKE $7`},
	}

	for _, check := range checks {
		var got int
		switch check.name {
		case "persistent_cache":
			if err := store.db.QueryRow(check.query, guildID, "guild:"+guildID, "roles:"+guildID, guildID+":%", "member:"+guildID+":%", guildID+":%", "channel:"+guildID+":%").Scan(&got); err != nil {
				t.Fatalf("count %s: %v", check.name, err)
			}
		default:
			if err := store.db.QueryRow(check.query, guildID).Scan(&got); err != nil {
				t.Fatalf("count %s: %v", check.name, err)
			}
		}
		if got != want {
			t.Fatalf("unexpected %s count for guild %s: got=%d want=%d", check.name, guildID, got, want)
		}
	}
}