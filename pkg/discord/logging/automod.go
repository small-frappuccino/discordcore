package logging

import (
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/perf"
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

	if !shouldLogAutomodEvent(as.configManager, e.GuildID) {
		return
	}

	done := perf.StartGatewayEvent(
		"auto_moderation_action_execution",
		slog.String("guildID", e.GuildID),
		slog.String("channelID", e.ChannelID),
		slog.String("userID", e.UserID),
		slog.String("ruleID", e.RuleID),
	)
	defer done()

	logChannelID, ok := resolveAutomodLogChannel(s, as.configManager, e.GuildID)
	if !ok {
		return
	}

	// If adapters are wired, enqueue via TaskRouter for retries/backoff
	if as.adapters != nil {
		if err := as.adapters.EnqueueAutomodAction(logChannelID, e); err != nil {
			if errors.Is(err, task.ErrDuplicateTask) {
				log.ApplicationLogger().Debug("Dropped duplicate automod log task", "guildID", e.GuildID, "channelID", logChannelID, "userID", e.UserID, "ruleID", e.RuleID, "messageID", e.MessageID, "alertSystemMessageID", e.AlertSystemMessageID)
				return
			}
			log.ErrorLoggerRaw().Error("Failed to enqueue automod log task; falling back to synchronous send", "guildID", e.GuildID, "channelID", logChannelID, "userID", e.UserID, "err", err)
		} else {
			return
		}
	}

	// Build embed from event data (fallback when adapters are not available)
	title := "AutoMod Action"
	desc := "Native AutoMod rule triggered."
	channelValue := formatChannelLabel(e.ChannelID)
	if e.ChannelID == "" {
		channelValue = "DM/Unknown"
	}
	embed := &discordgo.MessageEmbed{
		Title:       title,
		Description: desc,
		Color:       theme.AutomodAction(),
		Timestamp:   time.Now().Format(time.RFC3339),
		Fields: []*discordgo.MessageEmbedField{
			{Name: "User", Value: formatUserRef(e.UserID), Inline: true},
			{Name: "Channel", Value: channelValue, Inline: true},
		},
	}

	// Include rule info
	if e.RuleID != "" {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{Name: "Rule ID", Value: "`" + e.RuleID + "`", Inline: true})
	}
	if e.MatchedKeyword != "" {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{Name: "Matched", Value: "`" + e.MatchedKeyword + "`", Inline: true})
	}
	// Include a short excerpt (content or matched content if present)
	content := e.Content
	if strings.TrimSpace(content) == "" && e.MatchedContent != "" {
		content = e.MatchedContent
	}
	if strings.TrimSpace(content) != "" {
		excerpt := content
		if len(excerpt) > 200 {
			excerpt = excerpt[:200] + "..."
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

func shouldLogAutomodEvent(configManager *files.ConfigManager, guildID string) bool {
	if configManager == nil {
		return false
	}
	cfg := configManager.Config()
	if cfg == nil {
		return false
	}
	if !cfg.ResolveFeatures(guildID).Logging.Automod {
		return false
	}
	rc := cfg.ResolveRuntimeConfig(guildID)
	return !rc.DisableAutomodLogs
}

func resolveAutomodLogChannel(session *discordgo.Session, configManager *files.ConfigManager, guildID string) (string, bool) {
	if configManager == nil {
		return "", false
	}
	gcfg := configManager.GuildConfig(guildID)
	if gcfg == nil {
		return "", false
	}

	// Prefer dedicated automod channel; fallback to moderation channel for backward compatibility.
	channelID := strings.TrimSpace(gcfg.Channels.AutomodLog)
	if channelID == "" {
		channelID = strings.TrimSpace(gcfg.Channels.ModerationLog)
	}
	if channelID == "" {
		return "", false
	}

	botID := ""
	if session != nil && session.State != nil && session.State.User != nil {
		botID = session.State.User.ID
	}
	if err := validateModerationLogChannel(session, guildID, channelID, botID); err != nil {
		log.ErrorLoggerRaw().Error("Automod log channel validation failed", "guildID", guildID, "channelID", channelID, "err", err)
		return "", false
	}

	return channelID, true
}

// sanitizeForCodeBlock prevents breaking out of the code fence and removes backticks.
func sanitizeForCodeBlock(input string) string {
	// Replace backticks and normalize newlines for safer preview in a code block
	s := strings.ReplaceAll(input, "`", "'")
	// Discord code blocks tolerate newlines; keep them but trim excessive whitespace
	return strings.TrimSpace(s)
}
