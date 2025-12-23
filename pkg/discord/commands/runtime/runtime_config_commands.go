package runtime

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
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
// - core.CommandRouter routes slash commands only, so component + modal interactions
//   are handled via discordgo.Session.AddHandler(...) in commands/handler.go.
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
}

type panelState struct {
	Mode   pageMode
	Group  string
	Key    runtimeKey
	Filter string // reserved for future search; not wired yet
}

func (s panelState) encode() string {
	// mode|group|key
	return string(s.Mode) + stateSep + s.Group + stateSep + string(s.Key)
}

func decodeState(raw string) panelState {
	// Expected: mode|group|key
	// Use SplitN to avoid accepting extra separators as additional state fields.
	parts := strings.SplitN(raw, stateSep, 3)
	st := panelState{Mode: pageMain, Group: "ALL", Key: runtimeKeyALICE_BOT_THEME}
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

// --- Specs registry (single source of truth) ---

const (
	// THEME
	runtimeKeyALICE_BOT_THEME runtimeKey = "ALICE_BOT_THEME"

	// SERVICES (LOGGING)
	runtimeKeyALICE_DISABLE_DB_CLEANUP      runtimeKey = "ALICE_DISABLE_DB_CLEANUP"
	runtimeKeyALICE_DISABLE_AUTOMOD_LOGS    runtimeKey = "ALICE_DISABLE_AUTOMOD_LOGS"
	runtimeKeyALICE_DISABLE_MESSAGE_LOGS    runtimeKey = "ALICE_DISABLE_MESSAGE_LOGS"
	runtimeKeyALICE_DISABLE_ENTRY_EXIT_LOGS runtimeKey = "ALICE_DISABLE_ENTRY_EXIT_LOGS"
	runtimeKeyALICE_DISABLE_REACTION_LOGS   runtimeKey = "ALICE_DISABLE_REACTION_LOGS"
	runtimeKeyALICE_DISABLE_USER_LOGS       runtimeKey = "ALICE_DISABLE_USER_LOGS"

	// MESSAGE CACHE
	runtimeKeyALICE_MESSAGE_CACHE_TTL_HOURS runtimeKey = "ALICE_MESSAGE_CACHE_TTL_HOURS"
	runtimeKeyALICE_MESSAGE_DELETE_ON_LOG   runtimeKey = "ALICE_MESSAGE_DELETE_ON_LOG"
	runtimeKeyALICE_MESSAGE_CACHE_CLEANUP   runtimeKey = "ALICE_MESSAGE_CACHE_CLEANUP"

	// BACKFILL (ENTRY/EXIT)
	runtimeKeyALICE_BACKFILL_ENTRY_EXIT_ENABLED    runtimeKey = "ALICE_BACKFILL_ENTRY_EXIT_ENABLED"
	runtimeKeyALICE_BACKFILL_ENTRY_EXIT_CHANNEL_ID runtimeKey = "ALICE_BACKFILL_ENTRY_EXIT_CHANNEL_ID"
	runtimeKeyALICE_BACKFILL_ENTRY_EXIT_START_DAY  runtimeKey = "ALICE_BACKFILL_ENTRY_EXIT_START_DAY"

	// BOT ROLE PERMISSION MIRRORING (SAFETY)
	runtimeKeyALICE_DISABLE_BOT_ROLE_PERM_MIRROR       runtimeKey = "ALICE_DISABLE_BOT_ROLE_PERM_MIRROR"
	runtimeKeyALICE_BOT_ROLE_PERM_MIRROR_ACTOR_ROLE_ID runtimeKey = "ALICE_BOT_ROLE_PERM_MIRROR_ACTOR_ROLE_ID"
)

func allSpecs() []spec {
	// Keep groups stable and short (helps readability in embed fields)
	return []spec{
		{
			Key:         runtimeKeyALICE_BOT_THEME,
			Group:       "THEME",
			Type:        vtString,
			DefaultHint: "(default)",
			ShortHelp:   "Theme name (empty = default)",
			RestartHint: restartRecommended,
			MaxInputLen: 60,
		},
		{
			Key:         runtimeKeyALICE_DISABLE_DB_CLEANUP,
			Group:       "SERVICES (LOGGING)",
			Type:        vtBool,
			DefaultHint: "false",
			ShortHelp:   "Disable periodic DB cleanup",
			RestartHint: restartRequired, // still a goroutine in runner; hot-apply intentionally not handled
		},
		{
			Key:         runtimeKeyALICE_DISABLE_AUTOMOD_LOGS,
			Group:       "SERVICES (LOGGING)",
			Type:        vtBool,
			DefaultHint: "false",
			ShortHelp:   "Disable automod logging service startup",
			RestartHint: restartRecommended,
		},
		{
			Key:         runtimeKeyALICE_DISABLE_MESSAGE_LOGS,
			Group:       "SERVICES (LOGGING)",
			Type:        vtBool,
			DefaultHint: "false",
			ShortHelp:   "Disable message logging service startup",
			RestartHint: restartRecommended,
		},
		{
			Key:         runtimeKeyALICE_DISABLE_ENTRY_EXIT_LOGS,
			Group:       "SERVICES (LOGGING)",
			Type:        vtBool,
			DefaultHint: "false",
			ShortHelp:   "Disable entry/exit logging service startup",
			RestartHint: restartRecommended,
		},
		{
			Key:         runtimeKeyALICE_DISABLE_REACTION_LOGS,
			Group:       "SERVICES (LOGGING)",
			Type:        vtBool,
			DefaultHint: "false",
			ShortHelp:   "Disable reaction logging service startup",
			RestartHint: restartRecommended,
		},
		{
			Key:         runtimeKeyALICE_DISABLE_USER_LOGS,
			Group:       "SERVICES (LOGGING)",
			Type:        vtBool,
			DefaultHint: "false",
			ShortHelp:   "Disable user log handlers (avatars/roles)",
			RestartHint: restartRecommended,
		},
		{
			Key:         runtimeKeyALICE_MESSAGE_CACHE_TTL_HOURS,
			Group:       "MESSAGE CACHE",
			Type:        vtInt,
			DefaultHint: "72",
			ShortHelp:   "Cache TTL in hours for message edit/delete logging (0 = default)",
			RestartHint: restartRequired,
			MaxInputLen: 8,
		},
		{
			Key:         runtimeKeyALICE_MESSAGE_DELETE_ON_LOG,
			Group:       "MESSAGE CACHE",
			Type:        vtBool,
			DefaultHint: "false",
			ShortHelp:   "Delete cached message record after it is logged",
			RestartHint: restartRecommended,
		},
		{
			Key:         runtimeKeyALICE_MESSAGE_CACHE_CLEANUP,
			Group:       "MESSAGE CACHE",
			Type:        vtBool,
			DefaultHint: "false",
			ShortHelp:   "Cleanup expired cached messages on startup",
			RestartHint: restartRecommended,
		},
		{
			Key:         runtimeKeyALICE_BACKFILL_ENTRY_EXIT_ENABLED,
			Group:       "BACKFILL",
			Type:        vtBool,
			DefaultHint: "false",
			ShortHelp:   "Auto-dispatch entry/exit backfill task on startup",
			RestartHint: restartRequired,
		},
		{
			Key:         runtimeKeyALICE_BACKFILL_ENTRY_EXIT_CHANNEL_ID,
			Group:       "BACKFILL",
			Type:        vtString,
			DefaultHint: "(empty)",
			ShortHelp:   "Channel ID to backfill from (required to run)",
			RestartHint: restartRequired,
			MaxInputLen: 32,
		},
		{
			Key:         runtimeKeyALICE_BACKFILL_ENTRY_EXIT_START_DAY,
			Group:       "BACKFILL",
			Type:        vtDate,
			DefaultHint: "today (UTC)",
			ShortHelp:   "Start day (YYYY-MM-DD) for backfill",
			RestartHint: restartRequired,
			MaxInputLen: 16,
		},
		{
			Key:         runtimeKeyALICE_DISABLE_BOT_ROLE_PERM_MIRROR,
			Group:       "SAFETY",
			Type:        vtBool,
			DefaultHint: "false",
			ShortHelp:   "Disable bot role permission mirroring safety feature",
			RestartHint: restartRecommended, // effective at event time; no restart needed for behavior
		},
		{
			Key:         runtimeKeyALICE_BOT_ROLE_PERM_MIRROR_ACTOR_ROLE_ID,
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
	checker := core.NewPermissionChecker(router.GetSession(), router.GetConfigManager())
	group := core.NewGroupCommand(groupName, "Manage server configuration", checker)

	group.AddSubCommand(newRuntimeSubCommand(cc.configManager))

	// Register group under existing /config namespace.
	router.RegisterCommand(group)
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

	rc, err := loadRuntimeConfig(ctx.Config)
	if err != nil {
		return core.NewResponseBuilder(ctx.Session).Ephemeral().Error(ctx.Interaction, fmt.Sprintf("Failed to load runtime config: %v", err))
	}

	st := panelState{
		Mode:  pageMain,
		Group: "ALL",
		Key:   runtimeKeyALICE_BOT_THEME,
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

func loadRuntimeConfig(cm *files.ConfigManager) (files.RuntimeConfig, error) {
	if cm == nil {
		return files.RuntimeConfig{}, fmt.Errorf("config manager is nil")
	}
	_ = cm.LoadConfig() // best effort
	cfg := cm.Config()
	if cfg == nil {
		return files.RuntimeConfig{}, nil
	}
	return cfg.RuntimeConfig, nil
}

func saveRuntimeConfig(cm *files.ConfigManager, rc files.RuntimeConfig) error {
	if cm == nil {
		return fmt.Errorf("config manager is nil")
	}
	_ = cm.LoadConfig() // best effort
	cfg := cm.Config()
	if cfg == nil {
		return fmt.Errorf("settings.json not loaded (config is nil)")
	}
	cfg.RuntimeConfig = rc
	return cm.SaveConfig()
}

// --- RuntimeConfig mapping (get/set/reset) ---

func getValue(rc files.RuntimeConfig, k runtimeKey) (string, bool) {
	switch k {
	case runtimeKeyALICE_BOT_THEME:
		return rc.ALICE_BOT_THEME, true

	case runtimeKeyALICE_DISABLE_DB_CLEANUP:
		return fmtBool(rc.ALICE_DISABLE_DB_CLEANUP), true
	case runtimeKeyALICE_DISABLE_AUTOMOD_LOGS:
		return fmtBool(rc.ALICE_DISABLE_AUTOMOD_LOGS), true
	case runtimeKeyALICE_DISABLE_MESSAGE_LOGS:
		return fmtBool(rc.ALICE_DISABLE_MESSAGE_LOGS), true
	case runtimeKeyALICE_DISABLE_ENTRY_EXIT_LOGS:
		return fmtBool(rc.ALICE_DISABLE_ENTRY_EXIT_LOGS), true
	case runtimeKeyALICE_DISABLE_REACTION_LOGS:
		return fmtBool(rc.ALICE_DISABLE_REACTION_LOGS), true
	case runtimeKeyALICE_DISABLE_USER_LOGS:
		return fmtBool(rc.ALICE_DISABLE_USER_LOGS), true

	case runtimeKeyALICE_MESSAGE_CACHE_TTL_HOURS:
		return strconv.Itoa(rc.ALICE_MESSAGE_CACHE_TTL_HOURS), true
	case runtimeKeyALICE_MESSAGE_DELETE_ON_LOG:
		return fmtBool(rc.ALICE_MESSAGE_DELETE_ON_LOG), true
	case runtimeKeyALICE_MESSAGE_CACHE_CLEANUP:
		return fmtBool(rc.ALICE_MESSAGE_CACHE_CLEANUP), true

	case runtimeKeyALICE_BACKFILL_ENTRY_EXIT_ENABLED:
		return fmtBool(rc.ALICE_BACKFILL_ENTRY_EXIT_ENABLED), true
	case runtimeKeyALICE_BACKFILL_ENTRY_EXIT_CHANNEL_ID:
		return rc.ALICE_BACKFILL_ENTRY_EXIT_CHANNEL_ID, true
	case runtimeKeyALICE_BACKFILL_ENTRY_EXIT_START_DAY:
		return rc.ALICE_BACKFILL_ENTRY_EXIT_START_DAY, true

	case runtimeKeyALICE_DISABLE_BOT_ROLE_PERM_MIRROR:
		return fmtBool(rc.ALICE_DISABLE_BOT_ROLE_PERM_MIRROR), true
	case runtimeKeyALICE_BOT_ROLE_PERM_MIRROR_ACTOR_ROLE_ID:
		return rc.ALICE_BOT_ROLE_PERM_MIRROR_ACTOR_ROLE_ID, true

	default:
		return "", false
	}
}

func resetValue(rc files.RuntimeConfig, k runtimeKey) (files.RuntimeConfig, bool) {
	switch k {
	case runtimeKeyALICE_BOT_THEME:
		rc.ALICE_BOT_THEME = ""
		return rc, true

	case runtimeKeyALICE_DISABLE_DB_CLEANUP:
		rc.ALICE_DISABLE_DB_CLEANUP = false
		return rc, true
	case runtimeKeyALICE_DISABLE_AUTOMOD_LOGS:
		rc.ALICE_DISABLE_AUTOMOD_LOGS = false
		return rc, true
	case runtimeKeyALICE_DISABLE_MESSAGE_LOGS:
		rc.ALICE_DISABLE_MESSAGE_LOGS = false
		return rc, true
	case runtimeKeyALICE_DISABLE_ENTRY_EXIT_LOGS:
		rc.ALICE_DISABLE_ENTRY_EXIT_LOGS = false
		return rc, true
	case runtimeKeyALICE_DISABLE_REACTION_LOGS:
		rc.ALICE_DISABLE_REACTION_LOGS = false
		return rc, true
	case runtimeKeyALICE_DISABLE_USER_LOGS:
		rc.ALICE_DISABLE_USER_LOGS = false
		return rc, true

	case runtimeKeyALICE_MESSAGE_CACHE_TTL_HOURS:
		rc.ALICE_MESSAGE_CACHE_TTL_HOURS = 0
		return rc, true
	case runtimeKeyALICE_MESSAGE_DELETE_ON_LOG:
		rc.ALICE_MESSAGE_DELETE_ON_LOG = false
		return rc, true
	case runtimeKeyALICE_MESSAGE_CACHE_CLEANUP:
		rc.ALICE_MESSAGE_CACHE_CLEANUP = false
		return rc, true

	case runtimeKeyALICE_BACKFILL_ENTRY_EXIT_ENABLED:
		rc.ALICE_BACKFILL_ENTRY_EXIT_ENABLED = false
		return rc, true
	case runtimeKeyALICE_BACKFILL_ENTRY_EXIT_CHANNEL_ID:
		rc.ALICE_BACKFILL_ENTRY_EXIT_CHANNEL_ID = ""
		return rc, true
	case runtimeKeyALICE_BACKFILL_ENTRY_EXIT_START_DAY:
		rc.ALICE_BACKFILL_ENTRY_EXIT_START_DAY = ""
		return rc, true

	case runtimeKeyALICE_DISABLE_BOT_ROLE_PERM_MIRROR:
		rc.ALICE_DISABLE_BOT_ROLE_PERM_MIRROR = false
		return rc, true
	case runtimeKeyALICE_BOT_ROLE_PERM_MIRROR_ACTOR_ROLE_ID:
		rc.ALICE_BOT_ROLE_PERM_MIRROR_ACTOR_ROLE_ID = ""
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
			if sp.Key == runtimeKeyALICE_BACKFILL_ENTRY_EXIT_START_DAY {
				rc.ALICE_BACKFILL_ENTRY_EXIT_START_DAY = ""
				return rc, nil
			}
			return rc, nil
		}
		if _, err := time.Parse("2006-01-02", raw); err != nil {
			return rc, fmt.Errorf("invalid date (expected YYYY-MM-DD)")
		}
		if sp.Key == runtimeKeyALICE_BACKFILL_ENTRY_EXIT_START_DAY {
			rc.ALICE_BACKFILL_ENTRY_EXIT_START_DAY = raw
			return rc, nil
		}
		return rc, fmt.Errorf("unsupported date key")
	case vtString:
		// Empty string is allowed to reset to default behavior
		switch sp.Key {
		case runtimeKeyALICE_BOT_THEME:
			rc.ALICE_BOT_THEME = raw
			return rc, nil
		case runtimeKeyALICE_BACKFILL_ENTRY_EXIT_CHANNEL_ID:
			rc.ALICE_BACKFILL_ENTRY_EXIT_CHANNEL_ID = raw
			return rc, nil
		case runtimeKeyALICE_BOT_ROLE_PERM_MIRROR_ACTOR_ROLE_ID:
			rc.ALICE_BOT_ROLE_PERM_MIRROR_ACTOR_ROLE_ID = raw
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
	case runtimeKeyALICE_MESSAGE_CACHE_TTL_HOURS:
		// Accept 0 to mean "use default" (service will fall back).
		if v < 0 {
			return rc, fmt.Errorf("must be >= 0")
		}
		// Guardrail against absurd values.
		if v > 24*365 {
			return rc, fmt.Errorf("too large (max %d)", 24*365)
		}
		rc.ALICE_MESSAGE_CACHE_TTL_HOURS = v
		return rc, nil
	default:
		return rc, fmt.Errorf("not an int key")
	}
}

func setBool(rc files.RuntimeConfig, k runtimeKey, v bool) (files.RuntimeConfig, error) {
	switch k {
	case runtimeKeyALICE_DISABLE_DB_CLEANUP:
		rc.ALICE_DISABLE_DB_CLEANUP = v
	case runtimeKeyALICE_DISABLE_AUTOMOD_LOGS:
		rc.ALICE_DISABLE_AUTOMOD_LOGS = v
	case runtimeKeyALICE_DISABLE_MESSAGE_LOGS:
		rc.ALICE_DISABLE_MESSAGE_LOGS = v
	case runtimeKeyALICE_DISABLE_ENTRY_EXIT_LOGS:
		rc.ALICE_DISABLE_ENTRY_EXIT_LOGS = v
	case runtimeKeyALICE_DISABLE_REACTION_LOGS:
		rc.ALICE_DISABLE_REACTION_LOGS = v
	case runtimeKeyALICE_DISABLE_USER_LOGS:
		rc.ALICE_DISABLE_USER_LOGS = v
	case runtimeKeyALICE_MESSAGE_DELETE_ON_LOG:
		rc.ALICE_MESSAGE_DELETE_ON_LOG = v
	case runtimeKeyALICE_MESSAGE_CACHE_CLEANUP:
		rc.ALICE_MESSAGE_CACHE_CLEANUP = v
	case runtimeKeyALICE_BACKFILL_ENTRY_EXIT_ENABLED:
		rc.ALICE_BACKFILL_ENTRY_EXIT_ENABLED = v
	case runtimeKeyALICE_DISABLE_BOT_ROLE_PERM_MIRROR:
		rc.ALICE_DISABLE_BOT_ROLE_PERM_MIRROR = v
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

	desc := strings.Join([]string{
		"Painel para editar **runtime_config** (substitui as env vars operacionais).",
		"",
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
		raw, _ := getValue(rc, sp.Key)
		display := formatForEmbed(raw, sp)
		line := fmt.Sprintf("`%s`: **%s**", sp.Key, display)
		grouped[sp.Group] = append(grouped[sp.Group], line)
	}

	groupOrder := []string{"THEME", "SERVICES (LOGGING)", "MESSAGE CACHE", "BACKFILL", "SAFETY"}
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

	lines := []string{
		fmt.Sprintf("`%s`", sp.Key),
		"",
		fmt.Sprintf("**Grupo:** %s", sp.Group),
		fmt.Sprintf("**Tipo:** %s", sp.Type),
		fmt.Sprintf("**Default:** %s", sp.DefaultHint),
		fmt.Sprintf("**Atual:** %s", cur),
		"",
		fmt.Sprintf("**Descrição:** %s", sp.ShortHelp),
		fmt.Sprintf("**Efeito:** %s", sp.RestartHint),
	}

	return &discordgo.MessageEmbed{
		Title:       "CONFIG (RUNTIME) — DETAILS",
		Description: strings.Join(lines, "\n"),
		Color:       theme.Muted(),
		Footer: &discordgo.MessageEmbedFooter{
			Text: "Use BACK para retornar ao painel.",
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}
}

func renderHelpEmbed() *discordgo.MessageEmbed {
	desc := strings.Join([]string{
		"Este painel edita `settings.json` em `runtime_config`.",
		"",
		"**Notas:**",
		"• Os nomes continuam em CAPS para manter compatibilidade mental com as env vars.",
		"• O bot não lê mais essas opções via environment (token continua sendo env).",
		"• Algumas mudanças podem ser hot-apply (THEME e alguns ALICE_DISABLE_*).",
		"",
		"**Como editar:**",
		"1) Filtre por grupo (opcional) e selecione uma key.",
		"2) Boolean: use TOGGLE.",
		"3) Outros tipos: use EDIT e preencha o modal.",
		"4) RESET limpa o valor e volta ao default do código.",
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
	return discordgo.ActionsRow{
		Components: []discordgo.MessageComponent{
			discordgo.Button{CustomID: cidButtonHelp + stateSep + st.withMode(pageHelp).encode(), Label: "HELP", Style: discordgo.SecondaryButton},
			discordgo.Button{CustomID: cidButtonMain + stateSep + st.withMode(pageMain).encode(), Label: "MAIN", Style: discordgo.SecondaryButton},
		},
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

// HandleRuntimeConfigInteractions should be registered on the discordgo session (outside the slash router).
// It captures the runtime hot-apply manager via closure, so the panel can apply changes immediately
// after persisting settings.json.
func HandleRuntimeConfigInteractions(configManager *files.ConfigManager, applier *runtimeapply.Manager) func(s *discordgo.Session, i *discordgo.InteractionCreate) {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if i == nil || s == nil {
			return
		}

		switch i.Type {
		case discordgo.InteractionMessageComponent:
			handleComponent(s, i, configManager, applier)
		case discordgo.InteractionModalSubmit:
			handleModalSubmit(s, i, configManager, applier)
		default:
			return
		}
	}
}

func handleComponent(s *discordgo.Session, i *discordgo.InteractionCreate, configManager *files.ConfigManager, applier *runtimeapply.Manager) {
	cc := i.MessageComponentData()
	if !strings.HasPrefix(cc.CustomID, customIDPrefix) {
		return
	}

	action, st := parseActionAndState(cc.CustomID)
	if action == "" {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredMessageUpdate,
		})
		_ = editInteractionMessage(s, i, errorEmbed("Invalid interaction state"), nil)
		return
	}

	// If this interaction is going to open a modal, we must NOT ack with a message update first.
	// Otherwise the modal response can fail because an interaction can only be responded to once.
	if action != cidButtonEdit {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredMessageUpdate,
		})
	}

	rc, err := loadRuntimeConfig(configManager)
	if err != nil {
		if action == cidButtonEdit {
			_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Flags: discordgo.MessageFlagsEphemeral,
					Embeds: []*discordgo.MessageEmbed{
						errorEmbed(fmt.Sprintf("Failed to load runtime config: %v", err)),
					},
				},
			})
			return
		}
		_ = editInteractionMessage(s, i, errorEmbed(fmt.Sprintf("Failed to load runtime config: %v", err)), nil)
		return
	}

	switch action {
	case cidSelectGroup:
		if len(cc.Values) == 0 {
			embed := renderMainEmbed(rc, st.withMode(pageMain))
			_ = editInteractionMessage(s, i, embed, renderMainComponents(rc, st.withMode(pageMain)))
			return
		}
		st = decodeState(cc.Values[0]).withMode(pageMain)
		st = ensureKeyInGroup(st)
		embed := renderMainEmbed(rc, st)
		_ = editInteractionMessage(s, i, embed, renderMainComponents(rc, st))
		return

	case cidSelectKey:
		if len(cc.Values) == 0 {
			embed := renderMainEmbed(rc, st.withMode(pageMain))
			_ = editInteractionMessage(s, i, embed, renderMainComponents(rc, st.withMode(pageMain)))
			return
		}
		st = decodeState(cc.Values[0]).withMode(pageMain)
		embed := renderMainEmbed(rc, st)
		_ = editInteractionMessage(s, i, embed, renderMainComponents(rc, st))
		return

	case cidButtonMain, cidButtonBack:
		st = st.withMode(pageMain)
		st = ensureKeyInGroup(st)
		embed := renderMainEmbed(rc, st)
		_ = editInteractionMessage(s, i, embed, renderMainComponents(rc, st))
		return

	case cidButtonHelp:
		st = st.withMode(pageHelp)
		embed := renderHelpEmbed()
		_ = editInteractionMessage(s, i, embed, renderHelpComponents(st))
		return

	case cidButtonDetail:
		st = st.withMode(pageDetail)
		embed := renderDetailsEmbed(rc, st)
		_ = editInteractionMessage(s, i, embed, renderDetailComponents(st))
		return

	case cidButtonReload:
		rc, _ = loadRuntimeConfig(configManager)
		st = ensureKeyInGroup(st)
		switch st.Mode {
		case pageHelp:
			embed := renderHelpEmbed()
			_ = editInteractionMessage(s, i, embed, renderHelpComponents(st))
		case pageDetail:
			embed := renderDetailsEmbed(rc, st)
			_ = editInteractionMessage(s, i, embed, renderDetailComponents(st))
		default:
			embed := renderMainEmbed(rc, st.withMode(pageMain))
			_ = editInteractionMessage(s, i, embed, renderMainComponents(rc, st.withMode(pageMain)))
		}
		return

	case cidButtonReset:
		st = st.withMode(pageMain)
		rc2, ok := resetValue(rc, st.Key)
		if !ok {
			_ = editInteractionMessage(s, i, errorEmbed("Unknown key"), nil)
			return
		}
		if err := saveRuntimeConfig(configManager, rc2); err != nil {
			_ = editInteractionMessage(s, i, errorEmbed(fmt.Sprintf("Failed to save: %v", err)), nil)
			return
		}
		hotApplyBestEffort(applier, rc2)
		embed := renderMainEmbed(rc2, st)
		_ = editInteractionMessage(s, i, embed, renderMainComponents(rc2, st))
		return

	case cidButtonToggle:
		st = st.withMode(pageMain)
		sp, ok := specByKey(st.Key)
		if !ok {
			_ = editInteractionMessage(s, i, errorEmbed("Unknown key"), nil)
			return
		}
		if sp.Type != vtBool {
			_ = editInteractionMessage(s, i, errorEmbed("TOGGLE is only valid for boolean keys"), renderMainComponents(rc, st))
			return
		}
		rc2, err := toggleBool(rc, st.Key)
		if err != nil {
			_ = editInteractionMessage(s, i, errorEmbed(fmt.Sprintf("Toggle failed: %v", err)), renderMainComponents(rc, st))
			return
		}
		if err := saveRuntimeConfig(configManager, rc2); err != nil {
			_ = editInteractionMessage(s, i, errorEmbed(fmt.Sprintf("Failed to save: %v", err)), nil)
			return
		}
		hotApplyBestEffort(applier, rc2)
		embed := renderMainEmbed(rc2, st)
		_ = editInteractionMessage(s, i, embed, renderMainComponents(rc2, st))
		return

	case cidButtonEdit:
		sp, ok := specByKey(st.Key)
		if !ok {
			// This interaction path normally opens a modal, so we intentionally do NOT
			// ack with a message update earlier. If we hit an error, we must still
			// respond once to avoid an "interaction failed" on the client.
			_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Flags: discordgo.MessageFlagsEphemeral,
					Embeds: []*discordgo.MessageEmbed{
						errorEmbed("Unknown key"),
					},
				},
			})
			return
		}
		if sp.Type == vtBool {
			_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Flags: discordgo.MessageFlagsEphemeral,
					Embeds: []*discordgo.MessageEmbed{
						errorEmbed("EDIT is not valid for boolean keys (use TOGGLE)"),
					},
				},
			})
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

		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
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
		})
		return

	default:
		_ = editInteractionMessage(s, i, errorEmbed("Unknown action"), nil)
		return
	}
}

