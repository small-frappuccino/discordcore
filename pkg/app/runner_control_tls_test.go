package app

import (
	"reflect"
	"testing"

	"github.com/bwmarrin/discordgo"
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
