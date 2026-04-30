package qotd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	discordqotd "github.com/small-frappuccino/discordcore/pkg/discord/qotd"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/storage"
)

const collectorRecentQuestionLimit = 25

var defaultCollectorTitlePatterns = []string{"Question Of The Day", "question!!"}

type CollectorSummary struct {
	TotalQuestions  int
	RecentQuestions []storage.QOTDCollectedQuestionRecord
}

type CollectorRunResult struct {
	ScannedMessages int
	MatchedMessages int
	NewQuestions    int
	TotalQuestions  int
}

type CollectorRemoveDuplicatesResult struct {
	DeckID             string
	ScannedMessages    int
	MatchedMessages    int
	DuplicateQuestions int
	DeletedQuestions   int
}

type ImportArchivedQuestionsParams struct {
	DeckID          string
	SourceChannelID string
	AuthorIDs       []string
	TitlePatterns   []string
	StartDate       string
	BackupDir       string
}

type ImportArchivedQuestionsResult struct {
	DeckID             string
	ScannedMessages    int
	MatchedMessages    int
	StoredQuestions    int
	ImportedQuestions  int
	DuplicateQuestions int
	DeletedQuestions   int
	BackupPath         string
}

func (s *Service) GetCollectorSummary(ctx context.Context, guildID string) (CollectorSummary, error) {
	if err := s.validate(); err != nil {
		return CollectorSummary{}, err
	}
	guildID = strings.TrimSpace(guildID)
	if guildID == "" {
		return CollectorSummary{}, fmt.Errorf("%w: guild id is required", files.ErrInvalidQOTDInput)
	}

	total, err := s.store.CountQOTDCollectedQuestions(ctx, guildID)
	if err != nil {
		return CollectorSummary{}, err
	}
	recent, err := s.store.ListRecentQOTDCollectedQuestions(ctx, guildID, collectorRecentQuestionLimit)
	if err != nil {
		return CollectorSummary{}, err
	}
	return CollectorSummary{
		TotalQuestions:  total,
		RecentQuestions: recent,
	}, nil
}

func (s *Service) CollectArchivedQuestions(ctx context.Context, guildID string, session *discordgo.Session) (CollectorRunResult, error) {
	if err := s.validate(); err != nil {
		return CollectorRunResult{}, err
	}
	if session == nil {
		return CollectorRunResult{}, ErrDiscordUnavailable
	}

	guildID = strings.TrimSpace(guildID)
	if guildID == "" {
		return CollectorRunResult{}, fmt.Errorf("%w: guild id is required", files.ErrInvalidQOTDInput)
	}

	collector, err := s.collectorConfigForRun(guildID)
	if err != nil {
		return CollectorRunResult{}, err
	}

	matchedRecords, scanned, err := s.collectArchivedQuestionMatches(ctx, guildID, collector, session)
	if err != nil {
		return CollectorRunResult{}, err
	}

	newQuestions, err := s.store.CreateQOTDCollectedQuestions(ctx, matchedRecords)
	if err != nil {
		return CollectorRunResult{}, err
	}
	totalQuestions, err := s.store.CountQOTDCollectedQuestions(ctx, guildID)
	if err != nil {
		return CollectorRunResult{}, err
	}

	return CollectorRunResult{
		ScannedMessages: scanned,
		MatchedMessages: len(matchedRecords),
		NewQuestions:    newQuestions,
		TotalQuestions:  totalQuestions,
	}, nil
}

func (s *Service) RemoveDeckDuplicatesFromCollector(ctx context.Context, guildID, deckID string) (CollectorRemoveDuplicatesResult, error) {
	if err := s.validate(); err != nil {
		return CollectorRemoveDuplicatesResult{}, err
	}

	guildID = strings.TrimSpace(guildID)
	if guildID == "" {
		return CollectorRemoveDuplicatesResult{}, fmt.Errorf("%w: guild id is required", files.ErrInvalidQOTDInput)
	}

	deck, err := s.resolveDashboardDeck(guildID, deckID)
	if err != nil {
		return CollectorRemoveDuplicatesResult{}, err
	}
	matchedRecords, err := s.store.ListAllQOTDCollectedQuestions(ctx, guildID)
	if err != nil {
		return CollectorRemoveDuplicatesResult{}, err
	}

	result := CollectorRemoveDuplicatesResult{
		DeckID:          deck.ID,
		ScannedMessages: len(matchedRecords),
		MatchedMessages: len(matchedRecords),
	}
	if len(matchedRecords) == 0 {
		return result, nil
	}

	questions, err := s.store.ListQOTDQuestions(ctx, guildID, deck.ID)
	if err != nil {
		return CollectorRemoveDuplicatesResult{}, err
	}
	if len(questions) == 0 {
		return result, nil
	}

	matchedText := make(map[string]struct{}, len(matchedRecords))
	for _, record := range matchedRecords {
		key := normalizeCollectorQuestionComparisonText(record.QuestionText)
		if key == "" {
			continue
		}
		matchedText[key] = struct{}{}
	}
	if len(matchedText) == 0 {
		return result, nil
	}

	for _, question := range questions {
		if _, ok := matchedText[normalizeCollectorQuestionComparisonText(question.Body)]; !ok {
			continue
		}
		result.DuplicateQuestions++
		if isImmutableQuestion(question) {
			continue
		}
		if err := s.store.DeleteQOTDQuestion(ctx, guildID, question.ID); err != nil {
			return CollectorRemoveDuplicatesResult{}, err
		}
		result.DeletedQuestions++
	}

	return result, nil
}

