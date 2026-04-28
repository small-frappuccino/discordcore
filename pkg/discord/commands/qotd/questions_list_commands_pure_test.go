package qotd

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	discordqotd "github.com/small-frappuccino/discordcore/pkg/discord/qotd"
	"github.com/small-frappuccino/discordcore/pkg/files"
	applicationqotd "github.com/small-frappuccino/discordcore/pkg/qotd"
	"github.com/small-frappuccino/discordcore/pkg/storage"
)

func TestQuestionsListPaginationStillUpdatesAfterUnderlyingStateChanges(t *testing.T) {
	const (
		guildID = "guild-1"
		ownerID = "owner-1"
	)

	buildView := func(firstStatus string, firstUsedAt *time.Time) []storage.QOTDQuestionRecord {
		questions := make([]storage.QOTDQuestionRecord, 0, 12)
		for idx := 1; idx <= 12; idx++ {
			status := string(applicationqotd.QuestionStatusReady)
			usedAt := (*time.Time)(nil)
			if idx == 1 {
				status = firstStatus
				usedAt = firstUsedAt
			}
			questions = append(questions, storage.QOTDQuestionRecord{
				ID:            int64(idx),
				DisplayID:     int64(idx),
				GuildID:       guildID,
				DeckID:        files.LegacyQOTDDefaultDeckID,
				Body:          fmt.Sprintf("Question %02d", idx),
				Status:        status,
				QueuePosition: int64(idx),
				UsedAt:        usedAt,
			})
		}
		return questions
	}

	usedAt := time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC)
	service := &listCommandStubService{
		settings: files.QOTDConfig{
			ActiveDeckID: files.LegacyQOTDDefaultDeckID,
			Decks: []files.QOTDDeckConfig{{
				ID:   files.LegacyQOTDDefaultDeckID,
				Name: files.LegacyQOTDDefaultDeckName,
			}},
		},
		views: [][]storage.QOTDQuestionRecord{
			buildView(string(applicationqotd.QuestionStatusReady), nil),
			buildView(string(applicationqotd.QuestionStatusUsed), &usedAt),
			buildView(string(applicationqotd.QuestionStatusUsed), &usedAt),
		},
	}

	session, rec := newQOTDCommandTestSession(t)
	router, _ := newQOTDCommandTestRouterWithService(t, session, guildID, ownerID, service)

	router.HandleInteraction(session, newQOTDSlashInteraction(guildID, ownerID, questionsListSubCommand, nil))
	initialResp := rec.lastResponse(t)
	requirePublicResponse(t, initialResp)
	if !strings.Contains(initialResp.Data.Embeds[0].Description, "ID:1 • ready • publishes next") {
		t.Fatalf("expected initial list to show question 1 as the next ready question, got %q", initialResp.Data.Embeds[0].Description)
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
		t.Fatalf("expected updated first page to show question 1 as used, got %q", prevResp.Data.Embeds[0].Description)
	}
	if !strings.Contains(prevResp.Data.Embeds[0].Description, "ID:2 • ready • publishes next") {
		t.Fatalf("expected updated first page to move the queue to question 2, got %q", prevResp.Data.Embeds[0].Description)
	}
}

func TestQuestionsListIdleTimeoutResetsOnActivity(t *testing.T) {
	fired := make(chan struct{}, 2)
	command := &questionsListCommand{
		idleTimeout: 80 * time.Millisecond,
		editComponents: func(_ *discordgo.Session, channelID, messageID string, components []discordgo.MessageComponent) error {
			if channelID != "channel-1" || messageID != "message-1" {
				t.Fatalf("unexpected message target: channel=%q message=%q", channelID, messageID)
			}
			if len(components) != 0 {
				t.Fatalf("expected controls to be cleared, got %+v", components)
			}
			fired <- struct{}{}
			return nil
		},
	}

	command.armQuestionsListIdleTimeout(&discordgo.Session{}, "channel-1", "message-1")
	time.Sleep(40 * time.Millisecond)
	command.armQuestionsListIdleTimeout(&discordgo.Session{}, "channel-1", "message-1")

	select {
	case <-fired:
		t.Fatal("expected renewed activity to keep controls visible before the new timeout expires")
	case <-time.After(55 * time.Millisecond):
	}

	select {
	case <-fired:
	case <-time.After(400 * time.Millisecond):
		t.Fatal("expected idle timeout to hide controls after inactivity")
	}

	select {
	case <-fired:
		t.Fatal("expected controls to be hidden only once for the same message")
	case <-time.After(40 * time.Millisecond):
	}
}

