package partners

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/messageupdate"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

type rendererStub struct {
	lastTemplate PartnerBoardTemplate
	lastPartners []PartnerRecord
	embeds       []*discordgo.MessageEmbed
	err          error
	calls        int
}

func (s *rendererStub) Render(template PartnerBoardTemplate, partners []PartnerRecord) ([]*discordgo.MessageEmbed, error) {
	s.calls++
	s.lastTemplate = template
	s.lastPartners = append([]PartnerRecord(nil), partners...)
	if s.err != nil {
		return nil, s.err
	}
	return s.embeds, nil
}

type updaterStub struct {
	lastTarget messageupdate.EmbedUpdateTarget
	lastEmbeds []*discordgo.MessageEmbed
	err        error
	calls      int
}

func (s *updaterStub) UpdateEmbeds(_ *discordgo.Session, target messageupdate.EmbedUpdateTarget, embeds []*discordgo.MessageEmbed) error {
	s.calls++
	s.lastTarget = target
	s.lastEmbeds = append([]*discordgo.MessageEmbed(nil), embeds...)
	return s.err
}

func newSyncServiceTestManager(t *testing.T, cfg *files.BotConfig) *files.ConfigManager {
	t.Helper()
	mgr := files.NewConfigManagerWithPath(filepath.Join(t.TempDir(), "settings.json"))
	if err := mgr.LoadConfig(); err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg != nil {
		for _, guild := range cfg.Guilds {
			if err := mgr.AddGuildConfig(guild); err != nil {
				t.Fatalf("add guild config %q: %v", guild.GuildID, err)
			}
		}
	}

	return mgr
}

func newSyncTestSession(t *testing.T) *discordgo.Session {
	t.Helper()
	session, err := discordgo.New("Bot test-token")
	if err != nil {
		t.Fatalf("create discord session: %v", err)
	}
	return session
}

func TestBoardSyncServiceSyncGuildSuccess(t *testing.T) {
	t.Parallel()

	cfg := &files.BotConfig{
		Guilds: []files.GuildConfig{
			{
				GuildID: "g1",
				PartnerBoard: files.PartnerBoardConfig{
					Target: files.EmbedUpdateTargetConfig{
						Type:      files.EmbedUpdateTargetTypeChannelMessage,
						MessageID: "123456789012345678",
						ChannelID: "223456789012345678",
					},
					Template: files.PartnerBoardTemplateConfig{
						Title:                 "Partners",
						LineTemplate:          "{name} {link}",
						SectionHeaderTemplate: "{fandom}",
					},
					Partners: []files.PartnerEntryConfig{
						{
							Fandom: "Genshin Impact",
							Name:   "Citlali Mains",
							Link:   "https://discord.com/invite/Citlali",
						},
					},
				},
			},
		},
	}
	mgr := newSyncServiceTestManager(t, cfg)
	renderer := &rendererStub{
		embeds: []*discordgo.MessageEmbed{
			{
				Title:       "Partners",
				Description: "Rendered",
			},
		},
	}
	updater := &updaterStub{}
	service := NewBoardSyncServiceWithDependencies(mgr, renderer, updater)

	err := service.SyncGuild(context.Background(), newSyncTestSession(t), "g1")
	if err != nil {
		t.Fatalf("sync guild: %v", err)
	}

	if renderer.calls != 1 {
		t.Fatalf("expected renderer to be called once, got %d", renderer.calls)
	}
	if updater.calls != 1 {
		t.Fatalf("expected updater to be called once, got %d", updater.calls)
	}
	if renderer.lastTemplate.Title != "Partners" {
		t.Fatalf("unexpected template passed to renderer: %+v", renderer.lastTemplate)
	}
	if len(renderer.lastPartners) != 1 {
		t.Fatalf("unexpected partners passed to renderer: %+v", renderer.lastPartners)
	}
	if renderer.lastPartners[0].Link != "https://discord.gg/citlali" {
		t.Fatalf("expected canonical invite link from config manager, got %+v", renderer.lastPartners[0])
	}
	if updater.lastTarget.Type != messageupdate.TargetTypeChannelMessage {
		t.Fatalf("unexpected target passed to updater: %+v", updater.lastTarget)
	}
	if updater.lastTarget.MessageID != "123456789012345678" || updater.lastTarget.ChannelID != "223456789012345678" {
		t.Fatalf("unexpected target IDs passed to updater: %+v", updater.lastTarget)
	}
}

