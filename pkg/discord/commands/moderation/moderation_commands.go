package moderation

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	"github.com/small-frappuccino/discordcore/pkg/discord/logging"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/theme"
)

const (
	maxAuditLogReasonLen = 512
	minSnowflakeLength   = 15
	maxSnowflakeLength   = 21
	cleanMaxDelete       = 100
	cleanMaxFetch        = 500
)

var userMentionRe = regexp.MustCompile(`^<@!?(\d+)>$`)

// RegisterModerationCommands registers slash commands under the /moderation group.
func RegisterModerationCommands(router *core.CommandRouter) {
	checker := router.GetPermissionChecker()
	if checker == nil {
		checker = core.NewPermissionChecker(router.GetSession(), router.GetConfigManager())
	}
	moderationGroup := core.NewGroupCommand("moderation", "Moderation commands", checker)

	moderationGroup.AddSubCommand(newBanCommand())
	moderationGroup.AddSubCommand(newMassBanCommand())

	router.RegisterCommand(newCleanCommand())
	router.RegisterCommand(moderationGroup)
}

type banCommand struct{}

func newBanCommand() *banCommand { return &banCommand{} }

func (c *banCommand) Name() string { return "ban" }

func (c *banCommand) Description() string { return "Ban a user by ID or mention" }

func (c *banCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "user",
			Description: "User ID or mention to ban",
			Required:    true,
		},
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "reason",
			Description: "Reason for the ban",
			Required:    false,
		},
	}
}

func (c *banCommand) RequiresGuild() bool { return true }

func (c *banCommand) RequiresPermissions() bool { return true }

func (c *banCommand) Handle(ctx *core.Context) error {
	extractor := core.NewOptionExtractor(core.GetSubCommandOptions(ctx.Interaction))

	rawUserID, err := extractor.StringRequired("user")
	if err != nil {
		return core.NewCommandError(err.Error(), true)
	}

	userID, ok := normalizeUserID(rawUserID)
	if !ok {
		return core.NewCommandError("Invalid user ID or mention.", true)
	}

	reason, truncated := sanitizeReason(extractor.String("reason"))

	banCtx, err := prepareBanContext(ctx)
	if err != nil {
		return err
	}

	if ok, reasonText := canBanTarget(ctx, banCtx, userID); !ok {
		return core.NewCommandError(fmt.Sprintf("Cannot ban `%s`: %s.", userID, reasonText), true)
	}

	if err := ctx.Session.GuildBanCreateWithReason(ctx.GuildID, userID, reason, 0); err != nil {
		return core.NewCommandError(fmt.Sprintf("Failed to ban user %s: %v", userID, err), true)
	}

	message := fmt.Sprintf("Banned user `%s`. Reason: %s", userID, reason)
	if truncated {
		message += " (reason truncated to 512 characters)"
	}
	sendModerationLog(ctx, moderationLogPayload{
		Action:      "ban",
		TargetID:    userID,
		TargetLabel: userID,
		Reason:      reason,
		RequestedBy: ctx.UserID,
	})
	return core.NewResponseBuilder(ctx.Session).Success(ctx.Interaction, message)
}

type massBanCommand struct{}

func newMassBanCommand() *massBanCommand { return &massBanCommand{} }

func (c *massBanCommand) Name() string { return "massban" }

func (c *massBanCommand) Description() string { return "Ban multiple users by ID or mention" }

func (c *massBanCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "members",
			Description: "Space, comma, or semicolon separated user IDs or mentions",
			Required:    true,
		},
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "reason",
			Description: "Reason for the bans",
			Required:    false,
		},
	}
}

func (c *massBanCommand) RequiresGuild() bool { return true }

func (c *massBanCommand) RequiresPermissions() bool { return true }

