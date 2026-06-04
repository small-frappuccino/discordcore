package logging

import (
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/perf"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/logpolicy"
	"github.com/small-frappuccino/discordcore/pkg/task"
	"github.com/small-frappuccino/discordcore/pkg/theme"
)

// Discord AutoMod logging — per-action gateway behavior and coalescing
//
// Discord emits one AUTO_MODERATION_ACTION_EXECUTION gateway event per
// *action* configured on a triggered rule, not one event per violation. A
// member-profile rule with both Block Member Interactions and Send Alert
// Message configured therefore fires TWO events for a single profile
// violation, with the same rule_id, user_id, and matched_content but
// different gateway sequence numbers and different Action.Type values.
// Message rules with Block Message + Send Alert Message behave the same
// way, sharing the blocked MessageID across actions.
//
// To match Discord's chat-side "one notice per violation" parity, this
// package collapses the per-action stream through two layered defenses:
//
//   1. Per-violation idempotency key (task.AutomodIdempotencyKey): the key
//      derives from per-violation fields (MessageID, MatchedContent,
//      MatchedKeyword) and falls back to a 3-second (guild, rule, user)
//      tbucket. Per-action fields (gateway sequence, AlertSystemMessageID)
//      are excluded so multiple actions for one violation collide on the
//      same key and the router drops the second arrival.
//
//   2. SEND_ALERT_MESSAGE handler filter (handleAutoModerationAction):
//      events whose Action.Type is the alert action are dropped before any
//      key is computed. Discord posts its own native rich alert message to
//      the configured alert channel for these, so our embed would be
//      redundant; the filter also guarantees we cannot accidentally emit a
//      sibling embed if a future trigger type's payload shape breaks the
//      key-level coalescing. Rules configured with ONLY Send Alert Message
//      (no block action) therefore produce no embed on our side — Discord's
//      native alert remains the staff-visible notice.
//
// The user-facing embed reports what was blocked (matched keyword +
// offending fragment) and lets the description convey that the user has
// been restricted, without duplicating one log line per action type.
//
//
// Trigger types Discord can fire (see automodTrigger* constants):
//
//   1 Keyword            2 Harmful link       3 Spam
//   4 Keyword preset     5 Mention spam       6 Member profile
//
// Action types Discord can fire (see automodAction* constants):
//
//   1 Block message      2 Send alert message
//   3 Timeout            4 Block member interactions (quarantine)

// automodActionExecutionEventType is the gateway event type Discord uses for
// AutoMod action executions. Mirrored here so the raw *Event handler can
// filter without importing the discordgo-internal constant.
const automodActionExecutionEventType = "AUTO_MODERATION_ACTION_EXECUTION"

// AutoModeration trigger types. discordgo v0.29.0 only exports constants up to
// AutoModerationEventTriggerKeywordPreset (4); Discord also issues 5
// (MENTION_SPAM) and 6 (MEMBER_PROFILE). MEMBER_PROFILE has no message context
// (empty ChannelID and MessageID) and is what powers "Block Words in Member
// Profile Names" rules.
const (
	automodTriggerKeyword       = 1
	automodTriggerHarmfulLink   = 2
	automodTriggerSpam          = 3
	automodTriggerKeywordPreset = 4
	automodTriggerMentionSpam   = 5
	automodTriggerMemberProfile = 6
)

// AutoModeration action types. discordgo v0.29.0 only exports 1..3; Discord
// also issues 4 (BLOCK_MEMBER_INTERACTION, the "quarantine" applied to
// MEMBER_PROFILE triggers).
const (
	automodActionBlockMessage           = 1
	automodActionSendAlert              = 2
	automodActionTimeout                = 3
	automodActionBlockMemberInteraction = 4
)

const automodExcerptMaxLen = 200

// automodFallbackDedupTTL mirrors the router-level IdempotencyTTL configured in
// EnqueueAutomodAction. The fallback map only kicks in when the router-backed
// adapter is unavailable or has failed, so dedup behavior stays consistent
// across the normal and fallback paths.
const automodFallbackDedupTTL = 10 * time.Second

// automodFallbackDedupCleanupThreshold caps the in-process fallback map size
// before lazy cleanup runs.
const automodFallbackDedupCleanupThreshold = 64

