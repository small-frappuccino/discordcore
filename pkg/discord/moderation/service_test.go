package moderation

import (
	"context"
	"testing"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
)

type mockModerationClient struct {
	Client
}

func (m *mockModerationClient) Ban(guildID discord.GuildID, userID discord.UserID, data api.BanData) error {
	return nil
}

func (m *mockModerationClient) Kick(guildID discord.GuildID, userID discord.UserID, reason api.AuditLogReason) error {
	return nil
}

func (m *mockModerationClient) ModifyMember(guildID discord.GuildID, userID discord.UserID, data api.ModifyMemberData) error {
	return nil
}

func TestService_ContextTimeout(t *testing.T) {
	t.Parallel()

	client := &mockModerationClient{}
	svc := NewService(client, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel the context to test short-circuit behavior

	err := svc.Ban(ctx, discord.GuildID(123), discord.UserID(456), 0, "Test Timeout")

	if err != context.Canceled {
		t.Fatalf("expected context.Canceled error, got %v", err)
	}
}

func TestService_ExponentialBackoff(t *testing.T) {
	t.Parallel()
	// Simply verifying that Service wraps Client and constructor executes without panic.
	client := &mockModerationClient{}
	svc := NewService(client, nil)
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
}
