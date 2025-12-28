package metrics

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	"github.com/small-frappuccino/discordcore/pkg/task"
	"github.com/small-frappuccino/discordcore/pkg/util"
)

// RegisterMetricsCommands registers slash commands under the /metrics group.
func RegisterMetricsCommands(router *core.CommandRouter) {
	metricsGroup := core.NewGroupCommand("metrics", "Server statistics and metrics", router.GetPermissionChecker())
	metricsGroup.AddSubCommand(newActivityCommand())
	metricsGroup.AddSubCommand(newServerStatsCommand())
	metricsGroup.AddSubCommand(newBackfillStatusCommand())

	router.RegisterCommand(metricsGroup)
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

	// Open metrics DB (read-only usage)
	dbPath := util.GetMessageDBPath()
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return respondError(s, i, fmt.Sprintf("Failed to open metrics database: %v", err))
	}
	defer db.Close()

	ctxTimeout, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Collect activity
	msgTotalsByChannel, _ := queryTotals(ctxTimeout, db,
		"SELECT channel_id, SUM(count) FROM daily_message_metrics WHERE guild_id=? AND day>=? "+whereChannel(channelID)+" GROUP BY channel_id",
		argsFor(ctx.GuildID, cutoff, channelID)...,
	)
	msgTotalsByUser, _ := queryTotals(ctxTimeout, db,
		"SELECT user_id, SUM(count) FROM daily_message_metrics WHERE guild_id=? AND day>=? "+whereChannel(channelID)+" GROUP BY user_id",
		argsFor(ctx.GuildID, cutoff, channelID)...,
	)

	reactTotalsByChannel, _ := queryTotals(ctxTimeout, db,
		"SELECT channel_id, SUM(count) FROM daily_reaction_metrics WHERE guild_id=? AND day>=? "+whereChannel(channelID)+" GROUP BY channel_id",
		argsFor(ctx.GuildID, cutoff, channelID)...,
	)
	reactTotalsByUser, _ := queryTotals(ctxTimeout, db,
		"SELECT user_id, SUM(count) FROM daily_reaction_metrics WHERE guild_id=? AND day>=? "+whereChannel(channelID)+" GROUP BY user_id",
		argsFor(ctx.GuildID, cutoff, channelID)...,
	)

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
		Color:       0x5865F2, // blurple
		Description: "Message and reaction activity across channels and users.",
		Timestamp:   time.Now().Format(time.RFC3339),
		Fields:      fields,
	}

	return respondEmbed(s, i, embed)
}

// -------- Members Command (weekly/monthly enter/leave/net) --------

func newServerStatsCommand() core.SubCommand {
	return core.NewSimpleCommand(
		"serverstats",
		"Checks server health and member statistics from the database.",
		nil,
		handleServerStats,
		true,  // requiresGuild
		false, // requiresPermissions
	)
}

