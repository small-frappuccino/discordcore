//go:build integration

package qotd

import (
	"context"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/files"
	applicationqotd "github.com/small-frappuccino/discordcore/pkg/qotd"
)

func TestQuestionsQueueCommandShowsRealAutomaticStateAfterManualPublish(t *testing.T) {
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
	mustCreateQuestion(t, service, guildID, ownerID, files.LegacyQOTDDefaultDeckID, "Publish me first", applicationqotd.QuestionStatusReady)
	mustCreateQuestion(t, service, guildID, ownerID, files.LegacyQOTDDefaultDeckID, "Publish me next automatically", applicationqotd.QuestionStatusReady)

	router.HandleInteraction(session, newQOTDRootSlashInteraction(guildID, ownerID, publishSubCommandName, nil))
	publishResp := rec.lastResponse(t)
	requirePublicResponse(t, publishResp)
	if !strings.Contains(publishResp.Data.Content, "Published QOTD question ID 1 manually.") {
		t.Fatalf("expected manual publish confirmation before queue inspection, got %q", publishResp.Data.Content)
	}

	router.HandleInteraction(session, newQOTDSlashInteraction(guildID, ownerID, questionsQueueSubCommand, nil))
	resp := rec.lastResponse(t)
	requirePublicResponse(t, resp)
	if !strings.Contains(resp.Data.Content, "Automatic QOTD queue") {
		t.Fatalf("expected automatic queue summary, got %q", resp.Data.Content)
	}
	if !strings.Contains(resp.Data.Content, "Next automatic slot:") {
		t.Fatalf("expected queue command to describe the upcoming automatic slot, got %q", resp.Data.Content)
	}
	if !strings.Contains(resp.Data.Content, "slot already published") {
		t.Fatalf("expected queue command to show the upcoming slot is already occupied, got %q", resp.Data.Content)
	}
	if !strings.Contains(resp.Data.Content, "Next automatic slot question: QOTD question ID 1") {
		t.Fatalf("expected queue command to show which question now occupies the upcoming slot, got %q", resp.Data.Content)
	}
	if !strings.Contains(resp.Data.Content, "After that: QOTD question ID 2") {
		t.Fatalf("expected queue command to point at the remaining ready question after the occupied upcoming slot, got %q", resp.Data.Content)
	}
}

func TestQOTDPublishCommandPublishesManually(t *testing.T) {
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
	mustCreateQuestion(t, service, guildID, ownerID, files.LegacyQOTDDefaultDeckID, "Publish me", applicationqotd.QuestionStatusReady)

	router.HandleInteraction(session, newQOTDRootSlashInteraction(guildID, ownerID, publishSubCommandName, nil))
	resp := rec.lastResponse(t)
	requirePublicResponse(t, resp)
	if !strings.Contains(resp.Data.Content, "Published QOTD question ID 1 manually.") {
		t.Fatalf("expected publish confirmation, got %q", resp.Data.Content)
	}
	if !strings.Contains(resp.Data.Content, "https://discord.com/channels/") {
		t.Fatalf("expected publish response to include jump url, got %q", resp.Data.Content)
	}
	if len(fake.publishedParams) != 1 {
		t.Fatalf("expected fake publisher to be invoked once, got %d", len(fake.publishedParams))
	}
	if fake.publishedParams[0].ThreadName != "Question of the Day" {
		t.Fatalf("expected manual publish to use the fixed thread title, got %+v", fake.publishedParams[0])
	}

	questions, err := service.ListQuestions(context.Background(), guildID, files.LegacyQOTDDefaultDeckID)
	if err != nil {
		t.Fatalf("list questions after manual publish: %v", err)
	}
	if len(questions) != 1 || questions[0].Status != string(applicationqotd.QuestionStatusUsed) || questions[0].UsedAt == nil {
		t.Fatalf("expected manual publish to consume the queue question, got %+v", questions)
	}

	router.HandleInteraction(session, newQOTDSlashInteraction(guildID, ownerID, questionsListSubCommand, nil))
	listResp := rec.lastResponse(t)
	requirePublicResponse(t, listResp)
	if strings.Contains(listResp.Data.Embeds[0].Description, "publishes next") {
		t.Fatalf("expected questions list to remove the manually published question from the automatic queue, got %q", listResp.Data.Embeds[0].Description)
	}
}