// AutomodService listens for Discord native AutoMod executions and routes them to logging.
type AutomodService struct {
	session       *discordgo.Session
	configManager *files.ConfigManager
	adapters      *task.NotificationAdapters
	isRunning     bool

	// unsubscribe function for the registered handler
	handlerCancel func()

	// dedupChan routes fallback-dedup queries to the actor loop.
	dedupChan chan fallbackDedupReq
	dedupStop chan struct{}
	dedupDone chan struct{}
}

type fallbackDedupReq struct {
	key   string
	now   time.Time
	reply chan bool
}

// NewAutomodService news automod service.
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

	as.dedupChan = make(chan fallbackDedupReq)
	as.dedupStop = make(chan struct{})
	as.dedupDone = make(chan struct{})
	go as.runDedupActor()

	if as.session != nil {
		as.handlerCancel = as.session.AddHandler(as.handleRawEvent)
	}
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
	close(as.dedupStop)
	<-as.dedupDone
	as.isRunning = false
}

// handleRawEvent decomposes a raw gateway envelope, filters for AutoMod action
// executions, and forwards the typed payload alongside the envelope's gateway
// sequence number to handleAutoModerationAction. The sequence is preserved
// across Discord re-deliveries (including RESUME), so it is the most reliable
// dedup key available to the bot.
func (as *AutomodService) handleRawEvent(s *discordgo.Session, evt *discordgo.Event) {
	if evt == nil || evt.Type != automodActionExecutionEventType {
		return
	}

	e, ok := evt.Struct.(*discordgo.AutoModerationActionExecution)
	if !ok || e == nil {
		// discordgo guarantees registeredInterfaceProviders[evt.Type] populates
		// evt.Struct before dispatch, but documents that the struct may be
		// "partially populated or at default values" if unmarshalling failed
		// (wsapi.go:665-669). Fall back to a fresh unmarshal of RawData so we
		// don't silently drop events when discordgo changes the registered
		// type — small cost, defends the future-bump path.
		fallback := &discordgo.AutoModerationActionExecution{}
		if err := json.Unmarshal(evt.RawData, fallback); err != nil {
			log.ErrorLoggerRaw().Error("Failed to decode automod action execution payload", "type", evt.Type, "seq", evt.Sequence, "err", err)
			return
		}
		e = fallback
	}

	as.handleAutoModerationAction(s, e, evt.Sequence)
}

// handleAutoModerationAction logs native AutoMod events to the configured
// automod log channel. The sequence argument is the gateway sequence number
// from the *Event envelope and is recorded for observability only; the
// idempotency key is derived from per-violation payload fields so the
// multiple action events Discord fires for one trigger coalesce to a single
// log embed (see the package-level "AutoMod logging" comment block above).
//
// SEND_ALERT_MESSAGE action events are dropped here as a belt-and-suspenders
// complement to the per-violation key: Discord posts its own native rich
// alert message to the configured alert channel for those, and dropping
// them at the handler guarantees no sibling embed leaks through even if a
// future trigger type's payload shape breaks the key-level coalescing.
func (as *AutomodService) handleAutoModerationAction(s *discordgo.Session, e *discordgo.AutoModerationActionExecution, sequence int64) {
	if e == nil || e.GuildID == "" {
		return
	}

	done := perf.StartGatewayEvent(
		"auto_moderation_action_execution",
		slog.String("guildID", e.GuildID),
		slog.String("channelID", e.ChannelID),
		slog.String("userID", e.UserID),
		slog.String("ruleID", e.RuleID),
		slog.Int64("seq", sequence),
	)
	defer done()

	if int(e.Action.Type) == automodActionSendAlert {
		log.ApplicationLogger().Debug("Dropping SEND_ALERT_MESSAGE automod event; Discord posts its own native alert", "guildID", e.GuildID, "ruleID", e.RuleID, "userID", e.UserID, "seq", sequence)
		return
	}

	emit := logpolicy.ShouldEmitLogEvent(s, as.configManager, logpolicy.LogEventAutomodAction, e.GuildID)
	if !emit.Enabled {
		log.ApplicationLogger().Debug("Automod action notification suppressed by policy", "guildID", e.GuildID, "channelID", e.ChannelID, "userID", e.UserID, "seq", sequence, "reason", emit.Reason)
		return
	}
	logChannelID := emit.ChannelID
	idempotencyKey := task.AutomodIdempotencyKey(e)

	// If adapters are wired, enqueue via TaskRouter for retries/backoff
	if as.adapters != nil {
		if err := as.adapters.EnqueueAutomodActionWithKey(logChannelID, e, idempotencyKey); err != nil {
			if errors.Is(err, task.ErrDuplicateTask) {
				log.ApplicationLogger().Debug("Dropped duplicate automod log task", "guildID", e.GuildID, "channelID", logChannelID, "userID", e.UserID, "ruleID", e.RuleID, "seq", sequence, "messageID", e.MessageID, "alertSystemMessageID", e.AlertSystemMessageID)
				return
			}
			log.ErrorLoggerRaw().Error("Failed to enqueue automod log task; falling back to synchronous send", "guildID", e.GuildID, "channelID", logChannelID, "userID", e.UserID, "seq", sequence, "err", err)
		} else {
			return
		}
	}

	// Synchronous fallback (adapters nil or router enqueue failed). Apply an
	// in-process dedup so Gateway re-deliveries do not duplicate the embed.
	if as.fallbackShouldDedup(idempotencyKey) {
		log.ApplicationLogger().Debug("Dropped duplicate automod log task on fallback path", "guildID", e.GuildID, "channelID", logChannelID, "userID", e.UserID, "ruleID", e.RuleID, "seq", sequence)
		return
	}

	embed := buildAutomodEmbed(e)
	if _, err := s.ChannelMessageSendEmbed(logChannelID, embed); err != nil {
		log.ErrorLoggerRaw().Error("Failed to send native automod log message", "guildID", e.GuildID, "channelID", logChannelID, "userID", e.UserID, "seq", sequence, "err", err)
	}
}

