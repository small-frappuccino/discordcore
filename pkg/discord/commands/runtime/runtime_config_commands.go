package runtime

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	"github.com/small-frappuccino/discordcore/pkg/discord/perf"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/runtimeapply"
	"github.com/small-frappuccino/discordcore/pkg/theme"
)

func ptrInt(v int) *int { return &v }

// Refactor goals:
// - One source of truth for runtime keys (spec registry)
// - Clean, explicit state handling (no implicit "packed values" hacks)
// - Smaller, testable helpers (parsing/formatting/set/reset/render)
// - UX/QoL: grouping, search/filter, safe defaults, clear restart hints
//
// Notes / constraints:
// - Runtime config is bot-global.

const (
	groupName   = "config"
	commandName = "runtime"

	customIDPrefix = "runtimecfg:"

	// Component IDs
	cidSelectKey    = customIDPrefix + "select:key"
	cidSelectGroup  = customIDPrefix + "select:group"
	cidButtonMain   = customIDPrefix + "nav:main"
	cidButtonHelp   = customIDPrefix + "nav:help"
	cidButtonBack   = customIDPrefix + "nav:back"
	cidButtonDetail = customIDPrefix + "action:details"
	cidButtonToggle = customIDPrefix + "action:toggle"
	cidButtonEdit   = customIDPrefix + "action:edit"
	cidButtonReset  = customIDPrefix + "action:reset"
	cidButtonReload = customIDPrefix + "action:reload"

	// Modal IDs / fields
	modalEditValueID = customIDPrefix + "modal:edit"
	modalFieldValue  = "value"

	// State encoding
	stateSep = "|"
)

type pageMode string

const (
	pageMain   pageMode = "main"
	pageHelp   pageMode = "help"
	pageDetail pageMode = "detail"
)

type runtimeKey string

type valueType string

const (
	vtBool   valueType = "bool"
	vtString valueType = "string"
	vtDate   valueType = "date"
	vtInt    valueType = "int"
)

type restartHint string

const (
	restartRequired    restartHint = "restart required"
	restartRecommended restartHint = "restart recommended"
)

type spec struct {
	Key          runtimeKey
	Group        string
	Type         valueType
	DefaultHint  string
	ShortHelp    string
	RestartHint  restartHint
	MaxInputLen  int // for modal input
	RedactInMain bool
	GuildOnly    bool
}

type panelState struct {
	Mode   pageMode
	Group  string
	Key    runtimeKey
	Filter string // reserved for future search; not wired yet
	Scope  string // "global" or guildID
}

func (s panelState) encode() string {
	// mode|group|key|scope
	return string(s.Mode) + stateSep + s.Group + stateSep + string(s.Key) + stateSep + s.Scope
}

func decodeState(raw string) panelState {
	// Expected: mode|group|key|scope
	// Use SplitN to avoid accepting extra separators as additional state fields.
	parts := strings.SplitN(raw, stateSep, 4)
	st := panelState{Mode: pageMain, Group: "ALL", Key: runtimeKeyBotTheme, Scope: "global"}
	if len(parts) >= 1 {
		if v := strings.TrimSpace(parts[0]); v != "" {
			st.Mode = pageMode(v)
		}
	}
	if len(parts) >= 2 {
		if v := strings.TrimSpace(parts[1]); v != "" {
			st.Group = v
		}
	}
	if len(parts) >= 3 {
		if v := strings.TrimSpace(parts[2]); v != "" {
			st.Key = runtimeKey(v)
		}
	}
	if len(parts) >= 4 {
		if v := strings.TrimSpace(parts[3]); v != "" {
			st.Scope = v
		}
	}
	return sanitizeState(st)
}

func sanitizeState(st panelState) panelState {
	switch st.Mode {
	case pageMain, pageHelp, pageDetail:
		// ok
	default:
		st.Mode = pageMain
	}

	if st.Group == "" {
		st.Group = "ALL"
	}
	if st.Group != "ALL" {
		valid := false
		for _, g := range allGroups() {
			if g == st.Group {
				valid = true
				break
			}
		}
		if !valid {
			st.Group = "ALL"
		}
	}

	return ensureKeyInGroup(st)
}

func (s panelState) withMode(m pageMode) panelState  { s.Mode = m; return s }
func (s panelState) withGroup(g string) panelState   { s.Group = g; return s }
func (s panelState) withKey(k runtimeKey) panelState { s.Key = k; return s }
func (s panelState) withScope(sc string) panelState  { s.Scope = sc; return s }

// --- Specs registry (single source of truth) ---

const (
	// THEME
	runtimeKeyBotTheme runtimeKey = "bot_theme"

	// SERVICES (LOGGING)
	runtimeKeyDisableDBCleanup     runtimeKey = "disable_db_cleanup"
	runtimeKeyDisableAutomodLogs   runtimeKey = "disable_automod_logs"
	runtimeKeyDisableMessageLogs   runtimeKey = "disable_message_logs"
	runtimeKeyDisableEntryExitLogs runtimeKey = "disable_entry_exit_logs"
	runtimeKeyDisableReactionLogs  runtimeKey = "disable_reaction_logs"
	runtimeKeyDisableUserLogs      runtimeKey = "disable_user_logs"
	runtimeKeyModerationLogging    runtimeKey = "moderation_logging"

	// PRESENCE WATCH
	runtimeKeyPresenceWatchUserID runtimeKey = "presence_watch_user_id"
	runtimeKeyPresenceWatchBot    runtimeKey = "presence_watch_bot"

	// MESSAGE CACHE
	runtimeKeyMessageCacheTTLHours runtimeKey = "message_cache_ttl_hours"
	runtimeKeyMessageDeleteOnLog   runtimeKey = "message_delete_on_log"
	runtimeKeyMessageCacheCleanup  runtimeKey = "message_cache_cleanup"

	// BACKFILL (ENTRY/EXIT)
	runtimeKeyBackfillChannelID   runtimeKey = "backfill_channel_id"
	runtimeKeyBackfillStartDay    runtimeKey = "backfill_start_day"
	runtimeKeyBackfillInitialDate runtimeKey = "backfill_initial_date"

	// BOT ROLE PERMISSION MIRRORING (SAFETY)
	runtimeKeyDisableBotRolePermMirror     runtimeKey = "disable_bot_role_perm_mirror"
	runtimeKeyBotRolePermMirrorActorRoleID runtimeKey = "bot_role_perm_mirror_actor_role_id"
)

