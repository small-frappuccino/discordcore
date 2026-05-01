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

func TestQuestionsRemoveCommandDeletesByID(t *testing.T) {
	const (
		guildID = "guild-1"
		ownerID = "owner-1"
	)

	session, rec := newQOTDCommandTestSession(t)
	router, cm, service, _ := newIntegrationQOTDCommandTestRouter(t, session, guildID, ownerID)
	mustConfigureQOTDDecks(t, cm, guildID, files.QOTDConfig{
		ActiveDeckID: files.LegacyQOTDDefaultDeckID,
		Decks: []files.QOTDDeckConfig{{
			ID:   files.LegacyQOTDDefaultDeckID,
			Name: files.LegacyQOTDDefaultDeckName,
		}},
	})
	mustCreateQuestion(t, service, guildID, ownerID, files.LegacyQOTDDefaultDeckID, "Question to remove", applicationqotd.QuestionStatusReady)

	questions, err := service.ListQuestions(context.Background(), guildID, files.LegacyQOTDDefaultDeckID)
	if err != nil {
		t.Fatalf("list questions before remove: %v", err)
	}
	if len(questions) != 1 {
		t.Fatalf("expected one question before remove, got %d", len(questions))
	}

	router.HandleInteraction(session, newQOTDSlashInteraction(guildID, ownerID, questionsRemoveSubCommand, []*discordgo.ApplicationCommandInteractionDataOption{
		qotdIntOpt(questionsIDOptionName, questions[0].DisplayID),
	}))

	resp := rec.lastResponse(t)
	requirePublicResponse(t, resp)
	if !strings.Contains(resp.Data.Content, fmt.Sprintf("Removed QOTD question ID %d", questions[0].DisplayID)) {
		t.Fatalf("expected remove confirmation with ID, got %q", resp.Data.Content)
	}

	remaining, err := service.ListQuestions(context.Background(), guildID, files.LegacyQOTDDefaultDeckID)
	if err != nil {
		t.Fatalf("list questions after remove: %v", err)
	}
	if len(remaining) != 0 {
		t.Fatalf("expected question removal to persist, got %+v", remaining)
	}
}

func TestQuestionsNextCommandSetsSelectedQuestionAsNextReady(t *testing.T) {
	const (
		guildID = "guild-1"
		ownerID = "owner-1"
	)

	fake := &fakePublisher{}
	session, rec := newQOTDCommandTestSession(t)
	router, cm, service, store := newIntegrationQOTDCommandTestRouterWithPublisher(t, session, guildID, ownerID, fake)
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

	created := make([]*storage.QOTDQuestionRecord, 0, 6)
	for idx := 1; idx <= 6; idx++ {
		question, err := service.CreateQuestion(context.Background(), guildID, ownerID, applicationqotd.QuestionMutation{
			DeckID: files.LegacyQOTDDefaultDeckID,
			Body:   fmt.Sprintf("Question %02d", idx),
			Status: applicationqotd.QuestionStatusReady,
		})
		if err != nil {
			t.Fatalf("CreateQuestion(%d) failed: %v", idx, err)
		}
		created = append(created, question)
	}

	for idx := 0; idx < 4; idx++ {
		usedAt := time.Date(2026, 4, 3, 13, idx, 0, 0, time.UTC)
		created[idx].Status = string(applicationqotd.QuestionStatusUsed)
		created[idx].UsedAt = &usedAt
		if _, err := store.UpdateQOTDQuestion(context.Background(), *created[idx]); err != nil {
			t.Fatalf("UpdateQOTDQuestion(%d) failed: %v", idx, err)
		}
	}

	questions, err := service.ListQuestions(context.Background(), guildID, files.LegacyQOTDDefaultDeckID)
	if err != nil {
		t.Fatalf("list questions before next command: %v", err)
	}
	selected := created[5]
	if questions[5].ID != selected.ID || questions[5].DisplayID != 6 {
		t.Fatalf("expected selected question to begin at visible ID 6, got %+v", questions)
	}

	router.HandleInteraction(session, newQOTDSlashInteraction(guildID, ownerID, questionsNextSubCommand, []*discordgo.ApplicationCommandInteractionDataOption{
		qotdIntOpt(questionsIDOptionName, 6),
	}))

	resp := rec.lastResponse(t)
	requirePublicResponse(t, resp)
	if !strings.Contains(resp.Data.Content, "QOTD question ID 6 is now the next ready question") {
		t.Fatalf("expected next-question confirmation, got %q", resp.Data.Content)
	}
	if !strings.Contains(resp.Data.Content, "ID 5") {
		t.Fatalf("expected next-question response to mention the new visible ID, got %q", resp.Data.Content)
	}

	updated, err := service.ListQuestions(context.Background(), guildID, files.LegacyQOTDDefaultDeckID)
	if err != nil {
		t.Fatalf("list questions after next command: %v", err)
	}
	if updated[4].ID != selected.ID || updated[4].DisplayID != 5 {
		t.Fatalf("expected selected question to become the next ready slot, got %+v", updated)
	}
	if updated[5].ID != created[4].ID || updated[5].DisplayID != 6 {
		t.Fatalf("expected the previous next question to shift back by one slot, got %+v", updated)
	}

	router.HandleInteraction(session, newQOTDSlashInteraction(guildID, ownerID, questionsListSubCommand, nil))

	listResp := rec.lastResponse(t)
	requirePublicResponse(t, listResp)
	if len(listResp.Data.Embeds) != 1 {
		t.Fatalf("expected one embed after list command, got %+v", listResp.Data.Embeds)
	}
	listDescription := listResp.Data.Embeds[0].Description
	if !strings.Contains(listDescription, "Question 06\" (ID:5 • ready • publishes next)") {
		t.Fatalf("expected reordered question to be marked as publishes next in list, got %q", listDescription)
	}
}

