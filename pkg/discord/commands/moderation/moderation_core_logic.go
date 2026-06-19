package moderation

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/logging"
	"github.com/small-frappuccino/discordcore/pkg/storage"
	"github.com/small-frappuccino/discordcore/pkg/theme"
	"github.com/small-frappuccino/discordgo"
)

const (
	maxAuditLogReasonLen = 512
	minSnowflakeLength   = 15
	maxSnowflakeLength   = 21
	timeoutMaxMinutes    = 28 * 24 * 60
)

var userMentionRe = regexp.MustCompile(`^<@!?(\d+)>$`)
var (
	fallbackCaseSeqMu sync.Mutex
	fallbackCaseSeq   = map[string]int64{}
)

func parseMemberIDs(input string) ([]string, []string) {
	rawIDs := strings.FieldsFunc(input, func(r rune) bool {
		return r == ',' || r == ';' || r == ' ' || r == '\n' || r == '\t'
	})

	unique := make(map[string]struct{})
	invalidSet := make(map[string]struct{})
	var invalid []string

	for _, id := range rawIDs {
		clean := strings.TrimSpace(id)
		if clean == "" {
			continue
		}
		normalized, ok := normalizeUserID(clean)
		if !ok {
			if _, exists := invalidSet[clean]; !exists {
				invalidSet[clean] = struct{}{}
				invalid = append(invalid, clean)
			}
			continue
		}
		unique[normalized] = struct{}{}
	}

	ids := make([]string, 0, len(unique))
	for id := range unique {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	sort.Strings(invalid)
	return ids, invalid
}

func buildBanCommandMessage(targetUsername, reason string, truncated bool) string {
	targetLabel := strings.TrimSpace(targetUsername)
	if targetLabel == "" {
		targetLabel = "unknown user"
	}
	message := fmt.Sprintf("%s was banned. Reason: %s.", targetLabel, reason)
	if truncated {
		message += " Reason was truncated to fit this reply."
	}
	return message
}

func buildMassBanCommandMessage(banned int) string {
	if banned == 1 {
		return "1 user was banned."
	}
	return fmt.Sprintf("%d users were banned.", banned)
}

func buildKickCommandMessage(targetUsername, reason string, truncated bool) string {
	targetLabel := strings.TrimSpace(targetUsername)
	if targetLabel == "" {
		targetLabel = "unknown user"
	}
	message := fmt.Sprintf("%s was kicked. Reason: %s.", targetLabel, reason)
	if truncated {
		message += " Reason was truncated to fit this reply."
	}
	return message
}

func buildMuteCommandMessage(targetUsername string, muteRole *discordgo.Role, reason string, truncated bool) string {
	targetLabel := strings.TrimSpace(targetUsername)
	if targetLabel == "" {
		targetLabel = "unknown user"
	}
	message := fmt.Sprintf("%s was muted with role %s. Reason: %s.", targetLabel, formatRoleDisplayName(muteRole), reason)
	if truncated {
		message += " Reason was truncated to fit this reply."
	}
	return message
}

func buildTimeoutCommandMessage(targetUsername string, minutes int64, reason string, truncated bool) string {
	targetLabel := strings.TrimSpace(targetUsername)
	if targetLabel == "" {
		targetLabel = "unknown user"
	}
	message := fmt.Sprintf("%s was timed out for %s. Reason: %s.", targetLabel, formatTimeoutDuration(minutes), reason)
	if truncated {
		message += " Reason was truncated to fit this reply."
	}
	return message
}

func buildWarnCommandMessage(targetUsername string, caseNumber int64, reason string, truncated bool) string {
	targetLabel := strings.TrimSpace(targetUsername)
	if targetLabel == "" {
		targetLabel = "unknown user"
	}
	message := fmt.Sprintf("%s was warned. Case #%d. Reason: %s.", targetLabel, caseNumber, reason)
	if truncated {
		message += " Reason was truncated to fit this reply."
	}
	return message
}

func buildWarningsCommandMessage(targetUsername string, warnings []storage.ModerationWarning) string {
	targetLabel := strings.TrimSpace(targetUsername)
	if targetLabel == "" {
		targetLabel = "unknown user"
	}
	if len(warnings) == 0 {
		return fmt.Sprintf("No warnings are recorded for %s. This reply stays private because moderation history should stay private.", targetLabel)
	}

	lines := []string{fmt.Sprintf("Here is the recent warning history for %s. This reply stays private because moderation history should stay private:", targetLabel)}
	for _, warning := range warnings {
		reason := strings.TrimSpace(warning.Reason)
		if reason == "" {
			reason = "No reason provided"
		}
		createdAt := warning.CreatedAt
		if createdAt.IsZero() {
			lines = append(lines, fmt.Sprintf("#%d • by <@%s> • %s", warning.CaseNumber, warning.ModeratorID, reason))
			continue
		}
		lines = append(lines, fmt.Sprintf("#%d • <t:%d:d> • by <@%s> • %s", warning.CaseNumber, createdAt.Unix(), warning.ModeratorID, reason))
	}
	return strings.Join(lines, "\n")
}

func formatTimeoutDuration(minutes int64) string {
	switch {
	case minutes >= 1440 && minutes%1440 == 0:
		return fmt.Sprintf("%d day(s)", minutes/1440)
	case minutes >= 60 && minutes%60 == 0:
		return fmt.Sprintf("%d hour(s)", minutes/60)
	default:
		return fmt.Sprintf("%d minute(s)", minutes)
	}
}

func permissionCheckerForContext(ctx *core.Context) *core.PermissionChecker {
	if ctx == nil {
		return nil
	}
	if router := ctx.Router(); router != nil {
		if checker := router.GetPermissionChecker(); checker != nil {
			return checker
		}

		checker := core.NewPermissionChecker(ctx.Session, ctx.Config)
		if store := router.GetStore(); store != nil {
			checker.SetStore(store)
		}
		return checker
	}

	return core.NewPermissionChecker(ctx.Session, ctx.Config)
}

type banContext struct {
	rolesByID    map[string]*discordgo.Role
	botID        string
	actorMember  *discordgo.Member
	botMember    *discordgo.Member
	actorRolePos int
	botRolePos   int
}

func sanitizeReason(input string) (string, bool) {
	reason := strings.TrimSpace(input)
	if reason == "" {
		return "No reason provided", false
	}
	reason = strings.ReplaceAll(reason, "\r", " ")
	reason = strings.ReplaceAll(reason, "\n", " ")
	reason = strings.TrimSpace(reason)
	if len(reason) <= maxAuditLogReasonLen {
		return reason, false
	}
	return reason[:maxAuditLogReasonLen], true
}

func normalizeUserID(input string) (string, bool) {
	clean := strings.TrimSpace(input)
	if clean == "" {
		return "", false
	}
	if match := userMentionRe.FindStringSubmatch(clean); len(match) == 2 {
		return match[1], true
	}
	if !isLikelySnowflake(clean) {
		return "", false
	}
	return clean, true
}

func isLikelySnowflake(value string) bool {
	if len(value) < minSnowflakeLength || len(value) > maxSnowflakeLength {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func prepareBanContext(ctx *core.Context) (*banContext, error) {
	return prepareModerationContext(
		ctx,
		discordgo.PermissionBanMembers,
		"You need the Ban Members permission to use this command.",
		"The bot needs the Ban Members permission to ban members.",
	)
}

func prepareKickContext(ctx *core.Context) (*banContext, error) {
	return prepareModerationContext(
		ctx,
		discordgo.PermissionKickMembers,
		"You need the Kick Members permission to use this command.",
		"The bot needs the Kick Members permission to kick members.",
	)
}

func prepareTimeoutContext(ctx *core.Context) (*banContext, error) {
	return prepareModerationContext(
		ctx,
		discordgo.PermissionModerateMembers,
		"You need the Moderate Members permission to use this command.",
		"The bot needs the Moderate Members permission to timeout members.",
	)
}

func prepareMuteContext(ctx *core.Context) (*banContext, error) {
	return prepareModerationContext(
		ctx,
		discordgo.PermissionManageRoles,
		"You need the Manage Roles permission to use this command.",
		"The bot needs the Manage Roles permission to mute members with the configured mute role.",
	)
}

func prepareWarnContext(ctx *core.Context) (*banContext, error) {
	return prepareModerationContext(
		ctx,
		discordgo.PermissionModerateMembers,
		"You need the Moderate Members permission to use this command.",
		"The bot needs the Moderate Members permission to manage warnings.",
	)
}

func prepareModerationContext(ctx *core.Context, requiredPermission int64, actorPermissionError, botPermissionError string) (*banContext, error) {
	if ctx == nil || ctx.Session == nil {
		return nil, &core.CommandError{Message: "Session not ready. Try again shortly.", Ephemeral: true}
	}

	checker := permissionCheckerForContext(ctx)
	if checker == nil {
		return nil, &core.CommandError{Message: "Permission resolver not available.", Ephemeral: true}
	}

	roles, err := checker.ResolveRoles(ctx.GuildID)
	if err != nil {
		log.ErrorLoggerRaw().Error(
			"Moderation context failed to resolve guild roles",
			"operation", "commands.moderation.prepare_context.resolve_roles",
			"guildID", ctx.GuildID,
			"userID", ctx.UserID,
			"err", err,
		)
		return nil, &core.CommandError{Message: "Failed to resolve server roles.", Ephemeral: true}
	}
	rolesByID := buildRoleIndex(roles)

	botID := ""
	if ctx.Session.State != nil && ctx.Session.State.User != nil {
		botID = ctx.Session.State.User.ID
	}
	if botID == "" {
		return nil, &core.CommandError{Message: "Bot identity not available.", Ephemeral: true}
	}

	var actorMember *discordgo.Member
	if ctx.Interaction != nil {
		actorMember = ctx.Interaction.Member
	}
	if actorMember == nil || actorMember.User == nil {
		var ok bool
		actorMember, ok, err = checker.ResolveMember(ctx.GuildID, ctx.UserID)
		if err != nil {
			log.ErrorLoggerRaw().Error(
				"Moderation context failed to resolve actor member",
				"operation", "commands.moderation.prepare_context.resolve_actor_member",
				"guildID", ctx.GuildID,
				"userID", ctx.UserID,
				"err", err,
			)
			return nil, &core.CommandError{Message: "Unable to resolve your member record.", Ephemeral: true}
		}
		if !ok || actorMember == nil {
			return nil, &core.CommandError{Message: "Unable to resolve your member record.", Ephemeral: true}
		}
	}

	botMember, ok, err := checker.ResolveMember(ctx.GuildID, botID)
	if err != nil {
		log.ErrorLoggerRaw().Error(
			"Moderation context failed to resolve bot member",
			"operation", "commands.moderation.prepare_context.resolve_bot_member",
			"guildID", ctx.GuildID,
			"botID", botID,
			"err", err,
		)
		return nil, &core.CommandError{Message: "Unable to resolve the bot member record.", Ephemeral: true}
	}
	if !ok || botMember == nil {
		return nil, &core.CommandError{Message: "Unable to resolve the bot member record.", Ephemeral: true}
	}

	if !memberHasPermission(actorMember, rolesByID, ctx.GuildID, requiredPermission) {
		return nil, &core.CommandError{Message: actorPermissionError, Ephemeral: true}
	}
	if !memberHasPermission(botMember, rolesByID, ctx.GuildID, requiredPermission) {
		return nil, &core.CommandError{Message: botPermissionError, Ephemeral: true}
	}

	return &banContext{
		rolesByID:    rolesByID,
		botID:        botID,
		actorMember:  actorMember,
		botMember:    botMember,
		actorRolePos: highestRolePosition(actorMember, rolesByID, ctx.GuildID),
		botRolePos:   highestRolePosition(botMember, rolesByID, ctx.GuildID),
	}, nil
}

func canBanTarget(ctx *core.Context, banCtx *banContext, targetID string) (bool, string) {
	return canModerateTarget(ctx, banCtx, targetID, "ban", false)
}

func canKickTarget(ctx *core.Context, actionCtx *banContext, targetID string) (bool, string) {
	return canModerateTarget(ctx, actionCtx, targetID, "kick", true)
}

func canTimeoutTarget(ctx *core.Context, actionCtx *banContext, targetID string) (bool, string) {
	return canModerateTarget(ctx, actionCtx, targetID, "timeout", true)
}

func canMuteTarget(ctx *core.Context, actionCtx *banContext, targetID string) (bool, string) {
	return canModerateTarget(ctx, actionCtx, targetID, "mute", true)
}

func canWarnTarget(ctx *core.Context, actionCtx *banContext, targetID string) (bool, string) {
	return canModerateTarget(ctx, actionCtx, targetID, "warn", true)
}

func canModerateTarget(ctx *core.Context, actionCtx *banContext, targetID, actionVerb string, requireMember bool) (bool, string) {
	if targetID == ctx.UserID {
		return false, "cannot " + actionVerb + " yourself"
	}
	if targetID == actionCtx.botID {
		return false, "cannot " + actionVerb + " the bot"
	}

	checker := permissionCheckerForContext(ctx)
	if checker == nil {
		if requireMember {
			return false, "target member could not be resolved right now"
		}
		return true, ""
	}

	targetMember, ok, err := checker.ResolveMember(ctx.GuildID, targetID)
	if err != nil {
		log.ErrorLoggerRaw().Error(
			"Moderation target validation failed to resolve target member",
			"operation", "commands.moderation.can_moderate_target.resolve_target_member",
			"guildID", ctx.GuildID,
			"targetID", targetID,
			"action", actionVerb,
			"err", err,
		)
		if requireMember {
			return false, "target member could not be resolved right now"
		}
		return true, ""
	}
	if !ok || targetMember == nil {
		if requireMember {
			return false, "target is not a member of this server"
		}
		return true, ""
	}

	targetPos := highestRolePosition(targetMember, actionCtx.rolesByID, ctx.GuildID)
	if actionCtx.actorRolePos <= targetPos {
		return false, "target has an equal or higher role than you"
	}
	if actionCtx.botRolePos <= targetPos {
		return false, "target has an equal or higher role than the bot"
	}
	return true, ""
}

func resolveConfiguredMuteRole(ctx *core.Context, actionCtx *banContext) (*discordgo.Role, string, error) {
	if ctx == nil || ctx.Config == nil {
		return nil, "", &core.CommandError{Message: "Configuration is not available right now.", Ephemeral: true}
	}
	cfg := ctx.Config.Config()
	if cfg == nil {
		return nil, "", &core.CommandError{Message: "Configuration is not available right now.", Ephemeral: true}
	}
	if !cfg.ResolveFeatures(ctx.GuildID).MuteRole {
		return nil, "", &core.CommandError{Message: "Mute role moderation is disabled for this server.", Ephemeral: true}
	}

	roleID := ""
	if ctx.GuildConfig != nil {
		roleID = strings.TrimSpace(ctx.GuildConfig.Roles.MuteRole)
	}
	if roleID == "" {
		for _, guild := range cfg.Guilds {
			if guild.GuildID == ctx.GuildID {
				roleID = strings.TrimSpace(guild.Roles.MuteRole)
				break
			}
		}
	}
	if roleID == "" {
		return nil, "", core.NewMissingConfigError(ctx.GuildID, "Mute Role", "/roles")
	}
	if actionCtx == nil {
		return nil, roleID, &core.CommandError{Message: "Mute role context is not available right now.", Ephemeral: true}
	}

	role, ok := actionCtx.rolesByID[roleID]
	if !ok || role == nil {
		return nil, roleID, &core.CommandError{Message: "Configured mute role is no longer available in this server.", Ephemeral: true}
	}
	if role.Managed {
		return nil, roleID, &core.CommandError{Message: "Configured mute role is managed by an integration and cannot be assigned manually.", Ephemeral: true}
	}
	if actionCtx.actorRolePos <= role.Position {
		return nil, roleID, &core.CommandError{Message: "Your highest role must stay above the configured mute role.", Ephemeral: true}
	}
	if actionCtx.botRolePos <= role.Position {
		return nil, roleID, &core.CommandError{Message: "My highest role must stay above the configured mute role.", Ephemeral: true}
	}
	return role, roleID, nil
}

func resolveRoleTargetMember(ctx *core.Context, targetID string) (*discordgo.Member, bool, string) {
	checker := permissionCheckerForContext(ctx)
	if checker == nil {
		return nil, false, "target member could not be resolved right now"
	}
	targetMember, ok, err := checker.ResolveMember(ctx.GuildID, targetID)
	if err != nil {
		log.ErrorLoggerRaw().Error(
			"Moderation role action failed to resolve target member",
			"operation", "commands.moderation.resolve_role_target_member",
			"guildID", ctx.GuildID,
			"targetID", targetID,
			"err", err,
		)
		return nil, false, "target member could not be resolved right now"
	}
	if !ok || targetMember == nil {
		return nil, false, "target is not a member of this server"
	}
	return targetMember, true, ""
}

func memberHasRole(member *discordgo.Member, roleID string) bool {
	if member == nil {
		return false
	}
	roleID = strings.TrimSpace(roleID)
	if roleID == "" {
		return false
	}
	for _, existingRoleID := range member.Roles {
		if strings.TrimSpace(existingRoleID) == roleID {
			return true
		}
	}
	return false
}

func formatRoleDisplayName(role *discordgo.Role) string {
	if role == nil {
		return "mute role"
	}
	if role.ID == role.Name || strings.TrimSpace(role.Name) == "" {
		return role.ID
	}
	return role.Name
}

func buildRoleIndex(roles []*discordgo.Role) map[string]*discordgo.Role {
	byID := make(map[string]*discordgo.Role, len(roles))
	for _, role := range roles {
		if role == nil || role.ID == "" {
			continue
		}
		byID[role.ID] = role
	}
	return byID
}

func resolveUserDisplayName(ctx *core.Context, userID string) string {
	if ctx == nil || ctx.Session == nil || userID == "" {
		return userID
	}

	checker := permissionCheckerForContext(ctx)
	if checker != nil {
		member, ok, err := checker.ResolveMember(ctx.GuildID, userID)
		if err != nil {
			log.ErrorLoggerRaw().Error(
				"Moderation failed to resolve display name member",
				"operation", "commands.moderation.resolve_display_name.resolve_member",
				"guildID", ctx.GuildID,
				"userID", userID,
				"err", err,
			)
		} else if ok && member != nil && member.User != nil {
			if username := strings.TrimSpace(member.User.Username); username != "" {
				return username
			}
		}
	}

	user, err := ctx.Session.User(userID)
	if err == nil && user != nil {
		if username := strings.TrimSpace(user.Username); username != "" {
			return username
		}
	}

	return userID
}

func memberHasPermission(member *discordgo.Member, rolesByID map[string]*discordgo.Role, guildID string, perm int64) bool {
	if member == nil || member.User == nil {
		return false
	}

	var permissions int64
	if role, ok := rolesByID[guildID]; ok && role != nil {
		permissions |= role.Permissions
	}
	for _, roleID := range member.Roles {
		if role, ok := rolesByID[roleID]; ok && role != nil {
			permissions |= role.Permissions
		}
	}

	if permissions&discordgo.PermissionAdministrator != 0 {
		return true
	}
	return permissions&perm != 0
}

func highestRolePosition(member *discordgo.Member, rolesByID map[string]*discordgo.Role, guildID string) int {
	if member == nil {
		return -1
	}

	pos := -1
	if role, ok := rolesByID[guildID]; ok && role != nil {
		pos = role.Position
	}
	for _, roleID := range member.Roles {
		if role, ok := rolesByID[roleID]; ok && role != nil && role.Position > pos {
			pos = role.Position
		}
	}
	return pos
}

type moderationLogPayload struct {
	Action        string
	TargetID      string
	TargetLabel   string
	Reason        string
	RequestedBy   string
	Extra         string
	CaseNumber    int64
	HasCaseNumber bool
}

func buildMassBanLogDetails(total, banned int, invalid, skipped, failed []string) string {
	parts := []string{fmt.Sprintf("Total: %d", total), fmt.Sprintf("Banned: %d", banned)}
	if len(invalid) > 0 {
		parts = append(parts, fmt.Sprintf("Invalid: %d", len(invalid)))
	}
	if len(skipped) > 0 {
		parts = append(parts, fmt.Sprintf("Skipped: %d", len(skipped)))
	}
	if len(failed) > 0 {
		parts = append(parts, fmt.Sprintf("Failed: %d", len(failed)))
	}
	return strings.Join(parts, " | ")
}

func nextGuildCaseNumber(ctx *core.Context) (int64, bool) {
	if ctx == nil || ctx.GuildID == "" {
		return 0, false
	}
	router := ctx.Router()
	if router == nil {
		return nextFallbackCaseNumber(ctx.GuildID), true
	}
	store := router.GetStore()
	if store == nil {
		return nextFallbackCaseNumber(ctx.GuildID), true
	}

	n, err := store.NextModerationCaseNumber(ctx.GuildID)
	if err != nil {
		log.ErrorLoggerRaw().Error("Failed to allocate moderation case number", "guildID", ctx.GuildID, "err", err)
		return nextFallbackCaseNumber(ctx.GuildID), true
	}
	return n, true
}

func moderationStoreFromContext(ctx *core.Context) *storage.Store {
	if ctx == nil {
		return nil
	}
	router := ctx.Router()
	if router == nil {
		return nil
	}
	return router.GetStore()
}

func nextFallbackCaseNumber(guildID string) int64 {
	fallbackCaseSeqMu.Lock()
	defer fallbackCaseSeqMu.Unlock()
	fallbackCaseSeq[guildID]++
	return fallbackCaseSeq[guildID]
}

func buildModerationCaseTitle(caseNumber int64, hasCaseNumber bool, actionType string) string {
	casePart := "?"
	if hasCaseNumber && caseNumber > 0 {
		casePart = fmt.Sprintf("%d", caseNumber)
	}
	actionType = strings.ToLower(strings.TrimSpace(actionType))
	if actionType == "" {
		actionType = "action"
	}
	return actionType + " | case " + casePart
}
func sendModerationLog(ctx *core.Context, payload moderationLogPayload) {
	sendModerationLogForEvent(ctx, payload, logging.LogEventModerationCase)
}

// moderationEventEmit reports the outcome of an attempt to publish a
// moderation event embed to the configured log channel.
//
// Callers that need to react to a failed audit-log post (the /clean
// command records a metric so the silent-loss becomes visible on
// /v1/health/moderation) invoke postModerationEventEmbed directly. The
// older fire-and-forget callers continue to use sendModerationLogForEvent
// which surfaces the failure on the error log so oncall picks it up.
type moderationEventEmit struct {
	// Enabled is false when the guild has the event type gated off; the
	// embed was not built and Err is nil. Callers treat this as "audit
	// surface intentionally disabled", not as a failure.
	Enabled bool
	// ChannelID is the resolved log-channel ID when Enabled is true.
	// Empty otherwise.
	ChannelID string
	// Err is the error returned by ChannelMessageSendEmbed, or nil on
	// success. Only meaningful when Enabled is true.
	Err error
}

// postModerationEventEmbed renders and posts the moderation-event embed.
// Returns an Enabled=false result without contacting Discord when the
// guild has the event type disabled. When enabled, returns the resolved
// channel ID and any send error so callers can decide whether to record
// the failure as a metric.
func postModerationEventEmbed(ctx *core.Context, payload moderationLogPayload, eventType logging.LogEventType) moderationEventEmit {
	if ctx == nil || ctx.Session == nil || ctx.Config == nil || ctx.GuildID == "" {
		return moderationEventEmit{}
	}
	botID := ""
	if ctx.Session.State != nil && ctx.Session.State.User != nil {
		botID = ctx.Session.State.User.ID
	}
	emit := logging.CheckFeatureEnabled(ctx.Config, eventType, ctx.GuildID)
	if !emit.Enabled {
		return moderationEventEmit{}
	}
	channelID := emit.ChannelID

	action := strings.TrimSpace(payload.Action)
	targetID := strings.TrimSpace(payload.TargetID)
	targetLabel := strings.TrimSpace(payload.TargetLabel)
	targetValue := "Unknown"
	switch {
	case targetID == "" && targetLabel != "":
		targetValue = targetLabel
	case targetID != "" && (targetLabel == "" || targetLabel == targetID):
		targetValue = "<@" + targetID + "> (`" + targetID + "`)"
	case targetID != "":
		targetValue = fmt.Sprintf("**%s** (<@%s>, `%s`)", targetLabel, targetID, targetID)
	}
	reason := strings.TrimSpace(payload.Reason)
	if reason == "" {
		reason = "No reason provided"
	}
	caseID := ""
	if ctx.Interaction != nil {
		caseID = strings.TrimSpace(ctx.Interaction.ID)
	}

	fields := []*discordgo.MessageEmbedField{
		{Name: "Action", Value: action, Inline: true},
	}
	if caseID != "" {
		fields = append(fields, &discordgo.MessageEmbedField{Name: "Case ID", Value: "`" + caseID + "`", Inline: true})
	}
	fields = append(fields,
		&discordgo.MessageEmbedField{Name: "Target", Value: targetValue, Inline: true},
		&discordgo.MessageEmbedField{Name: "Actor", Value: "<@" + botID + "> (`" + botID + "`)", Inline: true},
	)
	if payload.RequestedBy != "" {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "Requested By",
			Value:  "<@" + payload.RequestedBy + "> (`" + payload.RequestedBy + "`)",
			Inline: true,
		})
	}
	fields = append(fields, &discordgo.MessageEmbedField{
		Name:   "Reason",
		Value:  reason,
		Inline: false,
	})
	if payload.Extra != "" {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "Details",
			Value:  payload.Extra,
			Inline: false,
		})
	}

	embed := &discordgo.MessageEmbed{
		Title:       "Moderation Action",
		Color:       theme.AutomodAction(),
		Description: fmt.Sprintf("Moderation action executed by <@%s>.", botID),
		Fields:      fields,
		Timestamp:   time.Now().Format(time.RFC3339),
	}

	if _, err := ctx.Session.ChannelMessageSendEmbed(channelID, embed); err != nil {
		return moderationEventEmit{Enabled: true, ChannelID: channelID, Err: err}
	}
	return moderationEventEmit{Enabled: true, ChannelID: channelID}
}