func TestDescribeResetDeckResultMentionsCurrentSlotPause(t *testing.T) {
	t.Parallel()

	message := describeResetDeckResult(applicationqotd.ResetDeckResult{
		OfficialPostsCleared:                  1,
		SuppressedCurrentSlotAutomaticPublish: true,
	}, "Default")

	if !strings.Contains(message, "Automatic publishing for the current slot is paused until you publish manually.") {
		t.Fatalf("expected reset description to mention the temporary publish pause, got %q", message)
	}
	if !strings.Contains(message, "cleared 1 QOTD publish record") {
		t.Fatalf("expected reset description to preserve the reset summary, got %q", message)
	}
}

func TestFormatAutomaticQueueStateUsesCurrentSlotLabel(t *testing.T) {
	message := formatAutomaticQueueState(applicationqotd.AutomaticQueueState{
		Deck: files.QOTDDeckConfig{
			ID:        files.LegacyQOTDDefaultDeckID,
			Name:      files.LegacyQOTDDefaultDeckName,
			Enabled:   true,
			ChannelID: "channel-123",
		},
		ScheduleConfigured: true,
		Schedule:           dueQOTDCommandSchedule(),
		CanPublish:         true,
		SlotPublishAtUTC:   time.Date(2026, 4, 2, 12, 43, 0, 0, time.UTC),
		SlotStatus:         applicationqotd.AutomaticQueueSlotStatusDue,
	})

	if !strings.Contains(message, "Current automatic slot:") {
		t.Fatalf("expected queue formatter to describe the active slot generically, got %q", message)
	}
	if strings.Contains(message, "Today's automatic slot:") {
		t.Fatalf("expected queue formatter to avoid claiming the active slot is always today's, got %q", message)
	}
}

func TestQOTDPublishCommandTreatsRecoveredPublishedResultAsSuccess(t *testing.T) {
	const (
		guildID = "guild-1"
		ownerID = "owner-1"
	)

	publishedAt := time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC)
	service := &publishCommandStubService{
		settings: files.QOTDConfig{
			ActiveDeckID: files.LegacyQOTDDefaultDeckID,
			Decks: []files.QOTDDeckConfig{{
				ID:        files.LegacyQOTDDefaultDeckID,
				Name:      files.LegacyQOTDDefaultDeckName,
				Enabled:   true,
				ChannelID: "channel-123",
			}},
		},
		publishResult: &applicationqotd.PublishResult{
			Question: storage.QOTDQuestionRecord{
				ID:        17,
				DisplayID: 17,
				GuildID:   guildID,
				DeckID:    files.LegacyQOTDDefaultDeckID,
				Body:      "Recovered publish",
				Status:    string(applicationqotd.QuestionStatusUsed),
				UsedAt:    &publishedAt,
			},
			OfficialPost: storage.QOTDOfficialPostRecord{
				ID:                      99,
				GuildID:                 guildID,
				DeckID:                  files.LegacyQOTDDefaultDeckID,
				DeckNameSnapshot:        files.LegacyQOTDDefaultDeckName,
				QuestionID:              17,
				PublishMode:             string(applicationqotd.PublishModeManual),
				PublishDateUTC:          time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC),
				ChannelID:               "channel-123",
				DiscordStarterMessageID: "message-99",
				PublishedAt:             &publishedAt,
			},
			PostURL: discordqotd.BuildMessageJumpURL(guildID, "channel-123", "message-99"),
		},
	}

	session, rec := newQOTDCommandTestSession(t)
	router, _ := newQOTDCommandTestRouterWithService(t, session, guildID, ownerID, service)

	router.HandleInteraction(session, newQOTDRootSlashInteraction(guildID, ownerID, publishSubCommandName, nil))
	resp := rec.lastResponse(t)
	requirePublicResponse(t, resp)
	if !strings.Contains(resp.Data.Content, "Published QOTD question ID 17 manually.") {
		t.Fatalf("expected recovered publish to surface as success, got %q", resp.Data.Content)
	}
	if !strings.Contains(resp.Data.Content, "https://discord.com/channels/guild-1/channel-123/message-99") {
		t.Fatalf("expected recovered publish to include the existing jump url, got %q", resp.Data.Content)
	}
	if strings.Contains(resp.Data.Content, "An error occurred while executing the command") {
		t.Fatalf("expected recovered publish to avoid generic fallback errors, got %q", resp.Data.Content)
	}
	if service.publishCalls != 1 {
		t.Fatalf("expected publish command to call PublishNow once, got %d", service.publishCalls)
	}
	if service.lastPublishGuild != guildID || service.lastPublishSession != session {
		t.Fatalf("expected publish command to forward guild and session, got guild=%q session=%p", service.lastPublishGuild, service.lastPublishSession)
	}
}
