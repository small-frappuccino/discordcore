package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"iter"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/small-frappuccino/discordcore/pkg/system"
)

// SetBotSince sets the bot_since timestamp for a guild.
func (s *Store) SetBotSince(ctx context.Context, guildID string, t time.Time) error {
	if guildID == "" {
		return nil
	}
	if t.IsZero() {
		t = time.Now().UTC()
	}
	_, err := s.db.Exec(ctx,
		`INSERT INTO guild_meta (guild_id, bot_since)
         VALUES ($1, $2)
         ON CONFLICT(guild_id) DO UPDATE SET
           bot_since = CASE
             WHEN guild_meta.bot_since IS NULL OR $3 < guild_meta.bot_since THEN $4
             ELSE guild_meta.bot_since
           END`,
		guildID, t, t, t,
	)
	return err
}

// BotSince returns when the bot was first seen in a guild.
func (s *Store) BotSince(ctx context.Context, guildID string) (time.Time, bool, error) {
	row := s.db.QueryRow(ctx, `SELECT bot_since FROM guild_meta WHERE guild_id=$1`, guildID)
	var t sql.NullTime
	if err := row.Scan(&t); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return time.Time{}, false, nil
		}
		return time.Time{}, false, fmt.Errorf("Store.BotSince: %w", err)
	}
	if !t.Valid {
		return time.Time{}, false, nil
	}
	return t.Time, true, nil
}

func (s *Store) setRuntimeTimestamp(ctx context.Context, key string, t time.Time) error {
	if t.IsZero() {
		t = time.Now().UTC()
	}
	_, err := s.db.Exec(ctx,
		`INSERT INTO runtime_meta (key, ts) VALUES ($1, $2)
         ON CONFLICT(key) DO UPDATE SET ts=excluded.ts`,
		key, t.UTC(),
	)
	return err
}

func (s *Store) getRuntimeTimestamp(ctx context.Context, key string) (time.Time, bool, error) {
	var ts time.Time
	if err := s.db.QueryRow(ctx, `SELECT ts FROM runtime_meta WHERE key=$1`, key).Scan(&ts); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return time.Time{}, false, nil
		}
		return time.Time{}, false, fmt.Errorf("Store.getRuntimeTimestamp: %w", err)
	}
	return ts, true, nil
}

// SetHeartbeatForBot records the last-known "bot is running" timestamp for a specific instance.
func (s *Store) SetHeartbeatForBot(ctx context.Context, instanceID string, t time.Time) error {
	return s.setRuntimeTimestamp(ctx, "heartbeat_"+instanceID, t)
}

// SetLastEventForBot records the last event timestamp for a specific instance.
func (s *Store) SetLastEventForBot(ctx context.Context, instanceID string, t time.Time) error {
	return s.setRuntimeTimestamp(ctx, "last_event_"+instanceID, t)
}

// Heartbeat returns the last recorded heartbeat timestamp.
func (s *Store) Heartbeat(ctx context.Context) (time.Time, bool, error) {
	return s.getRuntimeTimestamp(ctx, "heartbeat")
}

// NextTicketID atomically increments and returns the next available ticket sequence ID.
func (s *Store) NextTicketID(ctx context.Context, guildID string) (int64, error) {
	var nextID int64
	err := s.db.QueryRow(ctx, `
		INSERT INTO ticket_sequences (guild_id, last_id)
		VALUES ($1, 1)
		ON CONFLICT (guild_id) DO UPDATE
		SET last_id = ticket_sequences.last_id + 1
		RETURNING last_id
	`, guildID).Scan(&nextID)
	if err != nil {
		return 0, fmt.Errorf("Store.NextTicketID: %w", err)
	}
	return nextID, nil
}