// fallbackShouldDedup reports whether key was seen within automodFallbackDedupTTL.
// Empty keys never dedup (no stable identifier available).
func (as *AutomodService) fallbackShouldDedup(key string) bool {
	return as.fallbackShouldDedupAt(key, time.Now())
}

func (as *AutomodService) fallbackShouldDedupAt(key string, now time.Time) bool {
	if !as.isRunning {
		return false
	}
	reply := make(chan bool, 1)
	req := fallbackDedupReq{key: key, now: now, reply: reply}
	select {
	case as.dedupChan <- req:
		return <-reply
	case <-as.dedupStop:
		return false
	}
}

func (as *AutomodService) runDedupActor() {
	defer close(as.dedupDone)
	cache := make(map[string]time.Time)

	for {
		select {
		case <-as.dedupStop:
			return
		case req := <-as.dedupChan:
			if req.key == "" {
				req.reply <- false
				continue
			}

			if len(cache) > automodFallbackDedupCleanupThreshold {
				for k, expiry := range cache {
					if req.now.After(expiry) {
						delete(cache, k)
					}
				}
			}

			if expiry, exists := cache[req.key]; exists && req.now.Before(expiry) {
				req.reply <- true
			} else {
				cache[req.key] = req.now.Add(automodFallbackDedupTTL)
				req.reply <- false
			}
		}
	}
}

// buildAutomodEmbed dispatches to the trigger-specific embed builder.
// MEMBER_PROFILE events have no message context and get a distinct embed; all
// other triggers reuse the message-keyword shape.
func buildAutomodEmbed(e *discordgo.AutoModerationActionExecution) *discordgo.MessageEmbed {
	if int(e.RuleTriggerType) == automodTriggerMemberProfile {
		return buildAutomodMemberProfileEmbed(e)
	}
	return buildAutomodMessageEmbed(e)
}