func (c *massBanCommand) Handle(ctx *core.Context) error {
	extractor := core.NewOptionExtractor(core.GetSubCommandOptions(ctx.Interaction))

	membersInput, err := extractor.StringRequired("members")
	if err != nil {
		return core.NewCommandError(err.Error(), true)
	}

	memberIDs, invalidTokens := parseMemberIDs(membersInput)
	if len(memberIDs) == 0 {
		return core.NewCommandError("No valid member IDs provided", true)
	}

	reason, truncated := sanitizeReason(extractor.String("reason"))

	banCtx, err := prepareBanContext(ctx)
	if err != nil {
		return err
	}

	bannedCount := 0
	var failed []string
	var skipped []string
	for _, memberID := range memberIDs {
		ok, reasonText := canBanTarget(ctx, banCtx, memberID)
		if !ok {
			skipped = append(skipped, fmt.Sprintf("%s (%s)", memberID, reasonText))
			continue
		}

		if err := ctx.Session.GuildBanCreateWithReason(ctx.GuildID, memberID, reason, 0); err != nil {
			failed = append(failed, fmt.Sprintf("%s (%v)", memberID, err))
			continue
		}
		bannedCount++
		sendModerationLog(ctx, moderationLogPayload{
			Action:      "ban",
			TargetID:    memberID,
			TargetLabel: memberID,
			Reason:      reason,
			RequestedBy: ctx.UserID,
		})
	}

	message := buildMassBanMessage(len(memberIDs), bannedCount, reason, truncated, invalidTokens, skipped, failed)
	return core.NewResponseBuilder(ctx.Session).Success(ctx.Interaction, message)
}

type cleanCommand struct{}

func newCleanCommand() *cleanCommand { return &cleanCommand{} }

func (c *cleanCommand) Name() string { return "clean" }

func (c *cleanCommand) Description() string {
	return "Delete recent messages in this channel"
}

func (c *cleanCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionInteger,
			Name:        "num",
			Description: "How many messages to delete (max 100)",
			Required:    true,
			MinValue:    floatPtr(1),
			MaxValue:    float64(cleanMaxDelete),
		},
		{
			Type:        discordgo.ApplicationCommandOptionUser,
			Name:        "user",
			Description: "Only delete messages from a specific user",
			Required:    false,
		},
	}
}

func (c *cleanCommand) RequiresGuild() bool { return true }

func (c *cleanCommand) RequiresPermissions() bool { return true }

func (c *cleanCommand) Handle(ctx *core.Context) error {
	if ctx == nil || ctx.Session == nil || ctx.Interaction == nil {
		return core.NewCommandError("Session not ready. Try again shortly.", true)
	}
	channelID := strings.TrimSpace(ctx.Interaction.ChannelID)
	if channelID == "" {
		return core.NewCommandError("Channel not available for this command.", true)
	}

	extractor := core.NewOptionExtractor(ctx.Interaction.ApplicationCommandData().Options)
	num := extractor.Int("num")
	if num <= 0 {
		return core.NewCommandError("Please provide a valid number of messages to delete.", true)
	}
	if num > cleanMaxDelete {
		return core.NewCommandError("You can delete up to 100 messages at a time.", true)
	}

	userID, userLabel, err := resolveCleanUserOption(ctx)
	if err != nil {
		return core.NewCommandError(err.Error(), true)
	}

	if err := ensureManageMessagesPermission(ctx); err != nil {
		return err
	}

	messages, err := fetchMessagesForClean(ctx.Session, channelID, int(num), userID)
	if err != nil {
		return core.NewCommandError(fmt.Sprintf("Failed to fetch messages: %v", err), true)
	}
	if len(messages) == 0 {
		return core.NewResponseBuilder(ctx.Session).Ephemeral().Info(ctx.Interaction, "No matching messages found.")
	}

	cutoff := time.Now().Add(-14 * 24 * time.Hour)
	deleteIDs := make([]string, 0, len(messages))
	skippedOld := 0
	for _, msg := range messages {
		if msg == nil || msg.ID == "" {
			continue
		}
		if !msg.Timestamp.IsZero() && msg.Timestamp.Before(cutoff) {
			skippedOld++
			continue
		}
		deleteIDs = append(deleteIDs, msg.ID)
	}
	if len(deleteIDs) == 0 {
		return core.NewResponseBuilder(ctx.Session).Ephemeral().Info(ctx.Interaction, "No messages could be deleted (all were too old).")
	}

	deleted, failed := deleteMessageIDs(ctx.Session, channelID, deleteIDs)
	filterLabel := "any user"
	if userID != "" {
		filterLabel = "<@" + userID + ">"
	}

	log.ApplicationLogger().Info(
		"Clean command executed",
		"guildID", ctx.GuildID,
		"channelID", channelID,
		"requested", num,
		"deleted", deleted,
		"skipped_old", skippedOld,
		"failed", failed,
		"user_filter", userID,
	)

	sendModerationLog(ctx, moderationLogPayload{
		Action:      "clean",
		TargetID:    userID,
		TargetLabel: userLabel,
		Reason:      fmt.Sprintf("Deleted %d message(s)", deleted),
		RequestedBy: ctx.UserID,
		Extra:       fmt.Sprintf("Channel: <#%s> (`%s`) | Filter: %s | Requested: %d | Deleted: %d | Skipped (old): %d | Failed: %d", channelID, channelID, filterLabel, num, deleted, skippedOld, failed),
	})

	message := fmt.Sprintf("Deleted %d message(s) in <#%s>.", deleted, channelID)
	if userID != "" {
		message = fmt.Sprintf("Deleted %d message(s) from <@%s> in <#%s>.", deleted, userID, channelID)
	}
	if skippedOld > 0 {
		message += fmt.Sprintf(" Skipped %d message(s) older than 14 days.", skippedOld)
	}
	if failed > 0 {
		message += fmt.Sprintf(" Failed to delete %d message(s).", failed)
	}

	return core.NewResponseBuilder(ctx.Session).Ephemeral().Success(ctx.Interaction, message)
}

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

