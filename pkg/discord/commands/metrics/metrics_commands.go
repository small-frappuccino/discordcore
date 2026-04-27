package metrics

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/storage"
	"github.com/small-frappuccino/discordcore/pkg/task"
	"github.com/small-frappuccino/discordcore/pkg/theme"
)

// RegisterMetricsCommands registers slash commands under the /metrics group.
func RegisterMetricsCommands(router *core.CommandRouter) {
	metricsGroup := core.NewGroupCommand("metrics", "Server statistics and metrics", router.GetPermissionChecker())
	activityCommand := newActivityCommand()
	serverStatsGroup := core.NewGroupCommand("serverstats", "Server health and member statistics.", nil)
	serverStatsHealthCommand := newServerStatsHealthCommand()
	serverStatsWeeklyCommand := newServerStatsPeriodicCommand("weekly", "Last 7 days of member metrics.", "7d")
	serverStatsMonthlyCommand := newServerStatsPeriodicCommand("monthly", "Last 30 days of member metrics.", "30d")
	serverStatsThreeMonthsCommand := newServerStatsPeriodicCommand("3months", "Last 90 days of member metrics.", "90d")

	serverStatsGroup.AddSubCommand(serverStatsHealthCommand)
	serverStatsGroup.AddSubCommand(serverStatsWeeklyCommand)
	serverStatsGroup.AddSubCommand(serverStatsMonthlyCommand)
	serverStatsGroup.AddSubCommand(serverStatsThreeMonthsCommand)
	metricsGroup.AddSubCommand(activityCommand)
	metricsGroup.AddSubCommand(serverStatsGroup)

	router.RegisterSlashCommand(metricsGroup)
}

// -------- Activity Command (messages + reactions) --------

func newActivityCommand() core.SubCommand {
	opts := []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "range",
			Description: "Time range to aggregate",
			Required:    false,
			Choices: []*discordgo.ApplicationCommandOptionChoice{
				{Name: "24h", Value: "24h"},
				{Name: "7d", Value: "7d"},
				{Name: "30d", Value: "30d"},
			},
		},
		{
			Type:        discordgo.ApplicationCommandOptionChannel,
			Name:        "channel",
			Description: "Filter by channel",
			Required:    false,
		},
		{
			Type:        discordgo.ApplicationCommandOptionInteger,
			Name:        "top",
			Description: "How many items to show per section (default 5, max 10)",
			Required:    false,
			MinValue:    floatPtr(1),
		},
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "section",
			Description: "Which sections to include",
			Required:    false,
			Choices: []*discordgo.ApplicationCommandOptionChoice{
				{Name: "both", Value: "both"},
				{Name: "messages", Value: "messages"},
				{Name: "reactions", Value: "reactions"},
			},
		},
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "scope",
			Description: "Aggregate by channels, users, or both",
			Required:    false,
			Choices: []*discordgo.ApplicationCommandOptionChoice{
				{Name: "both", Value: "both"},
				{Name: "channels", Value: "channels"},
				{Name: "users", Value: "users"},
			},
		},
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "format",
			Description: "Embed format",
			Required:    false,
			Choices: []*discordgo.ApplicationCommandOptionChoice{
				{Name: "full", Value: "full"},
				{Name: "compact", Value: "compact"},
			},
		},
	}

	return core.NewSimpleCommand(
		"activity",
		"Show message and reaction activity (by channels and users)",
		opts,
		handleActivity,
		true,  // requiresGuild
		false, // requiresPermissions
	)
}