func buildAutomodMessageEmbed(e *discordgo.AutoModerationActionExecution) *discordgo.MessageEmbed {
	desc := "Blocked content detected in a message."
	if e.GuildID != "" && e.ChannelID != "" && e.MessageID != "" {
		desc += "\n[Jump to message](https://discord.com/channels/" + e.GuildID + "/" + e.ChannelID + "/" + e.MessageID + ")"
	}
	embed := &discordgo.MessageEmbed{
		Title:       "AutoMod • Message Blocked",
		Description: desc,
		Color:       theme.AutomodAction(),
		Timestamp:   time.Now().Format(time.RFC3339),
		Fields: []*discordgo.MessageEmbedField{
			{Name: "User", Value: formatUserRef(e.UserID), Inline: true},
			{Name: "Channel", Value: automodChannelLabel(e.ChannelID), Inline: true},
		},
	}
	if label := automodTriggerLabel(e.RuleTriggerType); label != "" {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{Name: "Trigger", Value: label, Inline: true})
	}
	if e.RuleID != "" {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{Name: "Rule ID", Value: "`" + e.RuleID + "`", Inline: true})
	}
	if e.MatchedKeyword != "" {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{Name: "Matched keyword", Value: "`" + e.MatchedKeyword + "`", Inline: true})
	}
	if excerpt := automodExcerpt(e); excerpt != "" {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{Name: "Excerpt", Value: "```" + excerpt + "```", Inline: false})
	}
	return embed
}

func buildAutomodMemberProfileEmbed(e *discordgo.AutoModerationActionExecution) *discordgo.MessageEmbed {
	// The per-action Action.Type is intentionally not surfaced on the
	// embed: the package-level coalescing collapses Block Member
	// Interactions + Send Alert Message into a single embed per violation,
	// and "user is quarantined" is already conveyed by the description.
	embed := &discordgo.MessageEmbed{
		Title:       "AutoMod • Member Profile Quarantined",
		Description: "Blocked words detected in this member's profile. The user is quarantined until the profile is updated.",
		Color:       theme.AutomodAction(),
		Timestamp:   time.Now().Format(time.RFC3339),
		Fields: []*discordgo.MessageEmbedField{
			{Name: "Member", Value: formatUserRef(e.UserID), Inline: true},
			{Name: "Trigger", Value: "Member profile", Inline: true},
		},
	}
	if e.RuleID != "" {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{Name: "Rule ID", Value: "`" + e.RuleID + "`", Inline: true})
	}
	if e.MatchedKeyword != "" {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{Name: "Matched keyword", Value: "`" + e.MatchedKeyword + "`", Inline: true})
	}
	if excerpt := automodExcerpt(e); excerpt != "" {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{Name: "Offending fragment", Value: "```" + excerpt + "```", Inline: false})
	}
	return embed
}

func automodTriggerLabel(t discordgo.AutoModerationRuleTriggerType) string {
	switch int(t) {
	case automodTriggerKeyword:
		return "Keyword"
	case automodTriggerHarmfulLink:
		return "Harmful link"
	case automodTriggerSpam:
		return "Spam"
	case automodTriggerKeywordPreset:
		return "Keyword preset"
	case automodTriggerMentionSpam:
		return "Mention spam"
	case automodTriggerMemberProfile:
		return "Member profile"
	}
	return ""
}

// automodActionLabel returns a human-readable label for a Discord AutoMod
// action type. The standard embed builders deliberately do not
// surface this label because the per-action stream is coalesced into one
// embed per violation. See the package-level "AutoMod logging" comment block.
func automodActionLabel(t discordgo.AutoModerationActionType) string {
	switch int(t) {
	case automodActionBlockMessage:
		return "Block message"
	case automodActionSendAlert:
		return "Send alert"
	case automodActionTimeout:
		return "Timeout"
	case automodActionBlockMemberInteraction:
		return "Block member interactions"
	}
	return ""
}

func automodChannelLabel(channelID string) string {
	if strings.TrimSpace(channelID) == "" {
		return "Unknown"
	}
	return formatChannelLabel(channelID)
}

func automodExcerpt(e *discordgo.AutoModerationActionExecution) string {
	content := strings.TrimSpace(e.Content)
	if content == "" {
		content = strings.TrimSpace(e.MatchedContent)
	}
	if content == "" {
		return ""
	}
	if len(content) > automodExcerptMaxLen {
		content = content[:automodExcerptMaxLen] + "..."
	}
	return sanitizeForCodeBlock(content)
}

// sanitizeForCodeBlock prevents breaking out of the code fence and removes backticks.
func sanitizeForCodeBlock(input string) string {
	// Replace backticks and normalize newlines for safer preview in a code block
	s := strings.ReplaceAll(input, "`", "'")
	// Discord code blocks tolerate newlines; keep them but trim excessive whitespace
	return strings.TrimSpace(s)
}
