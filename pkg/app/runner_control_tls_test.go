package app

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/util"
)

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

func TestListBotGuildIDsFromSessionState(t *testing.T) {
	t.Run("nil session fails", func(t *testing.T) {
		if _, err := listBotGuildIDsFromSessionState(nil); err == nil {
			t.Fatal("expected error for nil session")
		}
	})

	t.Run("deduplicates and trims", func(t *testing.T) {
		state := discordgo.NewState()
		state.Ready.Guilds = []*discordgo.Guild{
			{ID: "g1"},
			{ID: " g1 "},
			{ID: "g2"},
			nil,
			{ID: ""},
		}
		session := &discordgo.Session{State: state}

		got, err := listBotGuildIDsFromSessionState(session)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		want := []string{"g1", "g2"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("unexpected guild ids: got=%v want=%v", got, want)
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
	util.SetAppName("alicebot-run-options-test")

	runtime, err := resolveControlRuntime(context.Background(), RunOptions{
		Control: ControlOptions{
			BindAddr:     defaultLocalHTTPSControlAddr,
			PublicOrigin: defaultLocalHTTPSPublicOrigin,
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
	if runtime.publicOrigin != defaultLocalHTTPSPublicOrigin {
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
	util.SetAppName("alicebot-run-options-test")

	runtime, err := resolveControlRuntime(context.Background(), RunOptions{
		Control: ControlOptions{
			PublicOrigin: "https://alice.localhost:8443",
		},
	})
	if err != nil {
		t.Fatalf("resolve control runtime: %v", err)
	}
	if runtime.oauthConfig == nil {
		t.Fatal("expected oauth config")
	}
	if runtime.oauthConfig.RedirectURI != "https://alice.localhost:8443/auth/discord/callback" {
		t.Fatalf("unexpected derived redirect uri: %+v", runtime.oauthConfig)
	}
	wantStorePath := filepath.Join(util.ApplicationCachesPath, "control", "oauth_sessions.json")
	if runtime.oauthConfig.SessionStorePath != wantStorePath {
		t.Fatalf("unexpected oauth session store path: got=%q want=%q", runtime.oauthConfig.SessionStorePath, wantStorePath)
	}
}