func allSpecs() []spec {
	// Keep groups stable and short (helps readability in embed fields)
	return []spec{
		{
			Key:         runtimeKeyBotTheme,
			Group:       "THEME",
			Type:        vtString,
			DefaultHint: "(default)",
			ShortHelp:   "Theme name (empty = default)",
			RestartHint: restartRecommended,
			MaxInputLen: 60,
		},
		{
			Key:         runtimeKeyDisableDBCleanup,
			Group:       "SERVICES (LOGGING)",
			Type:        vtBool,
			DefaultHint: "false",
			ShortHelp:   "Disable periodic DB cleanup",
			RestartHint: restartRequired, // still a goroutine in runner; hot-apply intentionally not handled
		},
		{
			Key:         runtimeKeyDisableAutomodLogs,
			Group:       "SERVICES (LOGGING)",
			Type:        vtBool,
			DefaultHint: "false",
			ShortHelp:   "Disable automod logging service startup",
			RestartHint: restartRecommended,
		},
		{
			Key:         runtimeKeyDisableMessageLogs,
			Group:       "SERVICES (LOGGING)",
			Type:        vtBool,
			DefaultHint: "false",
			ShortHelp:   "Disable message logging service startup",
			RestartHint: restartRecommended,
		},
		{
			Key:         runtimeKeyDisableEntryExitLogs,
			Group:       "SERVICES (LOGGING)",
			Type:        vtBool,
			DefaultHint: "false",
			ShortHelp:   "Disable entry/exit logging service startup",
			RestartHint: restartRecommended,
		},
		{
			Key:         runtimeKeyDisableReactionLogs,
			Group:       "SERVICES (LOGGING)",
			Type:        vtBool,
			DefaultHint: "false",
			ShortHelp:   "Disable reaction logging service startup",
			RestartHint: restartRecommended,
		},
		{
			Key:         runtimeKeyDisableUserLogs,
			Group:       "SERVICES (LOGGING)",
			Type:        vtBool,
			DefaultHint: "false",
			ShortHelp:   "Disable user log handlers (avatars/roles)",
			RestartHint: restartRecommended,
		},
		{
			Key:         runtimeKeyModerationLogging,
			Group:       "MODERATION",
			Type:        vtBool,
			DefaultHint: "true",
			ShortHelp:   "Enable/disable moderation case embeds",
			RestartHint: restartRecommended,
		},
		{
			Key:         runtimeKeyPresenceWatchUserID,
			Group:       "PRESENCE WATCH",
			Type:        vtString,
			DefaultHint: "(empty)",
			ShortHelp:   "Log presence updates for a specific user ID",
			RestartHint: restartRecommended,
			MaxInputLen: 32,
		},
		{
			Key:         runtimeKeyPresenceWatchBot,
			Group:       "PRESENCE WATCH",
			Type:        vtBool,
			DefaultHint: "false",
			ShortHelp:   "Log presence updates for the bot user",
			RestartHint: restartRecommended,
		},
		{
			Key:         runtimeKeyMessageCacheTTLHours,
			Group:       "MESSAGE CACHE",
			Type:        vtInt,
			DefaultHint: "72",
			ShortHelp:   "Cache TTL in hours for message edit/delete logging (0 = default)",
			RestartHint: restartRequired,
			MaxInputLen: 8,
		},
		{
			Key:         runtimeKeyMessageDeleteOnLog,
			Group:       "MESSAGE CACHE",
			Type:        vtBool,
			DefaultHint: "false",
			ShortHelp:   "Delete cached message record after it is logged",
			RestartHint: restartRecommended,
		},
		{
			Key:         runtimeKeyMessageCacheCleanup,
			Group:       "MESSAGE CACHE",
			Type:        vtBool,
			DefaultHint: "false",
			ShortHelp:   "Cleanup expired cached messages on startup",
			RestartHint: restartRecommended,
		},
		{
			Key:         runtimeKeyBackfillChannelID,
			Group:       "BACKFILL",
			Type:        vtString,
			DefaultHint: "(empty)",
			ShortHelp:   "Channel ID to backfill from (required to run)",
			RestartHint: restartRequired,
			MaxInputLen: 32,
		},
		{
			Key:         runtimeKeyBackfillStartDay,
			Group:       "BACKFILL",
			Type:        vtDate,
			DefaultHint: "today (UTC)",
			ShortHelp:   "Start day (YYYY-MM-DD) for backfill",
			RestartHint: restartRequired,
			MaxInputLen: 16,
		},
		{
			Key:         runtimeKeyBackfillInitialDate,
			Group:       "BACKFILL",
			Type:        vtDate,
			DefaultHint: "(empty)",
			ShortHelp:   "Initial scan start date (fixed) when never processed",
			RestartHint: restartRequired,
			MaxInputLen: 16,
			GuildOnly:   true,
		},
		{
			Key:         runtimeKeyDisableBotRolePermMirror,
			Group:       "SAFETY",
			Type:        vtBool,
			DefaultHint: "false",
			ShortHelp:   "Disable bot role permission mirroring safety feature",
			RestartHint: restartRecommended, // effective at event time; no restart needed for behavior
		},
		{
			Key:         runtimeKeyBotRolePermMirrorActorRoleID,
			Group:       "SAFETY",
			Type:        vtString,
			DefaultHint: "(default)",
			ShortHelp:   "Role ID used as the actor when mirroring permissions",
			RestartHint: restartRecommended,
			MaxInputLen: 32,
		},
	}
}

func specByKey(k runtimeKey) (spec, bool) {
	for _, sp := range allSpecs() {
		if sp.Key == k {
			return sp, true
		}
	}
	return spec{}, false
}

func allGroups() []string {
	set := map[string]struct{}{"ALL": {}}
	for _, sp := range allSpecs() {
		set[sp.Group] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for g := range set {
		out = append(out, g)
	}
	sort.Strings(out)
	// Keep ALL first if present
	if len(out) > 0 && out[0] != "ALL" {
		for i := range out {
			if out[i] == "ALL" {
				out[0], out[i] = out[i], out[0]
				break
			}
		}
	}
	return out
}

func specsForGroup(group string) []spec {
	if strings.TrimSpace(group) == "" || group == "ALL" {
		// return deterministic order by group then key
		sps := append([]spec(nil), allSpecs()...)
		sort.Slice(sps, func(i, j int) bool {
			if sps[i].Group == sps[j].Group {
				return string(sps[i].Key) < string(sps[j].Key)
			}
			return sps[i].Group < sps[j].Group
		})
		return sps
	}

	var out []spec
	for _, sp := range allSpecs() {
		if sp.Group == group {
			out = append(out, sp)
		}
	}
	sort.Slice(out, func(i, j int) bool { return string(out[i].Key) < string(out[j].Key) })
	return out
}

// --- Command wiring ---

type ConfigCommands struct {
	configManager *files.ConfigManager
}

func NewRuntimeConfigCommands(configManager *files.ConfigManager) *ConfigCommands {
	return &ConfigCommands{configManager: configManager}
}

// RegisterCommands registers `/config runtime`.
func (cc *ConfigCommands) RegisterCommands(router *core.CommandRouter) {
	newRuntimeInteractionCatalog(cc.configManager).register(router)
}

type runtimeSubCommand struct {
	configManager *files.ConfigManager
}

func newRuntimeSubCommand(configManager *files.ConfigManager) *runtimeSubCommand {
	return &runtimeSubCommand{configManager: configManager}
}

func (c *runtimeSubCommand) Name() string { return commandName }
func (c *runtimeSubCommand) Description() string {
	return "View and edit bot runtime configuration (replaces env vars)"
}
func (c *runtimeSubCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionBoolean,
			Name:        "ephemeral",
			Description: "Show panel as ephemeral (recommended)",
			Required:    false,
		},
	}
}
func (c *runtimeSubCommand) RequiresGuild() bool       { return false }
func (c *runtimeSubCommand) RequiresPermissions() bool { return true }

