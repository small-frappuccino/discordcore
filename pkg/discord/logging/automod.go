package logging

import (
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/task"
	"github.com/small-frappuccino/discordcore/pkg/theme"
)

// AutomodService listens to messages and enforces a simple keyword-based moderation.
type AutomodService struct {
	session       *discordgo.Session
	configManager *files.ConfigManager
	adapters      *task.NotificationAdapters
	isRunning     bool

	// unsubscribe function for the registered handler
	handlerCancel func()
}

func NewAutomodService(session *discordgo.Session, configManager *files.ConfigManager) *AutomodService {
	return &AutomodService{
		session:       session,
		configManager: configManager,
	}
}

// SetAdapters allows wiring TaskRouter adapters for async notifications.
func (as *AutomodService) SetAdapters(adapters *task.NotificationAdapters) {
	as.adapters = adapters
}

// Start registers handlers.
func (as *AutomodService) Start() {
	if as.isRunning {
		return
	}
	as.isRunning = true

	// Use Discord native AutoMod: listen for action execution events
	as.handlerCancel = as.session.AddHandler(as.handleAutoModerationAction)
}

// Stop stops the service (no-op for now).
func (as *AutomodService) Stop() {
	if !as.isRunning {
		return
	}
	if as.handlerCancel != nil {
		as.handlerCancel()
		as.handlerCancel = nil
	}
	as.isRunning = false
}

// handleAutoModerationAction logs native AutoMod events to the configured automod log channel
func (as *AutomodService) handleAutoModerationAction(s *discordgo.Session, e *discordgo.AutoModerationActionExecution) {
	if e == nil || e.GuildID == "" {
		return
	}

	cfg := as.configManager.Config()
	if cfg != nil {
		rc := cfg.ResolveRuntimeConfig(e.GuildID)
		if rc.DisableAutomodLogs {
			return
		}
	}

	botID := ""
	if s != nil && s.State != nil && s.State.User != nil {
		botID = s.State.User.ID
	}
	if !ShouldLogModerationEvent(as.configManager, e.GuildID, "", botID, ModerationSourceGateway) {
		return
	}

	logChannelID, ok := ResolveModerationLogChannel(s, as.configManager, e.GuildID)
	if !ok {
		return
	}

	// If adapters are wired, enqueue via TaskRouter for retries/backoff
	if as.adapters != nil {
		if err := as.adapters.EnqueueAutomodAction(logChannelID, e); err != nil {
			log.ErrorLoggerRaw().Error("Failed to enqueue automod log task", "guildID", e.GuildID, "channelID", logChannelID, "userID", e.UserID, "err", err)
		}
		return
	}

	// Build embed from event data (fallback when adapters are not available)
	title := "AutoMod action executed"
	desc := "A native AutoMod rule was triggered."
	embed := &discordgo.MessageEmbed{
		Title:       title,
		Description: desc,
		Color:       theme.AutomodAction(),
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
		log.ErrorLoggerRaw().Error("Failed to send native automod log message", "guildID", e.GuildID, "channelID", logChannelID, "userID", e.UserID, "err", err)
	}
}

// sanitizeForCodeBlock prevents breaking out of the code fence and removes backticks.
func sanitizeForCodeBlock(input string) string {
	// Replace backticks and normalize newlines for safer preview in a code block
	s := strings.ReplaceAll(input, "`", "'")
	// Discord code blocks tolerate newlines; keep them but trim excessive whitespace
	return strings.TrimSpace(s)
}