// sendModerationLogForEvent is the fire-and-forget wrapper preserved for
// moderation commands that do not have a Metrics seam wired. Without a
// metric backing, the error log is the only oncall-visible surface for
// audit-channel post failures, so we route through ErrorLoggerRaw().Error
// to match the existing convention in user_prune, automod, and the
// monitoring user-event paths. Callers that want a metric should invoke
// postModerationEventEmbed directly and react to the returned error.
func sendModerationLogForEvent(ctx *core.Context, payload moderationLogPayload, eventType logging.LogEventType) {
	emit := postModerationEventEmbed(ctx, payload, eventType)
	if !emit.Enabled || emit.Err == nil {
		return
	}
	log.ErrorLoggerRaw().Error(
		"Failed to send moderation log",
		"operation", "moderation.audit_log.send_failed",
		"guildID", ctx.GuildID,
		"channelID", emit.ChannelID,
		"action", strings.TrimSpace(payload.Action),
		"eventType", string(eventType),
		"err", emit.Err,
	)
}

func resolveModerationCaseEmbedMeta(action, actionType string) (string, string, string) {
	switch compactModerationActionKey(action) {
	case "ban", "massban", "memberbanadd":
		return "ban", "Offender", "Details"
	case "unban", "memberbanremove":
		return "unban", "User", "Details"
	case "kick", "memberkick":
		return "kick", "Offender", "Details"
	case "mute", "memberroleupdate":
		return "mute", "Offender", "Details"
	case "timeout":
		return "timeout", "Offender", "Details"
	case "untimeout":
		return "untimeout", "User", "Details"
	case "warn":
		return "warn", "Offender", "Details"
	default:
		label := strings.ToLower(strings.TrimSpace(actionType))
		if label == "" {
			label = "action"
		}
		return label, "Target", "Details"
	}
}

