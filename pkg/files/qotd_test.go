package files

import (
	"errors"
	"testing"
)

func TestNormalizeQOTDConfigRequiresForumAndTagsWhenEnabled(t *testing.T) {
	t.Parallel()

	if _, err := NormalizeQOTDConfig(QOTDConfig{Enabled: true}); err == nil {
		t.Fatal("expected enabled qotd config without forum/tag ids to fail")
	}
}

func TestSetQOTDConfigCanonicalizesCoreFields(t *testing.T) {
	t.Parallel()

	mgr, _ := newTransactionalTestManager(t, &BotConfig{
		Guilds: []GuildConfig{{GuildID: "g1"}},
	}, nil)

	err := mgr.SetQOTDConfig("g1", QOTDConfig{
		Enabled:        true,
		ForumChannelID: " 123456789012345678 ",
		QuestionTagID:  " 223456789012345678 ",
		ReplyTagID:     " 323456789012345678 ",
	})
	if err != nil {
		t.Fatalf("SetQOTDConfig() failed: %v", err)
	}

	cfg, err := mgr.GetQOTDConfig("g1")
	if err != nil {
		t.Fatalf("GetQOTDConfig() failed: %v", err)
	}
	if !cfg.Enabled {
		t.Fatal("expected qotd config to remain enabled")
	}
	if cfg.ForumChannelID != "123456789012345678" {
		t.Fatalf("expected trimmed forum channel id, got %q", cfg.ForumChannelID)
	}
	if cfg.QuestionTagID != "223456789012345678" {
		t.Fatalf("expected trimmed question tag id, got %q", cfg.QuestionTagID)
	}
	if cfg.ReplyTagID != "323456789012345678" {
		t.Fatalf("expected trimmed reply tag id, got %q", cfg.ReplyTagID)
	}
}

func TestSetQOTDConfigRollsBackOnSaveError(t *testing.T) {
	t.Parallel()

	saveErr := errors.New("save failed")
	mgr, _ := newTransactionalTestManager(t, &BotConfig{
		Guilds: []GuildConfig{{GuildID: "g1"}},
	}, saveErr)

	err := mgr.SetQOTDConfig("g1", QOTDConfig{
		Enabled:        true,
		ForumChannelID: "123456789012345678",
		QuestionTagID:  "223456789012345678",
		ReplyTagID:     "323456789012345678",
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
