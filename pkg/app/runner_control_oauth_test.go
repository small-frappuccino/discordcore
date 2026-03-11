package app

import (
	"path/filepath"
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/util"
)

func TestLoadControlDiscordOAuthConfigFromEnv(t *testing.T) {
	t.Run("not configured", func(t *testing.T) {
		t.Setenv(controlDiscordOAuthClientIDEnv, "")
		t.Setenv(controlDiscordOAuthClientSecretEnv, "")
		t.Setenv(controlDiscordOAuthRedirectURIEnv, "")
		t.Setenv(controlDiscordOAuthIncludeGuildMembersReadEnv, "")
		t.Setenv(controlDiscordOAuthSessionStorePathEnv, "")

		cfg, err := loadControlDiscordOAuthConfigFromEnv("")
		if err != nil {
			t.Fatalf("expected no error when oauth config is absent, got %v", err)
		}
		if cfg != nil {
			t.Fatalf("expected nil config when oauth env vars are absent, got %+v", cfg)
		}
	})

	t.Run("incomplete config fails", func(t *testing.T) {
		t.Setenv(controlDiscordOAuthClientIDEnv, "")
		t.Setenv(controlDiscordOAuthClientSecretEnv, "")
		t.Setenv(controlDiscordOAuthRedirectURIEnv, "https://example.invalid/auth/discord/callback")
		t.Setenv(controlDiscordOAuthIncludeGuildMembersReadEnv, "")
		t.Setenv(controlDiscordOAuthSessionStorePathEnv, "")

		cfg, err := loadControlDiscordOAuthConfigFromEnv("")
		if err == nil {
			t.Fatalf("expected error for incomplete oauth config, got cfg=%+v", cfg)
		}
	})

	t.Run("include member scope without credentials fails", func(t *testing.T) {
		t.Setenv(controlDiscordOAuthClientIDEnv, "")
		t.Setenv(controlDiscordOAuthClientSecretEnv, "")
		t.Setenv(controlDiscordOAuthRedirectURIEnv, "")
		t.Setenv(controlDiscordOAuthIncludeGuildMembersReadEnv, "true")
		t.Setenv(controlDiscordOAuthSessionStorePathEnv, "")

		cfg, err := loadControlDiscordOAuthConfigFromEnv("")
		if err == nil {
			t.Fatalf("expected error for include scope without oauth credentials, got cfg=%+v", cfg)
		}
	})

	t.Run("complete config uses repo client id", func(t *testing.T) {
		t.Setenv(controlDiscordOAuthClientIDEnv, "")
		t.Setenv(controlDiscordOAuthClientSecretEnv, "client-secret")
		t.Setenv(controlDiscordOAuthRedirectURIEnv, "http://127.0.0.1:8080/auth/discord/callback")
		t.Setenv(controlDiscordOAuthIncludeGuildMembersReadEnv, "true")
		t.Setenv(controlDiscordOAuthSessionStorePathEnv, "")

		cfg, err := loadControlDiscordOAuthConfigFromEnv("")
		if err != nil {
			t.Fatalf("expected complete oauth config to parse, got %v", err)
		}
		if cfg == nil {
			t.Fatal("expected non-nil oauth config")
		}
		if cfg.ClientID != defaultControlDiscordOAuthClientID || cfg.ClientSecret != "client-secret" {
			t.Fatalf("unexpected client credentials in cfg: %+v", cfg)
		}
		if cfg.RedirectURI != "http://127.0.0.1:8080/auth/discord/callback" {
			t.Fatalf("unexpected redirect URI: %+v", cfg)
		}
		if !cfg.IncludeGuildsMembersRead {
			t.Fatalf("expected IncludeGuildsMembersRead=true, got %+v", cfg)
		}
		wantStorePath := filepath.Join(util.ApplicationCachesPath, "control", "oauth_sessions.json")
		if cfg.SessionStorePath != wantStorePath {
			t.Fatalf("unexpected default oauth session store path: got=%q want=%q", cfg.SessionStorePath, wantStorePath)
		}
	})

	t.Run("missing redirect without public origin fails", func(t *testing.T) {
		t.Setenv(controlDiscordOAuthClientIDEnv, "")
		t.Setenv(controlDiscordOAuthClientSecretEnv, "client-secret")
		t.Setenv(controlDiscordOAuthRedirectURIEnv, "")
		t.Setenv(controlDiscordOAuthIncludeGuildMembersReadEnv, "false")
		t.Setenv(controlDiscordOAuthSessionStorePathEnv, "")

		cfg, err := loadControlDiscordOAuthConfigFromEnv("")
		if err == nil {
			t.Fatalf("expected missing redirect without public origin to fail, got cfg=%+v", cfg)
		}
	})

	t.Run("missing redirect derives from public origin", func(t *testing.T) {
		t.Setenv(controlDiscordOAuthClientIDEnv, "")
		t.Setenv(controlDiscordOAuthClientSecretEnv, "client-secret")
		t.Setenv(controlDiscordOAuthRedirectURIEnv, "")
		t.Setenv(controlDiscordOAuthIncludeGuildMembersReadEnv, "false")
		t.Setenv(controlDiscordOAuthSessionStorePathEnv, "")

		cfg, err := loadControlDiscordOAuthConfigFromEnv("https://alice.localhost:8443")
		if err != nil {
			t.Fatalf("expected missing redirect to derive from public origin, got %v", err)
		}
		if cfg == nil {
			t.Fatal("expected non-nil oauth config")
		}
		if cfg.RedirectURI != "https://alice.localhost:8443/auth/discord/callback" {
			t.Fatalf("unexpected derived redirect URI: %+v", cfg)
		}
	})

	t.Run("explicit client id overrides repo default", func(t *testing.T) {
		t.Setenv(controlDiscordOAuthClientIDEnv, "override-client-id")
		t.Setenv(controlDiscordOAuthClientSecretEnv, "client-secret")
		t.Setenv(controlDiscordOAuthRedirectURIEnv, "http://127.0.0.1:8080/auth/discord/callback")
		t.Setenv(controlDiscordOAuthIncludeGuildMembersReadEnv, "false")
		t.Setenv(controlDiscordOAuthSessionStorePathEnv, "")

		cfg, err := loadControlDiscordOAuthConfigFromEnv("")
		if err != nil {
			t.Fatalf("expected override client id config to parse, got %v", err)
		}
		if cfg == nil {
			t.Fatal("expected non-nil oauth config")
		}
		if cfg.ClientID != "override-client-id" {
			t.Fatalf("expected env override client id, got %+v", cfg)
		}
	})

	t.Run("explicit session store path", func(t *testing.T) {
		t.Setenv(controlDiscordOAuthClientIDEnv, "")
		t.Setenv(controlDiscordOAuthClientSecretEnv, "client-secret")
		t.Setenv(controlDiscordOAuthRedirectURIEnv, "http://127.0.0.1:8080/auth/discord/callback")
		t.Setenv(controlDiscordOAuthIncludeGuildMembersReadEnv, "false")
		t.Setenv(controlDiscordOAuthSessionStorePathEnv, "/tmp/oauth-sessions.json")

		cfg, err := loadControlDiscordOAuthConfigFromEnv("")
		if err != nil {
			t.Fatalf("expected complete oauth config to parse, got %v", err)
		}
		if cfg == nil {
			t.Fatal("expected non-nil oauth config")
		}
		if cfg.SessionStorePath != "/tmp/oauth-sessions.json" {
			t.Fatalf("unexpected oauth session store path: %+v", cfg)
		}
	})
}
