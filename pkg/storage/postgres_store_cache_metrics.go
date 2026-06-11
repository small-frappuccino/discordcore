package storage

import (
	"context"
	"database/sql"
	"fmt"
	"iter"
	"strings"
	"time"
)

// CacheEntryRecord is a single persisted cache row keyed by Key within a
// CacheType namespace; Data is the serialized payload and ExpiresAt bounds its
// validity.
type CacheEntryRecord struct {
	Key       string
	CacheType string
	GuildID   string
	Data      string
	ExpiresAt time.Time
}

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
	if key == "" || cacheType == "" || data == "" {
		return nil
	}
	_, err := s.exec(
		`INSERT INTO persistent_cache (cache_key, cache_type, data, expires_at, cached_at)
         VALUES ($1, $2, $3, $4, $5)
         ON CONFLICT(cache_key) DO UPDATE SET
           data=excluded.data,
           expires_at=excluded.expires_at,
           cached_at=excluded.cached_at`,
		key, cacheType, data, expiresAt, time.Now().UTC(),
	)
	return err
}

// UpsertCacheEntriesContext upserts cache entries context.
func (s *Store) UpsertCacheEntriesContext(ctx context.Context, entries []CacheEntryRecord) error {
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

	// Group by GuildID
	byGuild := make(map[string][]CacheEntryRecord)
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

		// Prepare the base query with optional guild_id column matching
		var err error
		if guildID != "" {
			guildIDs := make([]string, len(batch))
			for i := range batch {
				guildIDs[i] = guildID
			}
			_, err = s.db.ExecContext(ctx,
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
			_, err = s.db.ExecContext(ctx,
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
			// Catch foreign key violations (SQLSTATE 23503) for deleted guilds
			if strings.Contains(err.Error(), "23503") || strings.Contains(err.Error(), "foreign key constraint") || strings.Contains(err.Error(), "violates foreign key") {
				continue
			}
			return err
		}
	}

	return nil
}

// GetCacheEntry retrieves a cache entry from persistent storage
func (s *Store) GetCacheEntry(key string) (cacheType, data string, expiresAt time.Time, ok bool, err error) {
	row := s.queryRow(
		`SELECT cache_type, data, expires_at FROM persistent_cache WHERE cache_key=$1`,
		key,
	)
	err = row.Scan(&cacheType, &data, &expiresAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", "", time.Time{}, false, nil
		}
		return "", "", time.Time{}, false, fmt.Errorf("Store.GetCacheEntry: %w", err)
	}
	if time.Now().After(expiresAt) {
		return "", "", time.Time{}, false, nil
	}
	return cacheType, data, expiresAt, true, nil
}

// CacheEntry represents a fetched cache entry
type CacheEntry struct {
	Key       string
	Data      string
	ExpiresAt time.Time
}

// GetCacheEntriesByType retrieves all cache entries of a specific type
func (s *Store) GetCacheEntriesByType(cacheType string) iter.Seq2[CacheEntry, error] {
	return func(yield func(CacheEntry, error) bool) {
		rows, err := s.query(
			`SELECT cache_key, data, expires_at FROM persistent_cache
         WHERE cache_type=$1 AND expires_at > $2`,
			cacheType, time.Now().UTC(),
		)
		if err != nil {
			yield(CacheEntry{}, fmt.Errorf("Store.GetCacheEntriesByType: %w", err))
			return
		}
		defer rows.Close()

		for rows.Next() {
			var entry CacheEntry
			if err := rows.Scan(&entry.Key, &entry.Data, &entry.ExpiresAt); err != nil {
				yield(CacheEntry{}, fmt.Errorf("Store.GetCacheEntriesByType: %w", err))
				return
			}
			if !yield(entry, nil) {
				return
			}
		}
		if err := rows.Err(); err != nil {
			yield(CacheEntry{}, fmt.Errorf("Store.GetCacheEntriesByType: %w", err))
		}
	}
}

// DeleteCacheEntry removes a cache entry from persistent storage
func (s *Store) DeleteCacheEntry(key string) error {
	_, err := s.exec(`DELETE FROM persistent_cache WHERE cache_key=$1`, key)
	return err
}

// CleanupExpiredCacheEntries removes all expired cache entries
func (s *Store) CleanupExpiredCacheEntries() error {
	_, err := s.exec(`DELETE FROM persistent_cache WHERE expires_at <= $1`, time.Now().UTC())
	return err
}

// DeleteCacheEntriesByPrefix deletes all cache entries with keys starting with the given prefix
func (s *Store) DeleteCacheEntriesByPrefix(prefix string) error {
	if prefix == "" {
		return nil
	}
	_, err := s.exec(`DELETE FROM persistent_cache WHERE cache_key LIKE $1`, prefix+"%")
	return err
}

