package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// SetBotSince sets the bot_since timestamp for a guild (keeps the earliest time).
func (s *Store) SetBotSince(ctx context.Context, guildID string, t time.Time) error {
	if s.db == nil {
		return fmt.Errorf("store not initialized")
	}
	if guildID == "" {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if t.IsZero() {
		t = time.Now().UTC()
	}
	_, err := s.execContext(
		ctx,
		`INSERT INTO guild_meta (guild_id, bot_since)
         VALUES (?, ?)
         ON CONFLICT(guild_id) DO UPDATE SET
           bot_since = CASE
             WHEN guild_meta.bot_since IS NULL OR ? < guild_meta.bot_since THEN ?
             ELSE guild_meta.bot_since
           END`,
		guildID, t, t, t,
	)
	return err
}

// BotSince returns when the bot was first seen in a guild, if available.
func (s *Store) BotSince(ctx context.Context, guildID string) (time.Time, bool, error) {
	if s.db == nil {
		return time.Time{}, false, fmt.Errorf("store not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	row := s.queryRowContext(ctx, `SELECT bot_since FROM guild_meta WHERE guild_id=?`, guildID)
	var t sql.NullTime
	if err := row.Scan(&t); err != nil {
		if err == sql.ErrNoRows {
			return time.Time{}, false, nil
		}
		return time.Time{}, false, err
	}
	if !t.Valid {
		return time.Time{}, false, nil
	}
	return t.Time, true, nil
}

func (s *Store) setRuntimeTimestamp(ctx context.Context, key string, t time.Time) error {
	if s.db == nil {
		return fmt.Errorf("store not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if t.IsZero() {
		t = time.Now().UTC()
	}
	_, err := s.execContext(
		ctx,
		`INSERT INTO runtime_meta (key, ts) VALUES (?, ?)
         ON CONFLICT(key) DO UPDATE SET ts=excluded.ts`,
		key, t.UTC(),
	)
	return err
}

func (s *Store) getRuntimeTimestamp(ctx context.Context, key string) (time.Time, bool, error) {
	if s.db == nil {
		return time.Time{}, false, fmt.Errorf("store not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	row := s.queryRowContext(ctx, `SELECT ts FROM runtime_meta WHERE key=?`, key)
	var ts time.Time
	if err := row.Scan(&ts); err != nil {
		if err == sql.ErrNoRows {
			return time.Time{}, false, nil
		}
		return time.Time{}, false, err
	}
	return ts, true, nil
}

func runtimeMetaKey(baseKey, botInstanceID string) string {
	baseKey = strings.TrimSpace(baseKey)
	botInstanceID = strings.TrimSpace(botInstanceID)
	if baseKey == "" || botInstanceID == "" {
		return baseKey
	}
	return baseKey + ":" + botInstanceID
}

// SetHeartbeat records the last-known "bot is running" timestamp.
func (s *Store) SetHeartbeat(ctx context.Context, t time.Time) error {
	return s.setRuntimeTimestamp(ctx, "heartbeat", t)
}

// SetHeartbeatForBot records the last-known "bot is running" timestamp for one bot instance.
func (s *Store) SetHeartbeatForBot(ctx context.Context, botInstanceID string, t time.Time) error {
	return s.setRuntimeTimestamp(ctx, runtimeMetaKey("heartbeat", botInstanceID), t)
}

// Heartbeat returns the last recorded heartbeat timestamp, if any.
func (s *Store) Heartbeat(ctx context.Context) (time.Time, bool, error) {
	return s.getRuntimeTimestamp(ctx, "heartbeat")
}

// HeartbeatForBot returns the last recorded heartbeat timestamp for one bot instance, if any.
func (s *Store) HeartbeatForBot(ctx context.Context, botInstanceID string) (time.Time, bool, error) {
	return s.getRuntimeTimestamp(ctx, runtimeMetaKey("heartbeat", botInstanceID))
}

// SetLastEvent records the last time a relevant Discord event was processed.
func (s *Store) SetLastEvent(ctx context.Context, t time.Time) error {
	return s.setRuntimeTimestamp(ctx, "last_event", t)
}

// SetLastEventForBot records the last time a relevant Discord event was processed for one bot instance.
func (s *Store) SetLastEventForBot(ctx context.Context, botInstanceID string, t time.Time) error {
	return s.setRuntimeTimestamp(ctx, runtimeMetaKey("last_event", botInstanceID), t)
}

// LastEvent returns the last recorded event timestamp, if any.
func (s *Store) LastEvent(ctx context.Context) (time.Time, bool, error) {
	return s.getRuntimeTimestamp(ctx, "last_event")
}

// LastEventForBot returns the last recorded event timestamp for one bot instance, if any.
func (s *Store) LastEventForBot(ctx context.Context, botInstanceID string) (time.Time, bool, error) {
	return s.getRuntimeTimestamp(ctx, runtimeMetaKey("last_event", botInstanceID))
}

// SetMetadata records a timestamp associated with a specific key.
func (s *Store) SetMetadata(ctx context.Context, key string, t time.Time) error {
	return s.setRuntimeTimestamp(ctx, key, t)
}

// Metadata retrieves the timestamp for a specific key.
func (s *Store) Metadata(ctx context.Context, key string) (time.Time, bool, error) {
	return s.getRuntimeTimestamp(ctx, key)
}
