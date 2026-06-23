package clean

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/utils/httputil"
	"github.com/small-frappuccino/discordcore/pkg/clean"
	"golang.org/x/sync/errgroup"
)

// Metrics defines the observability surface for Discord-facing cleanup operations.
type Metrics interface {
	RecordCleanAttempt()
	RecordCleanSuccess(durationMs int64, deleted int)
	RecordCleanFailure(cause string, durationMs int64)
	RecordCleanDeleteFailure(class string)
	RecordCleanAuditLogFailure()
}

// NopMetrics implements Metrics as no-ops to allow safe concurrent drops when unconfigured.
type NopMetrics struct{}

func (NopMetrics) RecordCleanAttempt()                               {}
func (NopMetrics) RecordCleanSuccess(durationMs int64, deleted int)  {}
func (NopMetrics) RecordCleanFailure(cause string, durationMs int64) {}
func (NopMetrics) RecordCleanDeleteFailure(class string)             {}
func (NopMetrics) RecordCleanAuditLogFailure()                       {}

// Client specifies the Arikawa interface bounds required to fetch and eliminate messages.
type Client interface {
	Messages(channelID discord.ChannelID, limit uint) ([]discord.Message, error)
	MessagesBefore(channelID discord.ChannelID, before discord.MessageID, limit uint) ([]discord.Message, error)
	DeleteMessages(channelID discord.ChannelID, messageIDs []discord.MessageID, reason api.AuditLogReason) error
	DeleteMessage(channelID discord.ChannelID, messageID discord.MessageID, reason api.AuditLogReason) error
	SendMessageComplex(channelID discord.ChannelID, data api.SendMessageData) (*discord.Message, error)
}

// Service orchestrates the discord-facing lifecycle of a clean command operation, handling API pagination, batch fallback degradation, and telemetry.
type Service struct {
	client  Client
	metrics Metrics
	logger  *slog.Logger
	now     func() time.Time
	wg      sync.WaitGroup
}

// NewService initializes a Clean service bounded by the provided client and metrics adapters.
func NewService(client Client, metrics Metrics, logger *slog.Logger) *Service {
	if metrics == nil {
		metrics = NopMetrics{}
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		client:  client,
		metrics: metrics,
		logger:  logger,
		now:     time.Now,
	}
}

// Close gracefully waits for all pending async operations (like audit logging) to finish.
func (s *Service) Close() error {
	s.wg.Wait()
	return nil
}