func buildMassBanMessage(total, banned int, reason string, truncated bool, invalid, skipped, failed []string) string {
	message := fmt.Sprintf("Banned %d of %d user(s). Reason: %s", banned, total, reason)
	if truncated {
		message += "\nNote: reason truncated to 512 characters."
	}
	if len(invalid) > 0 {
		message += "\nInvalid: " + strings.Join(invalid, ", ")
	}
	if len(skipped) > 0 {
		message += "\nSkipped: " + strings.Join(skipped, "; ")
	}
	if len(failed) > 0 {
		message += "\nFailed: " + strings.Join(failed, "; ")
	}
	return message
}

func resolveCleanUserOption(ctx *core.Context) (string, string, error) {
	if ctx == nil || ctx.Interaction == nil {
		return "", "", nil
	}
	for _, opt := range ctx.Interaction.ApplicationCommandData().Options {
		if opt == nil || opt.Name != "user" {
			continue
		}
		switch opt.Type {
		case discordgo.ApplicationCommandOptionUser:
			if user := opt.UserValue(ctx.Session); user != nil {
				return user.ID, user.Username, nil
			}
			if raw, ok := opt.Value.(string); ok {
				if normalized, ok := normalizeUserID(raw); ok {
					return normalized, "", nil
				}
				return "", "", fmt.Errorf("Invalid user ID or mention.")
			}
		case discordgo.ApplicationCommandOptionString:
			raw := strings.TrimSpace(opt.StringValue())
			if raw == "" {
				return "", "", nil
			}
			normalized, ok := normalizeUserID(raw)
			if !ok {
				return "", "", fmt.Errorf("Invalid user ID or mention.")
			}
			return normalized, "", nil
		}
	}
	return "", "", nil
}

func ensureManageMessagesPermission(ctx *core.Context) error {
	if ctx == nil || ctx.Session == nil {
		return core.NewCommandError("Session not ready. Try again shortly.", true)
	}

	roles, err := getGuildRoles(ctx.Session, ctx.GuildID)
	if err != nil {
		return core.NewCommandError("Failed to resolve server roles.", true)
	}
	rolesByID := buildRoleIndex(roles)

	ownerID, _ := getGuildOwnerID(ctx.Session, ctx.GuildID)

	botID := ""
	if ctx.Session.State != nil && ctx.Session.State.User != nil {
		botID = ctx.Session.State.User.ID
	}
	if botID == "" {
		return core.NewCommandError("Bot identity not available.", true)
	}

	actorMember := ctx.Interaction.Member
	if actorMember == nil || actorMember.User == nil {
		var ok bool
		actorMember, ok = getMember(ctx.Session, ctx.GuildID, ctx.UserID)
		if !ok || actorMember == nil {
			return core.NewCommandError("Unable to resolve your member record.", true)
		}
	}

	botMember, ok := getMember(ctx.Session, ctx.GuildID, botID)
	if !ok || botMember == nil {
		return core.NewCommandError("Unable to resolve the bot member record.", true)
	}

	actorIsOwner := ctx.IsOwner || (ownerID != "" && ctx.UserID == ownerID)
	botIsOwner := ownerID != "" && botID == ownerID

	if !actorIsOwner && !memberHasPermission(actorMember, rolesByID, ctx.GuildID, ownerID, discordgo.PermissionManageMessages) {
		return core.NewCommandError("You need the Manage Messages permission to use this command.", true)
	}
	if !botIsOwner && !memberHasPermission(botMember, rolesByID, ctx.GuildID, ownerID, discordgo.PermissionManageMessages) {
		return core.NewCommandError("I need the Manage Messages permission to delete messages.", true)
	}
	return nil
}