func TestBoardSyncServiceSyncGuildConfigError(t *testing.T) {
	t.Parallel()

	mgr := newSyncServiceTestManager(t, &files.BotConfig{
		Guilds: []files.GuildConfig{{GuildID: "g1"}},
	})
	service := NewBoardSyncServiceWithDependencies(
		mgr,
		&rendererStub{},
		&updaterStub{},
	)

	err := service.SyncGuild(context.Background(), newSyncTestSession(t), "missing")
	if err == nil {
		t.Fatal("expected sync to fail for missing guild")
	}
	if !strings.Contains(err.Error(), "sync partner board guild_id=missing") {
		t.Fatalf("expected contextual guild error, got: %v", err)
	}
	if !strings.Contains(strings.ToLower(err.Error()), "load board config") {
		t.Fatalf("expected load board context, got: %v", err)
	}
}

func TestBoardSyncServiceSyncGuildRenderError(t *testing.T) {
	t.Parallel()

	mgr := newSyncServiceTestManager(t, &files.BotConfig{
		Guilds: []files.GuildConfig{
			{
				GuildID: "g1",
				PartnerBoard: files.PartnerBoardConfig{
					Target: files.EmbedUpdateTargetConfig{
						Type:      files.EmbedUpdateTargetTypeChannelMessage,
						MessageID: "123456789012345678",
						ChannelID: "223456789012345678",
					},
				},
			},
		},
	})
	renderErr := errors.New("render failed")
	renderer := &rendererStub{err: renderErr}
	updater := &updaterStub{}
	service := NewBoardSyncServiceWithDependencies(mgr, renderer, updater)

	err := service.SyncGuild(context.Background(), newSyncTestSession(t), "g1")
	if err == nil {
		t.Fatal("expected sync to fail on renderer error")
	}
	if !errors.Is(err, renderErr) {
		t.Fatalf("expected wrapped render error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "render embeds") {
		t.Fatalf("expected render context in error, got: %v", err)
	}
	if updater.calls != 0 {
		t.Fatalf("expected updater not to be called when render fails, got %d calls", updater.calls)
	}
}

func TestBoardSyncServiceSyncGuildUpdateError(t *testing.T) {
	t.Parallel()

	mgr := newSyncServiceTestManager(t, &files.BotConfig{
		Guilds: []files.GuildConfig{
			{
				GuildID: "g1",
				PartnerBoard: files.PartnerBoardConfig{
					Target: files.EmbedUpdateTargetConfig{
						Type:      files.EmbedUpdateTargetTypeChannelMessage,
						MessageID: "123456789012345678",
						ChannelID: "223456789012345678",
					},
				},
			},
		},
	})
	renderer := &rendererStub{
		embeds: []*discordgo.MessageEmbed{{Title: "ok"}},
	}
	updateErr := errors.New("publish failed")
	updater := &updaterStub{err: updateErr}
	service := NewBoardSyncServiceWithDependencies(mgr, renderer, updater)

	err := service.SyncGuild(context.Background(), newSyncTestSession(t), "g1")
	if err == nil {
		t.Fatal("expected sync to fail on update error")
	}
	if !errors.Is(err, updateErr) {
		t.Fatalf("expected wrapped update error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "publish embeds") {
		t.Fatalf("expected publish context in error, got: %v", err)
	}
}

func TestBoardSyncServiceSyncGuildContextDone(t *testing.T) {
	t.Parallel()

	mgr := newSyncServiceTestManager(t, &files.BotConfig{
		Guilds: []files.GuildConfig{
			{
				GuildID: "g1",
				PartnerBoard: files.PartnerBoardConfig{
					Target: files.EmbedUpdateTargetConfig{
						Type:      files.EmbedUpdateTargetTypeChannelMessage,
						MessageID: "123456789012345678",
						ChannelID: "223456789012345678",
					},
				},
			},
		},
	})
	service := NewBoardSyncServiceWithDependencies(
		mgr,
		&rendererStub{embeds: []*discordgo.MessageEmbed{{Title: "ok"}}},
		&updaterStub{},
	)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := service.SyncGuild(ctx, newSyncTestSession(t), "g1")
	if err == nil {
		t.Fatal("expected sync to fail for cancelled context")
	}
	if !strings.Contains(err.Error(), "context done") {
		t.Fatalf("expected context-done detail in error, got: %v", err)
	}
}