// DeleteCacheEntriesByTypeAndPrefix deletes cache entries filtered by cache_type and key prefix
func (s *Store) DeleteCacheEntriesByTypeAndPrefix(cacheType, keyPrefix string) error {
	if cacheType == "" || keyPrefix == "" {
		return nil
	}
	_, err := s.exec(`DELETE FROM persistent_cache WHERE cache_type=$1 AND cache_key LIKE $2`, cacheType, keyPrefix+"%")
	return err
}

// PersistentCacheStats is a typed snapshot of the persistent_cache table. The
// pkg/discord/cache snapshot embeds this directly so the /v1/health/cache route
// can scrape both in-memory and persisted counters in one JSON document.
type PersistentCacheStats struct {
	Total  int            `json:"total"`
	ByType map[string]int `json:"by_type,omitempty"`
}

// GetCacheStatsContext returns the number of non-expired entries in
// persistent_cache, broken down by cache_type and totalled. The context bounds
// the query so HTTP scrapers do not stall an unhealthy database connection
// forever.
func (s *Store) GetCacheStatsContext(ctx context.Context) (PersistentCacheStats, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT cache_type, COUNT(*) FROM persistent_cache
         WHERE expires_at > $1 GROUP BY cache_type`, time.Now().UTC())
	if err != nil {
		return PersistentCacheStats{}, fmt.Errorf("Store.GetCacheStatsContext: %w", err)
	}
	defer rows.Close()

	stats := PersistentCacheStats{ByType: make(map[string]int)}
	for rows.Next() {
		var cacheType string
		var count int
		if err := rows.Scan(&cacheType, &count); err != nil {
			return PersistentCacheStats{}, fmt.Errorf("Store.GetCacheStatsContext: %w", err)
		}
		stats.ByType[cacheType] = count
		stats.Total += count
	}
	if err := rows.Err(); err != nil {
		return PersistentCacheStats{}, fmt.Errorf("Store.GetCacheStatsContext: %w", err)
	}
	if len(stats.ByType) == 0 {
		stats.ByType = nil
	}
	return stats, nil
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
	query := fmt.Sprintf(
		`INSERT INTO %s (guild_id, channel_id, user_id, day, count)
         VALUES ($1, $2, $3, $4, 1)
         ON CONFLICT(guild_id, channel_id, user_id, day) DO UPDATE SET
           count = %s.count + 1`,
		tableName,
		tableName,
	)
	_, err := s.execContext(ctx, query, guildID, channelID, userID, dailyMetricDay(at))
	return err
}

func (s *Store) incrementDailyMetricByUser(ctx context.Context, tableName, guildID, userID string, at time.Time) error {
	query := fmt.Sprintf(
		`INSERT INTO %s (guild_id, user_id, day, count)
         VALUES ($1, $2, $3, 1)
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

	normalized := normalizeDailyMessageCountDeltas(deltas)
	if len(normalized) == 0 {
		return nil
	}

	guildIDs := make([]string, len(normalized))
	channelIDs := make([]string, len(normalized))
	userIDs := make([]string, len(normalized))
	days := make([]string, len(normalized))
	counts := make([]int64, len(normalized))

	for i, delta := range normalized {
		guildIDs[i] = delta.GuildID
		channelIDs[i] = delta.ChannelID
		userIDs[i] = delta.UserID
		days[i] = delta.Day
		counts[i] = delta.Count
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO daily_message_metrics (guild_id, channel_id, user_id, day, count)
         SELECT * FROM UNNEST($1::text[], $2::text[], $3::text[], $4::date[], $5::bigint[])
         ON CONFLICT(guild_id, channel_id, user_id, day) DO UPDATE SET
           count = daily_message_metrics.count + excluded.count`,
		guildIDs, channelIDs, userIDs, days, counts,
	)
	return err
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

func (s *Store) metricTotalsByDimension(ctx context.Context, tableName, dimension, guildID, cutoffDay, channelID string) iter.Seq2[MetricTotal, error] {
	return func(yield func(MetricTotal, error) bool) {
		if guildID == "" || cutoffDay == "" {
			return
		}
		baseSQL := fmt.Sprintf(
			"SELECT %s, SUM(count) FROM %s WHERE guild_id=$1 AND day>=$2",
			dimension, tableName,
		)
		args := []any{guildID, cutoffDay}
		if channelID != "" {
			baseSQL += " AND channel_id=$1"
			args = append(args, channelID)
		}
		baseSQL += fmt.Sprintf(" GROUP BY %s ORDER BY 2 DESC", dimension)

		rows, err := s.db.QueryContext(ctx, baseSQL, args...)
		if err != nil {
			yield(MetricTotal{}, fmt.Errorf("Store.metricTotalsByDimension: %w", err))
			return
		}
		defer rows.Close()

		for rows.Next() {
			var key string
			var total sql.NullInt64
			if err := rows.Scan(&key, &total); err != nil {
				yield(MetricTotal{}, fmt.Errorf("Store.metricTotalsByDimension: %w", err))
				return
			}
			if total.Valid {
				if !yield(MetricTotal{Key: key, Total: total.Int64}, nil) {
					return
				}
			}
		}
		if err := rows.Err(); err != nil {
			yield(MetricTotal{}, fmt.Errorf("Store.metricTotalsByDimension: %w", err))
		}
	}
}

// MessageTotalsByChannel messages totals by channel.
func (s *Store) MessageTotalsByChannel(ctx context.Context, guildID, cutoffDay, channelID string) iter.Seq2[MetricTotal, error] {
	return s.metricTotalsByDimension(ctx, "daily_message_counts", "channel_id", guildID, cutoffDay, channelID)
}

// MessageTotalsByUser messages totals by user.
func (s *Store) MessageTotalsByUser(ctx context.Context, guildID, cutoffDay, channelID string) iter.Seq2[MetricTotal, error] {
	return s.metricTotalsByDimension(ctx, "daily_message_counts", "user_id", guildID, cutoffDay, channelID)
}

// ReactionTotalsByChannel reactions totals by channel.
func (s *Store) ReactionTotalsByChannel(ctx context.Context, guildID, cutoffDay, channelID string) iter.Seq2[MetricTotal, error] {
	return s.metricTotalsByDimension(ctx, "daily_reaction_counts", "channel_id", guildID, cutoffDay, channelID)
}

// ReactionTotalsByUser reactions totals by user.
func (s *Store) ReactionTotalsByUser(ctx context.Context, guildID, cutoffDay, channelID string) iter.Seq2[MetricTotal, error] {
	return s.metricTotalsByDimension(ctx, "daily_reaction_counts", "user_id", guildID, cutoffDay, channelID)
}

// CountDistinctMemberJoins counts distinct member joins.
func (s *Store) CountDistinctMemberJoins(ctx context.Context, guildID string) (int64, error) {
	if guildID == "" {
		return 0, nil
	}
	var total sql.NullInt64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(DISTINCT user_id) FROM member_joins WHERE guild_id=$1`, guildID).Scan(&total); err != nil {
		return 0, fmt.Errorf("Store.CountDistinctMemberJoins: %w", err)
	}
	if total.Valid {
		return total.Int64, nil
	}
	return 0, nil
}

