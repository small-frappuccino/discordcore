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