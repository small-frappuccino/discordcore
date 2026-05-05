package qotd

import (
	"context"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

type fakeGuildLifecycleService struct {
	publishCalls   []string
	reconcileCalls []string
	publishCh      chan string
	reconcileCh    chan string
}

func (f *fakeGuildLifecycleService) PublishScheduledIfDue(_ context.Context, guildID string, _ *discordgo.Session) (bool, error) {
	f.publishCalls = append(f.publishCalls, guildID)
	if f.publishCh != nil {
		select {
		case f.publishCh <- guildID:
		default:
		}
	}
	return false, nil
}

func (f *fakeGuildLifecycleService) ReconcileGuild(_ context.Context, guildID string, _ *discordgo.Session) error {
	f.reconcileCalls = append(f.reconcileCalls, guildID)
	if f.reconcileCh != nil {
		select {
		case f.reconcileCh <- guildID:
		default:
		}
	}
	return nil
}

func TestRuntimeServiceLoopRunsPublishCycleOnStartAndInterval(t *testing.T) {
	configManager := files.NewMemoryConfigManager()
	for _, guild := range []files.GuildConfig{
		{
			GuildID:       "g-enabled",
			BotInstanceID: "main",
			QOTD: files.QOTDConfig{
				ActiveDeckID: files.LegacyQOTDDefaultDeckID,
				Decks: []files.QOTDDeckConfig{{
					ID:        files.LegacyQOTDDefaultDeckID,
					Name:      files.LegacyQOTDDefaultDeckName,
					Enabled:   true,
					ChannelID: "question-enabled",
				}},
			},
		},
		{
			GuildID:       "g-disabled",
			BotInstanceID: "main",
			QOTD: files.QOTDConfig{
				ActiveDeckID: files.LegacyQOTDDefaultDeckID,
				Decks: []files.QOTDDeckConfig{{
					ID:        files.LegacyQOTDDefaultDeckID,
					Name:      files.LegacyQOTDDefaultDeckName,
					ChannelID: "question-disabled",
				}},
			},
		},
	} {
		if err := configManager.AddGuildConfig(guild); err != nil {
			t.Fatalf("AddGuildConfig(%s) failed: %v", guild.GuildID, err)
		}
	}

	fake := &fakeGuildLifecycleService{
		publishCh:   make(chan string, 8),
		reconcileCh: make(chan string, 8),
	}
	service := NewRuntimeServiceForBot(&discordgo.Session{}, configManager, fake, "main", "main")
	service.publishInterval = 10 * time.Millisecond
	service.reconcileEvery = time.Hour
	service.now = func() time.Time {
		return time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC)
	}

	service.Start()
	t.Cleanup(service.Stop)
	if !service.IsRunning() {
		t.Fatal("expected runtime service to report running after Start()")
	}

	for idx := 0; idx < 2; idx++ {
		select {
		case guildID := <-fake.publishCh:
			if guildID != "g-enabled" {
				t.Fatalf("unexpected publish cycle guild %q", guildID)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for publish cycle %d", idx+1)
		}
	}

	service.Stop()

	if service.IsRunning() {
		t.Fatal("expected runtime service to stop running after Stop()")
	}
	if len(fake.publishCalls) < 2 {
		t.Fatalf("expected startup and interval publish cycles, got %v", fake.publishCalls)
	}
	for _, guildID := range fake.publishCalls {
		if guildID != "g-enabled" {
			t.Fatalf("expected runtime loop to publish only for enabled guilds, got %v", fake.publishCalls)
		}
	}
	if len(fake.reconcileCalls) == 0 || fake.reconcileCalls[0] != "g-enabled" {
		t.Fatalf("expected startup reconcile cycle to include enabled guild first, got %v", fake.reconcileCalls)
	}
	if len(fake.reconcileCalls) < 2 || fake.reconcileCalls[1] != "g-disabled" {
		t.Fatalf("expected startup reconcile cycle to include configured disabled guild too, got %v", fake.reconcileCalls)
	}
}

func TestRuntimeServiceCyclesUseScopedGuilds(t *testing.T) {
	t.Parallel()

	configManager := files.NewMemoryConfigManager()
	for _, guild := range []files.GuildConfig{
		{
			GuildID:       "g-enabled",
			BotInstanceID: "main",
			QOTD: files.QOTDConfig{
				ActiveDeckID: files.LegacyQOTDDefaultDeckID,
				Decks: []files.QOTDDeckConfig{{
					ID:        files.LegacyQOTDDefaultDeckID,
					Name:      files.LegacyQOTDDefaultDeckName,
					Enabled:   true,
					ChannelID: "question-enabled",
				}},
			},
		},
		{
			GuildID:       "g-configured-disabled",
			BotInstanceID: "main",
			QOTD: files.QOTDConfig{
				ActiveDeckID: files.LegacyQOTDDefaultDeckID,
				Decks: []files.QOTDDeckConfig{{
					ID:        files.LegacyQOTDDefaultDeckID,
					Name:      files.LegacyQOTDDefaultDeckName,
					ChannelID: "question-disabled",
				}},
			},
		},
		{
			GuildID:       "g-other-runtime",
			BotInstanceID: "other",
			QOTD: files.QOTDConfig{
				ActiveDeckID: files.LegacyQOTDDefaultDeckID,
				Decks: []files.QOTDDeckConfig{{
					ID:        files.LegacyQOTDDefaultDeckID,
					Name:      files.LegacyQOTDDefaultDeckName,
					Enabled:   true,
					ChannelID: "question-other",
				}},
			},
		},
		{
			GuildID:       "g-empty",
			BotInstanceID: "main",
		},
	} {
		if err := configManager.AddGuildConfig(guild); err != nil {
			t.Fatalf("AddGuildConfig(%s) failed: %v", guild.GuildID, err)
		}
	}

	fake := &fakeGuildLifecycleService{}
	service := NewRuntimeServiceForBot(&discordgo.Session{}, configManager, fake, "main", "main")
	service.now = func() time.Time {
		return time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC)
	}

	service.runPublishCycle(service.clock())
	service.runReconcileCycle(service.clock())

	if len(fake.publishCalls) != 1 || fake.publishCalls[0] != "g-enabled" {
		t.Fatalf("expected publish cycle to include only enabled guilds on the runtime, got %v", fake.publishCalls)
	}
	if len(fake.reconcileCalls) != 2 {
		t.Fatalf("expected reconcile cycle to include configured guilds on the runtime, got %v", fake.reconcileCalls)
	}
	if fake.reconcileCalls[0] != "g-enabled" || fake.reconcileCalls[1] != "g-configured-disabled" {
		t.Fatalf("unexpected reconcile target order: %v", fake.reconcileCalls)
	}
}