func TestQuestionsNextCommandShowsSpecificErrorForUsedQuestion(t *testing.T) {
	const (
		guildID = "guild-1"
		ownerID = "owner-1"
	)

	fake := &fakePublisher{}
	session, rec := newQOTDCommandTestSession(t)
	router, cm, service, store := newIntegrationQOTDCommandTestRouterWithPublisher(t, session, guildID, ownerID, fake)
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

	mustCreateQuestion(t, service, guildID, ownerID, files.LegacyQOTDDefaultDeckID, "Already used", applicationqotd.QuestionStatusReady)
	questions, err := service.ListQuestions(context.Background(), guildID, files.LegacyQOTDDefaultDeckID)
	if err != nil {
		t.Fatalf("ListQuestions() failed: %v", err)
	}
	if len(questions) != 1 {
		t.Fatalf("expected one question before publish, got %+v", questions)
	}
	created := questions[0]
	usedAt := time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC)
	created.Status = string(applicationqotd.QuestionStatusUsed)
	created.UsedAt = &usedAt
	if _, err := store.UpdateQOTDQuestion(context.Background(), created); err != nil {
		t.Fatalf("UpdateQOTDQuestion() failed: %v", err)
	}

	router.HandleInteraction(session, newQOTDSlashInteraction(guildID, ownerID, questionsNextSubCommand, []*discordgo.ApplicationCommandInteractionDataOption{
		qotdIntOpt(questionsIDOptionName, created.DisplayID),
	}))

	resp := rec.lastResponse(t)
	requirePublicResponse(t, resp)
	if !strings.Contains(resp.Data.Content, fmt.Sprintf("QOTD question ID %d is already scheduled or used and cannot be set as next.", created.DisplayID)) {
		t.Fatalf("expected specific immutable-question error, got %q", resp.Data.Content)
	}
	if strings.Contains(resp.Data.Content, "An error occurred while executing the command") {
		t.Fatalf("expected command-specific error response, got generic fallback %q", resp.Data.Content)
	}
}

