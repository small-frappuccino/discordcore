package moderation

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/cleanup"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/logpolicy"
)

const (
	cleanCommandName        = "clean"
	cleanCountOptionName    = "count"
	cleanUserOptionName     = "user"
	cleanContainsOptionName = "contains"
	cleanFromOptionName     = "from"
	cleanToOptionName       = "to"
	cleanMaxDeleteCount     = 100
	cleanSearchWindow       = 1000
	cleanFetchPageSize      = 100
	// cleanBulkDeleteMaxAge sits one hour below Discord's 14-day bulk-delete
	// window so messages near the boundary route to the single-delete path
	// proactively. The earlier one-minute margin was tight enough that
	// normal request latency between local classification and Discord
	// receiving the bulk request could push a borderline message across
	// the 14-day line, causing Discord to reject the whole chunk with
	// 50034. The cleanup package still falls back to per-message deletes
	// if the race fires anyway, so this margin is a "make the race rare"
	// knob, not a hard guarantee.
	cleanBulkDeleteMaxAge      = (14 * 24 * time.Hour) - time.Hour
	cleanContainsDisplayMaxLen = 80
)

type cleanCommand struct {
	metrics Metrics
	now     func() time.Time
}

type cleanRequest struct {
	channelID string
	count     int
	userID    string
	contains  string
	fromID    string
	toID      string
}

type cleanResult struct {
	scanned       int
	matched       int
	deleted       int
	failed        int
	deletedBulk   int
	deletedSingle int
	skippedPinned int

	// failed* break the result.failed total down by cleanup.FailureClass so
	// describeCleanFailures and the audit-log embed can render a per-cause
	// breakdown. Their sum should equal result.failed once both the
	// bulk-preferred and single-only passes complete.
	failedForbidden      int
	failedMissingChannel int
	failedRateLimited    int
	failedTransient      int
	failedUnknown        int
}

func newCleanCommand(metrics Metrics) *cleanCommand {
	if metrics == nil {
		metrics = NopMetrics{}
	}
	return &cleanCommand{metrics: metrics, now: time.Now}
}

func (c *cleanCommand) Name() string { return cleanCommandName }

func (c *cleanCommand) Description() string { return "Delete recent messages in this channel" }

func (c *cleanCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionInteger,
			Name:        cleanCountOptionName,
			Description: "How many matching messages to remove (max 100)",
			Required:    true,
			MinValue:    floatPtr(1),
			MaxValue:    cleanMaxDeleteCount,
		},
		{
			Type:        discordgo.ApplicationCommandOptionUser,
			Name:        cleanUserOptionName,
			Description: "Only remove messages from this user",
			Required:    false,
		},
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        cleanContainsOptionName,
			Description: "Only remove messages containing this text",
			Required:    false,
		},
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        cleanFromOptionName,
			Description: "Older message ID bound; matching messages must be newer than this",
			Required:    false,
		},
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        cleanToOptionName,
			Description: "Newer message ID bound; matching messages must be older than this",
			Required:    false,
		},
	}
}

func (c *cleanCommand) RequiresGuild() bool { return true }

func (c *cleanCommand) DefaultMemberPermissions() int64 {
	return discordgo.PermissionManageMessages
}

func (c *cleanCommand) RequiresPermissions() bool { return true }

func (c *cleanCommand) InteractionAckPolicy() core.InteractionAckPolicy {
	return core.InteractionAckPolicy{Mode: core.InteractionAckModeDefer, Ephemeral: true}
}

func (c *cleanCommand) Handle(ctx *core.Context) error {
	start := c.now()
	c.metrics.RecordCleanAttempt()

	if enabled, _ := ctx.Config.Config().ResolveFeatures(ctx.GuildID).Lookup("moderation.clean"); !enabled {
		err := core.NewCommandError("Clean command is disabled for this server.", true)
		c.recordEarlyFailure(ctx, "", CleanFailureCauseFeatureDisabled, start, err)
		return err
	}
	request, err := parseCleanRequest(ctx)
	if err != nil {
		c.recordEarlyFailure(ctx, "", CleanFailureCauseInvalidRequest, start, err)
		return err
	}
	if err := validateCleanPermissions(ctx, request.channelID); err != nil {
		c.recordEarlyFailure(ctx, request.channelID, CleanFailureCausePermissionDenied, start, err)
		return err
	}

	result, err := c.executeClean(ctx, request, start)
	if err != nil {
		return err
	}
	duration := c.now().Sub(start)
	c.metrics.RecordCleanSuccess(duration, result.deleted)
	c.logCleanCompleted(ctx, request, result, duration)
	if result.deleted > 0 {
		c.sendCleanActionLog(ctx, request, result)
	}

	return core.NewResponseBuilder(ctx.Session).
		WithContext(ctx).
		Ephemeral().
		Success(ctx.Interaction, buildCleanCommandMessage(request, result))
}

