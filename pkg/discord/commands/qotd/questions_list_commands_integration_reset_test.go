//go:build integration

package qotd

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/files"
	applicationqotd "github.com/small-frappuccino/discordcore/pkg/qotd"
	"github.com/small-frappuccino/discordcore/pkg/storage"
)

func TestQuestionsResetCommandResetsDeckStateAndPreservesOrder(t *testing.T) {
	const (
		guildID = "guild-1"
		ownerID = "owner-1"
	)

	session, rec := newQOTDCommandTestSession(t)
	router, cm, service, store := newIntegrationQOTDCommandTestRouter(t, session, guildID, ownerID)
	mustConfigureQOTDDecks(t, cm, guildID, files.QOTDConfig{
		ActiveDeckID: files.LegacyQOTDDefaultDeckID,
		Schedule:     dueQOTDCommandSchedule(),
		Decks: []files.QOTDDeckConfig{{
			ID:        files.LegacyQOTDDefaultDeckID,
			Name:      files.LegacyQOTDDefaultDeckName,
			Enabled:   true,
			ChannelID: integrationDeckChannelID,
		}},
	})
	mustCreateQuestion(t, service, guildID, ownerID, files.LegacyQOTDDefaultDeckID, "Question 1", applicationqotd.QuestionStatusReady)
	mustCreateQuestion(t, service, guildID, ownerID, files.LegacyQOTDDefaultDeckID, "Question 2", applicationqotd.QuestionStatusReady)
	mustCreateQuestion(t, service, guildID, ownerID, files.LegacyQOTDDefaultDeckID, "Question 3", applicationqotd.QuestionStatusReady)

	questions, err := service.ListQuestions(context.Background(), guildID, files.LegacyQOTDDefaultDeckID)
	if err != nil {
		t.Fatalf("ListQuestions() failed: %v", err)
	}
	if len(questions) != 3 {
		t.Fatalf("expected three questions before reset, got %+v", questions)
	}
	if err := store.ReorderQOTDQuestions(context.Background(), guildID, files.LegacyQOTDDefaultDeckID, []int64{questions[2].ID, questions[0].ID, questions[1].ID}); err != nil {
		t.Fatalf("ReorderQOTDQuestions() failed: %v", err)
	}
	questions, err = service.ListQuestions(context.Background(), guildID, files.LegacyQOTDDefaultDeckID)
	if err != nil {
		t.Fatalf("ListQuestions(after reorder) failed: %v", err)
	}
	publishDate := time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC)
	usedAt := time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC)
	questions[0].Status = string(applicationqotd.QuestionStatusUsed)
	questions[0].UsedAt = &usedAt
	if _, err := store.UpdateQOTDQuestion(context.Background(), questions[0]); err != nil {
		t.Fatalf("UpdateQOTDQuestion(used) failed: %v", err)
	}
	questions[1].Status = string(applicationqotd.QuestionStatusReserved)
	questions[1].ScheduledForDateUTC = &publishDate
	if _, err := store.UpdateQOTDQuestion(context.Background(), questions[1]); err != nil {
		t.Fatalf("UpdateQOTDQuestion(reserved) failed: %v", err)
	}
	official, err := store.CreateQOTDOfficialPostProvisioning(context.Background(), storage.QOTDOfficialPostRecord{
		GuildID:              guildID,
		DeckID:               files.LegacyQOTDDefaultDeckID,
		DeckNameSnapshot:     files.LegacyQOTDDefaultDeckName,
		QuestionID:           questions[0].ID,
		PublishMode:          string(applicationqotd.PublishModeScheduled),
		PublishDateUTC:       publishDate,
		State:                string(applicationqotd.OfficialPostStateCurrent),
		ChannelID:            integrationDeckChannelID,
		QuestionTextSnapshot: questions[0].Body,
		GraceUntil:           time.Date(2026, 4, 4, 12, 43, 0, 0, time.UTC),
		ArchiveAt:            time.Date(2026, 4, 5, 12, 43, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("CreateQOTDOfficialPostProvisioning() failed: %v", err)
	}
	if _, err := store.FinalizeQOTDOfficialPost(context.Background(), official.ID, "questions-list-thread", "questions-list-entry", "thread-1", "message-1", "thread-1", usedAt); err != nil {
		t.Fatalf("FinalizeQOTDOfficialPost() failed: %v", err)
	}
	if _, err := store.UpsertQOTDSurface(context.Background(), storage.QOTDSurfaceRecord{
		GuildID:              guildID,
		DeckID:               files.LegacyQOTDDefaultDeckID,
		ChannelID:            integrationDeckChannelID,
		QuestionListThreadID: "questions-list-thread",
	}); err != nil {
		t.Fatalf("UpsertQOTDSurface() failed: %v", err)
	}

	router.HandleInteraction(session, newQOTDSlashInteraction(guildID, ownerID, questionsResetSubCommand, nil))
	resp := rec.lastResponse(t)
	requirePublicResponse(t, resp)
	if !strings.Contains(resp.Data.Content, "reset 2 QOTD question states") || !strings.Contains(resp.Data.Content, "cleared 1 QOTD publish record") {
		t.Fatalf("expected reset confirmation, got %q", resp.Data.Content)
	}
	if !strings.Contains(resp.Data.Content, "Question order was preserved.") {
		t.Fatalf("expected reset response to mention order preservation, got %q", resp.Data.Content)
	}

	questions, err = service.ListQuestions(context.Background(), guildID, files.LegacyQOTDDefaultDeckID)
	if err != nil {
		t.Fatalf("list questions after reset: %v", err)
	}
	if len(questions) != 3 {
		t.Fatalf("expected three questions after reset, got %+v", questions)
	}
	if questions[0].Body != "Question 3" || questions[1].Body != "Question 1" || questions[2].Body != "Question 2" {
		t.Fatalf("expected reset to preserve the reordered question order, got %+v", questions)
	}
	for _, question := range questions {
		if question.Status != string(applicationqotd.QuestionStatusReady) || question.UsedAt != nil || question.ScheduledForDateUTC != nil {
			t.Fatalf("expected question state to reset while preserving order, got %+v", questions)
		}
	}

	storedOfficial, err := store.GetQOTDOfficialPostByDate(context.Background(), guildID, publishDate)
	if err != nil {
		t.Fatalf("GetQOTDOfficialPostByDate() failed: %v", err)
	}
	if storedOfficial != nil {
		t.Fatalf("expected reset to clear the published slot record, got %+v", storedOfficial)
	}
	surface, err := store.GetQOTDSurfaceByDeck(context.Background(), guildID, files.LegacyQOTDDefaultDeckID)
	if err != nil {
		t.Fatalf("GetQOTDSurfaceByDeck() failed: %v", err)
	}
	if surface != nil {
		t.Fatalf("expected reset to clear the deck surface, got %+v", surface)
	}

	router.HandleInteraction(session, newQOTDSlashInteraction(guildID, ownerID, questionsListSubCommand, nil))
	listResp := rec.lastResponse(t)
	requirePublicResponse(t, listResp)
	if !strings.Contains(listResp.Data.Embeds[0].Description, "Question 3") || !strings.Contains(listResp.Data.Embeds[0].Description, "publishes next") {
		t.Fatalf("expected reset list to keep the reordered first ready question at the top, got %q", listResp.Data.Embeds[0].Description)
	}
}

func TestQuestionsResetAfterManualPublishKeepsCurrentSlotPausedAndListPaginationStillWorks(t *testing.T) {
	const (
		guildID = "guild-1"
		ownerID = "owner-1"
	)

	fake := &fakePublisher{}
	session, rec := newQOTDCommandTestSession(t)
	router, cm, service, _ := newIntegrationQOTDCommandTestRouterWithPublisher(t, session, guildID, ownerID, fake)
	mustConfigureQOTDDecks(t, cm, guildID, files.QOTDConfig{
		ActiveDeckID: files.LegacyQOTDDefaultDeckID,
		Schedule:     dueQOTDCommandSchedule(),
		Decks: []files.QOTDDeckConfig{{
			ID:        files.LegacyQOTDDefaultDeckID,
			Name:      files.LegacyQOTDDefaultDeckName,
			Enabled:   true,
			ChannelID: integrationDeckChannelID,
		}},
	})
	for idx := 1; idx <= 12; idx++ {
		mustCreateQuestion(t, service, guildID, ownerID, files.LegacyQOTDDefaultDeckID, fmt.Sprintf("Question %02d", idx), applicationqotd.QuestionStatusReady)
	}

	router.HandleInteraction(session, newQOTDRootSlashInteraction(guildID, ownerID, publishSubCommandName, nil))
	firstPublishResp := rec.lastResponse(t)
	requirePublicResponse(t, firstPublishResp)
	if !strings.Contains(firstPublishResp.Data.Content, "Published QOTD question ID 1 manually.") {
		t.Fatalf("expected first manual publish confirmation, got %q", firstPublishResp.Data.Content)
	}
	if len(fake.publishedParams) != 1 {
		t.Fatalf("expected first manual publish to invoke the publisher once, got %d", len(fake.publishedParams))
	}

	router.HandleInteraction(session, newQOTDSlashInteraction(guildID, ownerID, questionsResetSubCommand, nil))
	firstResetResp := rec.lastResponse(t)
	requirePublicResponse(t, firstResetResp)
	if !strings.Contains(firstResetResp.Data.Content, "cleared 1 QOTD publish record") {
		t.Fatalf("expected first reset to clear the current-slot publish record, got %q", firstResetResp.Data.Content)
	}
	if !strings.Contains(firstResetResp.Data.Content, "Automatic publishing for this slot remains paused while it is suppressed.") {
		t.Fatalf("expected first reset to pause the current slot after clearing it, got %q", firstResetResp.Data.Content)
	}

	router.HandleInteraction(session, newQOTDSlashInteraction(guildID, ownerID, questionsListSubCommand, nil))
	listResp := rec.lastResponse(t)
	requirePublicResponse(t, listResp)
	if !strings.Contains(listResp.Data.Embeds[0].Description, "Question 01") {
		t.Fatalf("expected questions list to show the first question after reset, got %q", listResp.Data.Embeds[0].Description)
	}
	if !strings.Contains(listResp.Data.Embeds[0].Description, "ID:1 • ready • publishes next") {
		t.Fatalf("expected first reset to restore question 1 as the next ready question, got %q", listResp.Data.Embeds[0].Description)
	}

	router.HandleInteraction(session, newQOTDRootSlashInteraction(guildID, ownerID, publishSubCommandName, nil))
	secondPublishResp := rec.lastResponse(t)
	requirePublicResponse(t, secondPublishResp)
	if !strings.Contains(secondPublishResp.Data.Content, "Published QOTD question ID 1 manually.") {
		t.Fatalf("expected second manual publish to republish question 1 explicitly, got %q", secondPublishResp.Data.Content)
	}
	if len(fake.publishedParams) != 2 {
		t.Fatalf("expected two manual publish attempts before the second reset, got %d", len(fake.publishedParams))
	}

	router.HandleInteraction(session, newQOTDComponentInteraction(guildID, ownerID, encodeQuestionsListState(questionsListRouteNext, questionsListState{
		UserID: ownerID,
		DeckID: files.LegacyQOTDDefaultDeckID,
		Page:   0,
	})))
	nextResp := rec.lastResponse(t)
	if nextResp.Type != discordgo.InteractionResponseUpdateMessage {
		t.Fatalf("expected next-page interaction to update the original list message, got type %v", nextResp.Type)
	}
	if !strings.Contains(nextResp.Data.Embeds[0].Description, "Question 11") {
		t.Fatalf("expected next-page interaction to reach page 2, got %q", nextResp.Data.Embeds[0].Description)
	}

	router.HandleInteraction(session, newQOTDComponentInteraction(guildID, ownerID, encodeQuestionsListState(questionsListRoutePrev, questionsListState{
		UserID: ownerID,
		DeckID: files.LegacyQOTDDefaultDeckID,
		Page:   1,
	})))
	prevResp := rec.lastResponse(t)
	if prevResp.Type != discordgo.InteractionResponseUpdateMessage {
		t.Fatalf("expected previous-page interaction to update the original list message, got type %v", prevResp.Type)
	}
	if !strings.Contains(prevResp.Data.Embeds[0].Description, "ID:1 • used") {
		t.Fatalf("expected pagination after manual publish to show question 1 as used, got %q", prevResp.Data.Embeds[0].Description)
	}
	if !strings.Contains(prevResp.Data.Embeds[0].Description, "ID:2 • ready • publishes next") {
		t.Fatalf("expected pagination after manual publish to move the queue to question 2, got %q", prevResp.Data.Embeds[0].Description)
	}

	router.HandleInteraction(session, newQOTDSlashInteraction(guildID, ownerID, questionsResetSubCommand, nil))
	secondResetResp := rec.lastResponse(t)
	requirePublicResponse(t, secondResetResp)
	if !strings.Contains(secondResetResp.Data.Content, "cleared 1 QOTD publish record") {
		t.Fatalf("expected second reset to clear the republished current-slot record, got %q", secondResetResp.Data.Content)
	}
	if !strings.Contains(secondResetResp.Data.Content, "Automatic publishing for this slot remains paused while it is suppressed.") {
		t.Fatalf("expected second reset to keep the current slot paused, got %q", secondResetResp.Data.Content)
	}
	if len(fake.publishedParams) != 2 {
		t.Fatalf("expected second reset not to trigger a new publish immediately, got %d publish attempts", len(fake.publishedParams))
	}

	published, err := service.PublishScheduledIfDue(context.Background(), guildID, session)
	if err != nil {
		t.Fatalf("PublishScheduledIfDue() after second reset failed: %v", err)
	}
	if published {
		t.Fatal("expected second reset to keep automatic publish paused for the current slot")
	}
	if len(fake.publishedParams) != 2 {
		t.Fatalf("expected paused current slot to prevent extra publish attempts, got %d", len(fake.publishedParams))
	}
}