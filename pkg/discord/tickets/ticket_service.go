package tickets

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	"github.com/small-frappuccino/discordcore/pkg/storage"
	"github.com/small-frappuccino/discordgo"
)

// TicketService represents ticket service.
type TicketService struct {
	store *storage.Store
}

// NewTicketService news ticket service.
func NewTicketService(store *storage.Store) *TicketService {
	return &TicketService{store: store}
}

// HandleCategorySelect handles category select.
func (s *TicketService) HandleCategorySelect(ctx *core.Context) error {
	data := ctx.Interaction.MessageComponentData()
	if len(data.Values) == 0 {
		return &core.CommandError{Message: "No category selected.", Ephemeral: true}
	}
	categoryName := data.Values[0]

	// 1. Resolve role from Config
	if ctx.GuildConfig == nil || !ctx.GuildConfig.Tickets.Enabled {
		return &core.CommandError{Message: "Tickets are not enabled on this server.", Ephemeral: true}
	}

	var roleID string
	for _, cat := range ctx.GuildConfig.Tickets.Categories {
		if strings.EqualFold(cat.Name, categoryName) {
			roleID = cat.RoleID
			break
		}
	}
	if roleID == "" {
		return &core.CommandError{Message: "Invalid category selected.", Ephemeral: true}
	}

	// 2. Check 500 channels limit
	channels, err := ctx.Session.GuildChannels(ctx.GuildID)
	if err != nil {
		return fmt.Errorf("fetch channels: %w", err)
	}
	if len(channels) >= 490 {
		return &core.CommandError{Message: "Cannot create ticket: The server is nearing the 500 channel limit.", Ephemeral: true}
	}

	// 3. Get next ticket ID
	timeoutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	nextID, err := s.store.NextTicketID(timeoutCtx, ctx.GuildID)
	if err != nil {
		return fmt.Errorf("next ticket id: %w", err)
	}

	channelName := fmt.Sprintf("ticket-%04d", nextID)

	// 4. Create channel
	overwrites := []*discordgo.PermissionOverwrite{
		{
			ID:   ctx.GuildID, // @everyone
			Type: discordgo.PermissionOverwriteTypeRole,
			Deny: discordgo.PermissionViewChannel,
		},
		{
			ID:    ctx.UserID,
			Type:  discordgo.PermissionOverwriteTypeMember,
			Allow: discordgo.PermissionViewChannel | discordgo.PermissionSendMessages | discordgo.PermissionReadMessageHistory,
		},
		{
			ID:    roleID,
			Type:  discordgo.PermissionOverwriteTypeRole,
			Allow: discordgo.PermissionViewChannel | discordgo.PermissionSendMessages | discordgo.PermissionReadMessageHistory,
		},
	}

	// Attempt to find a parent category if the select menu's channel has one
	var parentID string
	if ch, err := ctx.Session.State.Channel(ctx.Interaction.ChannelID); err == nil && ch.ParentID != "" {
		parentID = ch.ParentID
	}

	createdChannel, err := ctx.Session.GuildChannelCreateComplex(ctx.GuildID, discordgo.GuildChannelCreateData{
		Name:                 channelName,
		Type:                 discordgo.ChannelTypeGuildText,
		PermissionOverwrites: overwrites,
		ParentID:             parentID,
	})
	if err != nil {
		return fmt.Errorf("create channel: %w", err)
	}

	// 5. Send initial message
	msgContent := fmt.Sprintf("Welcome <@%s>! You have opened a ticket for **%s**.\n<@&%s> will be with you shortly.", ctx.UserID, categoryName, roleID)
	_, err = ctx.Session.ChannelMessageSendComplex(createdChannel.ID, &discordgo.MessageSend{
		Content: msgContent,
		Components: []discordgo.MessageComponent{
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.Button{
						CustomID: "ticket_close",
						Label:    "Close Ticket",
						Style:    discordgo.DangerButton,
					},
				},
			},
		},
		AllowedMentions: &discordgo.MessageAllowedMentions{
			Roles: []string{roleID},
			Users: []string{ctx.UserID},
		},
	})
	if err != nil {
		return fmt.Errorf("send initial message: %w", err)
	}

	// 6. Acknowledge the interaction
	return core.NewResponseBuilder(ctx.Session).WithContext(ctx).Ephemeral().Success(ctx.Interaction, fmt.Sprintf("Ticket created: <#%s>", createdChannel.ID))
}