func TestQuestionsRecoverCommandMovesUsedQuestionBackToReady(t *testing.T) {
	const (
		guildID = "guild-1"
		ownerID = "owner-1"
	)

	session, rec := newQOTDCommandTestSession(t)
	router, cm, service, store := newIntegrationQOTDCommandTestRouter(t, session, guildID, ownerID)
	mustConfigureQOTDDecks(t, cm, guildID, files.QOTDConfig{
		ActiveDeckID: files.LegacyQOTDDefaultDeckID,
		Decks: []files.QOTDDeckConfig{{
			ID:   files.LegacyQOTDDefaultDeckID,
			Name: files.LegacyQOTDDefaultDeckName,
		}},
	})

	created := make([]*storage.QOTDQuestionRecord, 0, 4)
	for idx := 1; idx <= 4; idx++ {
		question, err := service.CreateQuestion(context.Background(), guildID, ownerID, applicationqotd.QuestionMutation{
			DeckID: files.LegacyQOTDDefaultDeckID,
			Body:   fmt.Sprintf("Question %d", idx),
			Status: applicationqotd.QuestionStatusReady,
		})
		if err != nil {
			t.Fatalf("CreateQuestion(%d) failed: %v", idx, err)
		}
		created = append(created, question)
	}

	for _, idx := range []int{0, 3} {
		usedAt := time.Date(2026, 4, 3, 13, idx, 0, 0, time.UTC)
		created[idx].Status = string(applicationqotd.QuestionStatusUsed)
		created[idx].UsedAt = &usedAt
		if idx == 3 {
			publishedOnceAt := time.Date(2026, 4, 3, 13, idx, 0, 0, time.UTC)
			slotDate := time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC)
			created[idx].PublishedOnceAt = &publishedOnceAt
			created[idx].ScheduledForDateUTC = &slotDate
		}
		if _, err := store.UpdateQOTDQuestion(context.Background(), *created[idx]); err != nil {
			t.Fatalf("UpdateQOTDQuestion(%d) failed: %v", idx, err)
		}
	}

	questions, err := service.ListQuestions(context.Background(), guildID, files.LegacyQOTDDefaultDeckID)
	if err != nil {
		t.Fatalf("ListQuestions(before) failed: %v", err)
	}
	if questions[1].ID != created[1].ID || questions[1].DisplayID != 2 {
		t.Fatalf("expected question 2 to be the next ready question before recover, got %+v", questions)
	}
	if questions[3].ID != created[3].ID || questions[3].DisplayID != 4 {
		t.Fatalf("expected recover target to start at visible ID 4, got %+v", questions)
	}

	router.HandleInteraction(session, newQOTDSlashInteraction(guildID, ownerID, questionsRecoverSubCommand, []*discordgo.ApplicationCommandInteractionDataOption{
		qotdIntOpt(questionsIDOptionName, 4),
	}))

	resp := rec.lastResponse(t)
	requirePublicResponse(t, resp)
	if !strings.Contains(resp.Data.Content, "Recovered QOTD question ID 4 from used to ready") {
		t.Fatalf("expected recover confirmation with ID, got %q", resp.Data.Content)
	}
	if !strings.Contains(resp.Data.Content, "ID 2") {
		t.Fatalf("expected recover confirmation to mention the new visible ID, got %q", resp.Data.Content)
	}

	updated, err := service.ListQuestions(context.Background(), guildID, files.LegacyQOTDDefaultDeckID)
	if err != nil {
		t.Fatalf("ListQuestions() failed: %v", err)
	}
	if len(updated) != 4 {
		t.Fatalf("expected four questions after recover, got %+v", updated)
	}
	if updated[1].ID != created[3].ID || updated[1].DisplayID != 2 {
		t.Fatalf("expected recovered question to move to visible ID 2, got %+v", updated)
	}
	if updated[1].Status != string(applicationqotd.QuestionStatusReady) {
		t.Fatalf("expected recovered question status ready, got %+v", updated[1])
	}
	if updated[1].UsedAt != nil || updated[1].PublishedOnceAt != nil || updated[1].ScheduledForDateUTC != nil {
		t.Fatalf("expected recovered question to clear used/scheduled/published markers, got %+v", updated[1])
	}
	if updated[2].ID != created[1].ID || updated[2].DisplayID != 3 {
		t.Fatalf("expected previous next-ready question to shift back one slot, got %+v", updated)
	}
}