func (c *runtimeSubCommand) Handle(ctx *core.Context) error {
	extractor := core.NewOptionExtractor(core.GetSubCommandOptions(ctx.Interaction))
	ephemeral := extractor.Bool("ephemeral")
	if !optionWasProvided(ctx.Interaction, "ephemeral") {
		ephemeral = true
	}

	rc, err := loadRuntimeConfig(ctx.Config, "global")
	if err != nil {
		return core.NewResponseBuilder(ctx.Session).Ephemeral().Error(ctx.Interaction, fmt.Sprintf("Failed to load runtime config: %v", err))
	}

	st := panelState{
		Mode:  pageMain,
		Group: "ALL",
		Scope: "global",
	}

	if ctx.Interaction.GuildID != "" {
		st.Scope = ctx.Interaction.GuildID
		// Try to load guild config, if fails or empty, we still have the global one as base
		if grc, err := loadRuntimeConfig(ctx.Config, st.Scope); err == nil {
			rc = grc
		}
	}

	embed := renderMainEmbed(rc, st)
	components := renderMainComponents(rc, st)

	rm := core.NewResponseBuilder(ctx.Session).Build()
	cfg := core.ResponseConfig{
		Ephemeral:  ephemeral,
		WithEmbed:  true,
		Title:      embed.Title,
		Color:      embed.Color,
		Timestamp:  true,
		Components: components,
		Footer:     embed.Footer.Text,
	}
	return rm.WithConfig(cfg).Custom(ctx.Interaction, "", []*discordgo.MessageEmbed{embed})
}

// --- Persistence ---

func loadRuntimeConfig(cm *files.ConfigManager, scope string) (files.RuntimeConfig, error) {
	if cm == nil {
		return files.RuntimeConfig{}, fmt.Errorf("config manager is nil")
	}
	_ = cm.LoadConfig() // best effort
	cfg := cm.Config()
	if cfg == nil {
		return files.RuntimeConfig{}, nil
	}
	if scope == "" || scope == "global" {
		return cfg.RuntimeConfig, nil
	}
	// Per-guild
	gcfg := cm.GuildConfig(scope)
	if gcfg == nil {
		return files.RuntimeConfig{}, fmt.Errorf("guild not found")
	}
	return gcfg.RuntimeConfig, nil
}

func saveRuntimeConfig(cm *files.ConfigManager, rc files.RuntimeConfig, scope string) error {
	if cm == nil {
		return fmt.Errorf("config manager is nil")
	}
	_ = cm.LoadConfig() // best effort
	if scope == "" || scope == "global" {
		_, err := cm.UpdateRuntimeConfig(func(current *files.RuntimeConfig) error {
			*current = rc
			return nil
		})
		return err
	}

	_, err := cm.UpdateConfig(func(cfg *files.BotConfig) error {
		for i := range cfg.Guilds {
			if cfg.Guilds[i].GuildID == scope {
				cfg.Guilds[i].RuntimeConfig = rc
				return nil
			}
		}
		return fmt.Errorf("guild config for %s not found in memory during save", scope)
	})
	return err
}

// --- RuntimeConfig mapping (get/set/reset) ---

func getValue(rc files.RuntimeConfig, k runtimeKey) (string, bool) {
	switch k {
	case runtimeKeyBotTheme:
		return rc.BotTheme, true

	case runtimeKeyDisableDBCleanup:
		return fmtBool(rc.DisableDBCleanup), true
	case runtimeKeyDisableAutomodLogs:
		return fmtBool(rc.DisableAutomodLogs), true
	case runtimeKeyDisableMessageLogs:
		return fmtBool(rc.DisableMessageLogs), true
	case runtimeKeyDisableEntryExitLogs:
		return fmtBool(rc.DisableEntryExitLogs), true
	case runtimeKeyDisableReactionLogs:
		return fmtBool(rc.DisableReactionLogs), true
	case runtimeKeyDisableUserLogs:
		return fmtBool(rc.DisableUserLogs), true
	case runtimeKeyModerationLogging:
		return fmtBool(rc.ModerationLoggingEnabled()), true

	case runtimeKeyPresenceWatchUserID:
		return rc.PresenceWatchUserID, true
	case runtimeKeyPresenceWatchBot:
		return fmtBool(rc.PresenceWatchBot), true

	case runtimeKeyMessageCacheTTLHours:
		return strconv.Itoa(rc.MessageCacheTTLHours), true
	case runtimeKeyMessageDeleteOnLog:
		return fmtBool(rc.MessageDeleteOnLog), true
	case runtimeKeyMessageCacheCleanup:
		return fmtBool(rc.MessageCacheCleanup), true

	case runtimeKeyBackfillChannelID:
		return rc.BackfillChannelID, true
	case runtimeKeyBackfillStartDay:
		return rc.BackfillStartDay, true
	case runtimeKeyBackfillInitialDate:
		return rc.BackfillInitialDate, true

	case runtimeKeyDisableBotRolePermMirror:
		return fmtBool(rc.DisableBotRolePermMirror), true
	case runtimeKeyBotRolePermMirrorActorRoleID:
		return rc.BotRolePermMirrorActorRoleID, true

	default:
		return "", false
	}
}

func resetValue(rc files.RuntimeConfig, k runtimeKey) (files.RuntimeConfig, bool) {
	switch k {
	case runtimeKeyBotTheme:
		rc.BotTheme = ""
		return rc, true

	case runtimeKeyDisableDBCleanup:
		rc.DisableDBCleanup = false
		return rc, true
	case runtimeKeyDisableAutomodLogs:
		rc.DisableAutomodLogs = false
		return rc, true
	case runtimeKeyDisableMessageLogs:
		rc.DisableMessageLogs = false
		return rc, true
	case runtimeKeyDisableEntryExitLogs:
		rc.DisableEntryExitLogs = false
		return rc, true
	case runtimeKeyDisableReactionLogs:
		rc.DisableReactionLogs = false
		return rc, true
	case runtimeKeyDisableUserLogs:
		rc.DisableUserLogs = false
		return rc, true
	case runtimeKeyModerationLogging:
		rc.ModerationLogging = nil
		return rc, true

	case runtimeKeyPresenceWatchUserID:
		rc.PresenceWatchUserID = ""
		return rc, true
	case runtimeKeyPresenceWatchBot:
		rc.PresenceWatchBot = false
		return rc, true

	case runtimeKeyMessageCacheTTLHours:
		rc.MessageCacheTTLHours = 0
		return rc, true
	case runtimeKeyMessageDeleteOnLog:
		rc.MessageDeleteOnLog = false
		return rc, true
	case runtimeKeyMessageCacheCleanup:
		rc.MessageCacheCleanup = false
		return rc, true

	case runtimeKeyBackfillChannelID:
		rc.BackfillChannelID = ""
		return rc, true
	case runtimeKeyBackfillStartDay:
		rc.BackfillStartDay = ""
		return rc, true
	case runtimeKeyBackfillInitialDate:
		rc.BackfillInitialDate = ""
		return rc, true

	case runtimeKeyDisableBotRolePermMirror:
		rc.DisableBotRolePermMirror = false
		return rc, true
	case runtimeKeyBotRolePermMirrorActorRoleID:
		rc.BotRolePermMirrorActorRoleID = ""
		return rc, true

	default:
		return rc, false
	}
}