// recordEarlyFailure surfaces "/clean refused before the deletion pass"
// through both the Metrics seam (so /v1/health/moderation reflects it)
// and the application log (the primary audit trail per the QOTD pattern
// — channel embeds are secondary consumers). The /clean command only
// posts an audit-channel embed on successful deletion, so without this
// log line the audit story would be the slash-command reply alone, which
// disappears the moment the actor closes the ephemeral message.
func (c *cleanCommand) recordEarlyFailure(ctx *core.Context, channelID, cause string, start time.Time, err error) {
	duration := c.now().Sub(start)
	c.metrics.RecordCleanFailure(cause, duration)
	log.ApplicationLogger().Warn(
		"Clean command refused",
		"operation", "moderation.clean.refused",
		"guildID", ctx.GuildID,
		"channelID", channelID,
		"userID", ctx.UserID,
		"cause", cause,
		"durationMs", duration.Milliseconds(),
		"err", err,
	)
}

// logCleanCompleted is the primary audit trail for a successful /clean
// run. It contains every field the channel-embed audit consumer carries
// (actor, channel, filters, deletion sub-totals) so the log stream is a
// self-sufficient record even when the audit channel is unreachable.
func (c *cleanCommand) logCleanCompleted(ctx *core.Context, request cleanRequest, result cleanResult, duration time.Duration) {
	log.ApplicationLogger().Info(
		"Clean command completed",
		"operation", "moderation.clean.completed",
		"guildID", ctx.GuildID,
		"channelID", request.channelID,
		"userID", ctx.UserID,
		"requested", request.count,
		"scanned", result.scanned,
		"matched", result.matched,
		"deleted", result.deleted,
		"deletedBulk", result.deletedBulk,
		"deletedSingle", result.deletedSingle,
		"failed", result.failed,
		"skippedPinned", result.skippedPinned,
		"filterUser", request.userID,
		"filterContainsLen", len(request.contains),
		"filterFromID", request.fromID,
		"filterToID", request.toID,
		"durationMs", duration.Milliseconds(),
	)
}

// parseCleanRequest extracts and validates /clean's own options. The
// framework already guarantees that ctx, ctx.Interaction, ctx.GuildID, and
// ctx.UserID are populated by the time Handle runs (RequiresGuild=true is
// gated in permissionGateMiddleware, and RequiresPermissions=true forces
// UserID through the permission checker), so this function only validates
// the inputs that are actually /clean-specific.
func parseCleanRequest(ctx *core.Context) (cleanRequest, error) {
	channelID := strings.TrimSpace(ctx.Interaction.ChannelID)
	if channelID == "" {
		return cleanRequest{}, core.NewCommandError("This command needs a channel context before it can clean messages.", true)
	}

	options := core.GetSubCommandOptions(ctx.Interaction)
	extractor := core.NewOptionExtractor(options)
	count := int(extractor.Int(cleanCountOptionName))
	if count <= 0 || count > cleanMaxDeleteCount {
		return cleanRequest{}, core.NewCommandError("Count must be between 1 and 100.", true)
	}
	fromID, err := normalizeCleanMessageID(extractor.String(cleanFromOptionName), cleanFromOptionName)
	if err != nil {
		return cleanRequest{}, err
	}
	toID, err := normalizeCleanMessageID(extractor.String(cleanToOptionName), cleanToOptionName)
	if err != nil {
		return cleanRequest{}, err
	}
	if fromID != "" && toID != "" && compareSnowflakeIDs(fromID, toID) >= 0 {
		return cleanRequest{}, core.NewCommandError("The `from` message ID must be older than the `to` message ID.", true)
	}

	request := cleanRequest{
		channelID: channelID,
		count:     count,
		userID:    strings.TrimSpace(userOptionID(options, cleanUserOptionName)),
		contains:  sanitizeCleanContains(extractor.String(cleanContainsOptionName)),
		fromID:    fromID,
		toID:      toID,
	}
	return request, nil
}

