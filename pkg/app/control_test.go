package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/files"
)

func TestLoadControlDiscordOAuthConfigFromEnv(t *testing.T) {
	t.Run("default empty is nil", func(t *testing.T) {
		t.Setenv(controlDiscordOAuthClientIDEnv, "")
		t.Setenv(controlDiscordOAuthClientSecretEnv, "")
		t.Setenv(controlDiscordOAuthRedirectURIEnv, "")
		t.Setenv(controlDiscordOAuthIncludeGuildMembersReadEnv, "")
		t.Setenv(controlDiscordOAuthSessionStorePathEnv, "")

		cfg, err := loadControlDiscordOAuthConfigFromEnv("")
		if err != nil {
			t.Fatalf("expected nil error on empty oauth config, got %v", err)
		}
		if cfg != nil {
			t.Fatalf("expected nil config when env vars are absent, got %+v", cfg)
		}
	})

	t.Run("incomplete config fails", func(t *testing.T) {
		t.Setenv(controlDiscordOAuthClientIDEnv, "client-id")
		t.Setenv(controlDiscordOAuthClientSecretEnv, "")

		if _, err := loadControlDiscordOAuthConfigFromEnv(""); err == nil {
			t.Fatal("expected error for incomplete oauth config")
		}
	})

	t.Run("complete config", func(t *testing.T) {
		t.Setenv(controlDiscordOAuthClientIDEnv, "client-id")
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
		if cfg.ClientID != "client-id" || cfg.ClientSecret != "client-secret" {
			t.Fatalf("unexpected client credentials in cfg: %+v", cfg)
		}
		if cfg.RedirectURI != "http://127.0.0.1:8080/auth/discord/callback" {
			t.Fatalf("unexpected redirect URI: %+v", cfg)
		}
		if !cfg.IncludeGuildsMembersRead {
			t.Fatalf("expected IncludeGuildsMembersRead=true, got %+v", cfg)
		}
		wantStorePath := filepath.Join(files.ApplicationCachesPath, "control", "oauth_sessions.json")
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

		cfg, err := loadControlDiscordOAuthConfigFromEnv("https://bot.localhost:8443")
		if err != nil {
			t.Fatalf("expected missing redirect to derive from public origin, got %v", err)
		}
		if cfg == nil {
			t.Fatal("expected non-nil oauth config")
		}
		if cfg.RedirectURI != "https://bot.localhost:8443/auth/discord/callback" {
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
}

func TestLoadControlTLSFilesFromEnv(t *testing.T) {
	t.Run("not configured", func(t *testing.T) {
		t.Setenv(controlTLSCertFileEnv, "")
		t.Setenv(controlTLSKeyFileEnv, "")

		certFile, keyFile, err := loadControlTLSFilesFromEnv()
		if err != nil {
			t.Fatalf("expected no error when TLS config is absent, got %v", err)
		}
		if certFile != "" || keyFile != "" {
			t.Fatalf("expected empty TLS config, got cert=%q key=%q", certFile, keyFile)
		}
	})

	t.Run("incomplete config fails", func(t *testing.T) {
		t.Setenv(controlTLSCertFileEnv, "/tmp/cert.pem")
		t.Setenv(controlTLSKeyFileEnv, "")

		if _, _, err := loadControlTLSFilesFromEnv(); err == nil {
			t.Fatal("expected error for incomplete TLS config")
		}
	})

	t.Run("complete config", func(t *testing.T) {
		t.Setenv(controlTLSCertFileEnv, "/tmp/cert.pem")
		t.Setenv(controlTLSKeyFileEnv, "/tmp/key.pem")

		certFile, keyFile, err := loadControlTLSFilesFromEnv()
		if err != nil {
			t.Fatalf("expected complete TLS config to parse, got %v", err)
		}
		if certFile != "/tmp/cert.pem" || keyFile != "/tmp/key.pem" {
			t.Fatalf("unexpected TLS config: cert=%q key=%q", certFile, keyFile)
		}
	})
}

func TestResolveControlRuntimeUsesManagedLocalHTTPS(t *testing.T) {
	tempAppData := t.TempDir()
	t.Setenv("APPDATA", tempAppData)
	t.Setenv(controlTLSCertFileEnv, "")
	t.Setenv(controlTLSKeyFileEnv, "")
	t.Setenv(controlDiscordOAuthClientSecretEnv, "")
	t.Setenv(controlDiscordOAuthRedirectURIEnv, "")
	files.SetAppName("discordmain-run-options-test")

	runtime, err := resolveControlRuntime(context.Background(), RunOptions{
		Profile: RunProfileDiscordMain,
		Control: ControlOptions{
			LocalHTTPS: ControlLocalHTTPSOptions{
				Enabled:   true,
				AutoTrust: false,
			},
		},
	})
	if err != nil {
		t.Fatalf("resolve control runtime: %v", err)
	}
	if runtime.bindAddr != defaultLocalHTTPSControlAddr {
		t.Fatalf("unexpected bind addr: %+v", runtime)
	}
	if runtime.publicOrigin != defaultLocalHTTPSPublicOriginForProfile(RunProfileDiscordMain) {
		t.Fatalf("unexpected public origin: %+v", runtime)
	}
	if runtime.tlsCertFile == "" || runtime.tlsKeyFile == "" {
		t.Fatalf("expected managed local tls files, got %+v", runtime)
	}
	if _, err := os.Stat(runtime.tlsCertFile); err != nil {
		t.Fatalf("stat managed cert file: %v", err)
	}
	if _, err := os.Stat(runtime.tlsKeyFile); err != nil {
		t.Fatalf("stat managed key file: %v", err)
	}
}

func TestResolveControlRuntimeDerivesOAuthRedirectFromPublicOrigin(t *testing.T) {
	t.Setenv(controlTLSCertFileEnv, "")
	t.Setenv(controlTLSKeyFileEnv, "")
	t.Setenv(controlDiscordOAuthClientSecretEnv, "client-secret")
	t.Setenv(controlDiscordOAuthRedirectURIEnv, "")
	t.Setenv(controlDiscordOAuthSessionStorePathEnv, "")

	tempAppData := t.TempDir()
	t.Setenv("APPDATA", tempAppData)
	files.SetAppName("discordmain-run-options-test")

	runtime, err := resolveControlRuntime(context.Background(), RunOptions{
		Profile: RunProfileDiscordMain,
		Control: ControlOptions{
			PublicOrigin: "https://discordmain.localhost:8443",
		},
	})
	if err != nil {
		t.Fatalf("resolve control runtime: %v", err)
	}
	if runtime.oauthConfig == nil {
		t.Fatal("expected oauth config")
	}
	if runtime.oauthConfig.RedirectURI != "https://discordmain.localhost:8443/auth/discord/callback" {
		t.Fatalf("unexpected derived redirect uri: %+v", runtime.oauthConfig)
	}
	wantStorePath := filepath.Join(files.ApplicationCachesPath, "control", "oauth_sessions.json")
	if runtime.oauthConfig.SessionStorePath != wantStorePath {
		t.Fatalf("unexpected oauth session store path: got=%q want=%q", runtime.oauthConfig.SessionStorePath, wantStorePath)
	}
}