func setValue(rc files.RuntimeConfig, sp spec, raw string) (files.RuntimeConfig, error) {
	raw = strings.TrimSpace(raw)

	switch sp.Type {
	case vtBool:
		b, err := parseBool(raw)
		if err != nil {
			return rc, err
		}
		return setBool(rc, sp.Key, b)
	case vtInt:
		// Empty resets to default behavior (keep omitempty output as zero-value).
		if raw == "" {
			return resetValueOrErr(rc, sp.Key)
		}
		v, err := parseNonNegativeInt(raw)
		if err != nil {
			return rc, err
		}
		return setInt(rc, sp.Key, v)
	case vtDate:
		if raw == "" {
			if sp.Key == runtimeKeyBackfillStartDay {
				rc.BackfillStartDay = ""
				return rc, nil
			}
			if sp.Key == runtimeKeyBackfillInitialDate {
				rc.BackfillInitialDate = ""
				return rc, nil
			}
			return rc, nil
		}
		if _, err := time.Parse("2006-01-02", raw); err != nil {
			return rc, fmt.Errorf("invalid date (expected YYYY-MM-DD)")
		}
		if sp.Key == runtimeKeyBackfillStartDay {
			rc.BackfillStartDay = raw
			return rc, nil
		}
		if sp.Key == runtimeKeyBackfillInitialDate {
			rc.BackfillInitialDate = raw
			return rc, nil
		}
		return rc, fmt.Errorf("unsupported date key")
	case vtString:
		// Empty string is allowed to reset to default behavior
		switch sp.Key {
		case runtimeKeyBotTheme:
			rc.BotTheme = raw
			return rc, nil
		case runtimeKeyPresenceWatchUserID:
			rc.PresenceWatchUserID = raw
			return rc, nil
		case runtimeKeyBackfillChannelID:
			rc.BackfillChannelID = raw
			return rc, nil
		case runtimeKeyBotRolePermMirrorActorRoleID:
			rc.BotRolePermMirrorActorRoleID = raw
			return rc, nil
		default:
			return rc, fmt.Errorf("unsupported string key")
		}
	default:
		return rc, fmt.Errorf("unknown type")
	}
}

func resetValueOrErr(rc files.RuntimeConfig, k runtimeKey) (files.RuntimeConfig, error) {
	next, ok := resetValue(rc, k)
	if !ok {
		return rc, fmt.Errorf("unknown key")
	}
	return next, nil
}

func setInt(rc files.RuntimeConfig, k runtimeKey, v int) (files.RuntimeConfig, error) {
	switch k {
	case runtimeKeyMessageCacheTTLHours:
		// Accept 0 to mean "use default" (service will fall back).
		if v < 0 {
			return rc, fmt.Errorf("must be >= 0")
		}
		// Guardrail against absurd values.
		if v > 24*365 {
			return rc, fmt.Errorf("too large (max %d)", 24*365)
		}
		rc.MessageCacheTTLHours = v
		return rc, nil
	case runtimeKeyBackfillInitialDate:
		// String key (vtDate) handled in setValue switch
		return rc, fmt.Errorf("use setValue for string/date keys")
	default:
		return rc, fmt.Errorf("not an int key")
	}
}

func setBool(rc files.RuntimeConfig, k runtimeKey, v bool) (files.RuntimeConfig, error) {
	switch k {
	case runtimeKeyDisableDBCleanup:
		rc.DisableDBCleanup = v
	case runtimeKeyDisableAutomodLogs:
		rc.DisableAutomodLogs = v
	case runtimeKeyDisableMessageLogs:
		rc.DisableMessageLogs = v
	case runtimeKeyDisableEntryExitLogs:
		rc.DisableEntryExitLogs = v
	case runtimeKeyDisableReactionLogs:
		rc.DisableReactionLogs = v
	case runtimeKeyDisableUserLogs:
		rc.DisableUserLogs = v
	case runtimeKeyModerationLogging:
		rc.ModerationLogging = boolPtr(v)
	case runtimeKeyPresenceWatchBot:
		rc.PresenceWatchBot = v
	case runtimeKeyMessageDeleteOnLog:
		rc.MessageDeleteOnLog = v
	case runtimeKeyMessageCacheCleanup:
		rc.MessageCacheCleanup = v
	case runtimeKeyDisableBotRolePermMirror:
		rc.DisableBotRolePermMirror = v
	default:
		return rc, fmt.Errorf("not a bool key")
	}
	return rc, nil
}

func toggleBool(rc files.RuntimeConfig, k runtimeKey) (files.RuntimeConfig, error) {
	val, ok := getValue(rc, k)
	if !ok {
		return rc, fmt.Errorf("unknown key")
	}
	b, err := parseBool(val)
	if err != nil {
		// if formatting ever changes, fall back to "false"
		b = false
	}
	return setBool(rc, k, !b)
}

// --- Rendering (embed + components) ---