// UpsertCacheEntriesContext upserts cache entries utilizing UNNEST.
func (s *Store) UpsertCacheEntriesContext(ctx context.Context, entries []system.CacheEntryRecord) error {
	normalized := make([]system.CacheEntryRecord, 0, len(entries))
	for _, entry := range entries {
		if entry.Key == "" || entry.CacheType == "" || entry.Data == "" {
			continue
		}
		normalized = append(normalized, entry)
	}
	if len(normalized) == 0 {
		return nil
	}

	byGuild := make(map[string][]system.CacheEntryRecord)
	for _, entry := range normalized {
		byGuild[entry.GuildID] = append(byGuild[entry.GuildID], entry)
	}

	cachedAt := time.Now().UTC()

	for guildID, batch := range byGuild {
		keys := make([]string, len(batch))
		cacheTypes := make([]string, len(batch))
		datas := make([]string, len(batch))
		expiresAts := make([]time.Time, len(batch))
		cachedAts := make([]time.Time, len(batch))

		for i, entry := range batch {
			keys[i] = entry.Key
			cacheTypes[i] = entry.CacheType
			datas[i] = entry.Data
			expiresAts[i] = entry.ExpiresAt
			cachedAts[i] = cachedAt
		}

		var err error
		if guildID != "" {
			guildIDs := make([]string, len(batch))
			for i := range batch {
				guildIDs[i] = guildID
			}
			_, err = s.db.Exec(ctx,
				`INSERT INTO persistent_cache (cache_key, cache_type, guild_id, data, expires_at, cached_at)
				 SELECT * FROM UNNEST($1::text[], $2::text[], $3::text[], $4::text[], $5::timestamptz[], $6::timestamptz[])
				 ON CONFLICT(cache_key) DO UPDATE SET
					guild_id=excluded.guild_id,
					data=excluded.data,
					expires_at=excluded.expires_at,
					cached_at=excluded.cached_at`,
				keys, cacheTypes, guildIDs, datas, expiresAts, cachedAts,
			)
		} else {
			_, err = s.db.Exec(ctx,
				`INSERT INTO persistent_cache (cache_key, cache_type, data, expires_at, cached_at)
				 SELECT * FROM UNNEST($1::text[], $2::text[], $3::text[], $4::timestamptz[], $5::timestamptz[])
				 ON CONFLICT(cache_key) DO UPDATE SET
					data=excluded.data,
					expires_at=excluded.expires_at,
					cached_at=excluded.cached_at`,
				keys, cacheTypes, datas, expiresAts, cachedAts,
			)
		}

		if err != nil {
			if strings.Contains(err.Error(), "23503") || strings.Contains(err.Error(), "foreign key constraint") {
				continue
			}
			return err
		}
	}
	return nil
}

// GetCacheEntry retrieves a cache entry from persistent storage.
func (s *Store) GetCacheEntry(ctx context.Context, key string) (cacheType, data string, expiresAt time.Time, ok bool, err error) {
	err = s.db.QueryRow(ctx, `SELECT cache_type, data, expires_at FROM persistent_cache WHERE cache_key=$1`, key).Scan(&cacheType, &data, &expiresAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", time.Time{}, false, nil
		}
		return "", "", time.Time{}, false, fmt.Errorf("Store.GetCacheEntry: %w", err)
	}
	if time.Now().After(expiresAt) {
		return "", "", time.Time{}, false, nil
	}
	return cacheType, data, expiresAt, true, nil
}

// GetCacheEntriesByType streams cache entries via iter.Seq2.
func (s *Store) GetCacheEntriesByType(ctx context.Context, cacheType string) iter.Seq2[system.CacheEntry, error] {
	return func(yield func(system.CacheEntry, error) bool) {
		rows, err := s.db.Query(ctx, `SELECT cache_key, data, expires_at FROM persistent_cache WHERE cache_type=$1 AND expires_at > $2`, cacheType, time.Now().UTC())
		if err != nil {
			yield(system.CacheEntry{}, fmt.Errorf("Store.GetCacheEntriesByType: %w", err))
			return
		}
		defer rows.Close()

		for rows.Next() {
			var entry system.CacheEntry
			if err := rows.Scan(&entry.Key, &entry.Data, &entry.ExpiresAt); err != nil {
				yield(system.CacheEntry{}, fmt.Errorf("Store.GetCacheEntriesByType: %w", err))
				return
			}
			if !yield(entry, nil) {
				return
			}
		}
		if err := rows.Err(); err != nil {
			yield(system.CacheEntry{}, fmt.Errorf("Store.GetCacheEntriesByType: %w", err))
		}
	}
}