func normalizeCleanMessageID(input, optionName string) (string, error) {
	value := strings.TrimSpace(input)
	if value == "" {
		return "", nil
	}
	if !isLikelySnowflake(value) {
		return "", core.NewCommandError(fmt.Sprintf("Option `%s` must be a valid Discord message ID.", optionName), true)
	}
	return value, nil
}

func sanitizeCleanContains(input string) string {
	value := strings.TrimSpace(input)
	if value == "" {
		return ""
	}
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	return strings.TrimSpace(value)
}

// validateCleanPermissions checks that the actor and the bot both hold
// the channel-level permissions /clean needs. Framework guarantees apply:
// ctx.Session, ctx.Session.State, and ctx.Session.State.User are populated
// by the discordgo Ready handshake that precedes any interaction reception,
// and channelID has already been validated as non-empty by parseCleanRequest.
func validateCleanPermissions(ctx *core.Context, channelID string) error {
	botID := ctx.Session.State.User.ID

	if err := requireChannelPermissions(ctx.Session, ctx.UserID, channelID, discordgo.PermissionManageMessages, "You need the Manage Messages permission in this channel to use /clean."); err != nil {
		return err
	}
	botRequired := int64(discordgo.PermissionViewChannel | discordgo.PermissionReadMessageHistory | discordgo.PermissionManageMessages)
	if err := requireChannelPermissions(ctx.Session, botID, channelID, botRequired, "I need View Channel, Read Message History, and Manage Messages in this channel to use /clean."); err != nil {
		return err
	}
	return nil
}

// requireChannelPermissions is the package-private permission lookup used
// by validateCleanPermissions. All three identifying inputs are already
// validated upstream (session by the framework, userID by the permission
// gate or by the bot's own State.User, channelID by parseCleanRequest);
// only the actual Discord API call needs an error path.
func requireChannelPermissions(session *discordgo.Session, userID, channelID string, required int64, message string) error {
	perms, err := session.UserChannelPermissions(userID, channelID)
	if err != nil {
		return core.NewCommandError("Channel permissions could not be checked right now.", true)
	}
	if perms&discordgo.PermissionAdministrator == discordgo.PermissionAdministrator {
		return nil
	}
	if perms&required != required {
		return core.NewCommandError(message, true)
	}
	return nil
}

func (c *cleanCommand) executeClean(ctx *core.Context, request cleanRequest, start time.Time) (cleanResult, error) {
	matched, result, err := c.collectCleanTargets(ctx, request, start)
	if err != nil {
		return cleanResult{}, err
	}
	result.matched = len(matched)
	if len(matched) == 0 {
		return result, nil
	}

	now := c.now().UTC()
	bulkIDs := make([]string, 0, len(matched))
	singleIDs := make([]string, 0, len(matched))
	for _, message := range matched {
		if message == nil || strings.TrimSpace(message.ID) == "" {
			continue
		}
		if shouldSingleDeleteCleanMessage(message, now) {
			singleIDs = append(singleIDs, message.ID)
			continue
		}
		bulkIDs = append(bulkIDs, message.ID)
	}

	onDeleteError := func(messageID string, err error, class cleanup.FailureClass) {
		recordCleanFailure(&result, class)
		c.metrics.RecordCleanDeleteFailure(class)
		log.ApplicationLogger().Warn(
			"Clean command failed to delete message",
			"operation", "moderation.clean.delete_failed",
			"guildID", ctx.GuildID,
			"channelID", request.channelID,
			"messageID", messageID,
			"failureClass", FailureClassToken(class),
			"err", err,
		)
	}
	onChunkError := func(messageIDs []string, err error, class cleanup.FailureClass) {
		recordCleanChunkFailure(&result, class, len(messageIDs))
		for i := 0; i < len(messageIDs); i++ {
			c.metrics.RecordCleanDeleteFailure(class)
		}
		log.ApplicationLogger().Warn(
			"Clean command failed to bulk-delete chunk",
			"operation", "moderation.clean.bulk_delete_failed",
			"guildID", ctx.GuildID,
			"channelID", request.channelID,
			"chunkSize", len(messageIDs),
			"failureClass", FailureClassToken(class),
			"err", err,
		)
	}

	result.deletedBulk, result.failed = cleanup.DeleteMessages(ctx.Session, request.channelID, bulkIDs, cleanup.DeleteOptions{
		Mode:          cleanup.DeleteModeBulkPreferred,
		OnDeleteError: onDeleteError,
		OnChunkError:  onChunkError,
	})
	deletedSingle, failedSingle := cleanup.DeleteMessages(ctx.Session, request.channelID, singleIDs, cleanup.DeleteOptions{
		Mode:          cleanup.DeleteModeSingleOnly,
		OnDeleteError: onDeleteError,
	})
	result.deletedSingle = deletedSingle
	result.deleted = result.deletedBulk + result.deletedSingle
	result.failed += failedSingle
	return result, nil
}