func handleActivity(ctx *core.Context) error {
	s := ctx.Session
	i := ctx.Interaction
	if ctx.GuildID == "" {
		return respondError(s, i, "This command must be used in a server.")
	}

	// Parse options
	rangeOpt := getStringOpt(i, "range", "7d")
	topN := clampInt(int(getIntOpt(i, "top", 5)), 1, 10)
	channelID := getChannelOpt(s, i, "channel", "")
	section := strings.ToLower(getStringOpt(i, "section", "both")) // both|messages|reactions
	scope := strings.ToLower(getStringOpt(i, "scope", "both"))     // both|channels|users
	format := strings.ToLower(getStringOpt(i, "format", "full"))   // full|compact
	if format == "compact" {
		topN = clampInt(topN, 1, 3)
	}

	cutoff, label := cutoffForRange(rangeOpt)

	store := ctx.Router().GetStore()
	if store == nil {
		return respondError(s, i, "Metrics storage is not configured.")
	}

	ctxTimeout, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Collect activity
	msgTotalsByChannel, err := store.MessageTotalsByChannel(ctxTimeout, ctx.GuildID, cutoff, channelID)
	if err != nil {
		log.ErrorLoggerRaw().Error(
			"Metrics activity query failed",
			"operation", "metrics.activity.query.message_totals_by_channel",
			"guildID", ctx.GuildID,
			"channelID", channelID,
			"cutoffDay", cutoff,
			"err", err,
		)
		return respondError(s, i, "Failed to query activity metrics from the database. Try again shortly.")
	}

	msgTotalsByUser, err := store.MessageTotalsByUser(ctxTimeout, ctx.GuildID, cutoff, channelID)
	if err != nil {
		log.ErrorLoggerRaw().Error(
			"Metrics activity query failed",
			"operation", "metrics.activity.query.message_totals_by_user",
			"guildID", ctx.GuildID,
			"channelID", channelID,
			"cutoffDay", cutoff,
			"err", err,
		)
		return respondError(s, i, "Failed to query activity metrics from the database. Try again shortly.")
	}

	reactTotalsByChannel, err := store.ReactionTotalsByChannel(ctxTimeout, ctx.GuildID, cutoff, channelID)
	if err != nil {
		log.ErrorLoggerRaw().Error(
			"Metrics activity query failed",
			"operation", "metrics.activity.query.reaction_totals_by_channel",
			"guildID", ctx.GuildID,
			"channelID", channelID,
			"cutoffDay", cutoff,
			"err", err,
		)
		return respondError(s, i, "Failed to query activity metrics from the database. Try again shortly.")
	}

	reactTotalsByUser, err := store.ReactionTotalsByUser(ctxTimeout, ctx.GuildID, cutoff, channelID)
	if err != nil {
		log.ErrorLoggerRaw().Error(
			"Metrics activity query failed",
			"operation", "metrics.activity.query.reaction_totals_by_user",
			"guildID", ctx.GuildID,
			"channelID", channelID,
			"cutoffDay", cutoff,
			"err", err,
		)
		return respondError(s, i, "Failed to query activity metrics from the database. Try again shortly.")
	}

	// Build embed
	chFilterStr := ""
	if channelID != "" {
		chFilterStr = fmt.Sprintf(" in <#%s>", channelID)
	}
	title := fmt.Sprintf("Activity: %s%s", label, chFilterStr)

	fields := []*discordgo.MessageEmbedField{}
	includeMessages := section == "both" || section == "messages"
	includeReactions := section == "both" || section == "reactions"
	includeChannels := scope == "both" || scope == "channels"
	includeUsers := scope == "both" || scope == "users"

	if includeMessages && includeChannels {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "Messages - Top Channels",
			Value:  renderTop(msgTotalsByChannel, topN, func(id string) string { return channelMention(id) }),
			Inline: true,
		})
	}
	if includeMessages && includeUsers {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "Messages - Top Users",
			Value:  renderTop(msgTotalsByUser, topN, func(id string) string { return userMention(id) }),
			Inline: true,
		})
	}
	if includeReactions && includeChannels {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "Reactions - Top Channels",
			Value:  renderTop(reactTotalsByChannel, topN, func(id string) string { return channelMention(id) }),
			Inline: true,
		})
	}
	if includeReactions && includeUsers {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "Reactions - Top Users",
			Value:  renderTop(reactTotalsByUser, topN, func(id string) string { return userMention(id) }),
			Inline: true,
		})
	}

	embed := &discordgo.MessageEmbed{
		Title:       title,
		Color:       theme.Primary(),
		Description: "Message and reaction activity across channels and users.",
		Timestamp:   time.Now().Format(time.RFC3339),
		Fields:      fields,
	}

	return respondEmbed(s, i, embed)
}

// -------- Members Command (weekly/monthly enter/leave/net) --------