func TestRuntimeServiceCyclesUseQOTDDomainScopedGuilds(t *testing.T) {
	t.Parallel()

	configManager := files.NewMemoryConfigManager()
	for _, guild := range []files.GuildConfig{
		{
			GuildID:       "g-qotd-enabled",
			BotInstanceID: "main",
			DomainBotInstanceIDs: map[string]string{
				files.BotDomainQOTD: "companion",
			},
			QOTD: files.QOTDConfig{
				ActiveDeckID: files.LegacyQOTDDefaultDeckID,
				Decks: []files.QOTDDeckConfig{{
					ID:        files.LegacyQOTDDefaultDeckID,
					Name:      files.LegacyQOTDDefaultDeckName,
					Enabled:   true,
					ChannelID: "question-enabled",
				}},
			},
		},
		{
			GuildID:       "g-qotd-configured-disabled",
			BotInstanceID: "main",
			DomainBotInstanceIDs: map[string]string{
				files.BotDomainQOTD: "companion",
			},
			QOTD: files.QOTDConfig{
				ActiveDeckID: files.LegacyQOTDDefaultDeckID,
				Decks: []files.QOTDDeckConfig{{
					ID:        files.LegacyQOTDDefaultDeckID,
					Name:      files.LegacyQOTDDefaultDeckName,
					ChannelID: "question-disabled",
				}},
			},
		},
		{
			GuildID:       "g-default-main",
			BotInstanceID: "main",
			QOTD: files.QOTDConfig{
				ActiveDeckID: files.LegacyQOTDDefaultDeckID,
				Decks: []files.QOTDDeckConfig{{
					ID:        files.LegacyQOTDDefaultDeckID,
					Name:      files.LegacyQOTDDefaultDeckName,
					Enabled:   true,
					ChannelID: "question-alice",
				}},
			},
		},
	} {
		if err := configManager.AddGuildConfig(guild); err != nil {
			t.Fatalf("AddGuildConfig(%s) failed: %v", guild.GuildID, err)
		}
	}

	fake := &fakeGuildLifecycleService{}
	service := NewRuntimeServiceForBot(&discordgo.Session{}, configManager, fake, "companion", "main")
	service.now = func() time.Time {
		return time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC)
	}

	service.runPublishCycle(service.clock())
	service.runReconcileCycle(service.clock())

	if len(fake.publishCalls) != 1 || fake.publishCalls[0] != "g-qotd-enabled" {
		t.Fatalf("expected publish cycle to include only enabled qotd-domain guilds for companion, got %v", fake.publishCalls)
	}
	if len(fake.reconcileCalls) != 2 {
		t.Fatalf("expected reconcile cycle to include configured qotd-domain guilds for companion, got %v", fake.reconcileCalls)
	}
	if fake.reconcileCalls[0] != "g-qotd-enabled" || fake.reconcileCalls[1] != "g-qotd-configured-disabled" {
		t.Fatalf("unexpected reconcile target order: %v", fake.reconcileCalls)
	}
}
