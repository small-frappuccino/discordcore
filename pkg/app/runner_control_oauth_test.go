package app

import "testing"

func TestLoadControlDiscordOAuthConfigFromEnv(t *testing.T) {
	t.Run("not configured", func(t *testing.T) {
		t.Setenv(controlDiscordOAuthClientIDEnv, "")
		t.Setenv(controlDiscordOAuthClientSecretEnv, "")
		t.Setenv(controlDiscordOAuthRedirectURIEnv, "")
		t.Setenv(controlDiscordOAuthIncludeGuildMembersReadEnv, "")

		cfg, err := loadControlDiscordOAuthConfigFromEnv()
		if err != nil {
			t.Fatalf("expected no error when oauth config is absent, got %v", err)
		}
		if cfg != nil {
			t.Fatalf("expected nil config when oauth env vars are absent, got %+v", cfg)
		}
	})

	t.Run("incomplete config fails", func(t *testing.T) {
		t.Setenv(controlDiscordOAuthClientIDEnv, "client-id")
		t.Setenv(controlDiscordOAuthClientSecretEnv, "")
		t.Setenv(controlDiscordOAuthRedirectURIEnv, "")
		t.Setenv(controlDiscordOAuthIncludeGuildMembersReadEnv, "")

		cfg, err := loadControlDiscordOAuthConfigFromEnv()
		if err == nil {
			t.Fatalf("expected error for incomplete oauth config, got cfg=%+v", cfg)
		}
	})

	t.Run("include member scope without credentials fails", func(t *testing.T) {
		t.Setenv(controlDiscordOAuthClientIDEnv, "")
		t.Setenv(controlDiscordOAuthClientSecretEnv, "")
		t.Setenv(controlDiscordOAuthRedirectURIEnv, "")
		t.Setenv(controlDiscordOAuthIncludeGuildMembersReadEnv, "true")

		cfg, err := loadControlDiscordOAuthConfigFromEnv()
		if err == nil {
			t.Fatalf("expected error for include scope without oauth credentials, got cfg=%+v", cfg)
		}
	})

	t.Run("complete config", func(t *testing.T) {
		t.Setenv(controlDiscordOAuthClientIDEnv, "client-id")
		t.Setenv(controlDiscordOAuthClientSecretEnv, "client-secret")
		t.Setenv(controlDiscordOAuthRedirectURIEnv, "http://127.0.0.1:8080/auth/discord/callback")
		t.Setenv(controlDiscordOAuthIncludeGuildMembersReadEnv, "true")

		cfg, err := loadControlDiscordOAuthConfigFromEnv()
		if err != nil {
			t.Fatalf("expected complete oauth config to parse, got %v", err)
		}
		if cfg == nil {
			t.Fatal("expected non-nil oauth config")
		}
		if cfg.ClientID != "client-id" || cfg.ClientSecret != "client-secret" {
			t.Fatalf("unexpected client credentials in cfg: %+v", cfg)
		}
		if cfg.RedirectURI != "http://127.0.0.1:8080/auth/discord/callback" {
			t.Fatalf("unexpected redirect URI: %+v", cfg)
		}
		if !cfg.IncludeGuildsMembersRead {
			t.Fatalf("expected IncludeGuildsMembersRead=true, got %+v", cfg)
		}
	})
}