func handleServerStats(ctx *core.Context) error {
	s := ctx.Session
	i := ctx.Interaction
	if ctx.GuildID == "" {
		return respondError(s, i, "This command must be used in a server.")
	}

	// Open database
	dbPath := util.GetMessageDBPath()
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return respondError(s, i, fmt.Sprintf("Failed to open database: %v", err))
	}
	defer db.Close()

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
	totalHistoricJoins := querySum(ctxTimeout, db, "SELECT COUNT(DISTINCT user_id) FROM member_joins WHERE guild_id=?", ctx.GuildID)

	// 3) How many of those historically recorded members are still in the server
	// Since we don't have a `current_members` table that is 100% reliable in real-time without constant sync,
	// and iterating over all members via API/State can be expensive for large servers,
	// we use a best-effort approach based on what we already have.
	// If we have the member list in State, we can cross-check.

	stillPresentCount := 0
	if totalHistoricJoins > 0 {
		// Fetch all user_ids that have ever joined
		rows, err := db.QueryContext(ctxTimeout, "SELECT DISTINCT user_id FROM member_joins WHERE guild_id=?", ctx.GuildID)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var userID string
				if err := rows.Scan(&userID); err == nil {
					// Check if the user is present in the bot state cache
					if _, err := s.State.Member(ctx.GuildID, userID); err == nil {
						stillPresentCount++
					}
				}
			}
		}
	}

	// If stillPresentCount is 0 but we have members, the state may not be populated (missing member intents).
	// For bots with GuildMembers Intent, State usually contains members if the bot has "seen" them.
	// If stillPresentCount == 0 and totalHistoricJoins > 0 and currentMemberCount > 0, we warn that accuracy depends on the cache.

	fields := []*discordgo.MessageEmbedField{
		{
			Name:   "👥 Current Members",
			Value:  fmt.Sprintf("`%d` members currently in the server.", currentMemberCount),
			Inline: false,
		},
		{
			Name:   "📥 Join History",
			Value:  fmt.Sprintf("`%d` unique users recorded in the database since tracking began.", totalHistoricJoins),
			Inline: false,
		},
		{
			Name:   "✅ Retention",
			Value:  fmt.Sprintf("`%d` of historically recorded users are still in the server.", stillPresentCount),
			Inline: false,
		},
	}

	// Database health (optional, but useful for the "dbhealth" context)
	var dbSize int64
	if info, err := os.Stat(dbPath); err == nil {
		dbSize = info.Size()
	}

	embed := &discordgo.MessageEmbed{
		Title:       "📊 Server Health Stats",
		Color:       0x3498DB, // Blue
		Description: fmt.Sprintf("Data extracted from the database and bot state.\nDatabase size: `%s`", formatBytes(dbSize)),
		Fields:      fields,
		Timestamp:   time.Now().Format(time.RFC3339),
		Footer: &discordgo.MessageEmbedFooter{
			Text: "Note: retention accuracy depends on the bot's member cache.",
		},
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

func whereChannel(channelID string) string {
	if channelID == "" {
		return ""
	}
	return " AND channel_id=? "
}

func argsFor(guildID, cutoff, channelID string) []any {
	if channelID == "" {
		return []any{guildID, cutoff}
	}
	return []any{guildID, cutoff, channelID}
}

type kv struct {
	Key   string
	Total int64
}

func queryTotals(ctx context.Context, db *sql.DB, sqlText string, args ...any) ([]kv, error) {
	rows, err := db.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []kv
	for rows.Next() {
		var k string
		var t sql.NullInt64
		if err := rows.Scan(&k, &t); err != nil {
			return nil, err
		}
		if t.Valid {
			out = append(out, kv{Key: k, Total: t.Int64})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Total > out[j].Total })
	return out, nil
}

func renderTop(items []kv, n int, display func(id string) string) string {
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

func querySum(ctx context.Context, db *sql.DB, sqlText string, args ...any) int64 {
	var v sql.NullInt64
	_ = db.QueryRowContext(ctx, sqlText, args...).Scan(&v)
	if v.Valid {
		return v.Int64
	}
	return 0
}

func tableExists(ctx context.Context, db *sql.DB, tableName string) bool {
	var name string
	err := db.QueryRowContext(ctx,
		"SELECT name FROM sqlite_master WHERE type='table' AND name=? LIMIT 1",
		tableName,
	).Scan(&name)
	return err == nil && name == tableName
}

func formatMaybe(v int64, ok bool) string {
	if !ok {
		return "N/A"
	}
	return fmt.Sprintf("%d", v)
}

func formatMaybeNet(enters, leaves int64, hasLeaves bool) string {
	if !hasLeaves {
		return fmt.Sprintf("%d (without leaves)", enters)
	}
	return fmt.Sprintf("%d", enters-leaves)
}

// -------- Backfill Status Command --------

func newBackfillStatusCommand() core.SubCommand {
	return core.NewSimpleCommand(
		"backfill-status",
		"Shows the progress of welcome channel message reading for entry/exit metrics.",
		nil,
		handleBackfillStatus,
		true,  // requiresGuild
		false, // requiresPermissions
	)
}

func handleBackfillStatus(ctx *core.Context) error {
	s := ctx.Session
	i := ctx.Interaction

	if ctx.GuildConfig == nil {
		return respondError(s, i, "Guild configuration not found.")
	}

	channelID := strings.TrimSpace(ctx.GuildConfig.WelcomeBacklogChannelID)
	if channelID == "" {
		channelID = strings.TrimSpace(ctx.GuildConfig.UserEntryLeaveChannelID)
	}

	if channelID == "" {
		return respondError(s, i, "No welcome or entry/leave channel configured for this server.")
	}

	dbPath := util.GetMessageDBPath()
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return respondError(s, i, fmt.Sprintf("Failed to open database: %v", err))
	}
	defer db.Close()

	// 1) First message date
	var firstMsgDate string = "Unknown"
	// Actually, to get the absolute first message, we can't easily do it with a single call unless we know the snowflake.
	// Snowflake "0" or "1" is the beginning of time.
	msgs, err := s.ChannelMessages(channelID, 1, "", "1", "") // after "1"
	if err == nil && len(msgs) > 0 {
		firstMsgDate = msgs[0].Timestamp.UTC().Format("2006-01-02 15:04:05 UTC")
	}

	// 2) Last processed message date (from metadata)
	var lastProcessedDate string = "Never"
	row := db.QueryRow(`SELECT ts FROM runtime_meta WHERE key=?`, "backfill_progress:"+channelID)
	var ts time.Time
	if err := row.Scan(&ts); err == nil {
		lastProcessedDate = ts.UTC().Format("2006-01-02 15:04:05 UTC")
	}

	// 3) Global last event
	var lastEventDate string = "Never"
	row = db.QueryRow(`SELECT ts FROM runtime_meta WHERE key=?`, "last_event")
	if err := row.Scan(&ts); err == nil {
		lastEventDate = ts.UTC().Format("2006-01-02 15:04:05 UTC")
	}

	embed := &discordgo.MessageEmbed{
		Title: "📖 Backfill & Reading Status",
		Color: 0x2ECC71, // Green
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:  "📍 Target Channel",
				Value: fmt.Sprintf("<#%s> (`%s`)", channelID, channelID),
			},
			{
				Name:   "📅 First Message in Channel",
				Value:  fmt.Sprintf("`%s`", firstMsgDate),
				Inline: true,
			},
			{
				Name:   "🕒 Last Processed (Backfill)",
				Value:  fmt.Sprintf("`%s`", lastProcessedDate),
				Inline: true,
			},
			{
				Name:  "📡 Last Live Event Received",
				Value: fmt.Sprintf("`%s`", lastEventDate),
			},
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: "Reading progress tracks how far back the bot has scanned for entry/exit logs.",
		},
	}

	return respondEmbed(s, i, embed)
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
			channelID = strings.TrimSpace(ctx.GuildConfig.WelcomeBacklogChannelID)
			if channelID == "" {
				channelID = strings.TrimSpace(ctx.GuildConfig.UserEntryLeaveChannelID)
			}
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
		Color:       0x3498DB, // Blue
		Footer: &discordgo.MessageEmbedFooter{
			Text: "This process runs in the background. Use /metrics backfill-status to check progress.",
		},
	}

	return respondEmbed(s, i, embed)
}