func newServerStatsHealthCommand() core.SubCommand {
	return core.NewSimpleCommand(
		"health",
		"Checks server health and absolute member statistics.",
		nil,
		handleServerStatsHealth,
		true,
		false,
	)
}

func newServerStatsPeriodicCommand(name, desc, rangeVal string) core.SubCommand {
	return core.NewSimpleCommand(
		name,
		desc,
		nil,
		func(ctx *core.Context) error {
			return handleServerStatsPeriodic(ctx, rangeVal)
		},
		true,
		false,
	)
}

func handleServerStatsHealth(ctx *core.Context) error {
	s := ctx.Session
	i := ctx.Interaction
	if ctx.GuildID == "" {
		return respondError(s, i, "This command must be used in a server.")
	}

	store := ctx.Router().GetStore()
	if store == nil {
		return respondError(s, i, "Metrics storage is not configured.")
	}

	ctxTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 1) Current members (only those currently in the server)
	currentMemberCount := 0
	if g, err := s.State.Guild(ctx.GuildID); err == nil && g != nil {
		currentMemberCount = g.MemberCount
	}
	if currentMemberCount == 0 {
		// Fallback if the state doesn't have the information
		if g, err := s.Guild(ctx.GuildID); err == nil && g != nil {
			currentMemberCount = g.MemberCount
		}
	}

	// 2) Historical total of members who have ever joined (based on member_joins)
	totalHistoricJoins, totalHistoricJoinsErr := store.CountDistinctMemberJoins(ctxTimeout, ctx.GuildID)
	hasHistoricJoins := totalHistoricJoinsErr == nil

	// 3) How many of those historically recorded members are still in the server
	stillPresentCount := int64(0)
	hasStillPresent := hasHistoricJoins
	if hasHistoricJoins && totalHistoricJoins > 0 {
		joinedUserIDs, err := store.ListDistinctMemberJoinUserIDs(ctxTimeout, ctx.GuildID)
		if err != nil {
			hasStillPresent = false
			log.ErrorLoggerRaw().Error(
				"Metrics health retention query failed",
				"operation", "metrics.serverstats.health.retention_query",
				"guildID", ctx.GuildID,
				"err", err,
			)
		} else {
			for _, userID := range joinedUserIDs {
				// Check if the user is present in the bot state cache.
				if _, err := s.State.Member(ctx.GuildID, userID); err == nil {
					stillPresentCount++
				}
			}
		}
	}

	fields := []*discordgo.MessageEmbedField{
		{
			Name:   "👥 Current Members",
			Value:  fmt.Sprintf("`%d` members currently in the server.", currentMemberCount),
			Inline: false,
		},
		{
			Name:   "📥 Join History",
			Value:  fmt.Sprintf("`%s` unique users recorded in the database since tracking began.", formatMaybe(totalHistoricJoins, hasHistoricJoins)),
			Inline: false,
		},
		{
			Name:   "✅ Retention",
			Value:  fmt.Sprintf("`%s` of historically recorded users are still in the server.", formatMaybe(stillPresentCount, hasStillPresent)),
			Inline: false,
		},
	}

	// Database health
	dbSizeLabel := "N/A"
	if size, err := store.DatabaseSizeBytes(ctxTimeout); err == nil {
		dbSizeLabel = formatBytes(size)
	}

	embed := &discordgo.MessageEmbed{
		Title:       "📊 Server Health Stats",
		Color:       theme.Info(),
		Description: fmt.Sprintf("Data extracted from the database and bot state.\nDatabase size: `%s`", dbSizeLabel),
		Fields:      fields,
		Timestamp:   time.Now().Format(time.RFC3339),
		Footer: &discordgo.MessageEmbedFooter{
			Text: "Note: retention accuracy depends on the bot's member cache.",
		},
	}

	return respondEmbed(s, i, embed)
}