func handleModalSubmit(s *discordgo.Session, i *discordgo.InteractionCreate, configManager *files.ConfigManager, applier *runtimeapply.Manager) {
	m := i.ModalSubmitData()
	if !strings.HasPrefix(m.CustomID, modalEditValueID+stateSep) {
		return
	}

	// For modal submits, keep the panel usable by updating the original panel message.
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
	})

	rawState := strings.TrimPrefix(m.CustomID, modalEditValueID+stateSep)
	st := decodeState(rawState)

	sp, ok := specByKey(st.Key)
	if !ok {
		embed := errorEmbed("Unknown key")
		_ = editInteractionMessage(s, i, embed, renderMainComponents(files.RuntimeConfig{}, st.withMode(pageMain)))
		return
	}
	if sp.Type == vtBool {
		embed := errorEmbed("Invalid modal for bool key")
		_ = editInteractionMessage(s, i, embed, renderMainComponents(files.RuntimeConfig{}, st.withMode(pageMain)))
		return
	}

	val := extractModalValue(m, modalFieldValue)

	rc, err := loadRuntimeConfig(configManager)
	if err != nil {
		_ = editInteractionMessage(s, i, errorEmbed(fmt.Sprintf("Failed to load runtime config: %v", err)), nil)
		return
	}

	next, err := setValue(rc, sp, val)
	if err != nil {
		embed := errorEmbed(fmt.Sprintf("Invalid value: %v", err))
		st = ensureKeyInGroup(st.withMode(pageMain))
		_ = editInteractionMessage(s, i, embed, renderMainComponents(rc, st))
		return
	}
	if err := saveRuntimeConfig(configManager, next); err != nil {
		_ = editInteractionMessage(s, i, errorEmbed(fmt.Sprintf("Failed to save: %v", err)), nil)
		return
	}

	hotApplyBestEffort(applier, next)

	// After saving, return to MAIN with refreshed values so the user can keep navigating.
	st = ensureKeyInGroup(st.withMode(pageMain))
	embed := renderMainEmbed(next, st)
	_ = editInteractionMessage(s, i, embed, renderMainComponents(next, st))
}

