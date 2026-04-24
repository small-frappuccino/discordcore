package qotd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	discordqotd "github.com/small-frappuccino/discordcore/pkg/discord/qotd"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/storage"
)

const collectorRecentQuestionLimit = 25

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
	if len(records) == 0 {
		return "", nil
	}

	var builder strings.Builder
	for _, record := range records {
		builder.WriteString(strings.Join(strings.Fields(record.QuestionText), " "))
		builder.WriteByte('\n')
	}
	return builder.String(), nil
}

func (s *Service) collectorConfigForRun(guildID string) (files.QOTDCollectorConfig, error) {
	cfg, err := s.configManager.GetQOTDConfig(guildID)
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