func (s *Service) ImportArchivedQuestions(
	ctx context.Context,
	guildID, actorID string,
	session *discordgo.Session,
	params ImportArchivedQuestionsParams,
) (ImportArchivedQuestionsResult, error) {
	if err := s.validate(); err != nil {
		return ImportArchivedQuestionsResult{}, err
	}
	if session == nil {
		return ImportArchivedQuestionsResult{}, ErrDiscordUnavailable
	}

	guildID = strings.TrimSpace(guildID)
	if guildID == "" {
		return ImportArchivedQuestionsResult{}, fmt.Errorf("%w: guild id is required", files.ErrInvalidQOTDInput)
	}

	lifecycleLock := s.guildLifecycleLock(guildID)
	lifecycleLock.Lock()
	defer lifecycleLock.Unlock()

	deck, err := s.resolveDashboardDeck(guildID, params.DeckID)
	if err != nil {
		return ImportArchivedQuestionsResult{}, err
	}
	collector, err := s.collectorConfigForImport(guildID, params)
	if err != nil {
		return ImportArchivedQuestionsResult{}, err
	}

	matchedRecords, scanned, err := s.collectArchivedQuestionMatches(ctx, guildID, collector, session)
	if err != nil {
		return ImportArchivedQuestionsResult{}, err
	}
	orderedRecords := orderedCollectedQuestionRecords(matchedRecords)
	result := ImportArchivedQuestionsResult{
		DeckID:          deck.ID,
		ScannedMessages: scanned,
		MatchedMessages: len(orderedRecords),
	}
	if len(orderedRecords) == 0 {
		return result, nil
	}

	storedQuestions, err := s.store.CreateQOTDCollectedQuestions(ctx, orderedRecords)
	if err != nil {
		return ImportArchivedQuestionsResult{}, err
	}
	result.StoredQuestions = storedQuestions

	backupPath, err := writeCollectedQuestionsBackupTXT(params.BackupDir, guildID, deck.ID, orderedRecords, s.clock())
	if err != nil {
		return ImportArchivedQuestionsResult{}, err
	}
	result.BackupPath = backupPath

	uniqueRecords := uniqueCollectedQuestionRecords(orderedRecords)
	matchedText := make(map[string]struct{}, len(uniqueRecords))
	for _, record := range uniqueRecords {
		key := normalizeCollectorQuestionComparisonText(record.QuestionText)
		if key == "" {
			continue
		}
		matchedText[key] = struct{}{}
	}

	questions, err := s.store.ListQOTDQuestions(ctx, guildID, deck.ID)
	if err != nil {
		return ImportArchivedQuestionsResult{}, err
	}

	existingImmutableText := make(map[string]struct{}, len(matchedText))
	for _, question := range questions {
		key := normalizeCollectorQuestionComparisonText(question.Body)
		if _, ok := matchedText[key]; !ok {
			continue
		}
		result.DuplicateQuestions++
		if isImmutableQuestion(question) {
			existingImmutableText[key] = struct{}{}
			continue
		}
		if err := s.store.DeleteQOTDQuestion(ctx, guildID, question.ID); err != nil {
			return ImportArchivedQuestionsResult{}, err
		}
		result.DeletedQuestions++
	}

	importedIDs := make([]int64, 0, len(uniqueRecords))
	for _, record := range uniqueRecords {
		key := normalizeCollectorQuestionComparisonText(record.QuestionText)
		if key == "" {
			continue
		}
		if _, exists := existingImmutableText[key]; exists {
			continue
		}
		usedAt := record.SourceCreatedAt.UTC()
		created, err := s.store.CreateQOTDQuestion(ctx, storage.QOTDQuestionRecord{
			GuildID:         guildID,
			DeckID:          deck.ID,
			Body:            strings.Join(strings.Fields(record.QuestionText), " "),
			Status:          string(QuestionStatusUsed),
			CreatedBy:       normalizeActorID(actorID),
			UsedAt:          &usedAt,
			PublishedOnceAt: &usedAt,
		})
		if err != nil {
			return ImportArchivedQuestionsResult{}, err
		}
		importedIDs = append(importedIDs, created.ID)
		result.ImportedQuestions++
	}

	if len(importedIDs) > 0 {
		questions, err = s.store.ListQOTDQuestions(ctx, guildID, deck.ID)
		if err != nil {
			return ImportArchivedQuestionsResult{}, err
		}
		orderedIDs := importedQuestionOrder(questions, importedIDs)
		if len(orderedIDs) > 0 {
			if err := s.store.ReorderQOTDQuestions(ctx, guildID, deck.ID, orderedIDs); err != nil {
				return ImportArchivedQuestionsResult{}, err
			}
		}
	}

	return result, nil
}

