package messages

import (
	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/state"
	"github.com/small-frappuccino/discordcore/pkg/messages"
)

// ArikawaAdapter implements the domain messages.DiscordAdapter interface
// using the Arikawa SDK state.
type ArikawaAdapter struct {
	state *state.State
}

// NewArikawaAdapter creates a new ArikawaAdapter.
func NewArikawaAdapter(s *state.State) *ArikawaAdapter {
	return &ArikawaAdapter{state: s}
}

func (a *ArikawaAdapter) ChannelGuildID(channelID string) (string, error) {
	chID, err := discord.ParseSnowflake(channelID)
	if err != nil {
		return "", err
	}
	ch, err := a.state.Channel(discord.ChannelID(chID))
	if err != nil {
		return "", err
	}
	return ch.GuildID.String(), nil
}

func (a *ArikawaAdapter) MessageContent(channelID, messageID string) (string, error) {
	chID, err := discord.ParseSnowflake(channelID)
	if err != nil {
		return "", err
	}
	msgID, err := discord.ParseSnowflake(messageID)
	if err != nil {
		return "", err
	}
	msg, err := a.state.Message(discord.ChannelID(chID), discord.MessageID(msgID))
	if err != nil {
		return "", err
	}
	return msg.Content, nil
}

func (a *ArikawaAdapter) IsMessageAuthorBot(channelID, messageID string) (bool, error) {
	chID, err := discord.ParseSnowflake(channelID)
	if err != nil {
		return false, err
	}
	msgID, err := discord.ParseSnowflake(messageID)
	if err != nil {
		return false, err
	}
	msg, err := a.state.Message(discord.ChannelID(chID), discord.MessageID(msgID))
	if err != nil {
		return false, err
	}
	return msg.Author.Bot, nil
}

func (a *ArikawaAdapter) Username(userID string) (string, error) {
	uID, err := discord.ParseSnowflake(userID)
	if err != nil {
		return "", err
	}
	usr, err := a.state.User(discord.UserID(uID))
	if err != nil {
		return "", err
	}
	return usr.Username, nil
}

func (a *ArikawaAdapter) FetchMessageDeleteAuditLogs(guildID string) ([]messages.AuditLogMessageDeleteEntry, error) {
	gID, err := discord.ParseSnowflake(guildID)
	if err != nil {
		return nil, err
	}
	data := api.AuditLogData{
		ActionType: discord.MessageDelete,
		Limit:      10,
	}
	al, err := a.state.Client.AuditLog(discord.GuildID(gID), data)
	if err != nil {
		return nil, err
	}

	var results []messages.AuditLogMessageDeleteEntry
	for _, entry := range al.Entries {
		if entry.ActionType != discord.MessageDelete {
			continue
		}
		var channelID string
		if entry.Options.ChannelID != 0 {
			channelID = entry.Options.ChannelID.String()
		}

		results = append(results, messages.AuditLogMessageDeleteEntry{
			EntryID:   entry.ID.String(),
			TargetID:  entry.TargetID.String(),
			UserID:    entry.UserID.String(),
			ChannelID: channelID,
		})
	}
	return results, nil
}
