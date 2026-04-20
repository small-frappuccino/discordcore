package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

type qotdRowScanner interface {
	Scan(dest ...any) error
}

func normalizeQOTDQuestionRecord(rec QOTDQuestionRecord) (QOTDQuestionRecord, error) {
	rec.GuildID = strings.TrimSpace(rec.GuildID)
	rec.DeckID = strings.TrimSpace(rec.DeckID)
	rec.Body = strings.TrimSpace(rec.Body)
	rec.Status = strings.TrimSpace(rec.Status)
	rec.CreatedBy = strings.TrimSpace(rec.CreatedBy)
	rec.QueuePosition = maxInt64(rec.QueuePosition, 0)
	rec.ScheduledForDateUTC = normalizeQOTDDatePtr(rec.ScheduledForDateUTC)
	rec.UsedAt = normalizeQOTDTimePtr(rec.UsedAt)

	if rec.GuildID == "" {
		return QOTDQuestionRecord{}, fmt.Errorf("guild_id is required")
	}
	if rec.DeckID == "" {
		return QOTDQuestionRecord{}, fmt.Errorf("deck_id is required")
	}
	if rec.Body == "" {
		return QOTDQuestionRecord{}, fmt.Errorf("body is required")
	}
	if rec.Status == "" {
		rec.Status = "draft"
	}
	return rec, nil
}

func normalizeQOTDOfficialPostRecord(rec QOTDOfficialPostRecord) (QOTDOfficialPostRecord, error) {
	rec.GuildID = strings.TrimSpace(rec.GuildID)
	rec.DeckID = strings.TrimSpace(rec.DeckID)
	rec.DeckNameSnapshot = strings.TrimSpace(rec.DeckNameSnapshot)
	rec.PublishMode = strings.TrimSpace(rec.PublishMode)
	rec.State = strings.TrimSpace(rec.State)
	rec.ForumChannelID = strings.TrimSpace(rec.ForumChannelID)
	rec.QuestionListThreadID = strings.TrimSpace(rec.QuestionListThreadID)
	rec.QuestionListEntryMessageID = strings.TrimSpace(rec.QuestionListEntryMessageID)
	rec.DiscordThreadID = strings.TrimSpace(rec.DiscordThreadID)
	rec.DiscordStarterMessageID = strings.TrimSpace(rec.DiscordStarterMessageID)
	rec.AnswerChannelID = strings.TrimSpace(rec.AnswerChannelID)
	rec.QuestionTextSnapshot = strings.TrimSpace(rec.QuestionTextSnapshot)
	rec.PublishDateUTC = normalizeQOTDDateUTC(rec.PublishDateUTC)
	rec.PublishedAt = normalizeQOTDTimePtr(rec.PublishedAt)
	rec.ClosedAt = normalizeQOTDTimePtr(rec.ClosedAt)
	rec.ArchivedAt = normalizeQOTDTimePtr(rec.ArchivedAt)
	rec.LastReconciledAt = normalizeQOTDTimePtr(rec.LastReconciledAt)
	rec.GraceUntil = normalizeQOTDRequiredTime(rec.GraceUntil)
	rec.ArchiveAt = normalizeQOTDRequiredTime(rec.ArchiveAt)

	if rec.PublishMode == "" {
		rec.PublishMode = "scheduled"
	}

	if rec.GuildID == "" {
		return QOTDOfficialPostRecord{}, fmt.Errorf("guild_id is required")
	}
	if rec.DeckID == "" {
		return QOTDOfficialPostRecord{}, fmt.Errorf("deck_id is required")
	}
	if rec.DeckNameSnapshot == "" {
		return QOTDOfficialPostRecord{}, fmt.Errorf("deck_name_snapshot is required")
	}
	if rec.QuestionID <= 0 {
		return QOTDOfficialPostRecord{}, fmt.Errorf("question_id is required")
	}
	if rec.PublishMode == "" {
		return QOTDOfficialPostRecord{}, fmt.Errorf("publish_mode is required")
	}
	if rec.PublishDateUTC.IsZero() {
		return QOTDOfficialPostRecord{}, fmt.Errorf("publish_date_utc is required")
	}
	if rec.ForumChannelID == "" {
		return QOTDOfficialPostRecord{}, fmt.Errorf("forum_channel_id is required")
	}
	if rec.QuestionTextSnapshot == "" {
		return QOTDOfficialPostRecord{}, fmt.Errorf("question_text_snapshot is required")
	}
	if rec.GraceUntil.IsZero() {
		return QOTDOfficialPostRecord{}, fmt.Errorf("grace_until is required")
	}
	if rec.ArchiveAt.IsZero() {
		return QOTDOfficialPostRecord{}, fmt.Errorf("archive_at is required")
	}
	return rec, nil
}