func (s *Service) ExportCollectedQuestionsTXT(ctx context.Context, guildID string) (string, error) {
	if err := s.validate(); err != nil {
		return "", err
	}
	guildID = strings.TrimSpace(guildID)
	if guildID == "" {
		return "", fmt.Errorf("%w: guild id is required", files.ErrInvalidQOTDInput)
	}

	records, err := s.store.ListAllQOTDCollectedQuestions(ctx, guildID)
	if err != nil {
		return "", err
	}
	return renderCollectedQuestionsTXT(records), nil
}

func (s *Service) collectorConfigForImport(guildID string, params ImportArchivedQuestionsParams) (files.QOTDCollectorConfig, error) {
	collector := files.QOTDCollectorConfig{
		SourceChannelID: strings.TrimSpace(params.SourceChannelID),
		StartDate:       strings.TrimSpace(params.StartDate),
	}
	if collector.SourceChannelID == "" {
		return files.QOTDCollectorConfig{}, fmt.Errorf("%w: collector source_channel_id is required", files.ErrInvalidQOTDInput)
	}
	if !isDigitsOnly(collector.SourceChannelID) {
		return files.QOTDCollectorConfig{}, fmt.Errorf("%w: collector source_channel_id must be numeric", files.ErrInvalidQOTDInput)
	}
	if _, err := collectorStartTime(collector.StartDate); err != nil {
		return files.QOTDCollectorConfig{}, err
	}

	seenAuthorIDs := make(map[string]struct{}, len(params.AuthorIDs))
	for idx, authorID := range params.AuthorIDs {
		authorID = strings.TrimSpace(authorID)
		if authorID == "" {
			continue
		}
		if !isDigitsOnly(authorID) {
			return files.QOTDCollectorConfig{}, fmt.Errorf("%w: collector author_ids[%d] must be numeric", files.ErrInvalidQOTDInput, idx)
		}
		if _, exists := seenAuthorIDs[authorID]; exists {
			continue
		}
		seenAuthorIDs[authorID] = struct{}{}
		collector.AuthorIDs = append(collector.AuthorIDs, authorID)
	}
	if len(collector.AuthorIDs) == 0 {
		return files.QOTDCollectorConfig{}, fmt.Errorf("%w: collector author_ids must include at least one id", files.ErrInvalidQOTDInput)
	}

	titlePatterns := append([]string(nil), params.TitlePatterns...)
	if len(titlePatterns) == 0 {
		settings, err := s.Settings(guildID)
		if err != nil {
			return files.QOTDCollectorConfig{}, err
		}
		titlePatterns = append(titlePatterns, settings.Collector.TitlePatterns...)
	}
	if len(titlePatterns) == 0 {
		titlePatterns = append(titlePatterns, defaultCollectorTitlePatterns...)
	}
	collector.TitlePatterns = normalizeCollectorTitlePatterns(titlePatterns)
	if len(collector.TitlePatterns) == 0 {
		return files.QOTDCollectorConfig{}, fmt.Errorf("%w: collector title_patterns must include at least one pattern", files.ErrInvalidQOTDInput)
	}

	return collector, nil
}

