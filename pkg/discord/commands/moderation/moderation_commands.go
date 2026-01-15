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
