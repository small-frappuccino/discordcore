package files

import (
	"encoding/json"
	"errors"
	"testing"
)

type transactionalTestStore struct {
	cfg     *BotConfig
	saveErr error
}

func (s *transactionalTestStore) Load() (*BotConfig, error) {
	if s == nil || s.cfg == nil {
		return &BotConfig{Guilds: []GuildConfig{}}, nil
	}
	return cloneBotConfigPtr(s.cfg), nil
}

func (s *transactionalTestStore) Save(cfg *BotConfig) error {
	if s.saveErr != nil {
		return s.saveErr
	}
	s.cfg = cloneBotConfigPtr(cfg)
	return nil
}

func (s *transactionalTestStore) Exists() (bool, error) {
	return s != nil && s.cfg != nil, nil
}

func (s *transactionalTestStore) Describe() string {
	return "transactional-test-store"
}

func newTransactionalTestManager(t *testing.T, cfg *BotConfig, saveErr error) (*ConfigManager, *transactionalTestStore) {
	t.Helper()

	if cfg == nil {
		cfg = &BotConfig{Guilds: []GuildConfig{}}
	}
	if cfg.Guilds == nil {
		cfg.Guilds = []GuildConfig{}
	}

	store := &transactionalTestStore{
		cfg:     cloneBotConfigPtr(cfg),
		saveErr: saveErr,
	}
	mgr := NewConfigManagerWithStore(store)
	mgr.config = cloneBotConfigPtr(cfg)
	if _, err := mgr.rebuildGuildIndexLocked("test"); err != nil {
		t.Fatalf("rebuild index: %v", err)
	}
	return mgr, store
}

func TestUpdateRuntimeConfigRollsBackOnSaveError(t *testing.T) {
	t.Parallel()

	saveErr := errors.New("save failed")
	mgr, _ := newTransactionalTestManager(t, &BotConfig{
		RuntimeConfig: RuntimeConfig{BotTheme: "halloween"},
	}, saveErr)

	_, err := mgr.UpdateRuntimeConfig(func(rc *RuntimeConfig) error {
		rc.BotTheme = "winter"
		return nil
	})
	if !errors.Is(err, saveErr) {
		t.Fatalf("expected save error, got %v", err)
	}

	if got := mgr.SnapshotConfig().RuntimeConfig.BotTheme; got != "halloween" {
		t.Fatalf("expected runtime config rollback, got %q", got)
	}
}

func TestSetPartnerBoardTargetRollsBackOnSaveError(t *testing.T) {
	t.Parallel()

	saveErr := errors.New("save failed")
	mgr, _ := newTransactionalTestManager(t, &BotConfig{
		Guilds: []GuildConfig{{GuildID: "g1"}},
	}, saveErr)

	err := mgr.SetPartnerBoardTarget("g1", EmbedUpdateTargetConfig{
		Type:       EmbedUpdateTargetTypeWebhookMessage,
		MessageID:  "123456789012345678",
		WebhookURL: "https://discord.com/api/webhooks/123456789012345678/token",
	})
	if !errors.Is(err, saveErr) {
		t.Fatalf("expected save error, got %v", err)
	}

	cfg := mgr.SnapshotConfig()
	if len(cfg.Guilds) != 1 {
		t.Fatalf("expected guild config to remain intact, got %+v", cfg.Guilds)
	}
	if !cfg.Guilds[0].PartnerBoard.Target.IsZero() {
		t.Fatalf("expected partner board target rollback, got %+v", cfg.Guilds[0].PartnerBoard.Target)
	}
}

func TestCreateWebhookEmbedUpdateRollsBackOnSaveError(t *testing.T) {
	t.Parallel()

	saveErr := errors.New("save failed")
	mgr, _ := newTransactionalTestManager(t, &BotConfig{}, saveErr)

	err := mgr.CreateWebhookEmbedUpdate("", WebhookEmbedUpdateConfig{
		MessageID:  "100",
		WebhookURL: "https://discord.com/api/webhooks/1/token",
		Embed:      json.RawMessage(`{"title":"initial"}`),
	})
	if !errors.Is(err, saveErr) {
		t.Fatalf("expected save error, got %v", err)
	}

	cfg := mgr.SnapshotConfig()
	if updates := cfg.RuntimeConfig.NormalizedWebhookEmbedUpdates(); len(updates) != 0 {
		t.Fatalf("expected webhook updates rollback, got %+v", updates)
	}
}
