package partners

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/files"
)

type mutationNotifierStub struct {
	calls []string
	err   error
}

func (n *mutationNotifierStub) Notify(guildID string) error {
	n.calls = append(n.calls, guildID)
	return n.err
}

func newBoardAppTestManager(t *testing.T) *files.ConfigManager {
	t.Helper()

	mgr := files.NewConfigManagerWithPath(filepath.Join(t.TempDir(), "settings.json"))
	if err := mgr.LoadConfig(); err != nil {
		t.Fatalf("load config: %v", err)
	}
	if err := mgr.AddGuildConfig(files.GuildConfig{GuildID: "g1"}); err != nil {
		t.Fatalf("add guild config: %v", err)
	}
	return mgr
}

func TestBoardApplicationServiceMutationsTriggerNotify(t *testing.T) {
	t.Parallel()

	mgr := newBoardAppTestManager(t)
	notifier := &mutationNotifierStub{}
	service := NewBoardApplicationService(mgr, notifier)

	if err := service.SetPartnerBoardTarget("g1", files.EmbedUpdateTargetConfig{
		Type:      files.EmbedUpdateTargetTypeChannelMessage,
		MessageID: "123456789012345678",
		ChannelID: "223456789012345678",
	}); err != nil {
		t.Fatalf("set target: %v", err)
	}

	if err := service.SetPartnerBoardTemplate("g1", files.PartnerBoardTemplateConfig{
		Title:                 "Partners",
		SectionHeaderTemplate: "{fandom}",
		LineTemplate:          "{name} {link}",
	}); err != nil {
		t.Fatalf("set template: %v", err)
	}

	if err := service.CreatePartner("g1", files.PartnerEntryConfig{
		Fandom: "Genshin Impact",
		Name:   "Citlali Mains",
		Link:   "discord.gg/citlali",
	}); err != nil {
		t.Fatalf("create partner: %v", err)
	}

	if err := service.UpdatePartner("g1", "Citlali Mains", files.PartnerEntryConfig{
		Fandom: "Genshin Impact",
		Name:   "Citlali Hub",
		Link:   "discord.gg/citlali_hub",
	}); err != nil {
		t.Fatalf("update partner: %v", err)
	}

	if err := service.DeletePartner("g1", "Citlali Hub"); err != nil {
		t.Fatalf("delete partner: %v", err)
	}

	if got := len(notifier.calls); got != 5 {
		t.Fatalf("expected 5 notify calls, got %d (%v)", got, notifier.calls)
	}
	for i, guildID := range notifier.calls {
		if guildID != "g1" {
			t.Fatalf("unexpected notified guild at index %d: %q", i, guildID)
		}
	}
}

func TestBoardApplicationServiceReadsDoNotNotify(t *testing.T) {
	t.Parallel()

	mgr := newBoardAppTestManager(t)
	notifier := &mutationNotifierStub{}
	service := NewBoardApplicationService(mgr, notifier)

	if err := service.SetPartnerBoardTarget("g1", files.EmbedUpdateTargetConfig{
		Type:      files.EmbedUpdateTargetTypeChannelMessage,
		MessageID: "123456789012345678",
		ChannelID: "223456789012345678",
	}); err != nil {
		t.Fatalf("set target: %v", err)
	}
	if err := service.SetPartnerBoardTemplate("g1", files.PartnerBoardTemplateConfig{
		Title: "Partners",
	}); err != nil {
		t.Fatalf("set template: %v", err)
	}
	if err := service.CreatePartner("g1", files.PartnerEntryConfig{
		Fandom: "Genshin Impact",
		Name:   "Citlali Mains",
		Link:   "discord.gg/citlali",
	}); err != nil {
		t.Fatalf("create partner: %v", err)
	}

	notifier.calls = nil

	if _, err := service.GetPartnerBoard("g1"); err != nil {
		t.Fatalf("get board: %v", err)
	}
	if _, err := service.GetPartnerBoardTarget("g1"); err != nil {
		t.Fatalf("get target: %v", err)
	}
	if _, err := service.GetPartnerBoardTemplate("g1"); err != nil {
		t.Fatalf("get template: %v", err)
	}
	if _, err := service.ListPartners("g1"); err != nil {
		t.Fatalf("list partners: %v", err)
	}
	if _, err := service.GetPartner("g1", "Citlali Mains"); err != nil {
		t.Fatalf("get partner: %v", err)
	}

	if got := len(notifier.calls); got != 0 {
		t.Fatalf("expected no notify calls for reads, got %d", got)
	}
}

func TestBoardApplicationServiceNotifyErrorDoesNotFailMutation(t *testing.T) {
	t.Parallel()

	mgr := newBoardAppTestManager(t)
	notifier := &mutationNotifierStub{err: errors.New("queue full")}
	service := NewBoardApplicationService(mgr, notifier)

	if err := service.CreatePartner("g1", files.PartnerEntryConfig{
		Fandom: "ZZZ",
		Name:   "Jane Mains",
		Link:   "discord.gg/jane",
	}); err != nil {
		t.Fatalf("create partner should still succeed when notify fails: %v", err)
	}

	if _, err := service.GetPartner("g1", "Jane Mains"); err != nil {
		t.Fatalf("expected partner to persist despite notify failure: %v", err)
	}
	if got := len(notifier.calls); got != 1 {
		t.Fatalf("expected one notify call, got %d", got)
	}
}

func TestBoardApplicationServiceValidation(t *testing.T) {
	t.Parallel()

	service := NewBoardApplicationService(nil, nil)
	if err := service.CreatePartner("g1", files.PartnerEntryConfig{
		Name: "X",
		Link: "discord.gg/x",
	}); err == nil {
		t.Fatal("expected validation error for nil config manager")
	}
}