func handleServerStatsPeriodic(ctx *core.Context, rangeVal string) error {
	s := ctx.Session
	i := ctx.Interaction
	if ctx.GuildID == "" {
		return respondError(s, i, "This command must be used in a server.")
	}

	store := ctx.Router().GetStore()
	if store == nil {
		return respondError(s, i, "Metrics storage is not configured.")
	}

	ctxTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cutoff, label := cutoffForRange(rangeVal)

	joins, joinsErr := store.SumDailyMemberJoinsSince(ctxTimeout, ctx.GuildID, cutoff)
	leaves, leavesErr := store.SumDailyMemberLeavesSince(ctxTimeout, ctx.GuildID, cutoff)
	hasJoins := joinsErr == nil
	hasLeaves := leavesErr == nil

	fields := []*discordgo.MessageEmbedField{
		{
			Name:   "📥 Members Joined",
			Value:  fmt.Sprintf("`%s` joins in the last %s.", formatMaybe(joins, hasJoins), label),
			Inline: true,
		},
		{
			Name:   "📤 Members Left",
			Value:  fmt.Sprintf("`%s` leaves in the last %s.", formatMaybe(leaves, hasLeaves), label),
			Inline: true,
		},
		{
			Name:   "📈 Net Growth",
			Value:  fmt.Sprintf("`%s` members.", formatMaybeNet(joins, hasJoins, leaves, hasLeaves)),
			Inline: true,
		},
	}

	embed := &discordgo.MessageEmbed{
		Title:     fmt.Sprintf("📊 Server Stats (%s)", label),
		Color:     theme.Success(),
		Fields:    fields,
		Timestamp: time.Now().Format(time.RFC3339),
	}

	return respondEmbed(s, i, embed)
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// -------- Helpers --------

func respondError(s *discordgo.Session, i *discordgo.InteractionCreate, msg string) error {
	return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: msg,
			Flags:   1 << 6, // ephemeral
		},
	})
}

func respondEmbed(s *discordgo.Session, i *discordgo.InteractionCreate, embed *discordgo.MessageEmbed) error {
	return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
		},
	})
}

func floatPtr(v float64) *float64 { return &v }

func getStringOpt(i *discordgo.InteractionCreate, name, def string) string {
	for _, opt := range i.ApplicationCommandData().Options {
		if opt != nil && opt.Name == name && opt.Value != nil {
			if s, ok := opt.Value.(string); ok && s != "" {
				return s
			}
		}
	}
	return def
}

func getIntOpt(i *discordgo.InteractionCreate, name string, def int64) int64 {
	for _, opt := range i.ApplicationCommandData().Options {
		if opt != nil && opt.Name == name && opt.Value != nil {
			switch v := opt.Value.(type) {
			case int:
				return int64(v)
			case int64:
				return v
			case float64:
				return int64(v)
			}
		}
	}
	return def
}

func getBoolOpt(i *discordgo.InteractionCreate, name string, def bool) bool {
	for _, opt := range i.ApplicationCommandData().Options {
		if opt != nil && opt.Name == name && opt.Value != nil {
			if v, ok := opt.Value.(bool); ok {
				return v
			}
		}
	}
	return def
}

func getChannelOpt(s *discordgo.Session, i *discordgo.InteractionCreate, name, def string) string {
	for _, opt := range i.ApplicationCommandData().Options {
		if opt != nil && opt.Name == name {
			// Preferred: resolve via session-aware ChannelValue (no extra REST calls)
			if ch := opt.ChannelValue(s); ch != nil {
				return ch.ID
			}
			// Fallback: if the option value is already a string channel ID
			if v, ok := opt.Value.(string); ok && v != "" {
				return v
			}
		}
	}
	return def
}

func snowflakeZero() string { return "" }

func cutoffForRange(rangeOpt string) (cutoffDay string, label string) {
	now := time.Now().UTC()
	switch strings.ToLower(rangeOpt) {
	case "24h":
		return dayString(now.Add(-24 * time.Hour)), "Last 24h"
	case "30d":
		return dayString(now.AddDate(0, 0, -29)), "Last 30d"
	case "90d":
		return dayString(now.AddDate(0, 0, -89)), "Last 90d"
	default:
		return dayString(now.AddDate(0, 0, -6)), "Last 7d"
	}
}

func dayString(t time.Time) string {
	// Normalize to date (UTC)
	tt := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	return tt.Format("2006-01-02")
}