func normalizeQOTDThreadArchiveRecord(rec QOTDThreadArchiveRecord) (QOTDThreadArchiveRecord, error) {
	rec.GuildID = strings.TrimSpace(rec.GuildID)
	rec.SourceKind = strings.TrimSpace(rec.SourceKind)
	rec.DiscordThreadID = strings.TrimSpace(rec.DiscordThreadID)
	rec.ArchivedAt = normalizeQOTDRequiredTime(rec.ArchivedAt)

	if rec.GuildID == "" {
		return QOTDThreadArchiveRecord{}, fmt.Errorf("guild_id is required")
	}
	if rec.OfficialPostID <= 0 {
		return QOTDThreadArchiveRecord{}, fmt.Errorf("official_post_id is required")
	}
	if rec.SourceKind == "" {
		return QOTDThreadArchiveRecord{}, fmt.Errorf("source_kind is required")
	}
	if rec.DiscordThreadID == "" {
		return QOTDThreadArchiveRecord{}, fmt.Errorf("discord_thread_id is required")
	}
	if rec.ArchivedAt.IsZero() {
		return QOTDThreadArchiveRecord{}, fmt.Errorf("archived_at is required")
	}
	return rec, nil
}

func normalizeQOTDMessageArchives(threadArchiveID int64, msgs []QOTDMessageArchiveRecord) ([]QOTDMessageArchiveRecord, error) {
	if len(msgs) == 0 {
		return nil, nil
	}

	order := make([]string, 0, len(msgs))
	byMessage := make(map[string]QOTDMessageArchiveRecord, len(msgs))
	for _, msg := range msgs {
		msg.ThreadArchiveID = threadArchiveID
		msg.DiscordMessageID = strings.TrimSpace(msg.DiscordMessageID)
		msg.AuthorID = strings.TrimSpace(msg.AuthorID)
		msg.AuthorNameSnapshot = strings.TrimSpace(msg.AuthorNameSnapshot)
		msg.Content = strings.TrimSpace(msg.Content)
		msg.EmbedsJSON = cloneQOTDJSONRawMessage(msg.EmbedsJSON)
		msg.AttachmentsJSON = cloneQOTDJSONRawMessage(msg.AttachmentsJSON)
		if msg.CreatedAt.IsZero() {
			msg.CreatedAt = time.Now().UTC()
		} else {
			msg.CreatedAt = msg.CreatedAt.UTC()
		}

		if msg.DiscordMessageID == "" {
			return nil, fmt.Errorf("discord_message_id is required")
		}
		if _, ok := byMessage[msg.DiscordMessageID]; !ok {
			order = append(order, msg.DiscordMessageID)
		}
		byMessage[msg.DiscordMessageID] = msg
	}

	normalized := make([]QOTDMessageArchiveRecord, 0, len(order))
	for _, messageID := range order {
		normalized = append(normalized, byMessage[messageID])
	}
	return normalized, nil
}

