package qotd

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	discordqotd "github.com/small-frappuccino/discordcore/pkg/discord/qotd"
	"github.com/small-frappuccino/discordcore/pkg/files"
	applicationqotd "github.com/small-frappuccino/discordcore/pkg/qotd"
	"github.com/small-frappuccino/discordcore/pkg/storage"
)

func TestQOTDCommandsRegisterRoutesUnderQOTDDomain(t *testing.T) {
	session, _ := newQOTDCommandTestSession(t)
	service := &publishCommandStubService{}
	router, _ := newQOTDCommandTestRouterWithService(t, session, "guild-1", "owner-1", service)

	if got := router.InteractionRouteDomain(core.InteractionRouteKey{Kind: core.InteractionKindSlash, Path: "qotd publish"}); got != files.BotDomainQOTD {
		t.Fatalf("expected qotd publish slash route domain, got %q", got)
	}
	if got := router.InteractionRouteDomain(core.InteractionRouteKey{Kind: core.InteractionKindSlash, Path: "qotd questions list"}); got != files.BotDomainQOTD {
		t.Fatalf("expected qotd questions list slash route domain, got %q", got)
	}
	if got := router.InteractionRouteDomain(core.InteractionRouteKey{Kind: core.InteractionKindComponent, Path: questionsListRouteNext}); got != files.BotDomainQOTD {
		t.Fatalf("expected qotd questions component route domain, got %q", got)
	}
}

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

func TestQuestionsListFirstRouteUpdatesExistingMessage(t *testing.T) {
	const (
		guildID = "guild-1"
		ownerID = "owner-1"
	)

	service := &listCommandStubService{
		settings: files.QOTDConfig{
			ActiveDeckID: files.LegacyQOTDDefaultDeckID,
			Decks: []files.QOTDDeckConfig{{
				ID:   files.LegacyQOTDDefaultDeckID,
				Name: files.LegacyQOTDDefaultDeckName,
			}},
		},
	}
	questions := make([]storage.QOTDQuestionRecord, 0, 25)
	for idx := 1; idx <= 25; idx++ {
		questions = append(questions, storage.QOTDQuestionRecord{
			ID:            int64(idx),
			DisplayID:     int64(idx),
			GuildID:       guildID,
			DeckID:        files.LegacyQOTDDefaultDeckID,
			Body:          fmt.Sprintf("Question %02d", idx),
			Status:        string(applicationqotd.QuestionStatusReady),
			QueuePosition: int64(idx),
		})
	}
	service.views = [][]storage.QOTDQuestionRecord{questions}

	session, rec := newQOTDCommandTestSession(t)
	router, _ := newQOTDCommandTestRouterWithService(t, session, guildID, ownerID, service)

	router.HandleInteraction(session, newQOTDComponentInteraction(guildID, ownerID, encodeQuestionsListState(questionsListRouteFirst, questionsListState{
		UserID: ownerID,
		DeckID: files.LegacyQOTDDefaultDeckID,
		Page:   2,
	})))

	resp := rec.lastResponse(t)
	if resp.Type != discordgo.InteractionResponseUpdateMessage {
		t.Fatalf("expected << interaction to update the existing message, got type %v", resp.Type)
	}
	if !strings.Contains(resp.Data.Embeds[0].Description, "Question 01") {
		t.Fatalf("expected << to jump back to the first page from page 3, got %q", resp.Data.Embeds[0].Description)
	}
}