func renderMainEmbed(rc files.RuntimeConfig, st panelState) *discordgo.MessageEmbed {
	sp, _ := specByKey(st.Key)

	scopeDesc := "Global"
	if st.Scope != "global" {
		scopeDesc = fmt.Sprintf("Guild (`%s`)", st.Scope)
	}

	desc := strings.Join([]string{
		"Painel para editar **runtime_config** (substitui as env vars operacionais).",
		"",
		fmt.Sprintf("Escopo: **%s**", scopeDesc),
		fmt.Sprintf("Selecionada: `%s` • Tipo: **%s** • Default: **%s** • %s", sp.Key, sp.Type, sp.DefaultHint, sp.RestartHint),
		"Use os menus para filtrar e navegar, e os botões para editar.",
	}, "\n")

	fields := []*discordgo.MessageEmbedField{}
	fields = append(fields, groupFieldsForMain(rc, st)...)

	return &discordgo.MessageEmbed{
		Title:       "CONFIG (RUNTIME)",
		Description: desc,
		Color:       theme.Info(),
		Fields:      fields,
		Footer: &discordgo.MessageEmbedFooter{
			Text: "Dica: alterações podem ser aplicadas em tempo real para THEME e alguns ALICE_DISABLE_*.",
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}
}

func groupFieldsForMain(rc files.RuntimeConfig, st panelState) []*discordgo.MessageEmbedField {
	specs := specsForGroup(st.Group)

	grouped := map[string][]string{}
	for _, sp := range specs {
		if sp.GuildOnly && st.Scope == "global" {
			continue
		}
		raw, _ := getValue(rc, sp.Key)
		display := formatForEmbed(raw, sp)
		line := fmt.Sprintf("`%s`: **%s**", sp.Key, display)
		grouped[sp.Group] = append(grouped[sp.Group], line)
	}

	groupOrder := []string{"THEME", "SERVICES (LOGGING)", "MODERATION", "MESSAGE CACHE", "BACKFILL", "SAFETY"}
	fields := []*discordgo.MessageEmbedField{}

	if st.Group != "" && st.Group != "ALL" {
		lines := grouped[st.Group]
		fields = append(fields, fieldsForLines(st.Group, lines)...)
		return fields
	}

	for _, g := range groupOrder {
		lines := grouped[g]
		if len(lines) == 0 {
			continue
		}
		fields = append(fields, fieldsForLines(g, lines)...)
		if len(fields) >= 25 {
			break
		}
	}

	return fields
}

func fieldsForLines(name string, lines []string) []*discordgo.MessageEmbedField {
	// Discord embed limits: max 25 fields; each Field.Value up to 1024 characters.
	// This helper chunks long lists to avoid edit failures when the panel grows.
	if len(lines) == 0 {
		lines = []string{"(no keys)"}
	}

	const maxValueLen = 1024
	out := make([]*discordgo.MessageEmbedField, 0, 1)
	curName := name
	curVal := ""

	flush := func() {
		if strings.TrimSpace(curVal) == "" {
			curVal = "(no keys)"
		}
		out = append(out, &discordgo.MessageEmbedField{
			Name:   curName,
			Value:  curVal,
			Inline: false,
		})
		curName = name + " (cont.)"
		curVal = ""
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		candidate := line
		if curVal != "" {
			candidate = curVal + "\n" + line
		}
		if len(candidate) > maxValueLen {
			// If a single line is too long, truncate it (shouldn't happen with current formatting,
			// but keep it safe for future keys/values).
			if curVal == "" {
				tr := line
				if len(tr) > maxValueLen {
					tr = tr[:maxValueLen]
				}
				curVal = tr
				flush()
				continue
			}
			flush()
			curVal = line
			continue
		}
		curVal = candidate
	}
	flush()
	return out
}

func renderDetailsEmbed(rc files.RuntimeConfig, st panelState) *discordgo.MessageEmbed {
	sp, ok := specByKey(st.Key)
	if !ok {
		return errorEmbed("Unknown key")
	}
	raw, _ := getValue(rc, sp.Key)
	cur := formatForDetails(raw, sp)

	scopeDesc := "Global"
	if st.Scope != "global" {
		scopeDesc = fmt.Sprintf("Guild (`%s`)", st.Scope)
	}

	lines := []string{
		fmt.Sprintf("`%s`", sp.Key),
		"",
		fmt.Sprintf("**Scope:** %s", scopeDesc),
		fmt.Sprintf("**Group:** %s", sp.Group),
		fmt.Sprintf("**Type:** %s", sp.Type),
		fmt.Sprintf("**Default:** %s", sp.DefaultHint),
		fmt.Sprintf("**Current:** %s", cur),
		"",
		fmt.Sprintf("**Description:** %s", sp.ShortHelp),
		fmt.Sprintf("**Effect:** %s", sp.RestartHint),
	}

	if sp.GuildOnly {
		lines = append(lines, "", "⚠️ **Note:** This setting can only be configured per-guild.")
	}

	return &discordgo.MessageEmbed{
		Title:       "CONFIG (RUNTIME) — DETAILS",
		Description: strings.Join(lines, "\n"),
		Color:       theme.Muted(),
		Footer: &discordgo.MessageEmbedFooter{
			Text: "Use BACK to return to the panel.",
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}
}

func renderHelpEmbed() *discordgo.MessageEmbed {
	desc := strings.Join([]string{
		"This panel edits the persisted `runtime_config`.",
		"",
		"**Notes:**",
		"• Names stay in ALL CAPS to preserve mental compatibility with env vars.",
		"• The bot no longer reads these options from the environment (the token is still env).",
		"• Some changes can be hot-applied (THEME and some ALICE_DISABLE_*).",
		"",
		"**How to edit:**",
		"1) Filter by group (optional) and select a key.",
		"2) Boolean: use TOGGLE.",
		"3) Other types: use EDIT and fill the modal.",
		"4) RESET clears the value and restores the code default.",
	}, "\n")

	return &discordgo.MessageEmbed{
		Title:       "CONFIG (RUNTIME) — HELP",
		Description: desc,
		Color:       theme.Info(),
		Timestamp:   time.Now().Format(time.RFC3339),
	}
}

func renderMainComponents(rc files.RuntimeConfig, st panelState) []discordgo.MessageComponent {
	return []discordgo.MessageComponent{
		renderGroupSelectRow(st),
		renderKeySelectRow(st),
		renderActionRow(st),
		renderNavRow(st),
	}
}

func renderDetailComponents(st panelState) []discordgo.MessageComponent {
	return []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.Button{CustomID: cidButtonBack + stateSep + st.withMode(pageMain).encode(), Label: "BACK", Style: discordgo.SecondaryButton},
				discordgo.Button{CustomID: cidButtonReload + stateSep + st.withMode(pageDetail).encode(), Label: "RELOAD", Style: discordgo.SecondaryButton},
			},
		},
	}
}

func renderHelpComponents(st panelState) []discordgo.MessageComponent {
	return []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.Button{CustomID: cidButtonBack + stateSep + st.withMode(pageMain).encode(), Label: "BACK", Style: discordgo.SecondaryButton},
			},
		},
	}
}

func renderGroupSelectRow(st panelState) discordgo.ActionsRow {
	groups := allGroups()
	opts := make([]discordgo.SelectMenuOption, 0, len(groups))
	for _, g := range groups {
		opts = append(opts, discordgo.SelectMenuOption{
			Label:       g,
			Value:       st.withGroup(g).withMode(pageMain).encode(),
			Description: "Filter keys by group",
			Default:     g == st.Group,
		})
	}

	return discordgo.ActionsRow{
		Components: []discordgo.MessageComponent{
			discordgo.SelectMenu{
				CustomID:    cidSelectGroup,
				Placeholder: "Filter by group…",
				Options:     opts,
				MinValues:   ptrInt(1),
				MaxValues:   1,
			},
		},
	}
}

func renderKeySelectRow(st panelState) discordgo.ActionsRow {
	specs := specsForGroup(st.Group)

	// Filter out GuildOnly specs when in global scope
	filtered := make([]spec, 0, len(specs))
	for _, sp := range specs {
		if sp.GuildOnly && st.Scope == "global" {
			continue
		}
		filtered = append(filtered, sp)
	}
	specs = filtered

	tooMany := false
	if len(specs) > 25 {
		tooMany = true
		specs = specs[:25]
	}
	opts := make([]discordgo.SelectMenuOption, 0, len(specs))
	for _, sp := range specs {
		desc := sp.ShortHelp
		if len(desc) > 90 {
			desc = desc[:90]
		}
		opts = append(opts, discordgo.SelectMenuOption{
			Label:       string(sp.Key),
			Value:       st.withKey(sp.Key).withMode(pageMain).encode(),
			Description: desc,
			Default:     sp.Key == st.Key,
		})
	}

	placeholder := "Select key…"
	if st.Group != "" && st.Group != "ALL" {
		placeholder = "Select key in " + st.Group + "…"
	}
	if tooMany {
		if st.Group == "ALL" {
			placeholder = "Too many keys — select a group first…"
		} else {
			placeholder = "Showing first 25 keys in " + st.Group + "…"
		}
	}

	return discordgo.ActionsRow{
		Components: []discordgo.MessageComponent{
			discordgo.SelectMenu{
				CustomID:    cidSelectKey,
				Placeholder: placeholder,
				Options:     opts,
				MinValues:   ptrInt(1),
				MaxValues:   1,
			},
		},
	}
}