func fetchMessagesForClean(session *discordgo.Session, channelID string, target int, userID string) ([]*discordgo.Message, error) {
	if session == nil || channelID == "" || target <= 0 {
		return nil, nil
	}
	var out []*discordgo.Message
	beforeID := ""
	fetched := 0
	for fetched < cleanMaxFetch && len(out) < target {
		batchSize := minInt(100, cleanMaxFetch-fetched)
		msgs, err := session.ChannelMessages(channelID, batchSize, beforeID, "", "")
		if err != nil {
			return nil, err
		}
		if len(msgs) == 0 {
			break
		}
		fetched += len(msgs)
		for _, msg := range msgs {
			if msg == nil || msg.ID == "" {
				continue
			}
			if userID != "" {
				if msg.Author == nil || msg.Author.ID != userID {
					continue
				}
			}
			out = append(out, msg)
			if len(out) >= target {
				break
			}
		}
		beforeID = msgs[len(msgs)-1].ID
	}
	return out, nil
}

func deleteMessageIDs(session *discordgo.Session, channelID string, ids []string) (int, int) {
	if session == nil || channelID == "" || len(ids) == 0 {
		return 0, 0
	}
	deleted := 0
	failed := 0
	for _, chunk := range chunkStrings(ids, 100) {
		if len(chunk) == 1 {
			if err := session.ChannelMessageDelete(channelID, chunk[0]); err != nil {
				failed++
				continue
			}
			deleted++
			continue
		}
		if err := session.ChannelMessagesBulkDelete(channelID, chunk); err != nil {
			failed += len(chunk)
			continue
		}
		deleted += len(chunk)
	}
	return deleted, failed
}

