package qotd

import (
	"context"
	"sync"
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

type blockingContextLifecycleService struct {
	started chan struct{}
	done    chan struct{}
	errCh   chan error
	once    sync.Once
}

func (f *blockingContextLifecycleService) PublishScheduledIfDue(ctx context.Context, _ string, _ *discordgo.Session) (bool, error) {
	f.once.Do(func() {
		if f.started != nil {
			close(f.started)
		}
	})
	<-ctx.Done()
	if f.errCh != nil {
		f.errCh <- ctx.Err()
	}
	if f.done != nil {
		close(f.done)
	}
	return false, ctx.Err()
}

func (f *blockingContextLifecycleService) ReconcileGuild(context.Context, string, *discordgo.Session) error {
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
			GuildID:       "g-enabled-missing-channel",
			BotInstanceID: "main",
			QOTD: files.QOTDConfig{
				ActiveDeckID: files.LegacyQOTDDefaultDeckID,
				Decks: []files.QOTDDeckConfig{{
					ID:      files.LegacyQOTDDefaultDeckID,
					Name:    files.LegacyQOTDDefaultDeckName,
					Enabled: true,
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
		t.Fatalf("expected publish cycle to include only enabled guilds with channel targets on the runtime, got %v", fake.publishCalls)
	}
	if len(fake.reconcileCalls) != 3 {
		t.Fatalf("expected reconcile cycle to include configured guilds on the runtime, got %v", fake.reconcileCalls)
	}
	if fake.reconcileCalls[0] != "g-enabled" || fake.reconcileCalls[1] != "g-enabled-missing-channel" || fake.reconcileCalls[2] != "g-configured-disabled" {
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

func TestRuntimeServiceRestartResumesIntervalCycles(t *testing.T) {
	configManager := files.NewMemoryConfigManager()
	if err := configManager.AddGuildConfig(files.GuildConfig{
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
	}); err != nil {
		t.Fatalf("AddGuildConfig() failed: %v", err)
	}

	fake := &fakeGuildLifecycleService{publishCh: make(chan string, 16)}
	service := NewRuntimeServiceForBot(&discordgo.Session{}, configManager, fake, "main", "main")
	service.publishInterval = time.Hour
	service.reconcileEvery = time.Hour
	service.now = func() time.Time {
		return time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC)
	}

	service.Start()
	select {
	case guildID := <-fake.publishCh:
		if guildID != "g-enabled" {
			t.Fatalf("unexpected first start publish guild %q", guildID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for first start publish cycle")
	}
	service.Stop()

	service.publishInterval = 10 * time.Millisecond
	service.Start()
	t.Cleanup(service.Stop)

	select {
	case guildID := <-fake.publishCh:
		if guildID != "g-enabled" {
			t.Fatalf("unexpected restart startup publish guild %q", guildID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for restart startup publish cycle")
	}

	select {
	case guildID := <-fake.publishCh:
		if guildID != "g-enabled" {
			t.Fatalf("unexpected restart interval publish guild %q", guildID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for restart interval publish cycle")
	}
}

func TestRuntimeServiceStopCancelsInflightPublish(t *testing.T) {
	configManager := files.NewMemoryConfigManager()
	if err := configManager.AddGuildConfig(files.GuildConfig{
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
	}); err != nil {
		t.Fatalf("AddGuildConfig() failed: %v", err)
	}

	fake := &blockingContextLifecycleService{
		started: make(chan struct{}),
		done:    make(chan struct{}),
		errCh:   make(chan error, 1),
	}
	service := NewRuntimeServiceForBot(&discordgo.Session{}, configManager, fake, "main", "main")
	service.publishInterval = time.Hour
	service.reconcileEvery = time.Hour

	service.Start()
	select {
	case <-fake.started:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for publish operation to start")
	}

	stopped := make(chan struct{})
	go func() {
		service.Stop()
		close(stopped)
	}()

	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Stop() to cancel in-flight publish")
	}

	select {
	case <-fake.done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for publish operation to observe cancellation")
	}

	select {
	case err := <-fake.errCh:
		if err == nil {
			t.Fatal("expected in-flight publish to receive context cancellation")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for cancellation error")
	}
}
