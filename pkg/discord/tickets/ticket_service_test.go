//go:build integration

package tickets

import (
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	"github.com/small-frappuccino/discordcore/pkg/log"
)

func TestHandleCategorySelect_Success(t *testing.T) {
	guildID := "g1"
	ownerID := "u1"
	h := newTicketCommandTestHarness(t, guildID, ownerID)

	interaction := newTicketInteraction(guildID, "user-1", "channel-1", "ticket_category_select", []string{"Contact Staff"})

	ctx := &core.Context{
		Session:     h.session,
		Interaction: interaction,
		Config:      h.cm,
		GuildID:     guildID,
		UserID:      "user-1",
		GuildConfig: h.cm.GuildConfig(guildID),
		Logger:      log.GlobalLogger,
	}

	err := h.svc.HandleCategorySelect(ctx)
	if err != nil {
		t.Fatalf("HandleCategorySelect failed: %v", err)
	}

	h.rec.mu.Lock()
	defer h.rec.mu.Unlock()

	if len(h.rec.channelCreates) != 1 {
		t.Fatalf("expected exactly 1 channel create, got %d", len(h.rec.channelCreates))
	}

	cc := h.rec.channelCreates[0]
	if !strings.HasPrefix(cc.Name, "ticket-") {
		t.Errorf("expected channel name to start with ticket-, got %q", cc.Name)
	}

	// Verify permissions
	hasRolePerm := false
	hasUserPerm := false
	for _, ow := range cc.PermissionOverwrites {
		if ow.ID == "role-staff" && ow.Type == discordgo.PermissionOverwriteTypeRole {
			if ow.Allow&discordgo.PermissionSendMessages != 0 {
				hasRolePerm = true
			}
		}
		if ow.ID == "user-1" && ow.Type == discordgo.PermissionOverwriteTypeMember {
			if ow.Allow&discordgo.PermissionSendMessages != 0 {
				hasUserPerm = true
			}
		}
	}

	if !hasRolePerm {
		t.Error("expected role-staff to have SendMessages permission")
	}
	if !hasUserPerm {
		t.Error("expected user-1 to have SendMessages permission")
	}

	if len(h.rec.responses) != 1 {
		t.Fatalf("expected 1 interaction response, got %d", len(h.rec.responses))
	}
}

func TestHandleClose_Success(t *testing.T) {
	guildID := "g1"
	ownerID := "u1"
	h := newTicketCommandTestHarness(t, guildID, ownerID)

	interaction := newTicketInteraction(guildID, "user-1", "ticket-123-id", "ticket_close", nil)

	ctx := &core.Context{
		Session:     h.session,
		Interaction: interaction,
		Config:      h.cm,
		GuildID:     guildID,
		UserID:      "user-1",
		GuildConfig: h.cm.GuildConfig(guildID),
		Logger:      log.GlobalLogger,
	}

	err := h.svc.HandleClose(ctx)
	if err != nil {
		t.Fatalf("HandleClose failed: %v", err)
	}

	h.rec.mu.Lock()
	defer h.rec.mu.Unlock()

	if len(h.rec.channelEdits) != 1 {
		t.Fatalf("expected exactly 1 channel edit, got %d", len(h.rec.channelEdits))
	}

	ce := h.rec.channelEdits[0]
	if !strings.HasPrefix(ce.Name, "closed-") {
		t.Errorf("expected channel name to start with closed-, got %q", ce.Name)
	}

	if len(h.rec.messageSends) != 1 {
		t.Fatalf("expected exactly 1 message send (panel), got %d", len(h.rec.messageSends))
	}
}

func TestHandleReopen_Success(t *testing.T) {
	guildID := "g1"
	ownerID := "u1"
	h := newTicketCommandTestHarness(t, guildID, ownerID)

	interaction := newTicketInteraction(guildID, "user-1", "closed-123-id", "ticket_reopen", nil)

	ctx := &core.Context{
		Session:     h.session,
		Interaction: interaction,
		Config:      h.cm,
		GuildID:     guildID,
		UserID:      "user-1",
		GuildConfig: h.cm.GuildConfig(guildID),
		Logger:      log.GlobalLogger,
	}

	err := h.svc.HandleReopen(ctx)
	if err != nil {
		t.Fatalf("HandleReopen failed: %v", err)
	}

	h.rec.mu.Lock()
	defer h.rec.mu.Unlock()

	if len(h.rec.channelEdits) != 1 {
		t.Fatalf("expected exactly 1 channel edit, got %d", len(h.rec.channelEdits))
	}

	ce := h.rec.channelEdits[0]
	if !strings.HasPrefix(ce.Name, "ticket-") {
		t.Errorf("expected channel name to start with ticket-, got %q", ce.Name)
	}
}