func chunkStrings(values []string, size int) [][]string {
	if size <= 0 {
		return nil
	}
	var out [][]string
	for len(values) > 0 {
		if len(values) <= size {
			out = append(out, values)
			break
		}
		out = append(out, values[:size])
		values = values[size:]
	}
	return out
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func floatPtr(v float64) *float64 { return &v }

type banContext struct {
	rolesByID    map[string]*discordgo.Role
	ownerID      string
	botID        string
	actorMember  *discordgo.Member
	botMember    *discordgo.Member
	actorIsOwner bool
	botIsOwner   bool
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
	if ctx == nil || ctx.Session == nil {
		return nil, core.NewCommandError("Session not ready. Try again shortly.", true)
	}

	roles, err := getGuildRoles(ctx.Session, ctx.GuildID)
	if err != nil {
		return nil, core.NewCommandError("Failed to resolve server roles.", true)
	}
	rolesByID := buildRoleIndex(roles)

	ownerID, _ := getGuildOwnerID(ctx.Session, ctx.GuildID)

	botID := ""
	if ctx.Session.State != nil && ctx.Session.State.User != nil {
		botID = ctx.Session.State.User.ID
	}
	if botID == "" {
		return nil, core.NewCommandError("Bot identity not available.", true)
	}

	actorMember := ctx.Interaction.Member
	if actorMember == nil || actorMember.User == nil {
		var ok bool
		actorMember, ok = getMember(ctx.Session, ctx.GuildID, ctx.UserID)
		if !ok || actorMember == nil {
			return nil, core.NewCommandError("Unable to resolve your member record.", true)
		}
	}

	botMember, ok := getMember(ctx.Session, ctx.GuildID, botID)
	if !ok || botMember == nil {
		return nil, core.NewCommandError("Unable to resolve the bot member record.", true)
	}

	actorIsOwner := ctx.IsOwner || (ownerID != "" && ctx.UserID == ownerID)
	botIsOwner := ownerID != "" && botID == ownerID

	if !actorIsOwner && !memberHasPermission(actorMember, rolesByID, ctx.GuildID, ownerID, discordgo.PermissionBanMembers) {
		return nil, core.NewCommandError("You need the Ban Members permission to use this command.", true)
	}
	if !botIsOwner && !memberHasPermission(botMember, rolesByID, ctx.GuildID, ownerID, discordgo.PermissionBanMembers) {
		return nil, core.NewCommandError("I need the Ban Members permission to ban members.", true)
	}

	return &banContext{
		rolesByID:    rolesByID,
		ownerID:      ownerID,
		botID:        botID,
		actorMember:  actorMember,
		botMember:    botMember,
		actorIsOwner: actorIsOwner,
		botIsOwner:   botIsOwner,
		actorRolePos: highestRolePosition(actorMember, rolesByID, ctx.GuildID),
		botRolePos:   highestRolePosition(botMember, rolesByID, ctx.GuildID),
	}, nil
}

func canBanTarget(ctx *core.Context, banCtx *banContext, targetID string) (bool, string) {
	if targetID == ctx.UserID {
		return false, "cannot ban yourself"
	}
	if targetID == banCtx.botID {
		return false, "cannot ban the bot"
	}
	if banCtx.ownerID != "" && targetID == banCtx.ownerID {
		return false, "cannot ban the server owner"
	}

	targetMember, ok := getMember(ctx.Session, ctx.GuildID, targetID)
	if !ok || targetMember == nil {
		return true, ""
	}

	targetPos := highestRolePosition(targetMember, banCtx.rolesByID, ctx.GuildID)
	if !banCtx.actorIsOwner && banCtx.actorRolePos <= targetPos {
		return false, "target has an equal or higher role than you"
	}
	if !banCtx.botIsOwner && banCtx.botRolePos <= targetPos {
		return false, "target has an equal or higher role than the bot"
	}
	return true, ""
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

func getGuildRoles(session *discordgo.Session, guildID string) ([]*discordgo.Role, error) {
	if session == nil {
		return nil, fmt.Errorf("session not ready")
	}
	if session.State != nil {
		if g, _ := session.State.Guild(guildID); g != nil && len(g.Roles) > 0 {
			return g.Roles, nil
		}
	}
	return session.GuildRoles(guildID)
}

func getGuildOwnerID(session *discordgo.Session, guildID string) (string, bool) {
	if session == nil || guildID == "" {
		return "", false
	}
	if session.State != nil {
		if g, _ := session.State.Guild(guildID); g != nil && g.OwnerID != "" {
			return g.OwnerID, true
		}
	}
	guild, err := session.Guild(guildID)
	if err != nil || guild == nil || guild.OwnerID == "" {
		return "", false
	}
	return guild.OwnerID, true
}

func getMember(session *discordgo.Session, guildID, userID string) (*discordgo.Member, bool) {
	if session == nil || guildID == "" || userID == "" {
		return nil, false
	}
	if session.State != nil {
		if m, _ := session.State.Member(guildID, userID); m != nil {
			return m, true
		}
	}
	member, err := session.GuildMember(guildID, userID)
	if err != nil || member == nil {
		return nil, false
	}
	return member, true
}

func memberHasPermission(member *discordgo.Member, rolesByID map[string]*discordgo.Role, guildID, ownerID string, perm int64) bool {
	if member == nil || member.User == nil {
		return false
	}
	if ownerID != "" && member.User.ID == ownerID {
		return true
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
	Action      string
	TargetID    string
	TargetLabel string
	Reason      string
	RequestedBy string
	Extra       string
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

func sendModerationLog(ctx *core.Context, payload moderationLogPayload) {
	if ctx == nil || ctx.Session == nil || ctx.Config == nil || ctx.GuildID == "" {
		return
	}
	botID := ""
	if ctx.Session.State != nil && ctx.Session.State.User != nil {
		botID = ctx.Session.State.User.ID
	}
	if !logging.ShouldLogModerationEvent(ctx.Config, ctx.GuildID, botID, botID, logging.ModerationSourceCommand) {
		return
	}
	channelID, ok := logging.ResolveModerationLogChannel(ctx.Session, ctx.Config, ctx.GuildID)
	if !ok {
		return
	}

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
		log.ErrorLoggerRaw().Error("Failed to send moderation log", "guildID", ctx.GuildID, "channelID", channelID, "action", action, "err", err)
	}
}