func recordCleanFailure(result *cleanResult, class cleanup.FailureClass) {
	switch class {
	case cleanup.FailureClassForbidden:
		result.failedForbidden++
	case cleanup.FailureClassMissingChannel:
		result.failedMissingChannel++
	case cleanup.FailureClassRateLimited:
		result.failedRateLimited++
	case cleanup.FailureClassTransient:
		result.failedTransient++
	default:
		result.failedUnknown++
	}
}

func recordCleanChunkFailure(result *cleanResult, class cleanup.FailureClass, count int) {
	switch class {
	case cleanup.FailureClassForbidden:
		result.failedForbidden += count
	case cleanup.FailureClassMissingChannel:
		result.failedMissingChannel += count
	case cleanup.FailureClassRateLimited:
		result.failedRateLimited += count
	case cleanup.FailureClassTransient:
		result.failedTransient += count
	default:
		result.failedUnknown += count
	}
}

func cleanFetchErrorMessage(class cleanup.FailureClass) string {
	switch class {
	case cleanup.FailureClassForbidden:
		return "I lost permission to read message history in this channel. Re-check my channel overrides and try again."
	case cleanup.FailureClassMissingChannel:
		return "I couldn't reach this channel anymore — it may have been deleted or my access was removed."
	case cleanup.FailureClassRateLimited:
		return "Discord is rate-limiting me right now. Try again in a moment."
	case cleanup.FailureClassTransient:
		return "Discord had a transient error while loading messages. Try again shortly."
	default:
		return "I couldn't load recent messages from this channel. Make sure I can read message history here and try again."
	}
}

func (c *cleanCommand) collectCleanTargets(ctx *core.Context, request cleanRequest, start time.Time) ([]*discordgo.Message, cleanResult, error) {
	result := cleanResult{}
	matched := make([]*discordgo.Message, 0, request.count)
	before := request.toID
	containsNeedle := strings.ToLower(request.contains)

	for result.scanned < cleanSearchWindow && len(matched) < request.count {
		remaining := cleanSearchWindow - result.scanned
		limit := cleanFetchPageSize
		if remaining < limit {
			limit = remaining
		}
		if limit <= 0 {
			break
		}

		page, err := ctx.Session.ChannelMessages(request.channelID, limit, before, "", "")
		if err != nil {
			class := cleanup.ClassifyFetchError(err)
			log.ApplicationLogger().Warn(
				"Clean command failed to load channel messages",
				"operation", "moderation.clean.fetch_failed",
				"guildID", ctx.GuildID,
				"channelID", request.channelID,
				"failureClass", FailureClassToken(class),
				"err", err,
			)
			c.metrics.RecordCleanFailure(ClassifyCleanFetchFailure(class), c.now().Sub(start))
			return nil, cleanResult{}, core.NewCommandError(cleanFetchErrorMessage(class), true)
		}
		if len(page) == 0 {
			break
		}

		result.scanned += len(page)
		reachedLowerBound := false
		for _, message := range page {
			if cleanReachedLowerBound(message, request.fromID) {
				reachedLowerBound = true
				break
			}
			if !cleanMessageMatches(message, request.userID, containsNeedle) {
				continue
			}
			if message.Pinned {
				result.skippedPinned++
				continue
			}
			matched = append(matched, message)
			if len(matched) >= request.count {
				break
			}
		}

		if reachedLowerBound || len(page) < limit {
			break
		}
		before = strings.TrimSpace(page[len(page)-1].ID)
		if before == "" {
			break
		}
	}

	return matched, result, nil
}

func cleanMessageMatches(message *discordgo.Message, userID, containsNeedle string) bool {
	if message == nil || strings.TrimSpace(message.ID) == "" {
		return false
	}
	if userID != "" {
		if message.Author == nil || strings.TrimSpace(message.Author.ID) != userID {
			return false
		}
	}
	if containsNeedle != "" && !strings.Contains(strings.ToLower(message.Content), containsNeedle) {
		return false
	}
	return true
}

