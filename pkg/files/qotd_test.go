package files

import (
	"errors"
	"testing"
)

func TestNormalizeQOTDConfigRequiresDeliveryTargetsWhenEnabled(t *testing.T) {
	t.Parallel()

	if _, err := NormalizeQOTDConfig(QOTDConfig{Enabled: true}); err == nil {
		t.Fatal("expected enabled qotd config without delivery targets to fail")
	}
}

func TestSetQOTDConfigCanonicalizesMessageChannelFields(t *testing.T) {
	t.Parallel()

	mgr, _ := newTransactionalTestManager(t, &BotConfig{
		Guilds: []GuildConfig{{GuildID: "g1"}},
	}, nil)

	err := mgr.SetQOTDConfig("g1", QOTDConfig{
		Enabled:           true,
		QuestionChannelID: " 123456789012345678 ",
		ResponseChannelID: " 223456789012345678 ",
	})
	if err != nil {
		t.Fatalf("SetQOTDConfig() failed: %v", err)
	}

	cfg, err := mgr.GetQOTDConfig("g1")
	if err != nil {
		t.Fatalf("GetQOTDConfig() failed: %v", err)
	}
	deck, ok := cfg.ActiveDeck()
	if !ok {
		t.Fatal("expected qotd config to expose an active deck")
	}
	if !deck.Enabled {
		t.Fatal("expected qotd deck to remain enabled")
	}
	if deck.QuestionChannelID != "123456789012345678" {
		t.Fatalf("expected trimmed question channel id, got %q", deck.QuestionChannelID)
	}
	if deck.ResponseChannelID != "223456789012345678" {
		t.Fatalf("expected trimmed response channel id, got %q", deck.ResponseChannelID)
	}
	if cfg.ActiveDeckID != LegacyQOTDDefaultDeckID {
		t.Fatalf("expected default deck to become active, got %q", cfg.ActiveDeckID)
	}
}

func TestSetQOTDConfigRollsBackOnSaveError(t *testing.T) {
	t.Parallel()

	saveErr := errors.New("save failed")
	mgr, _ := newTransactionalTestManager(t, &BotConfig{
		Guilds: []GuildConfig{{GuildID: "g1"}},
	}, saveErr)

	err := mgr.SetQOTDConfig("g1", QOTDConfig{
		Enabled:           true,
		QuestionChannelID: "123456789012345678",
		ResponseChannelID: "223456789012345678",
	})
	if !errors.Is(err, saveErr) {
		t.Fatalf("expected save error, got %v", err)
	}

	cfg := mgr.SnapshotConfig()
	if len(cfg.Guilds) != 1 {
		t.Fatalf("expected guild config to remain intact, got %+v", cfg.Guilds)
	}
	if !cfg.Guilds[0].QOTD.IsZero() {
		t.Fatalf("expected qotd config rollback, got %+v", cfg.Guilds[0].QOTD)
	}
}