func renderCollectedQuestionsTXT(records []storage.QOTDCollectedQuestionRecord) string {
	if len(records) == 0 {
		return ""
	}

	var builder strings.Builder
	for _, record := range records {
		builder.WriteString(strings.Join(strings.Fields(record.QuestionText), " "))
		builder.WriteByte('\n')
	}
	return builder.String()
}

func orderedCollectedQuestionRecords(records []storage.QOTDCollectedQuestionRecord) []storage.QOTDCollectedQuestionRecord {
	if len(records) == 0 {
		return nil
	}

	ordered := append([]storage.QOTDCollectedQuestionRecord(nil), records...)
	sort.SliceStable(ordered, func(i, j int) bool {
		left := ordered[i].SourceCreatedAt.UTC()
		right := ordered[j].SourceCreatedAt.UTC()
		if !left.Equal(right) {
			return left.Before(right)
		}
		return strings.TrimSpace(ordered[i].SourceMessageID) < strings.TrimSpace(ordered[j].SourceMessageID)
	})
	return ordered
}

func uniqueCollectedQuestionRecords(records []storage.QOTDCollectedQuestionRecord) []storage.QOTDCollectedQuestionRecord {
	if len(records) == 0 {
		return nil
	}

	unique := make([]storage.QOTDCollectedQuestionRecord, 0, len(records))
	seen := make(map[string]struct{}, len(records))
	for _, record := range records {
		key := normalizeCollectorQuestionComparisonText(record.QuestionText)
		if key == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		unique = append(unique, record)
	}
	return unique
}

func importedQuestionOrder(questions []storage.QOTDQuestionRecord, importedIDs []int64) []int64 {
	if len(questions) == 0 {
		return nil
	}

	importedSet := make(map[int64]struct{}, len(importedIDs))
	for _, id := range importedIDs {
		importedSet[id] = struct{}{}
	}

	existingIDs := make([]int64, 0, len(questions))
	firstMutableIndex := -1
	for _, question := range questions {
		if _, imported := importedSet[question.ID]; imported {
			continue
		}
		if firstMutableIndex < 0 && !isImmutableQuestion(question) {
			firstMutableIndex = len(existingIDs)
		}
		existingIDs = append(existingIDs, question.ID)
	}
	if firstMutableIndex < 0 {
		firstMutableIndex = len(existingIDs)
	}

	ordered := make([]int64, 0, len(existingIDs)+len(importedIDs))
	ordered = append(ordered, existingIDs[:firstMutableIndex]...)
	ordered = append(ordered, importedIDs...)
	ordered = append(ordered, existingIDs[firstMutableIndex:]...)
	return ordered
}

func writeCollectedQuestionsBackupTXT(dir, guildID, deckID string, records []storage.QOTDCollectedQuestionRecord, now time.Time) (string, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" || len(records) == 0 {
		return "", nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create qotd backup directory: %w", err)
	}

	filename := fmt.Sprintf(
		"qotd-import-%s-%s-%s.txt",
		sanitizeBackupFileToken(guildID),
		sanitizeBackupFileToken(deckID),
		now.UTC().Format("20060102-150405"),
	)
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, []byte(renderCollectedQuestionsTXT(records)), 0o644); err != nil {
		return "", fmt.Errorf("write qotd backup file: %w", err)
	}
	return path, nil
}

func sanitizeBackupFileToken(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}

	var builder strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == '-', r == '_':
			builder.WriteRune(r)
		default:
			builder.WriteByte('-')
		}
	}

	cleaned := strings.Trim(builder.String(), "-")
	if cleaned == "" {
		return "unknown"
	}
	return cleaned
}

func normalizeCollectorTitlePatterns(patterns []string) []string {
	if len(patterns) == 0 {
		return nil
	}

	normalized := make([]string, 0, len(patterns))
	seen := make(map[string]struct{}, len(patterns))
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		key := strings.ToLower(pattern)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, pattern)
	}
	return normalized
}

