package qotd

import (
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/storage"
)

type QuestionsListEmbedParams struct {
	Locale         discordgo.Locale
	DeckName       string
	Questions      []storage.QOTDQuestionRecord
	Page           int
	PageSize       int
	TotalQuestions int
}

func BuildQuestionsListEmbed(params QuestionsListEmbedParams) *discordgo.MessageEmbed {
	locale := params.Locale
	if locale == discordgo.Unknown {
		locale = discordgo.EnglishUS
	}
	pageSize := params.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	totalQuestions := params.TotalQuestions
	if totalQuestions < 0 {
		totalQuestions = len(params.Questions)
	}
	totalPages := questionListPageCount(totalQuestions, pageSize)
	page := normalizeQuestionListPage(params.Page, totalPages)
	deckName := strings.TrimSpace(params.DeckName)
	if deckName == "" {
		deckName = te(locale, embedMsgDeckDefault)
	}

	description := buildQuestionsListDescription(locale, params.Questions, page, pageSize, totalQuestions, totalPages)
	return &discordgo.MessageEmbed{
		Title:       te(locale, embedMsgTitle),
		Description: description,
		Color:       officialQuestionEmbedColor,
		Footer: &discordgo.MessageEmbedFooter{
			Text: te(locale, embedMsgFooter, deckName, page+1, totalPages, totalQuestions),
		},
	}
}

func buildQuestionsListDescription(
	locale discordgo.Locale,
	questions []storage.QOTDQuestionRecord,
	page int,
	pageSize int,
	totalQuestions int,
	totalPages int,
) string {
	if totalQuestions == 0 {
		return te(locale, embedMsgEmpty)
	}

	nextReadyID := nextReadyQuestionID(questions)

	start := page * pageSize
	if start > len(questions) {
		start = len(questions)
	}
	end := start + pageSize
	if end > len(questions) {
		end = len(questions)
	}

	lines := make([]string, 0, end-start+2)
	for _, question := range questions[start:end] {
		lines = append(lines, formatQuestionsListEntry(locale, question, nextReadyID))
	}
	lines = append(lines, "")
	lines = append(lines, te(locale, embedMsgPageInfo, page+1, totalPages, totalQuestions))
	return strings.Join(lines, "\n")
}

func formatQuestionsListEntry(locale discordgo.Locale, question storage.QOTDQuestionRecord, nextReadyID int64) string {
	text := strings.Join(strings.Fields(strings.TrimSpace(question.Body)), " ")
	text = truncateEmbedText(text, 96)
	meta := make([]string, 0, 3)
	displayID := question.DisplayID
	if displayID <= 0 {
		displayID = question.ID
	}
	meta = append(meta, fmt.Sprintf("ID:%d", displayID))
	meta = append(meta, questionStatusLabel(locale, question.Status))
	if question.ID == nextReadyID {
		meta = append(meta, te(locale, embedMsgPublishNext))
	}
	return fmt.Sprintf("%s \"%s\" (%s)", questionStatusIcon(question.Status), text, strings.Join(meta, " • "))
}

func nextReadyQuestionID(questions []storage.QOTDQuestionRecord) int64 {
	for _, question := range questions {
		if canQuestionPublishNext(question) {
			return question.ID
		}
	}
	return 0
}

func canQuestionPublishNext(question storage.QOTDQuestionRecord) bool {
	if strings.TrimSpace(question.Status) != "ready" {
		return false
	}
	if question.PublishedOnceAt != nil && !question.PublishedOnceAt.IsZero() {
		return false
	}
	if question.ScheduledForDateUTC != nil && !question.ScheduledForDateUTC.IsZero() {
		return false
	}
	return true
}

func questionStatusIcon(status string) string {
	switch strings.TrimSpace(status) {
	case "ready":
		return "✅"
	case "draft":
		return "📝"
	case "reserved":
		return "📌"
	case "used":
		return "🚫"
	case "disabled":
		return "⏸️"
	default:
		return "❔"
	}
}

func questionStatusLabel(locale discordgo.Locale, status string) string {
	switch strings.TrimSpace(status) {
	case "ready":
		return te(locale, embedMsgStatusReady)
	case "draft":
		return te(locale, embedMsgStatusDraft)
	case "reserved":
		return te(locale, embedMsgStatusReserved)
	case "used":
		return te(locale, embedMsgStatusUsed)
	case "disabled":
		return te(locale, embedMsgStatusDisabled)
	default:
		return te(locale, embedMsgStatusUnknown)
	}
}

func questionListPageCount(totalQuestions int, pageSize int) int {
	if pageSize <= 0 {
		pageSize = 10
	}
	if totalQuestions <= 0 {
		return 1
	}
	pages := totalQuestions / pageSize
	if totalQuestions%pageSize != 0 {
		pages++
	}
	if pages <= 0 {
		return 1
	}
	return pages
}

func normalizeQuestionListPage(page int, totalPages int) int {
	if totalPages <= 0 {
		return 0
	}
	if page < 0 {
		return 0
	}
	if page >= totalPages {
		return totalPages - 1
	}
	return page
}