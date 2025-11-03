package metrics

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	"github.com/small-frappuccino/discordcore/pkg/util"
)

// RegisterMetricsCommands registers slash commands:
// - /activity: message and reactions activity
// - /members: weekly/monthly member enter/leave/net
func RegisterMetricsCommands(router *core.CommandRouter) {
	router.RegisterCommand(newActivityCommand())
	router.RegisterCommand(newMembersCommand())
}

// -------- Activity Command (messages + reactions) --------

func newActivityCommand() core.Command {
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

func newMembersCommand() core.Command {
	opts := []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionBoolean,
			Name:        "show_weekly",
			Description: "Include weekly metrics",
			Required:    false,
		},
		{
			Type:        discordgo.ApplicationCommandOptionBoolean,
			Name:        "show_monthly",
			Description: "Include monthly metrics",
			Required:    false,
		},
		{
			Type:        discordgo.ApplicationCommandOptionBoolean,
			Name:        "include_current",
			Description: "Include current member count",
			Required:    false,
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
		"members",
		"Weekly and monthly member join/leave and net gained",
		opts,
		handleMembers,
		true,  // requiresGuild
		false, // requiresPermissions
	)
}

func handleMembers(ctx *core.Context) error {
	s := ctx.Session
	i := ctx.Interaction
	if ctx.GuildID == "" {
		return respondError(s, i, "This command must be used in a server.")
	}

	// Options
	showWeekly := getBoolOpt(i, "show_weekly", true)
	showMonthly := getBoolOpt(i, "show_monthly", true)
	includeCurrent := getBoolOpt(i, "include_current", true)
	format := strings.ToLower(getStringOpt(i, "format", "full")) // full|compact

	// Open metrics DB
	dbPath := util.GetMessageDBPath()
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return respondError(s, i, fmt.Sprintf("Failed to open metrics database: %v", err))
	}
	defer db.Close()

	ctxTimeout, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Weekly window (last 7 days, inclusive of today)
	weeklyCutoff := dayString(time.Now().UTC().AddDate(0, 0, -6))
	// Monthly window (last 30 days, inclusive)
	monthlyCutoff := dayString(time.Now().UTC().AddDate(0, 0, -29))

	weeklyEnters := querySum(ctxTimeout, db, "SELECT COALESCE(SUM(count),0) FROM daily_member_joins WHERE guild_id=? AND day>=?", ctx.GuildID, weeklyCutoff)
	monthlyEnters := querySum(ctxTimeout, db, "SELECT COALESCE(SUM(count),0) FROM daily_member_joins WHERE guild_id=? AND day>=?", ctx.GuildID, monthlyCutoff)

	weeklyLeaves := querySum(ctxTimeout, db, "SELECT COALESCE(SUM(count),0) FROM daily_member_leaves WHERE guild_id=? AND day>=?", ctx.GuildID, weeklyCutoff)
	monthlyLeaves := querySum(ctxTimeout, db, "SELECT COALESCE(SUM(count),0) FROM daily_member_leaves WHERE guild_id=? AND day>=?", ctx.GuildID, monthlyCutoff)

	weeklyNet := weeklyEnters - weeklyLeaves
	monthlyNet := monthlyEnters - monthlyLeaves

	// Try to get current member count from state cache (best effort, no REST)
	currentMembers := ""
	if s != nil && s.State != nil {
		if g, _ := s.State.Guild(ctx.GuildID); g != nil {
			if g.MemberCount > 0 {
				currentMembers = fmt.Sprintf("%d", g.MemberCount)
			}
		}
	}

	desc := "Weekly and monthly member stats"
	fields := []*discordgo.MessageEmbedField{}

	if showWeekly {
		val := fmt.Sprintf(":inbox_tray: Entered: %d\n:outbox_tray: Left: %d\n:chart_with_upwards_trend: Net: %d", weeklyEnters, weeklyLeaves, weeklyNet)
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "Weekly",
			Value:  val,
			Inline: true,
		})
	}
	if showMonthly {
		val := fmt.Sprintf(":inbox_tray: Entered: %d\n:outbox_tray: Left: %d\n:chart_with_upwards_trend: Net: %d", monthlyEnters, monthlyLeaves, monthlyNet)
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "Monthly",
			Value:  val,
			Inline: true,
		})
	}

	if includeCurrent && currentMembers != "" {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "Current Members (approx.)",
			Value:  currentMembers,
			Inline: format == "compact",
		})
	}

	embed := &discordgo.MessageEmbed{
		Title:       "Members: Weekly and Monthly",
		Color:       0x2ECC71, // green
		Description: desc,
		Timestamp:   time.Now().Format(time.RFC3339),
		Fields:      fields,
	}

	return respondEmbed(s, i, embed)
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