// HandleClose handles close.
func (s *TicketService) HandleClose(ctx *core.Context) error {
	channelID := ctx.Interaction.ChannelID
	ch, err := ctx.Session.Channel(channelID)
	if err != nil {
		return fmt.Errorf("fetch channel: %w", err)
	}

	if !strings.HasPrefix(ch.Name, "ticket-") {
		return &core.CommandError{Message: "This is not an open ticket.", Ephemeral: true}
	}

	newName := strings.Replace(ch.Name, "ticket-", "closed-", 1)

	// Update name and permissions (remove SendMessages from user)
	// To do this properly, we need to find the user's overwrite and update it.
	// We'll just deny SendMessages for the initiator (assuming we can track them, or just deny for everyone except staff).
	// But the spec says: "Remoção atômica do bit SendMessages no bloco de permissão customizada do usuário iniciador."
	// We can try to infer the initiator if they are the one clicking close, or we can just iterate overwrites and remove SendMessages from all members.
	// Let's iterate all member overwrites and remove SendMessages.
	for _, ow := range ch.PermissionOverwrites {
		if ow.Type == discordgo.PermissionOverwriteTypeMember {
			err = ctx.Session.ChannelPermissionSet(channelID, ow.ID, discordgo.PermissionOverwriteTypeMember, ow.Allow & ^discordgo.PermissionSendMessages, ow.Deny|discordgo.PermissionSendMessages)
			if err != nil {
				return fmt.Errorf("update permissions: %w", err)
			}
		}
	}

	_, err = ctx.Session.ChannelEdit(channelID, &discordgo.ChannelEdit{
		Name: newName,
	})
	if err != nil {
		return fmt.Errorf("rename channel: %w", err)
	}

	// Send management panel
	_, err = ctx.Session.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
		Content: "Ticket closed.",
		Components: []discordgo.MessageComponent{
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.Button{
						CustomID: "ticket_transcript",
						Label:    "Transcript",
						Style:    discordgo.PrimaryButton,
					},
					discordgo.Button{
						CustomID: "ticket_reopen",
						Label:    "Reopen",
						Style:    discordgo.SecondaryButton,
					},
					discordgo.Button{
						CustomID: "ticket_delete",
						Label:    "Delete",
						Style:    discordgo.DangerButton,
					},
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("send management panel: %w", err)
	}

	// Ack
	return core.NewResponseBuilder(ctx.Session).WithContext(ctx).Ephemeral().Success(ctx.Interaction, "Ticket has been closed.")
}

// HandleReopen handles reopen.
func (s *TicketService) HandleReopen(ctx *core.Context) error {
	channelID := ctx.Interaction.ChannelID
	ch, err := ctx.Session.Channel(channelID)
	if err != nil {
		return fmt.Errorf("fetch channel: %w", err)
	}

	if !strings.HasPrefix(ch.Name, "closed-") {
		return &core.CommandError{Message: "This is not a closed ticket.", Ephemeral: true}
	}

	newName := strings.Replace(ch.Name, "closed-", "ticket-", 1)

	for _, ow := range ch.PermissionOverwrites {
		if ow.Type == discordgo.PermissionOverwriteTypeMember {
			err = ctx.Session.ChannelPermissionSet(channelID, ow.ID, discordgo.PermissionOverwriteTypeMember, ow.Allow|discordgo.PermissionSendMessages, ow.Deny & ^discordgo.PermissionSendMessages)
			if err != nil {
				return fmt.Errorf("update permissions: %w", err)
			}
		}
	}

	_, err = ctx.Session.ChannelEdit(channelID, &discordgo.ChannelEdit{
		Name: newName,
	})
	if err != nil {
		return fmt.Errorf("rename channel: %w", err)
	}

	_, err = ctx.Session.ChannelMessageSend(channelID, "Ticket reopened.")
	if err != nil {
		return fmt.Errorf("send reopen message: %w", err)
	}

	return core.NewResponseBuilder(ctx.Session).WithContext(ctx).Ephemeral().Success(ctx.Interaction, "Ticket has been reopened.")
}

// HandleDelete handles delete.
func (s *TicketService) HandleDelete(ctx *core.Context) error {
	channelID := ctx.Interaction.ChannelID
	_, err := ctx.Session.ChannelDelete(channelID)
	if err != nil {
		return fmt.Errorf("delete channel: %w", err)
	}
	return nil
}