// ExecuteClean computes and enacts the deletion payload. It guarantees that a failure during the deletion phase does not panic or infinitely block.
func (s *Service) ExecuteClean(ctx context.Context, channelID discord.ChannelID, filter clean.Filter, auditChannelID discord.ChannelID, requestedBy string) (int, error) {
	s.metrics.RecordCleanAttempt()
	start := s.now()

	messages, err := s.fetchAndFilter(channelID, filter)
	if err != nil {
		duration := s.now().Sub(start).Milliseconds()
		s.metrics.RecordCleanFailure("fetch_failed", duration)
		return 0, fmt.Errorf("fetch messages: %w", err)
	}

	if len(messages) == 0 {
		return 0, nil
	}

	categorized := clean.CategorizeMessages(messages, s.now)

	var deletedCount int32

	if len(categorized.BulkIDs) > 0 {
		bulkDiscordIDs := make([]discord.MessageID, 0, len(categorized.BulkIDs))
		for _, id := range categorized.BulkIDs {
			parsed, _ := discord.ParseSnowflake(id)
			bulkDiscordIDs = append(bulkDiscordIDs, discord.MessageID(parsed))
		}

		err := s.client.DeleteMessages(channelID, bulkDiscordIDs, "")
		if err != nil {
			var httpErr *httputil.HTTPError
			if errors.As(err, &httpErr) && httpErr.Code == 50034 {
				// Operational annotation: Code 50034 indicates some targets exceed the 14-day bulk delete threshold.
				// We intentionally swallow the error and gracefully cascade the failing payload directly into the single-deletion pipeline.
				s.logger.Warn("Bulk delete failed with 50034, falling back to sequential", "channel_id", channelID)
				categorized.SingleIDs = append(categorized.SingleIDs, categorized.BulkIDs...)
			} else {
				for i := 0; i < len(bulkDiscordIDs); i++ {
					s.metrics.RecordCleanDeleteFailure("bulk_error")
				}
				s.logger.Error("Bulk delete failed", "error", err, "channel_id", channelID)
			}
		} else {
			atomic.AddInt32(&deletedCount, int32(len(bulkDiscordIDs)))
		}
	}

	if len(categorized.SingleIDs) > 0 {
		eg, _ := errgroup.WithContext(ctx)
		eg.SetLimit(10)

		for _, idStr := range categorized.SingleIDs {
			idStr := idStr
			eg.Go(func() error {
				parsed, _ := discord.ParseSnowflake(idStr)
				err := s.client.DeleteMessage(channelID, discord.MessageID(parsed), "")
				if err != nil {
					s.metrics.RecordCleanDeleteFailure("single_error")
					s.logger.Warn("Single delete failed", "error", err, "message_id", idStr)
				} else {
					atomic.AddInt32(&deletedCount, 1)
				}
				return nil
			})
		}
		_ = eg.Wait()
	}

	finalDeleted := int(atomic.LoadInt32(&deletedCount))
	durationMs := s.now().Sub(start).Milliseconds()
	s.metrics.RecordCleanSuccess(durationMs, finalDeleted)

	if auditChannelID.IsValid() && finalDeleted > 0 {
		// Operational annotation: Audit logging is intentionally asynchronous. A failure here is non-fatal
		// and must not impact the primary execution loop's success report.
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.dispatchAuditLog(auditChannelID, channelID, finalDeleted, filter, requestedBy)
		}()
	}

	return finalDeleted, nil
}

func (s *Service) fetchAndFilter(channelID discord.ChannelID, filter clean.Filter) ([]clean.Message, error) {
	var allMessages []clean.Message
	var before discord.MessageID
	scanned := 0

	for scanned < clean.CleanSearchWindow && len(allMessages) < filter.Count {
		limit := uint(100)
		if clean.CleanSearchWindow-scanned < int(limit) {
			limit = uint(clean.CleanSearchWindow - scanned)
		}

		var page []discord.Message
		var err error
		if before.IsValid() {
			page, err = s.client.MessagesBefore(channelID, before, limit)
		} else {
			page, err = s.client.Messages(channelID, limit)
		}

		if err != nil {
			return nil, err
		}
		if len(page) == 0 {
			break
		}

		var cleanPage []clean.Message
		for _, m := range page {
			cleanPage = append(cleanPage, clean.Message{
				ID:        m.ID.String(),
				AuthorID:  m.Author.ID.String(),
				Content:   m.Content,
				Timestamp: m.Timestamp.Time(),
				Pinned:    m.Pinned,
			})
		}

		result := clean.ApplyFilter(cleanPage, filter, len(allMessages))
		allMessages = append(allMessages, result.Matched...)
		scanned += result.Scanned

		if len(page) > 0 {
			before = page[len(page)-1].ID
		}

		if result.Scanned < len(page) {
			break
		}
	}

	return allMessages, nil
}

func (s *Service) dispatchAuditLog(auditChannelID discord.ChannelID, targetChannelID discord.ChannelID, deleted int, filter clean.Filter, requestedBy string) {
	embed := discord.Embed{
		Title:       "Clean Command Executed",
		Color:       0x3498db,
		Description: fmt.Sprintf("Cleaned %d messages in <#%s>.", deleted, targetChannelID),
		Fields: []discord.EmbedField{
			{Name: "Requested By", Value: requestedBy, Inline: true},
		},
		Timestamp: discord.NewTimestamp(s.now()),
	}

	_, err := s.client.SendMessageComplex(auditChannelID, api.SendMessageData{
		Embeds: []discord.Embed{embed},
	})
	if err != nil {
		s.metrics.RecordCleanAuditLogFailure()
		s.logger.Error("Failed to send clean audit log", "error", err, "audit_channel_id", auditChannelID)
	}
}