func cleanReachedLowerBound(message *discordgo.Message, fromID string) bool {
	if strings.TrimSpace(fromID) == "" || message == nil {
		return false
	}
	messageID := strings.TrimSpace(message.ID)
	if messageID == "" {
		return false
	}
	return compareSnowflakeIDs(messageID, fromID) <= 0
}

func compareSnowflakeIDs(left, right string) int {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	if left == right {
		return 0
	}
	leftValue, leftErr := strconv.ParseUint(left, 10, 64)
	rightValue, rightErr := strconv.ParseUint(right, 10, 64)
	if leftErr == nil && rightErr == nil {
		switch {
		case leftValue < rightValue:
			return -1
		case leftValue > rightValue:
			return 1
		default:
			return 0
		}
	}
	if len(left) < len(right) {
		return -1
	}
	if len(left) > len(right) {
		return 1
	}
	if left < right {
		return -1
	}
	return 1
}

func shouldSingleDeleteCleanMessage(message *discordgo.Message, now time.Time) bool {
	if message == nil {
		return false
	}
	timestamp := message.Timestamp.UTC()
	if timestamp.IsZero() {
		return false
	}
	return now.Sub(timestamp) >= cleanBulkDeleteMaxAge
}

func buildCleanCommandMessage(request cleanRequest, result cleanResult) string {
	filters := describeCleanFilters(request)
	filterSuffix := ""
	if filters != "" {
		filterSuffix = " Filter: " + filters + "."
	}

	if result.deleted == 0 && result.matched == 0 {
		message := "No matching messages were found in this channel."
		if result.skippedPinned > 0 {
			message += fmt.Sprintf(" %d pinned message(s) were kept.", result.skippedPinned)
		}
		if result.scanned >= cleanSearchWindow {
			message += fmt.Sprintf(" Search stopped after the last %d messages.", cleanSearchWindow)
		}
		return message + filterSuffix
	}

	if result.deleted == 0 {
		message := fmt.Sprintf("I found %d matching message(s), but none could be removed.", result.matched)
		if breakdown := describeCleanFailures(result); breakdown != "" {
			message += " " + breakdown
		} else if result.failed > 0 {
			message += fmt.Sprintf(" %d delete request(s) failed.", result.failed)
		}
		return message + filterSuffix
	}

	message := fmt.Sprintf("Cleaned %d message(s) in this channel.", result.deleted)
	if result.deleted != request.count {
		message += fmt.Sprintf(" Requested %d.", request.count)
	}
	if result.deletedSingle > 0 {
		message += fmt.Sprintf(" %d older message(s) were removed one by one because Discord does not bulk-delete them.", result.deletedSingle)
	}
	if breakdown := describeCleanFailures(result); breakdown != "" {
		message += " " + breakdown
	} else if result.failed > 0 {
		message += fmt.Sprintf(" %d matching message(s) could not be removed.", result.failed)
	}
	if result.skippedPinned > 0 {
		message += fmt.Sprintf(" %d pinned message(s) were kept.", result.skippedPinned)
	}
	if result.scanned >= cleanSearchWindow && result.matched < request.count {
		message += fmt.Sprintf(" Search stopped after the last %d messages.", cleanSearchWindow)
	}
	return message + filterSuffix
}

func describeCleanFailures(result cleanResult) string {
	parts := make([]string, 0, 5)
	if result.failedForbidden > 0 {
		parts = append(parts, fmt.Sprintf("%d blocked by Discord permissions", result.failedForbidden))
	}
	if result.failedMissingChannel > 0 {
		parts = append(parts, fmt.Sprintf("%d skipped because the channel was not found", result.failedMissingChannel))
	}
	if result.failedRateLimited > 0 {
		parts = append(parts, fmt.Sprintf("%d rate limited by Discord", result.failedRateLimited))
	}
	if result.failedTransient > 0 {
		parts = append(parts, fmt.Sprintf("%d hit a transient Discord error", result.failedTransient))
	}
	if result.failedUnknown > 0 {
		parts = append(parts, fmt.Sprintf("%d failed for unknown reasons", result.failedUnknown))
	}
	if len(parts) == 0 {
		return ""
	}
	return "Failures: " + strings.Join(parts, ", ") + "."
}