func TestHandleCategorySelect_LimitReached(t *testing.T) {
	guildID := "g1"
	ownerID := "u1"
	h := newTicketCommandTestHarness(t, guildID, ownerID)

	// Mock 490 channels
	h.rec.mu.Lock()
	h.rec.mockChannelCount = 490
	h.rec.mu.Unlock()

	interaction := newTicketInteraction(guildID, "user-1", "channel-1", "ticket_category_select", []string{"Contact Staff"})
	ctx := &core.Context{
		Session:     h.session,
		Interaction: interaction,
		Config:      h.cm,
		GuildID:     guildID,
		UserID:      "user-1",
		GuildConfig: h.cm.GuildConfig(guildID),
		Logger:      log.GlobalLogger,
	}

	err := h.svc.HandleCategorySelect(ctx)
	if err == nil {
		t.Fatalf("expected error due to limit reached, got nil")
	}
	if !strings.Contains(err.Error(), "500 channel limit") {
		t.Errorf("expected 500 channel limit error, got %v", err)
	}
}

func TestHandleCategorySelect_ConfigDisabled(t *testing.T) {
	guildID := "g1"
	ownerID := "u1"
	h := newTicketCommandTestHarness(t, guildID, ownerID)

	// Disable config
	cfg := h.cm.GuildConfig(guildID)
	cfg.Tickets.Enabled = false
	h.cm.AddGuildConfig(*cfg)

	interaction := newTicketInteraction(guildID, "user-1", "channel-1", "ticket_category_select", []string{"Contact Staff"})
	ctx := &core.Context{
		Session:     h.session,
		Interaction: interaction,
		Config:      h.cm,
		GuildID:     guildID,
		UserID:      "user-1",
		GuildConfig: h.cm.GuildConfig(guildID),
		Logger:      log.GlobalLogger,
	}

	err := h.svc.HandleCategorySelect(ctx)
	if err == nil {
		t.Fatalf("expected error due to disabled tickets, got nil")
	}
	if !strings.Contains(err.Error(), "Tickets are not enabled") {
		t.Errorf("expected Tickets are not enabled error, got %v", err)
	}
}

func TestHandleCategorySelect_InvalidCategory(t *testing.T) {
	guildID := "g1"
	ownerID := "u1"
	h := newTicketCommandTestHarness(t, guildID, ownerID)

	interaction := newTicketInteraction(guildID, "user-1", "channel-1", "ticket_category_select", []string{"Invalid Category"})
	ctx := &core.Context{
		Session:     h.session,
		Interaction: interaction,
		Config:      h.cm,
		GuildID:     guildID,
		UserID:      "user-1",
		GuildConfig: h.cm.GuildConfig(guildID),
		Logger:      log.GlobalLogger,
	}

	err := h.svc.HandleCategorySelect(ctx)
	if err == nil {
		t.Fatalf("expected error due to invalid category, got nil")
	}
	if !strings.Contains(err.Error(), "Invalid category") {
		t.Errorf("expected Invalid category error, got %v", err)
	}
}

func TestHandleClose_NotOpenTicket(t *testing.T) {
	guildID := "g1"
	ownerID := "u1"
	h := newTicketCommandTestHarness(t, guildID, ownerID)

	interaction := newTicketInteraction(guildID, "user-1", "closed-123-id", "ticket_close", nil)
	ctx := &core.Context{
		Session:     h.session,
		Interaction: interaction,
		Config:      h.cm,
		GuildID:     guildID,
		UserID:      "user-1",
		GuildConfig: h.cm.GuildConfig(guildID),
		Logger:      log.GlobalLogger,
	}

	err := h.svc.HandleClose(ctx)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "not an open ticket") {
		t.Errorf("expected not an open ticket error, got %v", err)
	}
}

func TestHandleReopen_NotClosedTicket(t *testing.T) {
	guildID := "g1"
	ownerID := "u1"
	h := newTicketCommandTestHarness(t, guildID, ownerID)

	interaction := newTicketInteraction(guildID, "user-1", "ticket-123-id", "ticket_reopen", nil)
	ctx := &core.Context{
		Session:     h.session,
		Interaction: interaction,
		Config:      h.cm,
		GuildID:     guildID,
		UserID:      "user-1",
		GuildConfig: h.cm.GuildConfig(guildID),
		Logger:      log.GlobalLogger,
	}

	err := h.svc.HandleReopen(ctx)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "not a closed ticket") {
		t.Errorf("expected not a closed ticket error, got %v", err)
	}
}

func TestHandleDelete_Success(t *testing.T) {
	guildID := "g1"
	ownerID := "u1"
	h := newTicketCommandTestHarness(t, guildID, ownerID)

	interaction := newTicketInteraction(guildID, "user-1", "closed-123-id", "ticket_delete", nil)
	ctx := &core.Context{
		Session:     h.session,
		Interaction: interaction,
		Config:      h.cm,
		GuildID:     guildID,
		UserID:      "user-1",
		GuildConfig: h.cm.GuildConfig(guildID),
		Logger:      log.GlobalLogger,
	}

	err := h.svc.HandleDelete(ctx)
	if err != nil {
		t.Fatalf("HandleDelete failed: %v", err)
	}
}