// CleanupExpiredCacheEntries removes all expired cache entries.
func (s *Store) CleanupExpiredCacheEntries(ctx context.Context) error {
	_, err := s.db.Exec(ctx, `DELETE FROM persistent_cache WHERE expires_at <= $1`, time.Now().UTC())
	return err
}

// GetCacheStatsContext returns persistent_cache stats.
func (s *Store) GetCacheStatsContext(ctx context.Context) (system.PersistentCacheStats, error) {
	rows, err := s.db.Query(ctx, `SELECT cache_type, COUNT(*) FROM persistent_cache WHERE expires_at > $1 GROUP BY cache_type`, time.Now().UTC())
	if err != nil {
		return system.PersistentCacheStats{}, fmt.Errorf("Store.GetCacheStatsContext: %w", err)
	}
	defer rows.Close()

	stats := system.PersistentCacheStats{ByType: make(map[string]int)}
	for rows.Next() {
		var cacheType string
		var count int
		if err := rows.Scan(&cacheType, &count); err != nil {
			return system.PersistentCacheStats{}, fmt.Errorf("Store.GetCacheStatsContext: %w", err)
		}
		stats.ByType[cacheType] = count
		stats.Total += count
	}
	if err := rows.Err(); err != nil {
		return system.PersistentCacheStats{}, fmt.Errorf("Store.GetCacheStatsContext: %w", err)
	}
	if len(stats.ByType) == 0 {
		stats.ByType = nil
	}
	return stats, nil
}

// PurgeGuildModerationData drops all moderation warnings and resets the case counter.
func (s *Store) PurgeGuildModerationData(ctx context.Context, guildID string) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if rerr := tx.Rollback(ctx); rerr != nil && !errors.Is(rerr, pgx.ErrTxClosed) {
			err = errors.Join(err, fmt.Errorf("rollback failed: %w", rerr))
		}
	}()

	if _, err := tx.Exec(ctx, `DELETE FROM moderation_warnings WHERE guild_id = $1`, guildID); err != nil {
		return fmt.Errorf("delete moderation_warnings: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM moderation_cases WHERE guild_id = $1`, guildID); err != nil {
		return fmt.Errorf("delete moderation_cases: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

// IncrementDailyMemberJoinContext atomically increments the daily member join counter.
func (s *Store) IncrementDailyMemberJoinContext(ctx context.Context, guildID, userID string, timestamp time.Time) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO daily_member_joins (guild_id, date, count)
		VALUES ($1, CURRENT_DATE, 1)
		ON CONFLICT(guild_id, date) DO UPDATE SET count = daily_member_joins.count + 1
	`, guildID)
	return err
}

// IncrementDailyMemberLeaveContext atomically increments the daily member leave counter.
func (s *Store) IncrementDailyMemberLeaveContext(ctx context.Context, guildID, userID string, timestamp time.Time) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO daily_member_leaves (guild_id, date, count)
		VALUES ($1, CURRENT_DATE, 1)
		ON CONFLICT(guild_id, date) DO UPDATE SET count = daily_member_leaves.count + 1
	`, guildID)
	return err
}

// HeartbeatForBot returns the last recorded heartbeat timestamp for a specific instance.
func (s *Store) HeartbeatForBot(ctx context.Context, instanceID string) (time.Time, bool, error) {
	return s.getRuntimeTimestamp(ctx, "heartbeat_"+instanceID)
}

// LastEventForBot returns the last event timestamp for a specific instance.
func (s *Store) LastEventForBot(ctx context.Context, instanceID string) (time.Time, bool, error) {
	return s.getRuntimeTimestamp(ctx, "last_event_"+instanceID)
}

// Metadata returns an arbitrary metadata timestamp.
func (s *Store) Metadata(ctx context.Context, key string) (time.Time, bool, error) {
	return s.getRuntimeTimestamp(ctx, "meta_"+key)
}

// SetMetadata sets an arbitrary metadata timestamp.
func (s *Store) SetMetadata(ctx context.Context, key string, at time.Time) error {
	return s.setRuntimeTimestamp(ctx, "meta_"+key, at)
}