func describeCleanFilters(request cleanRequest) string {
	parts := make([]string, 0, 3)
	if request.userID != "" {
		parts = append(parts, fmt.Sprintf("from <@%s>", request.userID))
	}
	if request.contains != "" {
		parts = append(parts, fmt.Sprintf("containing `%s`", truncateCleanFilterDisplay(request.contains)))
	}
	if request.fromID != "" && request.toID != "" {
		parts = append(parts, fmt.Sprintf("between message IDs `%s` and `%s`", request.fromID, request.toID))
	} else if request.fromID != "" {
		parts = append(parts, fmt.Sprintf("newer than message ID `%s`", request.fromID))
	} else if request.toID != "" {
		parts = append(parts, fmt.Sprintf("older than message ID `%s`", request.toID))
	}
	return strings.Join(parts, " and ")
}

func truncateCleanFilterDisplay(value string) string {
	value = sanitizeCleanContains(value)
	if len(value) <= cleanContainsDisplayMaxLen {
		return value
	}
	if cleanContainsDisplayMaxLen <= 3 {
		return value[:cleanContainsDisplayMaxLen]
	}
	return value[:cleanContainsDisplayMaxLen-3] + "..."
}

// sendCleanActionLog posts the secondary audit-channel embed and surfaces
// any post failure through both Metrics and the application log. Unlike
// the void sendModerationLogForEvent path used by other mod commands, the
// /clean command treats audit-log POST failures as a first-class signal:
// the primary audit record is the application-log line written by
// logCleanCompleted, so a broken channel must not look like "everything
// worked" on /v1/health/moderation.
func (c *cleanCommand) sendCleanActionLog(ctx *core.Context, request cleanRequest, result cleanResult) {
	channelLabel := fmt.Sprintf("<#%s> (`%s`)", request.channelID, request.channelID)
	payload := moderationLogPayload{
		Action:      cleanCommandName,
		TargetLabel: channelLabel,
		Reason:      buildCleanLogReason(request),
		RequestedBy: ctx.UserID,
		Extra:       buildCleanLogDetails(request, result),
	}
	emit := postModerationEventEmbed(ctx, payload, logpolicy.LogEventCleanAction)
	if !emit.Enabled {
		return
	}
	if emit.Err != nil {
		c.metrics.RecordCleanAuditLogFailure()
		log.ApplicationLogger().Warn(
			"Clean command audit-log channel post failed",
			"operation", "moderation.clean.audit_log_failed",
			"guildID", ctx.GuildID,
			"sourceChannelID", request.channelID,
			"logChannelID", emit.ChannelID,
			"deleted", result.deleted,
			"err", emit.Err,
		)
	}
}

func buildCleanLogReason(request cleanRequest) string {
	if filters := describeCleanFilters(request); filters != "" {
		return "Recent channel cleanup with filters: " + filters
	}
	return "Recent channel cleanup"
}

func buildCleanLogDetails(request cleanRequest, result cleanResult) string {
	parts := []string{
		fmt.Sprintf("Requested: %d", request.count),
		fmt.Sprintf("Deleted: %d", result.deleted),
		fmt.Sprintf("Scanned: %d", result.scanned),
	}
	if result.failed > 0 {
		parts = append(parts, fmt.Sprintf("Failed: %d", result.failed))
	}
	if result.failedForbidden > 0 {
		parts = append(parts, fmt.Sprintf("Forbidden: %d", result.failedForbidden))
	}
	if result.failedMissingChannel > 0 {
		parts = append(parts, fmt.Sprintf("Missing channel: %d", result.failedMissingChannel))
	}
	if result.failedRateLimited > 0 {
		parts = append(parts, fmt.Sprintf("Rate limited: %d", result.failedRateLimited))
	}
	if result.failedTransient > 0 {
		parts = append(parts, fmt.Sprintf("Transient: %d", result.failedTransient))
	}
	if result.failedUnknown > 0 {
		parts = append(parts, fmt.Sprintf("Unknown failure: %d", result.failedUnknown))
	}
	if result.skippedPinned > 0 {
		parts = append(parts, fmt.Sprintf("Pinned kept: %d", result.skippedPinned))
	}
	if result.deletedSingle > 0 {
		parts = append(parts, fmt.Sprintf("Single delete: %d", result.deletedSingle))
	}
	if filters := describeCleanFilters(request); filters != "" {
		parts = append(parts, "Filters: "+filters)
	}
	return strings.Join(parts, " | ")
}
