package logging

import (
	"strings"
	"time"

	"github.com/alice-bnuy/discordcore/v2/internal/files"
	"github.com/alice-bnuy/logutil"
	"github.com/bwmarrin/discordgo"
)

// AutomodService listens to messages and enforces a simple keyword-based moderation.
type AutomodService struct {
	session       *discordgo.Session
	configManager *files.ConfigManager
	isRunning     bool
}

func NewAutomodService(session *discordgo.Session, configManager *files.ConfigManager) *AutomodService {
	return &AutomodService{
		session:       session,
		configManager: configManager,
	}
}

// Start registers handlers.
func (as *AutomodService) Start() {
	if as.isRunning {
		return
	}
	as.isRunning = true

	// Use Discord native AutoMod: listen for action execution events
	as.session.AddHandler(as.handleAutoModerationAction)
}

// Stop stops the service (no-op for now).
func (as *AutomodService) Stop() {
	if !as.isRunning {
		return
	}
	as.isRunning = false
}

// handleAutoModerationAction logs native AutoMod events to the configured automod log channel
func (as *AutomodService) handleAutoModerationAction(s *discordgo.Session, e *discordgo.AutoModerationActionExecution) {
	if e == nil || e.GuildID == "" {
		return
	}
	// Find guild config for logging
	guildCfg := as.configManager.GuildConfig(e.GuildID)
	if guildCfg == nil {
		return
	}

	logChannelID := guildCfg.AutomodLogChannelID
	if logChannelID == "" {
		logChannelID = guildCfg.CommandChannelID
	}
	if logChannelID == "" {
		return
	}

	// Build embed from event data
	title := "AutoMod action executed"
	desc := "A native AutoMod rule was triggered."
	embed := &discordgo.MessageEmbed{
		Title:       title,
		Description: desc,
		Color:       0xFF5555,
		Timestamp:   time.Now().Format(time.RFC3339),
		Fields: []*discordgo.MessageEmbedField{
			{Name: "User", Value: "<@" + e.UserID + "> (``" + e.UserID + "``)", Inline: true},
			{Name: "Channel", Value: func() string {
				if e.ChannelID != "" {
					return "<#" + e.ChannelID + ">"
				}
				return "(DM/unknown)"
			}(), Inline: true},
		},
	}

	// Include rule info
	if e.RuleID != "" {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{Name: "Rule ID", Value: "``" + e.RuleID + "``", Inline: true})
	}
	if e.MatchedKeyword != "" {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{Name: "Matched", Value: "``" + e.MatchedKeyword + "``", Inline: true})
	}
	// Include a short excerpt (content or matched content if present)
	content := e.Content
	if strings.TrimSpace(content) == "" && e.MatchedContent != "" {
		content = e.MatchedContent
	}
	if strings.TrimSpace(content) != "" {
		excerpt := content
		if len(excerpt) > 200 {
			excerpt = excerpt[:200] + "…"
		}
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "Excerpt",
			Value:  "```" + sanitizeForCodeBlock(excerpt) + "```",
			Inline: false,
		})
	}

	if _, err := s.ChannelMessageSendEmbed(logChannelID, embed); err != nil {
		logutil.WithFields(map[string]interface{}{
			"guildID":   e.GuildID,
			"channelID": logChannelID,
			"userID":    e.UserID,
			"error":     err,
		}).Error("Failed to send native automod log message")
	}
}

// sanitizeForCodeBlock prevents breaking out of the code fence and removes backticks.
func sanitizeForCodeBlock(input string) string {
	// Replace backticks and normalize newlines for safer preview in a code block
	s := strings.ReplaceAll(input, "`", "'")
	// Discord code blocks tolerate newlines; keep them but trim excessive whitespace
	return strings.TrimSpace(s)
}