func normalizeQOTDCollectedQuestionRecords(records []QOTDCollectedQuestionRecord) ([]QOTDCollectedQuestionRecord, error) {
	if len(records) == 0 {
		return nil, nil
	}

	order := make([]string, 0, len(records))
	byMessage := make(map[string]QOTDCollectedQuestionRecord, len(records))
	for _, record := range records {
		record.GuildID = strings.TrimSpace(record.GuildID)
		record.SourceChannelID = strings.TrimSpace(record.SourceChannelID)
		record.SourceMessageID = strings.TrimSpace(record.SourceMessageID)
		record.SourceAuthorID = strings.TrimSpace(record.SourceAuthorID)
		record.SourceAuthorNameSnapshot = strings.TrimSpace(record.SourceAuthorNameSnapshot)
		record.EmbedTitle = strings.TrimSpace(record.EmbedTitle)
		record.QuestionText = strings.Join(strings.Fields(strings.TrimSpace(record.QuestionText)), " ")
		record.SourceCreatedAt = normalizeQOTDRequiredTime(record.SourceCreatedAt)

		switch {
		case record.GuildID == "":
			return nil, fmt.Errorf("guild_id is required")
		case record.SourceChannelID == "":
			return nil, fmt.Errorf("source_channel_id is required")
		case record.SourceMessageID == "":
			return nil, fmt.Errorf("source_message_id is required")
		case record.SourceCreatedAt.IsZero():
			return nil, fmt.Errorf("source_created_at is required")
		case record.QuestionText == "":
			return nil, fmt.Errorf("question_text is required")
		}

		key := record.GuildID + "\x00" + record.SourceMessageID
		if _, exists := byMessage[key]; !exists {
			order = append(order, key)
		}
		byMessage[key] = record
	}

	normalized := make([]QOTDCollectedQuestionRecord, 0, len(order))
	for _, key := range order {
		normalized = append(normalized, byMessage[key])
	}
	return normalized, nil
}

func normalizeQOTDOrderedIDs(ids []int64) ([]int64, error) {
	if len(ids) == 0 {
		return nil, fmt.Errorf("ordered ids are required")
	}
	seen := make(map[int64]struct{}, len(ids))
	normalized := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return nil, fmt.Errorf("ordered ids must be positive")
		}
		if _, ok := seen[id]; ok {
			return nil, fmt.Errorf("ordered ids must be unique")
		}
		seen[id] = struct{}{}
		normalized = append(normalized, id)
	}
	return normalized, nil
}

func normalizeQOTDDeckIDs(deckIDs []string) []string {
	if len(deckIDs) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(deckIDs))
	normalized := make([]string, 0, len(deckIDs))
	for _, deckID := range deckIDs {
		deckID = strings.TrimSpace(deckID)
		if deckID == "" {
			continue
		}
		if _, ok := seen[deckID]; ok {
			continue
		}
		seen[deckID] = struct{}{}
		normalized = append(normalized, deckID)
	}
	return normalized
}