func TestQuestionsRecoverCommandMakesRecoveredQuestionNextWhenItAlreadySitsBeforeReadyQueue(t *testing.T) {
	const (
		guildID = "guild-1"
		ownerID = "owner-1"
	)

	session, rec := newQOTDCommandTestSession(t)
	router, cm, service, store := newIntegrationQOTDCommandTestRouter(t, session, guildID, ownerID)
	mustConfigureQOTDDecks(t, cm, guildID, files.QOTDConfig{
		ActiveDeckID: files.LegacyQOTDDefaultDeckID,
		Decks: []files.QOTDDeckConfig{{
			ID:   files.LegacyQOTDDefaultDeckID,
			Name: files.LegacyQOTDDefaultDeckName,
		}},
	})

	created := make([]*storage.QOTDQuestionRecord, 0, 4)
	for idx := 1; idx <= 4; idx++ {
		question, err := service.CreateQuestion(context.Background(), guildID, ownerID, applicationqotd.QuestionMutation{
			DeckID: files.LegacyQOTDDefaultDeckID,
			Body:   fmt.Sprintf("Question %d", idx),
			Status: applicationqotd.QuestionStatusReady,
		})
		if err != nil {
			t.Fatalf("CreateQuestion(%d) failed: %v", idx, err)
		}
		created = append(created, question)
	}

	for _, idx := range []int{0, 1} {
		usedAt := time.Date(2026, 4, 3, 13, idx, 0, 0, time.UTC)
		created[idx].Status = string(applicationqotd.QuestionStatusUsed)
		created[idx].UsedAt = &usedAt
		if _, err := store.UpdateQOTDQuestion(context.Background(), *created[idx]); err != nil {
			t.Fatalf("UpdateQOTDQuestion(%d) failed: %v", idx, err)
		}
	}

	questions, err := service.ListQuestions(context.Background(), guildID, files.LegacyQOTDDefaultDeckID)
	if err != nil {
		t.Fatalf("ListQuestions(before) failed: %v", err)
	}
	if questions[0].ID != created[0].ID || questions[0].DisplayID != 1 {
		t.Fatalf("expected recover target to start at visible ID 1, got %+v", questions)
	}
	if questions[2].ID != created[2].ID || questions[2].DisplayID != 3 {
		t.Fatalf("expected question 3 to be the next ready question before recover, got %+v", questions)
	}

	router.HandleInteraction(session, newQOTDSlashInteraction(guildID, ownerID, questionsRecoverSubCommand, []*discordgo.ApplicationCommandInteractionDataOption{
		qotdIntOpt(questionsIDOptionName, 1),
	}))

	resp := rec.lastResponse(t)
	requirePublicResponse(t, resp)
	if !strings.Contains(resp.Data.Content, "Recovered QOTD question ID 1 from used to ready") {
		t.Fatalf("expected recover confirmation with ID, got %q", resp.Data.Content)
	}
	if strings.Contains(resp.Data.Content, "listed as ID") {
		t.Fatalf("expected recovered question to keep visible ID 1 when it becomes next, got %q", resp.Data.Content)
	}

	updated, err := service.ListQuestions(context.Background(), guildID, files.LegacyQOTDDefaultDeckID)
	if err != nil {
		t.Fatalf("ListQuestions() failed: %v", err)
	}
	if updated[0].ID != created[0].ID || updated[0].DisplayID != 1 || updated[0].Status != string(applicationqotd.QuestionStatusReady) {
		t.Fatalf("expected recovered question to become the next ready slot at visible ID 1, got %+v", updated)
	}
	if updated[2].ID != created[2].ID || updated[2].DisplayID != 3 {
		t.Fatalf("expected prior next-ready question to remain behind the recovered question, got %+v", updated)
	}

	router.HandleInteraction(session, newQOTDSlashInteraction(guildID, ownerID, questionsListSubCommand, nil))
	listResp := rec.lastResponse(t)
	requirePublicResponse(t, listResp)
	if len(listResp.Data.Embeds) != 1 {
		t.Fatalf("expected one embed after list command, got %+v", listResp.Data.Embeds)
	}
	if !strings.Contains(listResp.Data.Embeds[0].Description, "Question 1\" (ID:1 • ready • publishes next)") {
		t.Fatalf("expected recovered question to be marked as publishes next in list, got %q", listResp.Data.Embeds[0].Description)
	}
}

func TestQuestionsRecoverCommandShowsSpecificErrorForNonUsedQuestion(t *testing.T) {
	const (
		guildID = "guild-1"
		ownerID = "owner-1"
	)

	session, rec := newQOTDCommandTestSession(t)
	router, cm, service, _ := newIntegrationQOTDCommandTestRouter(t, session, guildID, ownerID)
	mustConfigureQOTDDecks(t, cm, guildID, files.QOTDConfig{
		ActiveDeckID: files.LegacyQOTDDefaultDeckID,
		Decks: []files.QOTDDeckConfig{{
			ID:   files.LegacyQOTDDefaultDeckID,
			Name: files.LegacyQOTDDefaultDeckName,
		}},
	})

	created, err := service.CreateQuestion(context.Background(), guildID, ownerID, applicationqotd.QuestionMutation{
		DeckID: files.LegacyQOTDDefaultDeckID,
		Body:   "Still ready",
		Status: applicationqotd.QuestionStatusReady,
	})
	if err != nil {
		t.Fatalf("CreateQuestion() failed: %v", err)
	}

	router.HandleInteraction(session, newQOTDSlashInteraction(guildID, ownerID, questionsRecoverSubCommand, []*discordgo.ApplicationCommandInteractionDataOption{
		qotdIntOpt(questionsIDOptionName, created.DisplayID),
	}))

	resp := rec.lastResponse(t)
	requirePublicResponse(t, resp)
	if !strings.Contains(resp.Data.Content, fmt.Sprintf("QOTD question ID %d is not used and cannot be recovered.", created.DisplayID)) {
		t.Fatalf("expected non-used recover error, got %q", resp.Data.Content)
	}
	if strings.Contains(resp.Data.Content, "An error occurred while executing the command") {
		t.Fatalf("expected command-specific error response, got generic fallback %q", resp.Data.Content)
	}
}