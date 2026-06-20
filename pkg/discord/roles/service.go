package roles

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/utils/httputil"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

const (
	RolePanelComponentRouteID  = "roles_panel:toggle"
	rolePanelCustomIDSeparator = "|"
	rolePanelMaxButtonsPerRow  = 5

	discordErrUnknownChannel = 10003
	discordErrUnknownMessage = 10008
)

type rolePanelSyncFailure struct {
	Posting files.RolePanelPostingConfig
	Err     error
}

type rolePanelSyncResult struct {
	Edited  int
	Dropped []files.RolePanelPostingConfig
	Failed  []rolePanelSyncFailure
}

func (r rolePanelSyncResult) HasIssues() bool {
	return len(r.Dropped) > 0 || len(r.Failed) > 0
}

type RolePanelService struct {
	configManager *files.ConfigManager
	editMessage   func(client *api.Client, channelID discord.ChannelID, messageID discord.MessageID, data api.EditMessageData) error
	dropPostings  func(cm *files.ConfigManager, guildID, key string, messageIDs []string) error
}

func NewRolePanelService(configManager *files.ConfigManager) *RolePanelService {
	return &RolePanelService{
		configManager: configManager,
		editMessage:   defaultRolePanelEditMessage,
		dropPostings:  defaultRolePanelDropPostings,
	}
}

func RolePanelButtonCustomID(roleID string) string {
	return RolePanelComponentRouteID + rolePanelCustomIDSeparator + strings.TrimSpace(roleID)
}

func RolePanelButtonRoleIDFromCustomID(customID string) string {
	prefix := RolePanelComponentRouteID + rolePanelCustomIDSeparator
	if !strings.HasPrefix(customID, prefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(customID, prefix))
}

func (s *RolePanelService) Post(client *api.Client, channelID discord.ChannelID, panel files.RolePanelConfig) (*discord.Message, error) {
	embed := s.RenderEmbed(&panel)
	components := s.RenderComponents(&panel)

	data := api.SendMessageData{
		Embeds:     []discord.Embed{embed},
		Components: components,
	}
	return client.SendMessageComplex(channelID, data)
}

func (s *RolePanelService) DeletePosting(client *api.Client, channelID discord.ChannelID, messageID discord.MessageID) error {
	return client.DeleteMessage(channelID, messageID, "Role panel unposted via command")
}

func (s *RolePanelService) Sync(
	client *api.Client,
	guildID string,
	key string,
	postings []files.RolePanelPostingConfig,
	panel *files.RolePanelConfig,
) rolePanelSyncResult {
	var result rolePanelSyncResult
	if len(postings) == 0 {
		return result
	}

	var embeds []discord.Embed
	var components discord.ContainerComponents

	if panel != nil {
		embeds = []discord.Embed{s.RenderEmbed(panel)}
		components = s.RenderComponents(panel)
	}

	for _, posting := range postings {
		chID, errCh := discord.ParseSnowflake(posting.ChannelID)
		msgID, errMsg := discord.ParseSnowflake(posting.MessageID)
		if errCh != nil || errMsg != nil {
			result.Failed = append(result.Failed, rolePanelSyncFailure{Posting: posting, Err: errors.New("invalid snowflake")})
			continue
		}

		data := api.EditMessageData{
			Embeds:     &embeds,
			Components: &components,
		}

		// Ignoring webhook message edits for now as Arikawa client edit covers bot messages.
		err := s.editMessage(client, discord.ChannelID(chID), discord.MessageID(msgID), data)
		if err == nil {
			result.Edited++
			continue
		}

		if isRolePanelPostingMissingError(err) {
			result.Dropped = append(result.Dropped, posting)
			continue
		}

		result.Failed = append(result.Failed, rolePanelSyncFailure{Posting: posting, Err: err})
	}

	if len(result.Dropped) > 0 {
		ids := make([]string, 0, len(result.Dropped))
		for _, p := range result.Dropped {
			ids = append(ids, p.MessageID)
		}
		if dropErr := s.dropPostings(s.configManager, guildID, key, ids); dropErr != nil {
			slog.Warn("Service degradation intercepted and mitigated",
				slog.String("reason", "Role panel batch posting cleanup failed"),
				slog.String("guildID", guildID),
				slog.String("key", key),
				slog.String("error", dropErr.Error()),
			)
		}
	}

	return result
}

