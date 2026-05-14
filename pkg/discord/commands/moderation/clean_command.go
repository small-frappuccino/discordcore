package moderation

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/cleanup"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	"github.com/small-frappuccino/discordcore/pkg/discord/logging"
	"github.com/small-frappuccino/discordcore/pkg/log"
)

const (
	cleanCommandName           = "clean"
	cleanCountOptionName       = "count"
	cleanUserOptionName        = "user"
	cleanContainsOptionName    = "contains"
	cleanFromOptionName        = "from"
	cleanToOptionName          = "to"
	cleanMaxDeleteCount        = 100
	cleanSearchWindow          = 1000
	cleanFetchPageSize         = 100
	cleanBulkDeleteMaxAge      = (14 * 24 * time.Hour) - time.Minute
	cleanContainsDisplayMaxLen = 80
)

type cleanCommand struct{}

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
}

func newCleanCommand() *cleanCommand { return &cleanCommand{} }

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

func (c *cleanCommand) RequiresPermissions() bool { return true }

func (c *cleanCommand) InteractionAckPolicy() core.InteractionAckPolicy {
	return core.InteractionAckPolicy{Mode: core.InteractionAckModeDefer, Ephemeral: true}
}

func (c *cleanCommand) Handle(ctx *core.Context) error {
	request, err := parseCleanRequest(ctx)
	if err != nil {
		return err
	}
	if err := validateCleanPermissions(ctx, request.channelID); err != nil {
		return err
	}

	result, err := executeClean(ctx, request)
	if err != nil {
		return err
	}
	if result.deleted > 0 {
		sendCleanActionLog(ctx, request, result)
	}

	return core.NewResponseBuilder(ctx.Session).
		WithContext(ctx).
		Ephemeral().
		Success(ctx.Interaction, buildCleanCommandMessage(request, result))
}

func parseCleanRequest(ctx *core.Context) (cleanRequest, error) {
	if err := core.ValidateGuildContext(ctx); err != nil {
		return cleanRequest{}, err
	}
	if err := core.ValidateUserContext(ctx); err != nil {
		return cleanRequest{}, err
	}
	if ctx == nil || ctx.Interaction == nil {
		return cleanRequest{}, core.NewCommandError("Interaction context is not available right now.", true)
	}

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

func validateCleanPermissions(ctx *core.Context, channelID string) error {
	if ctx == nil || ctx.Session == nil {
		return core.NewCommandError("Session not ready. Try again shortly.", true)
	}
	if strings.TrimSpace(channelID) == "" {
		return core.NewCommandError("Channel context is missing for this clean request.", true)
	}
	botID := ""
	if ctx.Session.State != nil && ctx.Session.State.User != nil {
		botID = strings.TrimSpace(ctx.Session.State.User.ID)
	}
	if botID == "" {
		return core.NewCommandError("Bot identity is not available right now.", true)
	}

	if err := requireChannelPermissions(ctx.Session, ctx.UserID, channelID, discordgo.PermissionManageMessages, "You need the Manage Messages permission in this channel to use /clean."); err != nil {
		return err
	}
	botRequired := int64(discordgo.PermissionViewChannel | discordgo.PermissionReadMessageHistory | discordgo.PermissionManageMessages)
	if err := requireChannelPermissions(ctx.Session, botID, channelID, botRequired, "I need View Channel, Read Message History, and Manage Messages in this channel to use /clean."); err != nil {
		return err
	}
	return nil
}

func requireChannelPermissions(session *discordgo.Session, userID, channelID string, required int64, message string) error {
	if session == nil || strings.TrimSpace(userID) == "" || strings.TrimSpace(channelID) == "" {
		return core.NewCommandError("Channel permissions could not be checked right now.", true)
	}
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

func executeClean(ctx *core.Context, request cleanRequest) (cleanResult, error) {
	if ctx == nil || ctx.Session == nil {
		return cleanResult{}, core.NewCommandError("Session not ready. Try again shortly.", true)
	}

	matched, result, err := collectCleanTargets(ctx, request)
	if err != nil {
		return cleanResult{}, err
	}
	result.matched = len(matched)
	if len(matched) == 0 {
		return result, nil
	}

	now := time.Now().UTC()
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

	onDeleteError := func(messageID string, err error) {
		log.ErrorLoggerRaw().Error(
			"Clean command failed to delete message",
			"guildID", ctx.GuildID,
			"channelID", request.channelID,
			"messageID", messageID,
			"err", err,
		)
	}

	result.deletedBulk, result.failed = cleanup.DeleteMessages(ctx.Session, request.channelID, bulkIDs, cleanup.DeleteOptions{
		Mode:          cleanup.DeleteModeBulkPreferred,
		OnDeleteError: onDeleteError,
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

func collectCleanTargets(ctx *core.Context, request cleanRequest) ([]*discordgo.Message, cleanResult, error) {
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
			return nil, cleanResult{}, core.NewCommandError("I couldn't load recent messages from this channel. Make sure I can read message history here and try again.", true)
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
		if result.failed > 0 {
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
	if result.failed > 0 {
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

func sendCleanActionLog(ctx *core.Context, request cleanRequest, result cleanResult) {
	channelLabel := fmt.Sprintf("<#%s> (`%s`)", request.channelID, request.channelID)
	payload := moderationLogPayload{
		Action:      cleanCommandName,
		TargetLabel: channelLabel,
		Reason:      buildCleanLogReason(request),
		RequestedBy: ctx.UserID,
		Extra:       buildCleanLogDetails(request, result),
	}
	sendModerationLogForEvent(ctx, payload, logging.LogEventCleanAction)
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