func TestQOTDPublishCommandBlocksSecondPublishForCurrentSlot(t *testing.T) {
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
	mustCreateQuestion(t, service, guildID, ownerID, files.LegacyQOTDDefaultDeckID, "Publish me first", applicationqotd.QuestionStatusReady)
	mustCreateQuestion(t, service, guildID, ownerID, files.LegacyQOTDDefaultDeckID, "Publish me today too", applicationqotd.QuestionStatusReady)

	router.HandleInteraction(session, newQOTDRootSlashInteraction(guildID, ownerID, publishSubCommandName, nil))
	firstResp := rec.lastResponse(t)
	requirePublicResponse(t, firstResp)
	if !strings.Contains(firstResp.Data.Content, "Published QOTD question ID 1 manually.") {
		t.Fatalf("expected first manual publish confirmation, got %q", firstResp.Data.Content)
	}

	router.HandleInteraction(session, newQOTDRootSlashInteraction(guildID, ownerID, publishSubCommandName, nil))
	secondResp := rec.lastResponse(t)
	requirePublicResponse(t, secondResp)
	if !strings.Contains(secondResp.Data.Content, "already been published for the current slot") {
		t.Fatalf("expected second manual publish to be blocked for the current slot, got %q", secondResp.Data.Content)
	}
	if strings.Contains(secondResp.Data.Content, "An error occurred while executing the command") {
		t.Fatalf("expected command-specific publish response, got generic fallback %q", secondResp.Data.Content)
	}
	if len(fake.publishedParams) != 1 {
		t.Fatalf("expected only one real publish attempt for the current slot, got %d", len(fake.publishedParams))
	}
	if fake.publishedParams[0].QuestionText != "Publish me first" {
		t.Fatalf("expected the first publish to use the first ready question, got %+v", fake.publishedParams)
	}

	questions, err := service.ListQuestions(context.Background(), guildID, files.LegacyQOTDDefaultDeckID)
	if err != nil {
		t.Fatalf("list questions after second manual publish: %v", err)
	}
	if len(questions) != 2 || questions[0].Status != string(applicationqotd.QuestionStatusUsed) || questions[0].UsedAt == nil || questions[1].Status != string(applicationqotd.QuestionStatusReady) || questions[1].UsedAt != nil {
		t.Fatalf("expected only the first manual publish to consume a question, got %+v", questions)
	}
}

func TestQOTDPublishCommandCanSkipAutomaticSlotConsumptionIntegration(t *testing.T) {
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
	mustCreateQuestion(t, service, guildID, ownerID, files.LegacyQOTDDefaultDeckID, "Manual non-consuming publish", applicationqotd.QuestionStatusReady)
	mustCreateQuestion(t, service, guildID, ownerID, files.LegacyQOTDDefaultDeckID, "Still due automatically", applicationqotd.QuestionStatusReady)

	router.HandleInteraction(session, newQOTDRootSlashInteraction(guildID, ownerID, publishSubCommandName, []*discordgo.ApplicationCommandInteractionDataOption{{
		Name:  publishConsumeAutomaticSlotOptionName,
		Type:  discordgo.ApplicationCommandOptionBoolean,
		Value: false,
	}}))
	resp := rec.lastResponse(t)
	requirePublicResponse(t, resp)
	if !strings.Contains(resp.Data.Content, "without consuming the automatic slot") {
		t.Fatalf("expected non-consuming publish confirmation, got %q", resp.Data.Content)
	}

	queue, err := service.GetAutomaticQueueState(context.Background(), guildID, files.LegacyQOTDDefaultDeckID)
	if err != nil {
		t.Fatalf("GetAutomaticQueueState() failed: %v", err)
	}
	if queue.SlotStatus != applicationqotd.AutomaticQueueSlotStatusWaiting {
		t.Fatalf("expected automatic queue to move to the next upcoming slot, got %+v", queue)
	}
	if queue.SlotOfficialPost != nil {
		t.Fatalf("expected non-consuming publish to leave the automatic slot unoccupied, got %+v", queue)
	}
	if queue.NextReadyQuestion == nil || queue.NextReadyQuestion.Body != "Still due automatically" {
		t.Fatalf("expected next automatic question to advance after manual non-consuming publish, got %+v", queue)
	}
}