func clampInt(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func renderTop(items []storage.MetricTotal, n int, display func(id string) string) string {
	if len(items) == 0 {
		return "_no data_"
	}
	if n > len(items) {
		n = len(items)
	}
	var b strings.Builder
	for idx := 0; idx < n; idx++ {
		it := items[idx]
		fmt.Fprintf(&b, "%d) %s — **%d**\n", idx+1, display(it.Key), it.Total)
	}
	return b.String()
}

func channelMention(id string) string {
	if id == "" {
		return "`unknown-channel`"
	}
	return "<#" + id + ">"
}

func userMention(id string) string {
	if id == "" {
		return "`unknown-user`"
	}
	return "<@" + id + ">"
}

func formatMaybe(v int64, ok bool) string {
	if !ok {
		return "N/A"
	}
	return fmt.Sprintf("%d", v)
}

func formatMaybeNet(enters int64, hasEnters bool, leaves int64, hasLeaves bool) string {
	if !hasEnters || !hasLeaves {
		return "N/A"
	}
	return fmt.Sprintf("%+d", enters-leaves)
}

func newBackfillRunCommand() core.SubCommand {
	return core.NewSimpleCommand(
		"backfill-run",
		"Manually triggers a backfill for entry/exit logs.",
		[]*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionChannel,
				Name:        "channel",
				Description: "Channel to scan. Defaults to configured welcome channel.",
				Required:    false,
			},
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "days",
				Description: "How many days to scan back. Default is 7.",
				Required:    false,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "start_date",
				Description: "Start date (YYYY-MM-DD). If provided, 'days' is ignored.",
				Required:    false,
			},
		},
		handleBackfillRun,
		true, // requiresGuild
		true, // requiresPermissions (Admin)
	)
}

func handleBackfillRun(ctx *core.Context) error {
	s := ctx.Session
	i := ctx.Interaction

	router := ctx.Router()
	if router == nil {
		return respondError(s, i, "Command router not available.")
	}
	taskRouter := router.GetTaskRouter()
	if taskRouter == nil {
		return respondError(s, i, "Task router not available (Monitoring service might be disabled).")
	}

	channelID := getChannelOpt(s, i, "channel", "")
	if channelID == "" {
		if ctx.GuildConfig != nil {
			channelID = ctx.GuildConfig.Channels.BackfillChannelID()
		}
	}

	if channelID == "" {
		return respondError(s, i, "No channel specified and no default welcome channel configured.")
	}

	days := getIntOpt(i, "days", 7)
	startDateRaw := getStringOpt(i, "start_date", "")

	var taskType string
	var payload any
	var desc string

	if startDateRaw != "" {
		// Day mode
		_, err := time.Parse("2006-01-02", startDateRaw)
		if err != nil {
			return respondError(s, i, "Invalid start_date format. Use YYYY-MM-DD.")
		}
		taskType = "monitor.backfill_entry_exit_day"
		payload = struct{ ChannelID, Day string }{ChannelID: channelID, Day: startDateRaw}
		desc = fmt.Sprintf("Scanning channel <#%s> for day `%s`.", channelID, startDateRaw)
	} else {
		// Range mode
		now := time.Now().UTC()
		start := now.AddDate(0, 0, -int(days)).Format(time.RFC3339)
		end := now.Format(time.RFC3339)
		taskType = "monitor.backfill_entry_exit_range"
		payload = struct{ ChannelID, Start, End string }{ChannelID: channelID, Start: start, End: end}
		desc = fmt.Sprintf("Scanning channel <#%s> for the last `%d` days.", channelID, days)
	}

	err := taskRouter.Dispatch(context.Background(), task.Task{
		Type:    taskType,
		Payload: payload,
		Options: task.TaskOptions{GroupKey: "backfill:" + channelID},
	})

	if err != nil {
		return respondError(s, i, fmt.Sprintf("Failed to dispatch backfill task: %v", err))
	}

	embed := &discordgo.MessageEmbed{
		Title:       "▶️ Backfill Started",
		Description: desc,
		Color:       theme.Info(),
		Footer: &discordgo.MessageEmbedFooter{
			Text: "This process runs in the background. Use /metrics backfill-status to check progress.",
		},
	}

	return respondEmbed(s, i, embed)
}