func isDigitsOnly(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func (s *Service) collectorConfigForRun(guildID string) (files.QOTDCollectorConfig, error) {
	cfg, err := s.configManager.QOTDConfig(guildID)
	if err != nil {
		return files.QOTDCollectorConfig{}, err
	}

	collector := cfg.Collector
	if strings.TrimSpace(collector.SourceChannelID) == "" {
		return files.QOTDCollectorConfig{}, fmt.Errorf("%w: collector source_channel_id is required", files.ErrInvalidQOTDInput)
	}
	if len(collector.TitlePatterns) == 0 {
		return files.QOTDCollectorConfig{}, fmt.Errorf("%w: collector title_patterns must include at least one pattern", files.ErrInvalidQOTDInput)
	}
	return collector, nil
}

func (s *Service) collectArchivedQuestionMatches(ctx context.Context, guildID string, collector files.QOTDCollectorConfig, session *discordgo.Session) ([]storage.QOTDCollectedQuestionRecord, int, error) {
	startTime, err := collectorStartTime(collector.StartDate)
	if err != nil {
		return nil, 0, err
	}

	matchedRecords := make([]storage.QOTDCollectedQuestionRecord, 0, 32)
	scanned := 0
	beforeMessageID := ""
	stop := false

	for {
		page, err := s.publisher.FetchChannelMessages(ctx, session, collector.SourceChannelID, beforeMessageID, 100)
		if err != nil {
			return nil, 0, err
		}
		if len(page) == 0 {
			break
		}

		scanned += len(page)
		for _, message := range page {
			if !startTime.IsZero() && message.CreatedAt.UTC().Before(startTime) {
				stop = true
				break
			}
			if !collectorAllowsAuthor(message, collector.AuthorIDs) {
				continue
			}
			record, ok := extractCollectedQuestion(guildID, collector.SourceChannelID, message, collector.TitlePatterns)
			if !ok {
				continue
			}
			matchedRecords = append(matchedRecords, record)
		}

		if stop || len(page) < 100 {
			break
		}
		beforeMessageID = strings.TrimSpace(page[len(page)-1].MessageID)
		if beforeMessageID == "" {
			break
		}
	}

	return matchedRecords, scanned, nil
}

func collectorStartTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, nil
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: collector start_date must be YYYY-MM-DD", files.ErrInvalidQOTDInput)
	}
	return parsed.UTC(), nil
}

func collectorAllowsAuthor(message discordqotd.ArchivedMessage, authorIDs []string) bool {
	if len(authorIDs) == 0 {
		return true
	}
	authorID := strings.TrimSpace(message.AuthorID)
	if authorID == "" {
		return false
	}
	for _, allowedID := range authorIDs {
		if authorID == strings.TrimSpace(allowedID) {
			return true
		}
	}
	return false
}

func extractCollectedQuestion(guildID, sourceChannelID string, message discordqotd.ArchivedMessage, titlePatterns []string) (storage.QOTDCollectedQuestionRecord, bool) {
	sourceMessageID := strings.TrimSpace(message.MessageID)
	if sourceMessageID == "" {
		return storage.QOTDCollectedQuestionRecord{}, false
	}
	sourceCreatedAt := message.CreatedAt.UTC()
	if sourceCreatedAt.IsZero() {
		return storage.QOTDCollectedQuestionRecord{}, false
	}

	embeds := parseCollectorEmbeds(message.EmbedsJSON)
	for _, embed := range embeds {
		title := strings.TrimSpace(embed.Title)
		if !collectorTitleMatches(title, titlePatterns) {
			continue
		}
		questionText := extractCollectedQuestionText(embed.Description)
		if questionText == "" {
			continue
		}
		return storage.QOTDCollectedQuestionRecord{
			GuildID:                  guildID,
			SourceChannelID:          sourceChannelID,
			SourceMessageID:          sourceMessageID,
			SourceAuthorID:           strings.TrimSpace(message.AuthorID),
			SourceAuthorNameSnapshot: strings.TrimSpace(message.AuthorNameSnapshot),
			SourceCreatedAt:          sourceCreatedAt,
			EmbedTitle:               title,
			QuestionText:             questionText,
		}, true
	}
	return storage.QOTDCollectedQuestionRecord{}, false
}

func collectorTitleMatches(title string, patterns []string) bool {
	normalizedTitle := strings.ToLower(strings.TrimSpace(title))
	if normalizedTitle == "" {
		return false
	}
	for _, pattern := range patterns {
		if strings.Contains(normalizedTitle, strings.ToLower(strings.TrimSpace(pattern))) {
			return true
		}
	}
	return false
}

func extractCollectedQuestionText(description string) string {
	for _, line := range strings.Split(description, "\n") {
		normalized := strings.Join(strings.Fields(strings.TrimSpace(line)), " ")
		if normalized == "" {
			continue
		}
		return normalized
	}
	return ""
}

func normalizeCollectorQuestionComparisonText(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(value)), " "))
}

func parseCollectorEmbeds(raw json.RawMessage) []collectorEmbed {
	if len(raw) == 0 {
		return nil
	}
	var embeds []collectorEmbed
	if err := json.Unmarshal(raw, &embeds); err != nil {
		return nil
	}
	return embeds
}

type collectorEmbed struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}