func (s *RolePanelService) RenderEmbed(panel *files.RolePanelConfig) discord.Embed {
	embed := discord.Embed{}
	if title := strings.TrimSpace(panel.Title); title != "" {
		embed.Title = title
	}
	if desc := strings.TrimSpace(panel.Description); desc != "" {
		embed.Description = desc
	}
	if panel.Color > 0 {
		embed.Color = discord.Color(panel.Color)
	}

	authorName := strings.TrimSpace(panel.AuthorName)
	authorIcon := strings.TrimSpace(panel.AuthorIconURL)
	if authorName != "" || authorIcon != "" {
		embed.Author = &discord.EmbedAuthor{
			Name: authorName,
			Icon: authorIcon,
		}
	}

	footerText := strings.TrimSpace(panel.FooterText)
	footerIcon := strings.TrimSpace(panel.FooterIconURL)
	if footerText != "" || footerIcon != "" {
		embed.Footer = &discord.EmbedFooter{
			Text: footerText,
			Icon: footerIcon,
		}
	}

	if imageURL := strings.TrimSpace(panel.ImageURL); imageURL != "" {
		embed.Image = &discord.EmbedImage{URL: imageURL}
	}
	if thumbnailURL := strings.TrimSpace(panel.ThumbnailURL); thumbnailURL != "" {
		embed.Thumbnail = &discord.EmbedThumbnail{URL: thumbnailURL}
	}

	if len(panel.Fields) > 0 {
		embed.Fields = make([]discord.EmbedField, 0, len(panel.Fields))
		for _, f := range panel.Fields {
			embed.Fields = append(embed.Fields, discord.EmbedField{
				Name:   f.Name,
				Value:  f.Value,
				Inline: f.Inline,
			})
		}
	}

	return embed
}

func (s *RolePanelService) RenderComponents(panel *files.RolePanelConfig) discord.ContainerComponents {
	if len(panel.Buttons) == 0 {
		return nil
	}

	var rows discord.ContainerComponents
	var current discord.ActionRowComponent

	for _, b := range panel.Buttons {
		if len(current) == rolePanelMaxButtonsPerRow {
			c := current
			rows = append(rows, &c)
			current = discord.ActionRowComponent{}
		}
		current = append(current, buildRolePanelButton(b))
	}
	if len(current) > 0 {
		c := current
		rows = append(rows, &c)
	}
	return rows
}

func buildRolePanelButton(b files.RolePanelButtonConfig) *discord.ButtonComponent {
	button := &discord.ButtonComponent{
		Style:    discord.SecondaryButtonStyle(),
		Label:    strings.TrimSpace(b.Label),
		CustomID: discord.ComponentID(RolePanelButtonCustomID(b.RoleID)),
	}
	if b.HasEmoji() {
		id, _ := discord.ParseSnowflake(b.EmojiID)
		button.Emoji = &discord.ComponentEmoji{
			Name:     strings.TrimSpace(b.EmojiName),
			ID:       discord.EmojiID(id),
			Animated: b.EmojiAnimated,
		}
	}
	return button
}

func (s *RolePanelService) FormatSyncSummary(result rolePanelSyncResult, action string) string {
	if !result.HasIssues() && result.Edited == 0 {
		return ""
	}
	var lines []string
	if result.Edited > 0 {
		lines = append(lines, fmt.Sprintf("%s %d posting(s).", action, result.Edited))
	}
	if len(result.Dropped) > 0 {
		ids := make([]string, 0, len(result.Dropped))
		for _, p := range result.Dropped {
			ids = append(ids, p.MessageID)
		}
		lines = append(lines, fmt.Sprintf("Dropped %d orphaned posting(s) (message gone): %s.", len(result.Dropped), strings.Join(ids, ", ")))
	}
	if len(result.Failed) > 0 {
		details := make([]string, 0, len(result.Failed))
		for _, f := range result.Failed {
			details = append(details, fmt.Sprintf("message_id=%s (%v)", f.Posting.MessageID, f.Err))
		}
		lines = append(lines, fmt.Sprintf("Could not reconcile %d posting(s); these are kept on file for retry: %s.", len(result.Failed), strings.Join(details, "; ")))
	}
	return strings.Join(lines, "\n")
}

func FormatRolePanelButtonForList(b files.RolePanelButtonConfig) string {
	var sb strings.Builder
	if b.HasEmoji() {
		sb.WriteString(formatButtonEmojiDisplay(b))
		sb.WriteString(" ")
	}
	sb.WriteString("`")
	sb.WriteString(b.Label)
	sb.WriteString("` → <@&")
	sb.WriteString(b.RoleID)
	sb.WriteString(">")
	return sb.String()
}

func formatButtonEmojiDisplay(b files.RolePanelButtonConfig) string {
	name := strings.TrimSpace(b.EmojiName)
	if id := strings.TrimSpace(b.EmojiID); id != "" {
		prefix := ":"
		if b.EmojiAnimated {
			prefix = "a:"
		}
		if name == "" {
			name = "emoji"
		}
		return "<" + prefix + name + ":" + id + ">"
	}
	return name
}

func isRolePanelPostingMissingError(err error) bool {
	var httpErr *httputil.HTTPError
	if errors.As(err, &httpErr) {
		return httpErr.Code == discordErrUnknownChannel || httpErr.Code == discordErrUnknownMessage
	}
	return false
}

func defaultRolePanelEditMessage(client *api.Client, channelID discord.ChannelID, messageID discord.MessageID, data api.EditMessageData) error {
	if client == nil {
		return errors.New("discord client is nil")
	}
	_, err := client.EditMessageComplex(channelID, messageID, data)
	return err
}

func defaultRolePanelDropPostings(cm *files.ConfigManager, guildID, key string, messageIDs []string) error {
	if cm == nil {
		return errors.New("config manager is nil")
	}
	return cm.RemoveRolePanelPostings(guildID, key, messageIDs)
}