func sameQOTDIDSet(current, ordered []int64) bool {
	if len(current) != len(ordered) {
		return false
	}
	left := append([]int64(nil), current...)
	right := append([]int64(nil), ordered...)
	sort.Slice(left, func(i, j int) bool { return left[i] < left[j] })
	sort.Slice(right, func(i, j int) bool { return right[i] < right[j] })
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func scanQOTDQuestionRecord(scanner qotdRowScanner) (*QOTDQuestionRecord, error) {
	var record QOTDQuestionRecord
	var scheduledFor sql.NullTime
	var usedAt sql.NullTime
	if err := scanner.Scan(
		&record.ID,
		&record.GuildID,
		&record.DeckID,
		&record.Body,
		&record.Status,
		&record.QueuePosition,
		&record.CreatedBy,
		&scheduledFor,
		&usedAt,
		&record.CreatedAt,
		&record.UpdatedAt,
	); err != nil {
		return nil, err
	}
	record.ScheduledForDateUTC = timePtrFromNull(scheduledFor)
	record.UsedAt = timePtrFromNull(usedAt)
	return &record, nil
}

func scanQOTDOfficialPostRecord(scanner qotdRowScanner) (*QOTDOfficialPostRecord, error) {
	var record QOTDOfficialPostRecord
	var questionListThreadID sql.NullString
	var questionListEntryMessageID sql.NullString
	var threadID sql.NullString
	var starterMessageID sql.NullString
	var answerChannelID sql.NullString
	var publishedAt sql.NullTime
	var closedAt sql.NullTime
	var archivedAt sql.NullTime
	var reconciledAt sql.NullTime
	if err := scanner.Scan(
		&record.ID,
		&record.GuildID,
		&record.DeckID,
		&record.DeckNameSnapshot,
		&record.QuestionID,
		&record.PublishMode,
		&record.PublishDateUTC,
		&record.State,
		&record.ForumChannelID,
		&questionListThreadID,
		&questionListEntryMessageID,
		&threadID,
		&starterMessageID,
		&answerChannelID,
		&record.QuestionTextSnapshot,
		&publishedAt,
		&record.GraceUntil,
		&record.ArchiveAt,
		&closedAt,
		&archivedAt,
		&reconciledAt,
		&record.CreatedAt,
		&record.UpdatedAt,
	); err != nil {
		return nil, err
	}
	record.QuestionListThreadID = strings.TrimSpace(questionListThreadID.String)
	record.QuestionListEntryMessageID = strings.TrimSpace(questionListEntryMessageID.String)
	record.DiscordThreadID = threadID.String
	record.DiscordStarterMessageID = starterMessageID.String
	record.AnswerChannelID = strings.TrimSpace(answerChannelID.String)
	record.PublishedAt = timePtrFromNull(publishedAt)
	record.ClosedAt = timePtrFromNull(closedAt)
	record.ArchivedAt = timePtrFromNull(archivedAt)
	record.LastReconciledAt = timePtrFromNull(reconciledAt)
	record.PublishMode = strings.TrimSpace(record.PublishMode)
	record.PublishDateUTC = normalizeQOTDDateUTC(record.PublishDateUTC)
	record.GraceUntil = record.GraceUntil.UTC()
	record.ArchiveAt = record.ArchiveAt.UTC()
	return &record, nil
}

func scanQOTDCollectedQuestionRecord(scanner qotdRowScanner) (*QOTDCollectedQuestionRecord, error) {
	var record QOTDCollectedQuestionRecord
	var sourceAuthorID sql.NullString
	var sourceAuthorName sql.NullString
	if err := scanner.Scan(
		&record.ID,
		&record.GuildID,
		&record.SourceChannelID,
		&record.SourceMessageID,
		&sourceAuthorID,
		&sourceAuthorName,
		&record.SourceCreatedAt,
		&record.EmbedTitle,
		&record.QuestionText,
		&record.CreatedAt,
		&record.UpdatedAt,
	); err != nil {
		return nil, err
	}
	record.SourceAuthorID = strings.TrimSpace(sourceAuthorID.String)
	record.SourceAuthorNameSnapshot = strings.TrimSpace(sourceAuthorName.String)
	record.SourceCreatedAt = record.SourceCreatedAt.UTC()
	record.CreatedAt = record.CreatedAt.UTC()
	record.UpdatedAt = record.UpdatedAt.UTC()
	return &record, nil
}

func scanQOTDThreadArchiveRecord(scanner qotdRowScanner) (*QOTDThreadArchiveRecord, error) {
	var record QOTDThreadArchiveRecord
	if err := scanner.Scan(
		&record.ID,
		&record.GuildID,
		&record.OfficialPostID,
		&record.SourceKind,
		&record.DiscordThreadID,
		&record.ArchivedAt,
		&record.CreatedAt,
	); err != nil {
		return nil, err
	}
	record.ArchivedAt = record.ArchivedAt.UTC()
	return &record, nil
}

func nullableTime(value *time.Time) any {
	if value == nil || value.IsZero() {
		return nil
	}
	return value.UTC()
}

func nullableJSON(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	return raw
}

func cloneQOTDJSONRawMessage(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	out := make(json.RawMessage, len(raw))
	copy(out, raw)
	return out
}

func timePtrFromNull(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	normalized := value.Time.UTC()
	return &normalized
}

func int64PtrFromNull(value sql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	normalized := value.Int64
	return &normalized
}

func normalizeQOTDDateUTC(value time.Time) time.Time {
	if value.IsZero() {
		return time.Time{}
	}
	utc := value.UTC()
	return time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
}

func normalizeQOTDRequiredTime(value time.Time) time.Time {
	if value.IsZero() {
		return time.Time{}
	}
	return value.UTC()
}

func normalizeQOTDDatePtr(value *time.Time) *time.Time {
	if value == nil || value.IsZero() {
		return nil
	}
	normalized := normalizeQOTDDateUTC(*value)
	return &normalized
}

func normalizeQOTDTimePtr(value *time.Time) *time.Time {
	if value == nil || value.IsZero() {
		return nil
	}
	normalized := value.UTC()
	return &normalized
}

func zeroEmptyString(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}

func maxInt64(left, right int64) int64 {
	if left > right {
		return left
	}
	return right
}