func renderActionRow(st panelState) discordgo.ActionsRow {
	sp, _ := specByKey(st.Key)
	isBool := sp.Type == vtBool

	return discordgo.ActionsRow{
		Components: []discordgo.MessageComponent{
			discordgo.Button{
				CustomID: cidButtonDetail + stateSep + st.withMode(pageDetail).encode(),
				Label:    "DETAILS",
				Style:    discordgo.PrimaryButton,
			},
			discordgo.Button{
				CustomID: cidButtonToggle + stateSep + st.withMode(pageMain).encode(),
				Label:    "TOGGLE",
				Style:    discordgo.SecondaryButton,
				Disabled: !isBool,
			},
			discordgo.Button{
				CustomID: cidButtonEdit + stateSep + st.withMode(pageMain).encode(),
				Label:    "EDIT",
				Style:    discordgo.SuccessButton,
				Disabled: isBool,
			},
			discordgo.Button{
				CustomID: cidButtonReset + stateSep + st.withMode(pageMain).encode(),
				Label:    "RESET",
				Style:    discordgo.DangerButton,
			},
			discordgo.Button{
				CustomID: cidButtonReload + stateSep + st.withMode(pageMain).encode(),
				Label:    "RELOAD",
				Style:    discordgo.SecondaryButton,
			},
		},
	}
}

func renderNavRow(st panelState) discordgo.ActionsRow {
	components := []discordgo.MessageComponent{
		discordgo.Button{CustomID: cidButtonHelp + stateSep + st.withMode(pageHelp).encode(), Label: "HELP", Style: discordgo.SecondaryButton},
		discordgo.Button{CustomID: cidButtonMain + stateSep + st.withMode(pageMain).encode(), Label: "MAIN", Style: discordgo.SecondaryButton},
	}

	if st.Scope != "" && st.Scope != "global" {
		components = append(components, discordgo.Button{
			CustomID: cidButtonReload + stateSep + st.withScope("global").encode(),
			Label:    "SWITCH TO GLOBAL",
			Style:    discordgo.SecondaryButton,
		})
	}

	return discordgo.ActionsRow{
		Components: components,
	}
}

func formatForEmbed(raw string, sp spec) string {
	v := strings.TrimSpace(raw)
	if v == "" {
		return "—"
	}
	if sp.Type == vtInt && v == "0" {
		return "—"
	}
	if sp.RedactInMain {
		return "(configured)"
	}
	return v
}

func formatForDetails(raw string, _ spec) string {
	v := strings.TrimSpace(raw)
	if v == "" {
		return "—"
	}
	if v == "0" {
		return "—"
	}
	return v
}

// --- Interactions ---

type runtimeConfigApplier interface {
	Apply(ctx context.Context, next files.RuntimeConfig) error
}

func asRuntimeConfigApplier(applier *runtimeapply.Manager) runtimeConfigApplier {
	if applier == nil {
		return nil
	}
	return applier
}

func runtimeInteractionApplier(ctx *core.Context) runtimeConfigApplier {
	if ctx == nil || ctx.Router() == nil {
		return nil
	}
	return asRuntimeConfigApplier(ctx.Router().GetRuntimeApplier())
}

func startRuntimeConfigInteractionTrace(i *discordgo.InteractionCreate) func() {
	userID := interactionUserID(i)
	return perf.StartGatewayEvent(
		"interaction_create.runtime_config",
		slog.Int("interactionType", int(i.Type)),
		slog.String("guildID", i.GuildID),
		slog.String("userID", userID),
	)
}