func TestNextQuestionsListPageJumpsTenPagesForDoubleArrows(t *testing.T) {
	const totalPages = 78

	if got := nextQuestionsListPage(questionsListRouteLast, 33, totalPages); got != 43 {
		t.Fatalf("expected >> to jump forward 10 pages from 34 to 44, got page index %d", got)
	}
	if got := nextQuestionsListPage(questionsListRouteFirst, 33, totalPages); got != 23 {
		t.Fatalf("expected << to jump back 10 pages from 34 to 24, got page index %d", got)
	}
	if got := nextQuestionsListPage(questionsListRouteLast, 72, totalPages); got != 77 {
		t.Fatalf("expected >> to clamp at the last page, got page index %d", got)
	}
	if got := nextQuestionsListPage(questionsListRouteFirst, 4, totalPages); got != 0 {
		t.Fatalf("expected << to clamp at the first page, got page index %d", got)
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

func TestQuestionsImportCommandParsesIDsAndReportsSummary(t *testing.T) {
	const (
		guildID = "guild-1"
		ownerID = "owner-1"
	)

	service := &importCommandStubService{
		settings: files.QOTDConfig{
			ActiveDeckID: files.LegacyQOTDDefaultDeckID,
			Decks: []files.QOTDDeckConfig{{
				ID:   files.LegacyQOTDDefaultDeckID,
				Name: files.LegacyQOTDDefaultDeckName,
			}},
		},
		importResult: applicationqotd.ImportArchivedQuestionsResult{
			DeckID:             files.LegacyQOTDDefaultDeckID,
			ScannedMessages:    42,
			MatchedMessages:    12,
			StoredQuestions:    12,
			ImportedQuestions:  10,
			DuplicateQuestions: 2,
			DeletedQuestions:   2,
			BackupPath:         filepath.Join("backups", "qotd-imports", "qotd-import-guild-1-default-20260429-120000.txt"),
		},
	}

	session, rec := newQOTDCommandTestSession(t)
	router, _ := newQOTDCommandTestRouterWithService(t, session, guildID, ownerID, service)

	router.HandleInteraction(session, newQOTDSlashInteraction(guildID, ownerID, questionsImportSubCommand, []*discordgo.ApplicationCommandInteractionDataOption{
		{Name: questionsImportUsersName, Type: discordgo.ApplicationCommandOptionString, Value: "123456789012345678, <@!987654321098765432>"},
		{Name: questionsImportChannel, Type: discordgo.ApplicationCommandOptionChannel, Value: "223456789012345678"},
		{Name: questionsImportStartDate, Type: discordgo.ApplicationCommandOptionString, Value: "2026-01-01"},
	}))

	resp := rec.lastResponse(t)
	if resp.Type != discordgo.InteractionResponseDeferredChannelMessageWithSource {
		t.Fatalf("expected import command to defer initial response, got type %v", resp.Type)
	}

	finalContent := rec.lastEdit(t)
	if !strings.Contains(finalContent, "Scanned 42 messages") {
		t.Fatalf("expected import summary to mention scan count, got %q", finalContent)
	}
	if !strings.Contains(finalContent, "Imported 10 historical QOTD questions as used history.") {
		t.Fatalf("expected import summary to mention imported count, got %q", finalContent)
	}
	if !strings.Contains(finalContent, "qotd-imports") {
		t.Fatalf("expected import summary to mention local backup path, got %q", finalContent)
	}
	if service.importCalls != 1 {
		t.Fatalf("expected import command to call ImportArchivedQuestions once, got %d", service.importCalls)
	}
	if service.lastImportGuild != guildID || service.lastImportActor != ownerID || service.lastImportSession != session {
		t.Fatalf("expected import command to forward guild, actor, and session, got guild=%q actor=%q session=%p", service.lastImportGuild, service.lastImportActor, service.lastImportSession)
	}
	if service.lastImportParams.DeckID != files.LegacyQOTDDefaultDeckID {
		t.Fatalf("expected import command to use the active deck, got %+v", service.lastImportParams)
	}
	if service.lastImportParams.SourceChannelID != "223456789012345678" {
		t.Fatalf("expected import command to forward the selected channel, got %+v", service.lastImportParams)
	}
	if service.lastImportParams.StartDate != "2026-01-01" {
		t.Fatalf("expected import command to forward the start date, got %+v", service.lastImportParams)
	}
	if len(service.lastImportParams.AuthorIDs) != 2 || service.lastImportParams.AuthorIDs[0] != "123456789012345678" || service.lastImportParams.AuthorIDs[1] != "987654321098765432" {
		t.Fatalf("expected import command to parse user ids, got %+v", service.lastImportParams.AuthorIDs)
	}
	if strings.TrimSpace(service.lastImportParams.BackupDir) == "" {
		t.Fatalf("expected import command to provide a backup directory, got %+v", service.lastImportParams)
	}
}

func TestFormatAutomaticQueueStateUsesNextSlotLabel(t *testing.T) {
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

	if !strings.Contains(message, "Next automatic slot:") {
		t.Fatalf("expected queue formatter to describe the upcoming slot, got %q", message)
	}
	if strings.Contains(message, "Current automatic slot:") {
		t.Fatalf("expected queue formatter to stop describing the queue slot as the current slot, got %q", message)
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
	if service.lastPublishParams.ConsumeAutomaticSlot == nil || !*service.lastPublishParams.ConsumeAutomaticSlot {
		t.Fatalf("expected publish command to default to consuming the automatic slot, got %+v", service.lastPublishParams)
	}
}

func TestQOTDPublishCommandCanSkipAutomaticSlotConsumption(t *testing.T) {
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
				ID:        18,
				DisplayID: 18,
				GuildID:   guildID,
				DeckID:    files.LegacyQOTDDefaultDeckID,
				Body:      "Non-consuming publish",
				Status:    string(applicationqotd.QuestionStatusUsed),
				UsedAt:    &publishedAt,
			},
			OfficialPost: storage.QOTDOfficialPostRecord{
				ID:                    100,
				GuildID:               guildID,
				DeckID:                files.LegacyQOTDDefaultDeckID,
				DeckNameSnapshot:      files.LegacyQOTDDefaultDeckName,
				QuestionID:            18,
				PublishMode:           string(applicationqotd.PublishModeManual),
				ConsumeAutomaticSlot:  false,
				PublishDateUTC:        time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC),
				ChannelID:             "channel-123",
				DiscordStarterMessageID: "message-100",
				PublishedAt:           &publishedAt,
			},
			PostURL: discordqotd.BuildMessageJumpURL(guildID, "channel-123", "message-100"),
		},
	}

	session, rec := newQOTDCommandTestSession(t)
	router, _ := newQOTDCommandTestRouterWithService(t, session, guildID, ownerID, service)

	router.HandleInteraction(session, newQOTDRootSlashInteraction(guildID, ownerID, publishSubCommandName, []*discordgo.ApplicationCommandInteractionDataOption{{
		Name:  publishConsumeAutomaticSlotOptionName,
		Type:  discordgo.ApplicationCommandOptionBoolean,
		Value: false,
	}}))
	resp := rec.lastResponse(t)
	requirePublicResponse(t, resp)
	if !strings.Contains(resp.Data.Content, "without consuming the automatic slot") {
		t.Fatalf("expected publish response to mention non-consuming mode, got %q", resp.Data.Content)
	}
	if service.lastPublishParams.ConsumeAutomaticSlot == nil || *service.lastPublishParams.ConsumeAutomaticSlot {
		t.Fatalf("expected publish command to forward consume_automatic_slot=false, got %+v", service.lastPublishParams)
	}
}
