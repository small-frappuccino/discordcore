//go:build integration

package qotd

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/files"
	applicationqotd "github.com/small-frappuccino/discordcore/pkg/qotd"
)

func TestQuestionsListCommandUsesRequestedDeck(t *testing.T) {
	const (
		guildID = "guild-1"
		ownerID = "owner-1"
	)

	session, rec := newQOTDCommandTestSession(t)
	router, cm, service, _ := newIntegrationQOTDCommandTestRouter(t, session, guildID, ownerID)
	mustConfigureQOTDDecks(t, cm, guildID, files.QOTDConfig{
		ActiveDeckID: files.LegacyQOTDDefaultDeckID,
		Decks: []files.QOTDDeckConfig{
			{ID: files.LegacyQOTDDefaultDeckID, Name: files.LegacyQOTDDefaultDeckName},
			{ID: "spicy", Name: "Spicy"},
		},
	})
	mustCreateQuestion(t, service, guildID, ownerID, files.LegacyQOTDDefaultDeckID, "Default deck question", applicationqotd.QuestionStatusReady)
	mustCreateQuestion(t, service, guildID, ownerID, "spicy", "Spicy deck question", applicationqotd.QuestionStatusDraft)

	router.HandleInteraction(session, newQOTDSlashInteraction(guildID, ownerID, questionsListSubCommand, []*discordgo.ApplicationCommandInteractionDataOption{
		qotdStringOpt(questionsDeckOptionName, "Spicy"),
	}))

	resp := rec.lastResponse(t)
	requirePublicResponse(t, resp)
	if len(resp.Data.Embeds) != 1 {
		t.Fatalf("expected one embed, got %+v", resp.Data.Embeds)
	}
	embed := resp.Data.Embeds[0]
	if embed.Title != "☆ questions list! ☆" {
		t.Fatalf("unexpected embed title: %q", embed.Title)
	}
	if !strings.Contains(embed.Description, "Spicy deck question") {
		t.Fatalf("expected selected deck question in description, got %q", embed.Description)
	}
	if !strings.Contains(embed.Description, "ID:") {
		t.Fatalf("expected question ID in embed description, got %q", embed.Description)
	}
	if strings.Contains(embed.Description, "Default deck question") {
		t.Fatalf("expected response to exclude active deck question, got %q", embed.Description)
	}
	if embed.Footer == nil || !strings.Contains(embed.Footer.Text, "Spicy") {
		t.Fatalf("expected spicy deck footer, got %+v", embed.Footer)
	}
}

func TestQuestionsListCommandPaginatesWithButtons(t *testing.T) {
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
	for idx := 1; idx <= 12; idx++ {
		mustCreateQuestion(t, service, guildID, ownerID, files.LegacyQOTDDefaultDeckID, fmt.Sprintf("Question number %02d", idx), applicationqotd.QuestionStatusReady)
	}

	router.HandleInteraction(session, newQOTDSlashInteraction(guildID, ownerID, questionsListSubCommand, nil))
	firstResp := rec.lastResponse(t)
	requirePublicResponse(t, firstResp)
	if !strings.Contains(firstResp.Data.Embeds[0].Description, "Question number 01") {
		t.Fatalf("expected first page to contain first question, got %q", firstResp.Data.Embeds[0].Description)
	}
	if !strings.Contains(firstResp.Data.Embeds[0].Description, "ID:") {
		t.Fatalf("expected first page to include question IDs, got %q", firstResp.Data.Embeds[0].Description)
	}
	if strings.Contains(firstResp.Data.Embeds[0].Description, "Question number 11") {
		t.Fatalf("expected first page to exclude second page content, got %q", firstResp.Data.Embeds[0].Description)
	}

	nextCustomID := encodeQuestionsListState(questionsListRouteNext, questionsListState{
		UserID: ownerID,
		DeckID: files.LegacyQOTDDefaultDeckID,
		Page:   0,
	})
	router.HandleInteraction(session, newQOTDComponentInteraction(guildID, ownerID, nextCustomID))

	secondResp := rec.lastResponse(t)
	if secondResp.Type != discordgo.InteractionResponseUpdateMessage {
		t.Fatalf("expected update message response, got %+v", secondResp.Type)
	}
	if !strings.Contains(secondResp.Data.Embeds[0].Description, "Question number 11") {
		t.Fatalf("expected second page to contain later questions, got %q", secondResp.Data.Embeds[0].Description)
	}
	if strings.Contains(secondResp.Data.Embeds[0].Description, "Question number 01") {
		t.Fatalf("expected second page to exclude first page content, got %q", secondResp.Data.Embeds[0].Description)
	}
	if secondResp.Data.Embeds[0].Footer == nil || !strings.Contains(secondResp.Data.Embeds[0].Footer.Text, "Page 2/2") {
		t.Fatalf("expected second page footer, got %+v", secondResp.Data.Embeds[0].Footer)
	}
}

func TestQuestionsListComponentRejectsDifferentUser(t *testing.T) {
	const (
		guildID = "guild-1"
		ownerID = "owner-1"
		otherID = "other-user"
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
	mustCreateQuestion(t, service, guildID, ownerID, files.LegacyQOTDDefaultDeckID, "Question number 01", applicationqotd.QuestionStatusReady)

	router.HandleInteraction(session, newQOTDComponentInteraction(guildID, otherID, encodeQuestionsListState(questionsListRouteNext, questionsListState{
		UserID: ownerID,
		DeckID: files.LegacyQOTDDefaultDeckID,
		Page:   0,
	})))

	resp := rec.lastResponse(t)
	requireEphemeralResponse(t, resp)
	if !strings.Contains(resp.Data.Content, questionsListDeniedText) {
		t.Fatalf("expected denied interaction message, got %q", resp.Data.Content)
	}
}

func TestQuestionsAddCommandCreatesQuestionWithVisibleID(t *testing.T) {
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

	router.HandleInteraction(session, newQOTDSlashInteraction(guildID, ownerID, questionsAddSubCommand, []*discordgo.ApplicationCommandInteractionDataOption{
		qotdStringOpt(questionsBodyOptionName, "What is your favorite snack?"),
	}))

	resp := rec.lastResponse(t)
	requirePublicResponse(t, resp)
	if !strings.Contains(resp.Data.Content, "Added QOTD question ID") {
		t.Fatalf("expected add confirmation with ID, got %q", resp.Data.Content)
	}

	questions, err := service.ListQuestions(context.Background(), guildID, files.LegacyQOTDDefaultDeckID)
	if err != nil {
		t.Fatalf("list questions after add: %v", err)
	}
	if len(questions) != 1 {
		t.Fatalf("expected one question after add, got %d", len(questions))
	}
	if questions[0].Body != "What is your favorite snack?" {
		t.Fatalf("unexpected added question: %+v", questions[0])
	}
	if questions[0].DisplayID != 1 {
		t.Fatalf("expected added question to receive visible ID 1, got %+v", questions[0])
	}

	router.HandleInteraction(session, newQOTDSlashInteraction(guildID, ownerID, questionsListSubCommand, nil))
	listResp := rec.lastResponse(t)
	requirePublicResponse(t, listResp)
	if !strings.Contains(listResp.Data.Embeds[0].Description, fmt.Sprintf("ID:%d", questions[0].DisplayID)) {
		t.Fatalf("expected list embed to expose created question ID, got %q", listResp.Data.Embeds[0].Description)
	}
}