// ListDistinctMemberJoinUserIDs lists distinct member join user ids.
func (s *Store) ListDistinctMemberJoinUserIDs(ctx context.Context, guildID string) ([]string, error) {
	if guildID == "" {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT user_id FROM member_joins WHERE guild_id=$1`, guildID)
	if err != nil {
		return nil, fmt.Errorf("Store.ListDistinctMemberJoinUserIDs: %w", err)
	}
	defer rows.Close()
	out := make([]string, 0, 128)
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			return nil, fmt.Errorf("Store.ListDistinctMemberJoinUserIDs: %w", err)
		}
		if strings.TrimSpace(userID) != "" {
			out = append(out, userID)
		}
	}
	return out, rows.Err()
}

// SumDailyMemberJoinsSince sums daily member joins since.
func (s *Store) SumDailyMemberJoinsSince(ctx context.Context, guildID, cutoffDay string) (int64, error) {
	return s.sumMetricSince(ctx, `SELECT SUM(count) FROM daily_member_joins WHERE guild_id=$1 AND day>=$2`, guildID, cutoffDay)
}

// SumDailyMemberLeavesSince sums daily member leaves since.
func (s *Store) SumDailyMemberLeavesSince(ctx context.Context, guildID, cutoffDay string) (int64, error) {
	return s.sumMetricSince(ctx, `SELECT SUM(count) FROM daily_member_leaves WHERE guild_id=$1 AND day>=$2`, guildID, cutoffDay)
}

func (s *Store) sumMetricSince(ctx context.Context, query, guildID, cutoffDay string) (int64, error) {
	if guildID == "" || cutoffDay == "" {
		return 0, nil
	}
	var total sql.NullInt64
	if err := s.db.QueryRowContext(ctx, query, guildID, cutoffDay).Scan(&total); err != nil {
		return 0, fmt.Errorf("Store.sumMetricSince: %w", err)
	}
	if total.Valid {
		return total.Int64, nil
	}
	return 0, nil
}

// DatabaseSizeBytes databases size bytes.
func (s *Store) DatabaseSizeBytes(ctx context.Context) (int64, error) {
	var size sql.NullInt64
	if err := s.db.QueryRowContext(ctx, `SELECT pg_database_size(current_database())`).Scan(&size); err != nil {
		return 0, fmt.Errorf("Store.DatabaseSizeBytes: %w", err)
	}
	if size.Valid {
		return size.Int64, nil
	}
	return 0, nil
}
