package files

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
)

func newWebhookUpdatesTestManager(t *testing.T, cfg *BotConfig) *ConfigManager {
	t.Helper()

	if cfg == nil {
		cfg = &BotConfig{Guilds: []GuildConfig{}}
	}
	if cfg.Guilds == nil {
		cfg.Guilds = []GuildConfig{}
	}

	mgr := NewConfigManagerWithPath(filepath.Join(t.TempDir(), "settings.json"))
	mgr.config = cfg
	return mgr
}

func TestWebhookEmbedUpdatesCRUDGlobal(t *testing.T) {
	t.Parallel()

	mgr := newWebhookUpdatesTestManager(t, nil)
	initial := WebhookEmbedUpdateConfig{
		MessageID:  "100",
		WebhookURL: "https://discord.com/api/webhooks/1/token",
		Embed:      json.RawMessage(`{"title":"initial"}`),
	}

	if err := mgr.CreateWebhookEmbedUpdate("", initial); err != nil {
		t.Fatalf("create global webhook update: %v", err)
	}

	list, err := mgr.ListWebhookEmbedUpdates("global")
	if err != nil {
		t.Fatalf("list global webhook updates: %v", err)
	}
	if len(list) != 1 || list[0].MessageID != "100" {
		t.Fatalf("unexpected list after create: %+v", list)
	}

	got, err := mgr.GetWebhookEmbedUpdate("", "100")
	if err != nil {
		t.Fatalf("get global webhook update: %v", err)
	}
	if got.WebhookURL != initial.WebhookURL {
		t.Fatalf("unexpected webhook_url: got %q want %q", got.WebhookURL, initial.WebhookURL)
	}

	update := WebhookEmbedUpdateConfig{
		MessageID:  "101",
		WebhookURL: "https://discord.com/api/webhooks/1/token-updated",
		Embed:      json.RawMessage(`{"title":"updated"}`),
	}
	if err := mgr.UpdateWebhookEmbedUpdate("", "100", update); err != nil {
		t.Fatalf("update global webhook update: %v", err)
	}

	got, err = mgr.GetWebhookEmbedUpdate("", "101")
	if err != nil {
		t.Fatalf("get updated webhook update: %v", err)
	}
	if got.WebhookURL != update.WebhookURL {
		t.Fatalf("unexpected updated webhook_url: got %q want %q", got.WebhookURL, update.WebhookURL)
	}

	if err := mgr.DeleteWebhookEmbedUpdate("", "101"); err != nil {
		t.Fatalf("delete global webhook update: %v", err)
	}

	list, err = mgr.ListWebhookEmbedUpdates("")
	if err != nil {
		t.Fatalf("list after delete: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("expected empty list after delete, got %+v", list)
	}
}

func TestWebhookEmbedUpdatesCRUDGuildScope(t *testing.T) {
	t.Parallel()

	mgr := newWebhookUpdatesTestManager(t, &BotConfig{
		Guilds: []GuildConfig{
			{GuildID: "g1"},
		},
	})

	item := WebhookEmbedUpdateConfig{
		MessageID:  "200",
		WebhookURL: "https://discord.com/api/webhooks/2/token",
		Embed:      json.RawMessage(`{"title":"guild"}`),
	}
	if err := mgr.CreateWebhookEmbedUpdate("g1", item); err != nil {
		t.Fatalf("create guild webhook update: %v", err)
	}

	guildList, err := mgr.ListWebhookEmbedUpdates("g1")
	if err != nil {
		t.Fatalf("list guild webhook updates: %v", err)
	}
	if len(guildList) != 1 || guildList[0].MessageID != "200" {
		t.Fatalf("unexpected guild list: %+v", guildList)
	}

	globalList, err := mgr.ListWebhookEmbedUpdates("")
	if err != nil {
		t.Fatalf("list global webhook updates: %v", err)
	}
	if len(globalList) != 0 {
		t.Fatalf("expected no global updates, got %+v", globalList)
	}

	if err := mgr.CreateWebhookEmbedUpdate("g-missing", item); err == nil {
		t.Fatal("expected error for missing guild scope")
	}
}

func TestWebhookEmbedUpdatesCreateValidationAndDuplicates(t *testing.T) {
	t.Parallel()

	mgr := newWebhookUpdatesTestManager(t, nil)

	if err := mgr.CreateWebhookEmbedUpdate("", WebhookEmbedUpdateConfig{}); err == nil {
		t.Fatal("expected validation error for empty payload")
	}

	if err := mgr.CreateWebhookEmbedUpdate("", WebhookEmbedUpdateConfig{
		MessageID:  "300",
		WebhookURL: "not-a-url",
		Embed:      json.RawMessage(`{"title":"ok"}`),
	}); err == nil {
		t.Fatal("expected validation error for invalid webhook_url format")
	}

	if err := mgr.CreateWebhookEmbedUpdate("", WebhookEmbedUpdateConfig{
		MessageID:  "300",
		WebhookURL: "https://discord.com/api/webhooks/3",
		Embed:      json.RawMessage(`{"title":"ok"}`),
	}); err == nil {
		t.Fatal("expected validation error for webhook_url without token")
	}

	if err := mgr.CreateWebhookEmbedUpdate("", WebhookEmbedUpdateConfig{
		MessageID:  "300",
		WebhookURL: "https://discord.com/api/webhooks/abc/token",
		Embed:      json.RawMessage(`{"title":"ok"}`),
	}); err == nil {
		t.Fatal("expected validation error for non-numeric webhook id")
	}

	if err := mgr.CreateWebhookEmbedUpdate("", WebhookEmbedUpdateConfig{
		MessageID:  "300",
		WebhookURL: "https://discord.com/api/v10/webhooks/3/token",
		Embed:      json.RawMessage(`"not-object"`),
	}); err == nil {
		t.Fatal("expected validation error for non-object/non-array embed payload")
	}

	valid := WebhookEmbedUpdateConfig{
		MessageID:  "300",
		WebhookURL: "https://discord.com/api/v10/webhooks/3/token",
		Embed:      json.RawMessage(`{"title":"ok"}`),
	}
	if err := mgr.CreateWebhookEmbedUpdate("", valid); err != nil {
		t.Fatalf("create valid webhook update: %v", err)
	}
	if err := mgr.CreateWebhookEmbedUpdate("", valid); !errors.Is(err, ErrWebhookEmbedUpdateAlreadyExists) {
		t.Fatalf("expected duplicate error, got %v", err)
	}
}

func TestWebhookEmbedUpdatesLegacyFallbackMigration(t *testing.T) {
	t.Parallel()

	mgr := newWebhookUpdatesTestManager(t, &BotConfig{
		RuntimeConfig: RuntimeConfig{
			WebhookEmbedUpdate: WebhookEmbedUpdateConfig{
				MessageID:  "legacy",
				WebhookURL: "https://discord.com/api/webhooks/9/token",
				Embed:      json.RawMessage(`{"title":"legacy"}`),
			},
		},
	})

	list, err := mgr.ListWebhookEmbedUpdates("")
	if err != nil {
		t.Fatalf("list with legacy fallback: %v", err)
	}
	if len(list) != 1 || list[0].MessageID != "legacy" {
		t.Fatalf("expected legacy item in list fallback, got %+v", list)
	}

	if err := mgr.CreateWebhookEmbedUpdate("", WebhookEmbedUpdateConfig{
		MessageID:  "new",
		WebhookURL: "https://discord.com/api/webhooks/10/token",
		Embed:      json.RawMessage(`{"title":"new"}`),
	}); err != nil {
		t.Fatalf("create new item while legacy exists: %v", err)
	}

	cfg := mgr.Config()
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if !cfg.RuntimeConfig.WebhookEmbedUpdate.IsZero() {
		t.Fatalf("expected legacy single-field key to be cleared after canonical write, got %+v", cfg.RuntimeConfig.WebhookEmbedUpdate)
	}
	if len(cfg.RuntimeConfig.WebhookEmbedUpdates) != 2 {
		t.Fatalf("expected canonical list with 2 items, got %+v", cfg.RuntimeConfig.WebhookEmbedUpdates)
	}
}

func TestWebhookEmbedUpdatesUpdateDeleteNotFound(t *testing.T) {
	t.Parallel()

	mgr := newWebhookUpdatesTestManager(t, nil)
	update := WebhookEmbedUpdateConfig{
		MessageID:  "401",
		WebhookURL: "https://discord.com/api/webhooks/4/token",
		Embed:      json.RawMessage(`{"title":"u"}`),
	}

	if err := mgr.UpdateWebhookEmbedUpdate("", "missing", update); !errors.Is(err, ErrWebhookEmbedUpdateNotFound) {
		t.Fatalf("expected not found on update, got %v", err)
	}
	if err := mgr.DeleteWebhookEmbedUpdate("", "missing"); !errors.Is(err, ErrWebhookEmbedUpdateNotFound) {
		t.Fatalf("expected not found on delete, got %v", err)
	}
}
