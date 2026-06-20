package tickets

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/state"
	"github.com/diamondburned/arikawa/v3/utils/sendpart"
	pkgtickets "github.com/small-frappuccino/discordcore/pkg/tickets"
	"golang.org/x/sync/errgroup"
)

// Service encapsulates the Arikawa-specific operations for tickets.
type Service struct {
	state *state.State
}

// NewService constructs the Discord ticket service.
func NewService(state *state.State) *Service {
	return &Service{state: state}
}

// CreateTicketChannel spawns the ticket channel and applies initial permissions.
func (s *Service) CreateTicketChannel(ctx context.Context, guildID discord.GuildID, memberID discord.UserID, roleID discord.RoleID, channelName string, parentID discord.ChannelID) (*discord.Channel, error) {
	overwrites := []discord.Overwrite{
		{
			ID:   discord.Snowflake(guildID),
			Type: discord.OverwriteRole,
			Deny: discord.PermissionViewChannel,
		},
		{
			ID:    discord.Snowflake(memberID),
			Type:  discord.OverwriteMember,
			Allow: pkgtickets.ComputeOpenMemberAllow(),
		},
		{
			ID:    discord.Snowflake(roleID),
			Type:  discord.OverwriteRole,
			Allow: pkgtickets.ComputeOpenRoleAllow(),
		},
	}

	data := api.CreateChannelData{
		Name:       channelName,
		Type:       discord.GuildText,
		Overwrites: overwrites,
	}
	if parentID.IsValid() {
		data.CategoryID = parentID
	}

	ch, err := s.state.Client.CreateChannel(guildID, data)
	if err != nil {
		return nil, fmt.Errorf("create channel: %w", err)
	}

	return ch, nil
}

// FetchTranscript streams messages from the channel and encodes them as JSON.
func (s *Service) FetchTranscript(ctx context.Context, channelID discord.ChannelID, w io.WriteCloser) error {
	defer w.Close()

	if _, err := w.Write([]byte("[")); err != nil {
		return err
	}

	enc := json.NewEncoder(w)
	var beforeID discord.MessageID
	first := true

	for {
		var messages []discord.Message
		var err error
		if beforeID.IsValid() {
			messages, err = s.state.Client.MessagesBefore(channelID, beforeID, 100)
		} else {
			messages, err = s.state.Client.Messages(channelID, 100)
		}

		if err != nil {
			return fmt.Errorf("fetch messages: %w", err)
		}

		if len(messages) == 0 {
			break
		}

		for _, msg := range messages {
			if !first {
				if _, err := w.Write([]byte(",")); err != nil {
					return err
				}
			}
			first = false
			if err := enc.Encode(msg); err != nil {
				return err
			}
		}

		beforeID = messages[len(messages)-1].ID

		if len(messages) < 100 {
			break
		}
	}

	if _, err := w.Write([]byte("]")); err != nil {
		return err
	}

	return nil
}

// GenerateAndUploadTranscript coordinates transcript generation via an io.Pipe and errgroup.
func (s *Service) GenerateAndUploadTranscript(ctx context.Context, channelID, auditChannelID discord.ChannelID) error {
	pr, pw := io.Pipe()

	var eg errgroup.Group

	// Producer
	eg.Go(func() error {
		err := s.FetchTranscript(ctx, channelID, pw)
		if err != nil {
			// Critical for io.Pipe deadlocks invariant: propagate error immediately.
			pw.CloseWithError(err)
		}
		return err
	})

	// Consumer
	defer pr.Close()
	fileName := fmt.Sprintf("transcript-%s.json", channelID.String())
	data := api.SendMessageData{
		Content: fmt.Sprintf("Transcript for ticket <#%s> (Channel ID: %s)", channelID, channelID),
		Files: []sendpart.File{
			{
				Name:   fileName,
				Reader: pr,
			},
		},
	}

	_, uploadErr := s.state.Client.SendMessageComplex(auditChannelID, data)
	if uploadErr != nil {
		pr.CloseWithError(uploadErr)
	}

	encodeErr := eg.Wait()

	if uploadErr != nil {
		return fmt.Errorf("upload transcript: %w", uploadErr)
	}
	if encodeErr != nil {
		return fmt.Errorf("encode transcript: %w", encodeErr)
	}

	return nil
}

// CloseTicket locks a ticket by altering member permissions and renaming the channel.
func (s *Service) CloseTicket(ctx context.Context, ch *discord.Channel) error {
	newName := pkgtickets.OpenToClosedName(ch.Name)

	for _, ow := range ch.Overwrites {
		if ow.Type == discord.OverwriteMember {
			newAllow := pkgtickets.ComputeCloseMemberAllow(ow.Allow)
			newDeny := pkgtickets.ComputeCloseMemberDeny(ow.Deny)
			err := s.state.Client.EditChannelPermission(ch.ID, ow.ID, api.EditChannelPermissionData{
				Type:  discord.OverwriteMember,
				Allow: newAllow,
				Deny:  newDeny,
			})
			if err != nil {
				return fmt.Errorf("update permissions: %w", err)
			}
		}
	}

	err := s.state.Client.ModifyChannel(ch.ID, api.ModifyChannelData{
		Name: newName,
	})
	if err != nil {
		return fmt.Errorf("rename channel: %w", err)
	}

	return nil
}

// ReopenTicket unlocks a closed ticket.
func (s *Service) ReopenTicket(ctx context.Context, ch *discord.Channel) error {
	newName := pkgtickets.ClosedToOpenName(ch.Name)

	for _, ow := range ch.Overwrites {
		if ow.Type == discord.OverwriteMember {
			newAllow := pkgtickets.ComputeReopenMemberAllow(ow.Allow)
			newDeny := pkgtickets.ComputeReopenMemberDeny(ow.Deny)
			err := s.state.Client.EditChannelPermission(ch.ID, ow.ID, api.EditChannelPermissionData{
				Type:  discord.OverwriteMember,
				Allow: newAllow,
				Deny:  newDeny,
			})
			if err != nil {
				return fmt.Errorf("update permissions: %w", err)
			}
		}
	}

	err := s.state.Client.ModifyChannel(ch.ID, api.ModifyChannelData{
		Name: newName,
	})
	if err != nil {
		return fmt.Errorf("rename channel: %w", err)
	}

	return nil
}

// DeleteTicket completely removes the channel.
func (s *Service) DeleteTicket(ctx context.Context, channelID discord.ChannelID) error {
	return s.state.Client.DeleteChannel(channelID, api.AuditLogReason(""))
}
