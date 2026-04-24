package files

import (
	"errors"
	"testing"
)

func newPartnerBoardTestManager(t *testing.T, cfg *BotConfig) *ConfigManager {
	t.Helper()

	if cfg == nil {
		cfg = &BotConfig{Guilds: []GuildConfig{}}
	}
	if cfg.Guilds == nil {
		cfg.Guilds = []GuildConfig{}
	}

	mgr := NewMemoryConfigManager()
	mgr.config = cfg
	if _, err := mgr.rebuildGuildIndexLocked("test"); err != nil {
		t.Fatalf("rebuild index: %v", err)
	}
	return mgr
}

func TestPartnersCRUDAndDeterministicOrder(t *testing.T) {
	t.Parallel()

	mgr := newPartnerBoardTestManager(t, &BotConfig{
		Guilds: []GuildConfig{{GuildID: "g1"}},
	})

	if err := mgr.CreatePartner("g1", PartnerEntryConfig{
		Fandom: "ZZZ",
		Name:   "Jane Mains",
		Link:   "https://discord.com/invite/JaneMains",
	}); err != nil {
		t.Fatalf("create partner jane: %v", err)
	}
	if err := mgr.CreatePartner("g1", PartnerEntryConfig{
		Fandom: "Genshin Impact",
		Name:   "Citlali Mains",
		Link:   "discord.gg/Citlali",
	}); err != nil {
		t.Fatalf("create partner citlali: %v", err)
	}

	list, err := mgr.ListPartners("g1")
	if err != nil {
		t.Fatalf("list partners: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 partners, got %d", len(list))
	}

	// Deterministic order: fandom (asc), then name.
	if list[0].Fandom != "Genshin Impact" || list[0].Name != "Citlali Mains" {
		t.Fatalf("unexpected first partner order: %+v", list[0])
	}
	if list[1].Fandom != "ZZZ" || list[1].Name != "Jane Mains" {
		t.Fatalf("unexpected second partner order: %+v", list[1])
	}

	// Canonical invite format.
	if list[0].Link != "https://discord.gg/citlali" {
		t.Fatalf("unexpected canonical invite for citlali: %q", list[0].Link)
	}
	if list[1].Link != "https://discord.gg/janemains" {
		t.Fatalf("unexpected canonical invite for jane: %q", list[1].Link)
	}

	if err := mgr.UpdatePartner("g1", "Jane Mains", PartnerEntryConfig{
		Fandom: "ZZZ",
		Name:   "Jane Doe Mains",
		Link:   "https://discord.gg/janedoe",
	}); err != nil {
		t.Fatalf("update partner: %v", err)
	}

	got, err := mgr.Partner("g1", "jane doe mains")
	if err != nil {
		t.Fatalf("Partner() failed: %v", err)
	}
	if got.Name != "Jane Doe Mains" {
		t.Fatalf("unexpected updated name: %+v", got)
	}

	if err := mgr.DeletePartner("g1", "Citlali Mains"); err != nil {
		t.Fatalf("delete partner: %v", err)
	}
	afterDelete, err := mgr.ListPartners("g1")
	if err != nil {
		t.Fatalf("list after delete: %v", err)
	}
	if len(afterDelete) != 1 || afterDelete[0].Name != "Jane Doe Mains" {
		t.Fatalf("unexpected list after delete: %+v", afterDelete)
	}
}

func TestPartnersValidationAndDedup(t *testing.T) {
	t.Parallel()

	mgr := newPartnerBoardTestManager(t, &BotConfig{
		Guilds: []GuildConfig{{GuildID: "g1"}},
	})

	if err := mgr.CreatePartner("g1", PartnerEntryConfig{
		Fandom: "Genshin Impact",
		Name:   "Citlali Mains",
		Link:   "https://discord.gg/citlali",
	}); err != nil {
		t.Fatalf("create baseline partner: %v", err)
	}

	if err := mgr.CreatePartner("g1", PartnerEntryConfig{
		Fandom: "Genshin Impact",
		Name:   "citlali mains",
		Link:   "https://discord.gg/citlali2",
	}); !errors.Is(err, ErrPartnerAlreadyExists) {
		t.Fatalf("expected name duplicate error, got %v", err)
	}

	if err := mgr.CreatePartner("g1", PartnerEntryConfig{
		Fandom: "Genshin Impact",
		Name:   "Citlali Mains 2",
		Link:   "https://discord.com/invite/CITLALI",
	}); !errors.Is(err, ErrPartnerAlreadyExists) {
		t.Fatalf("expected link duplicate error, got %v", err)
	}

	if err := mgr.CreatePartner("g1", PartnerEntryConfig{
		Fandom: "Genshin Impact",
		Name:   "Invalid Invite",
		Link:   "https://example.com/invite/not-discord",
	}); err == nil {
		t.Fatal("expected invalid invite host error")
	}
}

