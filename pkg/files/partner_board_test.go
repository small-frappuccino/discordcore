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
