package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// SetBotSince sets the bot_since timestamp for a guild (keeps the earliest time).
func (s *Store) SetBotSince(guildID string, t time.Time) error {
	if s.db == nil {
		return fmt.Errorf("store not initialized")
	}
	if guildID == "" {
		return nil
	}
	if t.IsZero() {
		t = time.Now().UTC()
	}
	_, err := s.exec(
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

// GetBotSince returns when the bot was first seen in a guild, if available.
func (s *Store) GetBotSince(guildID string) (time.Time, bool, error) {
	if s.db == nil {
		return time.Time{}, false, fmt.Errorf("store not initialized")
	}
	row := s.queryRow(`SELECT bot_since FROM guild_meta WHERE guild_id=?`, guildID)
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
func (s *Store) SetHeartbeat(t time.Time) error {
	return s.SetHeartbeatContext(context.Background(), t)
}

// SetHeartbeatContext records the last-known "bot is running" timestamp with context support.
func (s *Store) SetHeartbeatContext(ctx context.Context, t time.Time) error {
	return s.setRuntimeTimestamp(ctx, "heartbeat", t)
}

// SetHeartbeatForBot records the last-known "bot is running" timestamp for one bot instance.
func (s *Store) SetHeartbeatForBot(botInstanceID string, t time.Time) error {
	return s.SetHeartbeatForBotContext(context.Background(), botInstanceID, t)
}

// SetHeartbeatForBotContext records the last-known "bot is running" timestamp for one bot instance with context support.
func (s *Store) SetHeartbeatForBotContext(ctx context.Context, botInstanceID string, t time.Time) error {
	return s.setRuntimeTimestamp(ctx, runtimeMetaKey("heartbeat", botInstanceID), t)
}

// GetHeartbeat returns the last recorded heartbeat timestamp, if any.
func (s *Store) GetHeartbeat() (time.Time, bool, error) {
	return s.getRuntimeTimestamp(context.Background(), "heartbeat")
}

// GetHeartbeatForBot returns the last recorded heartbeat timestamp for one bot instance, if any.
func (s *Store) GetHeartbeatForBot(botInstanceID string) (time.Time, bool, error) {
	return s.getRuntimeTimestamp(context.Background(), runtimeMetaKey("heartbeat", botInstanceID))
}

// SetLastEvent records the last time a relevant Discord event was processed.
func (s *Store) SetLastEvent(t time.Time) error {
	return s.SetLastEventContext(context.Background(), t)
}

// SetLastEventContext records the last time a relevant Discord event was processed with context support.
func (s *Store) SetLastEventContext(ctx context.Context, t time.Time) error {
	return s.setRuntimeTimestamp(ctx, "last_event", t)
}

// SetLastEventForBot records the last time a relevant Discord event was processed for one bot instance.
func (s *Store) SetLastEventForBot(botInstanceID string, t time.Time) error {
	return s.SetLastEventForBotContext(context.Background(), botInstanceID, t)
}

// SetLastEventForBotContext records the last time a relevant Discord event was processed for one bot instance with context support.
func (s *Store) SetLastEventForBotContext(ctx context.Context, botInstanceID string, t time.Time) error {
	return s.setRuntimeTimestamp(ctx, runtimeMetaKey("last_event", botInstanceID), t)
}

// GetLastEvent returns the last recorded event timestamp, if any.
func (s *Store) GetLastEvent() (time.Time, bool, error) {
	return s.getRuntimeTimestamp(context.Background(), "last_event")
}

// GetLastEventForBot returns the last recorded event timestamp for one bot instance, if any.
func (s *Store) GetLastEventForBot(botInstanceID string) (time.Time, bool, error) {
	return s.getRuntimeTimestamp(context.Background(), runtimeMetaKey("last_event", botInstanceID))
}

// SetMetadata records a timestamp associated with a specific key.
func (s *Store) SetMetadata(key string, t time.Time) error {
	return s.SetMetadataContext(context.Background(), key, t)
}

// SetMetadataContext records a timestamp associated with a specific key with context support.
func (s *Store) SetMetadataContext(ctx context.Context, key string, t time.Time) error {
	return s.setRuntimeTimestamp(ctx, key, t)
}

// GetMetadata retrieves the timestamp for a specific key.
func (s *Store) GetMetadata(key string) (time.Time, bool, error) {
	return s.GetMetadataContext(context.Background(), key)
}

// GetMetadataContext retrieves the timestamp for a specific key with context support.
func (s *Store) GetMetadataContext(ctx context.Context, key string) (time.Time, bool, error) {
	return s.getRuntimeTimestamp(ctx, key)
}