func handleComponent(s *discordgo.Session, i *discordgo.InteractionCreate, configManager *files.ConfigManager, applier runtimeConfigApplier) {
	cc := i.MessageComponentData()

	action, st := parseActionAndState(cc.CustomID)
	if action == "" {
		respondInteractionWithLog(s, i, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredMessageUpdate,
		}, "invalid_state.defer_update")
		editInteractionMessageWithLog(s, i, errorEmbed("Invalid interaction state"), nil, "invalid_state.render_error")
		return
	}

	respond := func(resp *discordgo.InteractionResponse, stage string) {
		respondInteractionWithLog(s, i, resp, action+"."+stage)
	}
	edit := func(embed *discordgo.MessageEmbed, components []discordgo.MessageComponent, stage string) {
		editInteractionMessageWithLog(s, i, embed, components, action+"."+stage)
	}

	// If this interaction is going to open a modal, we must NOT ack with a message update first.
	// Otherwise the modal response can fail because an interaction can only be responded to once.
	if action != cidButtonEdit {
		respond(&discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredMessageUpdate,
		}, "defer_update")
	}

	rc, err := loadRuntimeConfig(configManager, st.Scope)
	if err != nil {
		if action == cidButtonEdit {
			respond(&discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Flags: discordgo.MessageFlagsEphemeral,
					Embeds: []*discordgo.MessageEmbed{
						errorEmbed(fmt.Sprintf("Failed to load runtime config: %v", err)),
					},
				},
			}, "load_runtime_config_error")
			return
		}
		edit(errorEmbed(fmt.Sprintf("Failed to load runtime config: %v", err)), nil, "load_runtime_config_error")
		return
	}

	// Guard: enforce restrictions
	if sp, ok := specByKey(st.Key); ok {
		if sp.GuildOnly && st.Scope == "global" {
			// Skip editing if global
			if action == cidButtonEdit || action == cidButtonToggle || action == cidButtonReset {
				edit(errorEmbed("This setting can only be configured per-guild."), renderMainComponents(rc, st), "guild_only_restriction")
				return
			}
		}
	}

	switch action {
	case cidSelectGroup, cidSelectKey:
		if len(cc.Values) == 0 {
			embed := renderMainEmbed(rc, st.withMode(pageMain))
			edit(embed, renderMainComponents(rc, st.withMode(pageMain)), "select.empty_values")
			return
		}
		// The value in the select menu options is the full encoded state.
		st = decodeState(cc.Values[0])
		if refreshed, loadErr := loadRuntimeConfig(configManager, st.Scope); loadErr == nil {
			rc = refreshed
		} else {
			slog.Warn("Runtime config panel failed to refresh state after selection",
				"action", action,
				"scope", st.Scope,
				"key", string(st.Key),
				"err", loadErr,
			)
		}
		st = ensureKeyInGroup(st.withMode(pageMain))
		embed := renderMainEmbed(rc, st)
		edit(embed, renderMainComponents(rc, st), "select.apply_state")
		return

	case cidButtonMain, cidButtonBack:
		st = st.withMode(pageMain)
		st = ensureKeyInGroup(st)
		embed := renderMainEmbed(rc, st)
		edit(embed, renderMainComponents(rc, st), "nav.main")
		return

	case cidButtonHelp:
		st = st.withMode(pageHelp)
		embed := renderHelpEmbed()
		edit(embed, renderHelpComponents(st), "nav.help")
		return

	case cidButtonDetail:
		st = st.withMode(pageDetail)
		embed := renderDetailsEmbed(rc, st)
		edit(embed, renderDetailComponents(st), "nav.detail")
		return

	case cidButtonReload:
		if refreshed, loadErr := loadRuntimeConfig(configManager, st.Scope); loadErr == nil {
			rc = refreshed
		} else {
			slog.Warn("Runtime config panel failed to reload from storage",
				"action", action,
				"scope", st.Scope,
				"key", string(st.Key),
				"err", loadErr,
			)
		}
		st = ensureKeyInGroup(st)
		switch st.Mode {
		case pageHelp:
			embed := renderHelpEmbed()
			edit(embed, renderHelpComponents(st), "reload.help")
		case pageDetail:
			embed := renderDetailsEmbed(rc, st)
			edit(embed, renderDetailComponents(st), "reload.detail")
		default:
			embed := renderMainEmbed(rc, st.withMode(pageMain))
			edit(embed, renderMainComponents(rc, st.withMode(pageMain)), "reload.main")
		}
		return

	case cidButtonReset:
		st = st.withMode(pageMain)
		rc2, ok := resetValue(rc, st.Key)
		if !ok {
			edit(errorEmbed("Unknown key"), nil, "reset.unknown_key")
			return
		}
		if err := saveRuntimeConfig(configManager, rc2, st.Scope); err != nil {
			edit(errorEmbed(fmt.Sprintf("Failed to save: %v", err)), nil, "reset.save_error")
			return
		}
		applyErr := applyRuntimeConfigWithLog(applier, rc2, i, action+".reset.hot_apply", st)
		embed := renderMainEmbed(rc2, st)
		embed = withHotApplyWarning(embed, applyErr)
		edit(embed, renderMainComponents(rc2, st), "reset.render")
		return

	case cidButtonToggle:
		st = st.withMode(pageMain)
		sp, ok := specByKey(st.Key)
		if !ok {
			edit(errorEmbed("Unknown key"), nil, "toggle.unknown_key")
			return
		}
		if sp.Type != vtBool {
			edit(errorEmbed("TOGGLE is only valid for boolean keys"), renderMainComponents(rc, st), "toggle.invalid_type")
			return
		}
		rc2, err := toggleBool(rc, st.Key)
		if err != nil {
			edit(errorEmbed(fmt.Sprintf("Toggle failed: %v", err)), renderMainComponents(rc, st), "toggle.failed")
			return
		}
		if err := saveRuntimeConfig(configManager, rc2, st.Scope); err != nil {
			edit(errorEmbed(fmt.Sprintf("Failed to save: %v", err)), nil, "toggle.save_error")
			return
		}
		applyErr := applyRuntimeConfigWithLog(applier, rc2, i, action+".toggle.hot_apply", st)
		embed := renderMainEmbed(rc2, st)
		embed = withHotApplyWarning(embed, applyErr)
		edit(embed, renderMainComponents(rc2, st), "toggle.render")
		return

	case cidButtonEdit:
		sp, ok := specByKey(st.Key)
		if !ok {
			// This interaction path normally opens a modal, so we intentionally do NOT
			// ack with a message update earlier. If we hit an error, we must still
			// respond once to avoid an "interaction failed" on the client.
			respond(&discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Flags: discordgo.MessageFlagsEphemeral,
					Embeds: []*discordgo.MessageEmbed{
						errorEmbed("Unknown key"),
					},
				},
			}, "edit.unknown_key")
			return
		}
		if sp.Type == vtBool {
			respond(&discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Flags: discordgo.MessageFlagsEphemeral,
					Embeds: []*discordgo.MessageEmbed{
						errorEmbed("EDIT is not valid for boolean keys (use TOGGLE)"),
					},
				},
			}, "edit.invalid_type")
			return
		}

		cur, _ := getValue(rc, st.Key)
		if strings.TrimSpace(cur) == "" {
			cur = ""
		}
		if sp.Type == vtInt && strings.TrimSpace(cur) == "0" {
			cur = ""
		}

		maxLen := sp.MaxInputLen
		if maxLen <= 0 {
			maxLen = 200
		}
		label := fmt.Sprintf("%s (%s)", sp.Key, sp.Type)

		respond(&discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseModal,
			Data: &discordgo.InteractionResponseData{
				CustomID: modalEditValueID + stateSep + st.encode(),
				Title:    string(sp.Key),
				Components: []discordgo.MessageComponent{
					discordgo.ActionsRow{
						Components: []discordgo.MessageComponent{
							discordgo.TextInput{
								CustomID:    modalFieldValue,
								Label:       label,
								Style:       discordgo.TextInputShort,
								Placeholder: sp.DefaultHint,
								Value:       cur,
								Required:    false,
								MinLength:   0,
								MaxLength:   maxLen,
							},
						},
					},
				},
			},
		}, "edit.open_modal")
		return

	default:
		edit(errorEmbed("Unknown action"), nil, "unknown_action")
		return
	}
}

func handleModalSubmit(s *discordgo.Session, i *discordgo.InteractionCreate, configManager *files.ConfigManager, applier runtimeConfigApplier) {
	m := i.ModalSubmitData()
	rawState, ok := runtimeModalState(m.CustomID)
	if !ok {
		return
	}

	edit := func(embed *discordgo.MessageEmbed, components []discordgo.MessageComponent, stage string) {
		editInteractionMessageWithLog(s, i, embed, components, "modal_submit."+stage)
	}

	// For modal submits, keep the panel usable by updating the original panel message.
	respondInteractionWithLog(s, i, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
	}, "modal_submit.defer_update")

	st := decodeState(rawState)

	sp, ok := specByKey(st.Key)
	if !ok {
		embed := errorEmbed("Unknown key")
		edit(embed, renderMainComponents(files.RuntimeConfig{}, st.withMode(pageMain)), "unknown_key")
		return
	}
	if sp.Type == vtBool {
		embed := errorEmbed("Invalid modal for bool key")
		edit(embed, renderMainComponents(files.RuntimeConfig{}, st.withMode(pageMain)), "invalid_bool_key")
		return
	}

	val := extractModalValue(m, modalFieldValue)

	rc, err := loadRuntimeConfig(configManager, st.Scope)
	if err != nil {
		edit(errorEmbed(fmt.Sprintf("Failed to load runtime config: %v", err)), nil, "load_runtime_config_error")
		return
	}

	next, err := setValue(rc, sp, val)
	if err != nil {
		embed := errorEmbed(fmt.Sprintf("Invalid value: %v", err))
		st = ensureKeyInGroup(st.withMode(pageMain))
		edit(embed, renderMainComponents(rc, st), "invalid_value")
		return
	}
	if err := saveRuntimeConfig(configManager, next, st.Scope); err != nil {
		edit(errorEmbed(fmt.Sprintf("Failed to save: %v", err)), nil, "save_error")
		return
	}

	applyErr := applyRuntimeConfigWithLog(applier, next, i, "modal_submit.hot_apply", st)

	// After saving, return to MAIN with refreshed values so the user can keep navigating.
	st = ensureKeyInGroup(st.withMode(pageMain))
	embed := renderMainEmbed(next, st)
	embed = withHotApplyWarning(embed, applyErr)
	edit(embed, renderMainComponents(next, st), "render")
}

