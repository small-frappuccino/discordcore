package storage

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"
)

// DailyMessageCountDelta represents a delta to be applied to one daily message metric bucket.
type DailyMessageCountDelta struct {
	GuildID   string
	ChannelID string
	UserID    string
	Day       string
	Count     int64
}

// UpsertCacheEntry saves a cache entry to persistent storage
func (s *Store) UpsertCacheEntry(key, cacheType, data string, expiresAt time.Time) error {
	if s.db == nil {
		return fmt.Errorf("store not initialized")
	}
	if key == "" || cacheType == "" || data == "" {
		return nil
	}
	_, err := s.exec(
		`INSERT INTO persistent_cache (cache_key, cache_type, data, expires_at, cached_at)
         VALUES (?, ?, ?, ?, ?)
         ON CONFLICT(cache_key) DO UPDATE SET
           data=excluded.data,
           expires_at=excluded.expires_at,
           cached_at=excluded.cached_at`,
		key, cacheType, data, expiresAt, time.Now().UTC(),
	)
	return err
}

func (s *Store) UpsertCacheEntriesContext(ctx context.Context, entries []CacheEntryRecord) error {
	if s.db == nil {
		return fmt.Errorf("store not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	normalized := make([]CacheEntryRecord, 0, len(entries))
	for _, entry := range entries {
		if entry.Key == "" || entry.CacheType == "" || entry.Data == "" {
			continue
		}
		normalized = append(normalized, entry)
	}
	if len(normalized) == 0 {
		return nil
	}

	cachedAt := time.Now().UTC()
	return execValuesContext(ctx, s.db,
		`INSERT INTO persistent_cache (cache_key, cache_type, data, expires_at, cached_at) VALUES `,
		` ON CONFLICT(cache_key) DO UPDATE SET
			data=excluded.data,
			expires_at=excluded.expires_at,
			cached_at=excluded.cached_at`,
		len(normalized), 5,
		func(args []any, idx int) []any {
			entry := normalized[idx]
			return append(args, entry.Key, entry.CacheType, entry.Data, entry.ExpiresAt, cachedAt)
		},
	)
}

// GetCacheEntry retrieves a cache entry from persistent storage
func (s *Store) GetCacheEntry(key string) (cacheType, data string, expiresAt time.Time, ok bool, err error) {
	if s.db == nil {
		return "", "", time.Time{}, false, fmt.Errorf("store not initialized")
	}
	row := s.queryRow(
		`SELECT cache_type, data, expires_at FROM persistent_cache WHERE cache_key=?`,
		key,
	)
	err = row.Scan(&cacheType, &data, &expiresAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", "", time.Time{}, false, nil
		}
		return "", "", time.Time{}, false, err
	}
	if time.Now().After(expiresAt) {
		return "", "", time.Time{}, false, nil
	}
	return cacheType, data, expiresAt, true, nil
}

// GetCacheEntriesByType retrieves all cache entries of a specific type
func (s *Store) GetCacheEntriesByType(cacheType string) ([]struct {
	Key       string
	Data      string
	ExpiresAt time.Time
}, error) {
	if s.db == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	rows, err := s.query(
		`SELECT cache_key, data, expires_at FROM persistent_cache
         WHERE cache_type=? AND expires_at > ?`,
		cacheType, time.Now().UTC(),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []struct {
		Key       string
		Data      string
		ExpiresAt time.Time
	}
	for rows.Next() {
		var entry struct {
			Key       string
			Data      string
			ExpiresAt time.Time
		}
		if err := rows.Scan(&entry.Key, &entry.Data, &entry.ExpiresAt); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

// DeleteCacheEntry removes a cache entry from persistent storage
func (s *Store) DeleteCacheEntry(key string) error {
	if s.db == nil {
		return fmt.Errorf("store not initialized")
	}
	_, err := s.exec(`DELETE FROM persistent_cache WHERE cache_key=?`, key)
	return err
}

// CleanupExpiredCacheEntries removes all expired cache entries
func (s *Store) CleanupExpiredCacheEntries() error {
	if s.db == nil {
		return fmt.Errorf("store not initialized")
	}
	_, err := s.exec(`DELETE FROM persistent_cache WHERE expires_at <= ?`, time.Now().UTC())
	return err
}

// DeleteCacheEntriesByPrefix deletes all cache entries with keys starting with the given prefix
func (s *Store) DeleteCacheEntriesByPrefix(prefix string) error {
	if s.db == nil {
		return fmt.Errorf("store not initialized")
	}
	if prefix == "" {
		return nil
	}
	_, err := s.exec(`DELETE FROM persistent_cache WHERE cache_key LIKE ?`, prefix+"%")
	return err
}

// DeleteCacheEntriesByTypeAndPrefix deletes cache entries filtered by cache_type and key prefix
func (s *Store) DeleteCacheEntriesByTypeAndPrefix(cacheType, keyPrefix string) error {
	if s.db == nil {
		return fmt.Errorf("store not initialized")
	}
	if cacheType == "" || keyPrefix == "" {
		return nil
	}
	_, err := s.exec(`DELETE FROM persistent_cache WHERE cache_type=? AND cache_key LIKE ?`, cacheType, keyPrefix+"%")
	return err
}

// GetCacheStats returns statistics about the persistent cache
func (s *Store) GetCacheStats() (map[string]int, error) {
	if s.db == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	rows, err := s.query(
		`SELECT cache_type, COUNT(*) as count FROM persistent_cache
         WHERE expires_at > ? GROUP BY cache_type`,
		time.Now().UTC(),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := make(map[string]int)
	for rows.Next() {
		var cacheType string
		var count int
		if err := rows.Scan(&cacheType, &count); err != nil {
			return nil, err
		}
		stats[cacheType] = count
	}
	return stats, rows.Err()
}

// IncrementDailyMessageCount increments the per-day message count for a user in a channel.
func (s *Store) IncrementDailyMessageCount(guildID, channelID, userID string, at time.Time) error {
	return s.IncrementDailyMessageCountContext(context.Background(), guildID, channelID, userID, at)
}

func dailyMetricDay(at time.Time) string {
	if at.IsZero() {
		at = time.Now().UTC()
	} else {
		at = at.UTC()
	}
	return time.Date(at.Year(), at.Month(), at.Day(), 0, 0, 0, 0, time.UTC).Format("2006-01-02")
}

func (s *Store) incrementDailyMetricByChannelAndUser(ctx context.Context, tableName, guildID, channelID, userID string, at time.Time) error {
	if s.db == nil {
		return fmt.Errorf("store not initialized")
	}
	query := fmt.Sprintf(
		`INSERT INTO %s (guild_id, channel_id, user_id, day, count)
         VALUES (?, ?, ?, ?, 1)
         ON CONFLICT(guild_id, channel_id, user_id, day) DO UPDATE SET
           count = %s.count + 1`,
		tableName,
		tableName,
	)
	_, err := s.execContext(ctx, query, guildID, channelID, userID, dailyMetricDay(at))
	return err
}

func (s *Store) incrementDailyMetricByUser(ctx context.Context, tableName, guildID, userID string, at time.Time) error {
	if s.db == nil {
		return fmt.Errorf("store not initialized")
	}
	query := fmt.Sprintf(
		`INSERT INTO %s (guild_id, user_id, day, count)
         VALUES (?, ?, ?, 1)
         ON CONFLICT(guild_id, user_id, day) DO UPDATE SET
           count = %s.count + 1`,
		tableName,
		tableName,
	)
	_, err := s.execContext(ctx, query, guildID, userID, dailyMetricDay(at))
	return err
}

// IncrementDailyMessageCountContext increments the per-day message count for a user in a channel with context support.
func (s *Store) IncrementDailyMessageCountContext(ctx context.Context, guildID, channelID, userID string, at time.Time) error {
	if guildID == "" || channelID == "" || userID == "" {
		return nil
	}
	return s.incrementDailyMetricByChannelAndUser(ctx, "daily_message_metrics", guildID, channelID, userID, at)
}

// IncrementDailyMessageCountsContext applies a batch of daily message count deltas.
func (s *Store) IncrementDailyMessageCountsContext(ctx context.Context, deltas []DailyMessageCountDelta) error {
	if s.db == nil {
		return fmt.Errorf("store not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	normalized := normalizeDailyMessageCountDeltas(deltas)
	if len(normalized) == 0 {
		return nil
	}

	return execValuesContext(ctx, s.db,
		`INSERT INTO daily_message_metrics (guild_id, channel_id, user_id, day, count) VALUES `,
		` ON CONFLICT(guild_id, channel_id, user_id, day) DO UPDATE SET
           count = daily_message_metrics.count + excluded.count`,
		len(normalized),
		5,
		func(args []any, rowIndex int) []any {
			delta := normalized[rowIndex]
			return append(args, delta.GuildID, delta.ChannelID, delta.UserID, delta.Day, delta.Count)
		},
	)
}

func normalizeDailyMessageCountDeltas(deltas []DailyMessageCountDelta) []DailyMessageCountDelta {
	if len(deltas) == 0 {
		return nil
	}
	order := make([]string, 0, len(deltas))
	agg := make(map[string]DailyMessageCountDelta, len(deltas))
	for _, delta := range deltas {
		delta.GuildID = strings.TrimSpace(delta.GuildID)
		delta.ChannelID = strings.TrimSpace(delta.ChannelID)
		delta.UserID = strings.TrimSpace(delta.UserID)
		if delta.GuildID == "" || delta.ChannelID == "" || delta.UserID == "" || delta.Count == 0 {
			continue
		}
		if strings.TrimSpace(delta.Day) == "" {
			delta.Day = dailyMetricDay(time.Now().UTC())
		}
		key := strings.Join([]string{delta.GuildID, delta.ChannelID, delta.UserID, delta.Day}, ":")
		if current, ok := agg[key]; ok {
			current.Count += delta.Count
			agg[key] = current
			continue
		}
		order = append(order, key)
		agg[key] = delta
	}

	normalized := make([]DailyMessageCountDelta, 0, len(order))
	for _, key := range order {
		delta := agg[key]
		if delta.Count == 0 {
			continue
		}
		normalized = append(normalized, delta)
	}
	return normalized
}

// IncrementDailyReactionCount increments the per-day reaction count for a user in a channel.
func (s *Store) IncrementDailyReactionCount(guildID, channelID, userID string, at time.Time) error {
	return s.IncrementDailyReactionCountContext(context.Background(), guildID, channelID, userID, at)
}

// IncrementDailyReactionCountContext increments the per-day reaction count for a user in a channel with context support.
func (s *Store) IncrementDailyReactionCountContext(ctx context.Context, guildID, channelID, userID string, at time.Time) error {
	if guildID == "" || channelID == "" || userID == "" {
		return nil
	}
	return s.incrementDailyMetricByChannelAndUser(ctx, "daily_reaction_metrics", guildID, channelID, userID, at)
}

// IncrementDailyMemberJoin increments the per-day member join counter (per user).
func (s *Store) IncrementDailyMemberJoin(guildID, userID string, at time.Time) error {
	return s.IncrementDailyMemberJoinContext(context.Background(), guildID, userID, at)
}

// IncrementDailyMemberJoinContext increments the per-day member join counter (per user) with context support.
func (s *Store) IncrementDailyMemberJoinContext(ctx context.Context, guildID, userID string, at time.Time) error {
	if guildID == "" || userID == "" {
		return nil
	}
	return s.incrementDailyMetricByUser(ctx, "daily_member_joins", guildID, userID, at)
}

// IncrementDailyMemberLeave increments the per-day member leave counter (per user).
func (s *Store) IncrementDailyMemberLeave(guildID, userID string, at time.Time) error {
	return s.IncrementDailyMemberLeaveContext(context.Background(), guildID, userID, at)
}

// IncrementDailyMemberLeaveContext increments the per-day member leave counter (per user) with context support.
func (s *Store) IncrementDailyMemberLeaveContext(ctx context.Context, guildID, userID string, at time.Time) error {
	if guildID == "" || userID == "" {
		return nil
	}
	return s.incrementDailyMetricByUser(ctx, "daily_member_leaves", guildID, userID, at)
}

// MetricTotal represents an aggregated metric value for a specific key.
type MetricTotal struct {
	Key   string
	Total int64
}

func (s *Store) metricTotalsByDimension(ctx context.Context, tableName, dimension, guildID, cutoffDay, channelID string) ([]MetricTotal, error) {
	if s.db == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	if guildID == "" || cutoffDay == "" {
		return nil, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	baseSQL := fmt.Sprintf(
		"SELECT %s, SUM(count) FROM %s WHERE guild_id=? AND day>=?",
		dimension, tableName,
	)
	args := []any{guildID, cutoffDay}
	if channelID != "" {
		baseSQL += " AND channel_id=?"
		args = append(args, channelID)
	}
	baseSQL += fmt.Sprintf(" GROUP BY %s", dimension)

	rows, err := s.db.QueryContext(ctx, rebind(baseSQL), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]MetricTotal, 0, 16)
	for rows.Next() {
		var key string
		var total sql.NullInt64
		if err := rows.Scan(&key, &total); err != nil {
			return nil, err
		}
		if total.Valid {
			out = append(out, MetricTotal{Key: key, Total: total.Int64})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Total > out[j].Total })
	return out, nil
}

func (s *Store) MessageTotalsByChannel(ctx context.Context, guildID, cutoffDay, channelID string) ([]MetricTotal, error) {
	return s.metricTotalsByDimension(ctx, "daily_message_metrics", "channel_id", guildID, cutoffDay, channelID)
}

func (s *Store) MessageTotalsByUser(ctx context.Context, guildID, cutoffDay, channelID string) ([]MetricTotal, error) {
	return s.metricTotalsByDimension(ctx, "daily_message_metrics", "user_id", guildID, cutoffDay, channelID)
}

func (s *Store) ReactionTotalsByChannel(ctx context.Context, guildID, cutoffDay, channelID string) ([]MetricTotal, error) {
	return s.metricTotalsByDimension(ctx, "daily_reaction_metrics", "channel_id", guildID, cutoffDay, channelID)
}

func (s *Store) ReactionTotalsByUser(ctx context.Context, guildID, cutoffDay, channelID string) ([]MetricTotal, error) {
	return s.metricTotalsByDimension(ctx, "daily_reaction_metrics", "user_id", guildID, cutoffDay, channelID)
}

func (s *Store) CountDistinctMemberJoins(ctx context.Context, guildID string) (int64, error) {
	if s.db == nil {
		return 0, fmt.Errorf("store not initialized")
	}
	if guildID == "" {
		return 0, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	var total sql.NullInt64
	if err := s.db.QueryRowContext(ctx, rebind(`SELECT COUNT(DISTINCT user_id) FROM member_joins WHERE guild_id=?`), guildID).Scan(&total); err != nil {
		return 0, err
	}
	if total.Valid {
		return total.Int64, nil
	}
	return 0, nil
}

func (s *Store) ListDistinctMemberJoinUserIDs(ctx context.Context, guildID string) ([]string, error) {
	if s.db == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	if guildID == "" {
		return nil, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	rows, err := s.db.QueryContext(ctx, rebind(`SELECT DISTINCT user_id FROM member_joins WHERE guild_id=?`), guildID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]string, 0, 128)
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			return nil, err
		}
		if strings.TrimSpace(userID) != "" {
			out = append(out, userID)
		}
	}
	return out, rows.Err()
}

func (s *Store) SumDailyMemberJoinsSince(ctx context.Context, guildID, cutoffDay string) (int64, error) {
	return s.sumMetricSince(ctx, `SELECT SUM(count) FROM daily_member_joins WHERE guild_id=? AND day>=?`, guildID, cutoffDay)
}

func (s *Store) SumDailyMemberLeavesSince(ctx context.Context, guildID, cutoffDay string) (int64, error) {
	return s.sumMetricSince(ctx, `SELECT SUM(count) FROM daily_member_leaves WHERE guild_id=? AND day>=?`, guildID, cutoffDay)
}

func (s *Store) sumMetricSince(ctx context.Context, query, guildID, cutoffDay string) (int64, error) {
	if s.db == nil {
		return 0, fmt.Errorf("store not initialized")
	}
	if guildID == "" || cutoffDay == "" {
		return 0, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	var total sql.NullInt64
	if err := s.db.QueryRowContext(ctx, rebind(query), guildID, cutoffDay).Scan(&total); err != nil {
		return 0, err
	}
	if total.Valid {
		return total.Int64, nil
	}
	return 0, nil
}

func (s *Store) DatabaseSizeBytes(ctx context.Context) (int64, error) {
	if s.db == nil {
		return 0, fmt.Errorf("store not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	var size sql.NullInt64
	if err := s.db.QueryRowContext(ctx, `SELECT pg_database_size(current_database())`).Scan(&size); err != nil {
		return 0, err
	}
	if size.Valid {
		return size.Int64, nil
	}
	return 0, nil
}