func sendModerationCaseActionLog(ctx *core.Context, payload moderationLogPayload) {
	if ctx == nil || ctx.Session == nil || ctx.Config == nil || ctx.GuildID == "" {
		return
	}
	botID := ""
	if ctx.Session.State != nil && ctx.Session.State.User != nil {
		botID = ctx.Session.State.User.ID
	}
	emit := logging.CheckFeatureEnabled(ctx.Config, logging.LogEventModerationCase, ctx.GuildID)
	if !emit.Enabled {
		return
	}
	channelID := emit.ChannelID
	caseNumber, hasCaseNumber := payload.CaseNumber, payload.HasCaseNumber
	if !hasCaseNumber || caseNumber <= 0 {
		caseNumber, hasCaseNumber = nextGuildCaseNumber(ctx)
	}

	action := strings.TrimSpace(payload.Action)
	if action == "" {
		action = "member_ban_add"
	}
	actionType := resolveModerationActionType(action)
	actionLabel, targetFieldName, detailsFieldName := resolveModerationCaseEmbedMeta(action, actionType)
	reason := strings.TrimSpace(payload.Reason)
	if reason == "" {
		reason = "No reason provided"
	}
	targetID := strings.TrimSpace(payload.TargetID)
	targetLabel := strings.TrimSpace(payload.TargetLabel)
	targetValue := "Unknown target"
	switch {
	case targetID == "" && targetLabel != "":
		targetValue = targetLabel
	case targetID != "" && (targetLabel == "" || targetLabel == targetID):
		targetValue = fmt.Sprintf("<@%s> (`%s`)", targetID, targetID)
	case targetID != "":
		targetValue = fmt.Sprintf("**%s** (<@%s>, `%s`)", targetLabel, targetID, targetID)
	}
	actorID := strings.TrimSpace(payload.RequestedBy)
	if actorID == "" {
		actorID = botID
	}
	actorValue := fmt.Sprintf("<@%s> (`%s`)", actorID, actorID)

	eventAt := time.Now()
	eventID := strings.TrimSpace(targetID)
	if eventID == "" && ctx.Interaction != nil {
		eventID = strings.TrimSpace(ctx.Interaction.ID)
	}
	if eventID == "" {
		eventID = "unknown"
	}

	descriptionLines := []string{
		fmt.Sprintf("**%s:** %s", targetFieldName, targetValue),
		fmt.Sprintf("**Reason:** %s", reason),
		fmt.Sprintf("**Responsible moderator:** %s", actorValue),
	}
	if payload.Extra != "" {
		descriptionLines = append(descriptionLines, fmt.Sprintf("**%s:** %s", detailsFieldName, payload.Extra))
	}
	descriptionLines = append(descriptionLines, fmt.Sprintf("ID: `%s` • <t:%d:F>", eventID, eventAt.Unix()))

	embed := &discordgo.MessageEmbed{
		Title:       buildModerationCaseTitle(caseNumber, hasCaseNumber, actionLabel),
		Description: strings.Join(descriptionLines, "\n"),
		Color:       theme.AutomodAction(),
		Timestamp:   eventAt.Format(time.RFC3339),
	}

	if _, err := ctx.Session.ChannelMessageSendEmbed(channelID, embed); err != nil {
		log.ErrorLoggerRaw().Error(
			"Failed to send moderation case action log",
			"operation", "moderation.audit_log.case_send_failed",
			"guildID", ctx.GuildID,
			"channelID", channelID,
			"action", action,
			"err", err,
		)
	}
}