func hotApplyBestEffort(applier *runtimeapply.Manager, next files.RuntimeConfig) {
	if applier == nil {
		return
	}
	_ = applier.Apply(context.Background(), next)
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
		return st.withKey(runtimeKeyALICE_BOT_THEME)
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
	return st.withKey(runtimeKeyALICE_BOT_THEME)
}

// parseActionAndState decodes "action|mode|group|key"
func parseActionAndState(customID string) (action string, st panelState) {
	if customID == cidSelectGroup {
		return cidSelectGroup, panelState{Mode: pageMain, Group: "ALL", Key: runtimeKeyALICE_BOT_THEME}
	}
	if customID == cidSelectKey {
		return cidSelectKey, panelState{Mode: pageMain, Group: "ALL", Key: runtimeKeyALICE_BOT_THEME}
	}

	if !strings.Contains(customID, stateSep) {
		return "", panelState{}
	}
	parts := strings.SplitN(customID, stateSep, 2)
	if len(parts) != 2 {
		return "", panelState{}
	}
	action = parts[0]
	switch action {
	case cidSelectGroup, cidSelectKey,
		cidButtonMain, cidButtonHelp, cidButtonBack,
		cidButtonDetail, cidButtonToggle, cidButtonEdit,
		cidButtonReset, cidButtonReload:
		// ok
	default:
		return "", panelState{}
	}
	st = decodeState(parts[1])
	return action, st
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