func TestPartnersUpdateDeleteNotFound(t *testing.T) {
	t.Parallel()

	mgr := newPartnerBoardTestManager(t, &BotConfig{
		Guilds: []GuildConfig{{GuildID: "g1"}},
	})

	if err := mgr.UpdatePartner("g1", "missing", PartnerEntryConfig{
		Fandom: "ZZZ",
		Name:   "Jane Mains",
		Link:   "https://discord.gg/jane",
	}); !errors.Is(err, ErrPartnerNotFound) {
		t.Fatalf("expected ErrPartnerNotFound on update, got %v", err)
	}

	if err := mgr.DeletePartner("g1", "missing"); !errors.Is(err, ErrPartnerNotFound) {
		t.Fatalf("expected ErrPartnerNotFound on delete, got %v", err)
	}
}

func TestPartnerBoardTargetSetGet(t *testing.T) {
	t.Parallel()

	mgr := newPartnerBoardTestManager(t, &BotConfig{
		Guilds: []GuildConfig{{GuildID: "g1"}},
	})

	if err := mgr.SetPartnerBoardTarget("g1", EmbedUpdateTargetConfig{
		Type:       EmbedUpdateTargetTypeWebhookMessage,
		MessageID:  "123456789012345678",
		WebhookURL: "https://discord.com/api/webhooks/123456789012345678/token",
	}); err != nil {
		t.Fatalf("set webhook target: %v", err)
	}

	webhookTarget, err := mgr.PartnerBoardTarget("g1")
	if err != nil {
		t.Fatalf("PartnerBoardTarget(webhook) failed: %v", err)
	}
	if webhookTarget.Type != EmbedUpdateTargetTypeWebhookMessage {
		t.Fatalf("unexpected webhook target type: %+v", webhookTarget)
	}
	if webhookTarget.MessageID != "123456789012345678" {
		t.Fatalf("unexpected webhook target message ID: %+v", webhookTarget)
	}

	if err := mgr.SetPartnerBoardTarget("g1", EmbedUpdateTargetConfig{
		Type:      EmbedUpdateTargetTypeChannelMessage,
		MessageID: "223456789012345678",
		ChannelID: "323456789012345678",
	}); err != nil {
		t.Fatalf("set channel target: %v", err)
	}

	channelTarget, err := mgr.PartnerBoardTarget("g1")
	if err != nil {
		t.Fatalf("PartnerBoardTarget(channel) failed: %v", err)
	}
	if channelTarget.Type != EmbedUpdateTargetTypeChannelMessage {
		t.Fatalf("unexpected channel target type: %+v", channelTarget)
	}
	if channelTarget.ChannelID != "323456789012345678" {
		t.Fatalf("unexpected channel target channel ID: %+v", channelTarget)
	}
	if channelTarget.WebhookURL != "" {
		t.Fatalf("expected webhook URL to be cleared for channel target: %+v", channelTarget)
	}

	// Clear target
	if err := mgr.SetPartnerBoardTarget("g1", EmbedUpdateTargetConfig{}); err != nil {
		t.Fatalf("clear target: %v", err)
	}
	cleared, err := mgr.PartnerBoardTarget("g1")
	if err != nil {
		t.Fatalf("PartnerBoardTarget(cleared) failed: %v", err)
	}
	if !cleared.IsZero() {
		t.Fatalf("expected cleared target to be zero, got %+v", cleared)
	}
}

func TestPartnerBoardTargetValidation(t *testing.T) {
	t.Parallel()

	mgr := newPartnerBoardTestManager(t, &BotConfig{
		Guilds: []GuildConfig{{GuildID: "g1"}},
	})

	if err := mgr.SetPartnerBoardTarget("g1", EmbedUpdateTargetConfig{
		Type:      "invalid",
		MessageID: "1",
	}); err == nil {
		t.Fatal("expected validation error for invalid target type")
	}

	if err := mgr.SetPartnerBoardTarget("g1", EmbedUpdateTargetConfig{
		Type:      EmbedUpdateTargetTypeChannelMessage,
		MessageID: "1",
		ChannelID: "not-numeric",
	}); err == nil {
		t.Fatal("expected validation error for non-numeric channel_id")
	}
}