func interactionUserID(i *discordgo.InteractionCreate) string {
	if i == nil {
		return ""
	}
	if i.Member != nil && i.Member.User != nil {
		return i.Member.User.ID
	}
	if i.User != nil {
		return i.User.ID
	}
	return ""
}

func interactionLogFields(i *discordgo.InteractionCreate) []any {
	fields := []any{}
	if i == nil {
		return fields
	}
	fields = append(fields,
		"interactionType", int(i.Type),
		"interactionID", i.ID,
		"guildID", i.GuildID,
		"channelID", i.ChannelID,
	)
	if userID := interactionUserID(i); userID != "" {
		fields = append(fields, "userID", userID)
	}
	return fields
}

func respondInteractionWithLog(s *discordgo.Session, i *discordgo.InteractionCreate, resp *discordgo.InteractionResponse, reason string) {
	if s == nil || i == nil || i.Interaction == nil {
		slog.Error("Runtime config interaction respond skipped due to missing context", "reason", reason)
		return
	}
	if err := s.InteractionRespond(i.Interaction, resp); err != nil {
		fields := []any{"reason", reason, "err", err}
		fields = append(fields, interactionLogFields(i)...)
		slog.Error("Runtime config interaction respond failed", fields...)
	}
}

func editInteractionMessageWithLog(
	s *discordgo.Session,
	i *discordgo.InteractionCreate,
	embed *discordgo.MessageEmbed,
	components []discordgo.MessageComponent,
	reason string,
) {
	if s == nil || i == nil || i.Interaction == nil {
		slog.Error("Runtime config interaction edit skipped due to missing context", "reason", reason)
		return
	}
	if err := editInteractionMessage(s, i, embed, components); err != nil {
		fields := []any{"reason", reason, "err", err}
		fields = append(fields, interactionLogFields(i)...)
		slog.Error("Runtime config interaction edit failed", fields...)
	}
}

func applyRuntimeConfigWithLog(
	applier runtimeConfigApplier,
	next files.RuntimeConfig,
	i *discordgo.InteractionCreate,
	reason string,
	st panelState,
) error {
	if applier == nil {
		return nil
	}

	if err := applier.Apply(context.Background(), next); err != nil {
		fields := []any{
			"reason", reason,
			"scope", st.Scope,
			"key", string(st.Key),
			"err", err,
		}
		fields = append(fields, interactionLogFields(i)...)
		slog.Error("Runtime config hot-apply failed", fields...)
		return err
	}
	return nil
}

func withHotApplyWarning(embed *discordgo.MessageEmbed, applyErr error) *discordgo.MessageEmbed {
	if embed == nil || applyErr == nil {
		return embed
	}

	clone := *embed
	msg := fmt.Sprintf(
		"Saved runtime config, but failed to apply changes immediately. Restart may be required.\nError: %v",
		applyErr,
	)
	if strings.TrimSpace(clone.Description) == "" {
		clone.Description = msg
	} else {
		clone.Description = strings.TrimSpace(clone.Description) + "\n\n" + msg
	}
	return &clone
}

func extractModalValue(m discordgo.ModalSubmitInteractionData, fieldID string) string {
	for _, comp := range m.Components {
		row, ok := comp.(*discordgo.ActionsRow)
		if !ok || row == nil {
			continue
		}
		for _, c := range row.Components {
			ti, ok := c.(*discordgo.TextInput)
			if ok && ti.CustomID == fieldID {
				return ti.Value
			}
		}
	}
	return ""
}

func ensureKeyInGroup(st panelState) panelState {
	if st.Group == "" || st.Group == "ALL" {
		if _, ok := specByKey(st.Key); ok {
			return st
		}
		return st.withKey(runtimeKeyBotTheme)
	}

	for _, sp := range specsForGroup(st.Group) {
		if sp.Key == st.Key {
			return st
		}
	}
	sps := specsForGroup(st.Group)
	if len(sps) > 0 {
		return st.withKey(sps[0].Key)
	}
	return st.withKey(runtimeKeyBotTheme)
}

// parseActionAndState decodes "action|mode|group|key"
func parseActionAndState(customID string) (action string, st panelState) {
	routeID, rawState, hasState := strings.Cut(customID, stateSep)
	if !isKnownRuntimeComponentRoute(routeID) {
		return "", panelState{}
	}
	switch routeID {
	case cidSelectGroup, cidSelectKey:
		if hasState {
			return routeID, decodeState(rawState)
		}
		return routeID, panelState{Mode: pageMain, Group: "ALL", Key: runtimeKeyBotTheme}
	case cidButtonMain, cidButtonHelp, cidButtonBack,
		cidButtonDetail, cidButtonToggle, cidButtonEdit,
		cidButtonReset, cidButtonReload:
		if !hasState {
			return "", panelState{}
		}
		return routeID, decodeState(rawState)
	default:
		return "", panelState{}
	}
}

func runtimeModalState(customID string) (string, bool) {
	routeID, rawState, hasState := strings.Cut(customID, stateSep)
	if routeID != modalEditValueID || !hasState {
		return "", false
	}
	return rawState, true
}

func isKnownRuntimeComponentRoute(routeID string) bool {
	switch routeID {
	case cidSelectGroup, cidSelectKey,
		cidButtonMain, cidButtonHelp, cidButtonBack,
		cidButtonDetail, cidButtonToggle, cidButtonEdit,
		cidButtonReset, cidButtonReload:
		return true
	default:
		return false
	}
}

// --- Discord helpers ---

func editInteractionMessage(s *discordgo.Session, i *discordgo.InteractionCreate, embed *discordgo.MessageEmbed, components []discordgo.MessageComponent) error {
	embeds := []*discordgo.MessageEmbed{}
	if embed != nil {
		embeds = []*discordgo.MessageEmbed{embed}
	}
	_, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Embeds:     &embeds,
		Components: &components,
	})
	return err
}

func errorEmbed(msg string) *discordgo.MessageEmbed {
	return &discordgo.MessageEmbed{
		Title:       "CONFIG (RUNTIME) — ERROR",
		Description: msg,
		Color:       theme.Error(),
		Timestamp:   time.Now().Format(time.RFC3339),
	}
}

// --- Utilities (parsing, formatting, options presence) ---

func optionWasProvided(i *discordgo.InteractionCreate, name string) bool {
	if i == nil {
		return false
	}
	opts := core.GetSubCommandOptions(i)
	for _, o := range opts {
		if o != nil && o.Name == name {
			return true
		}
	}
	return false
}

func parseBool(s string) (bool, error) {
	v := strings.ToLower(strings.TrimSpace(s))
	switch v {
	case "1", "true", "yes", "y", "on", "enabled", "enable":
		return true, nil
	case "0", "false", "no", "n", "off", "disabled", "disable", "":
		return false, nil
	default:
		return false, fmt.Errorf("invalid bool (use true/false)")
	}
}

func parseNonNegativeInt(s string) (int, error) {
	v := strings.TrimSpace(s)
	if v == "" {
		return 0, fmt.Errorf("invalid int")
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("invalid int")
	}
	if n < 0 {
		return 0, fmt.Errorf("must be >= 0")
	}
	return n, nil
}

func fmtBool(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func boolPtr(v bool) *bool {
	return &v
}
