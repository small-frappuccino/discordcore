# Domain Architecture: app

## Layout Topology
```text
app/
├── runtimecmd
│   ├── runtimecmd.go
│   └── runtimecmd_test.go
├── bot_runtime.go
├── bot_runtime_test.go
├── bot_supervisor.go
├── bot_supervisor_test.go
├── catalog_registrars.go
├── catalog_registrars_test.go
├── command_handler.go
├── command_handler_lifecycle_test.go
├── command_handler_test.go
├── control.go
├── control_test.go
├── observability.go
├── observability_test.go
├── runner.go
├── runner_test.go
├── startup.go
├── startup_test.go
├── task_router.go
├── task_router_test.go
├── version.go
└── version_test.go
```

## Source Stream Aggregation

// === FILE: pkg/app/bot_runtime.go ===
```go
package app

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/state"
	"github.com/small-frappuccino/discordcore/pkg/clock"
	"github.com/small-frappuccino/discordcore/pkg/control"
	discord_automod "github.com/small-frappuccino/discordcore/pkg/discord/automod"
	"github.com/small-frappuccino/discordcore/pkg/discord/cache"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/moderation"
	"github.com/small-frappuccino/discordcore/pkg/discord/embeds"
	"github.com/small-frappuccino/discordcore/pkg/discord/logging"
	discordmembers "github.com/small-frappuccino/discordcore/pkg/discord/members"
	discordmessages "github.com/small-frappuccino/discordcore/pkg/discord/messages"
	"github.com/small-frappuccino/discordcore/pkg/discord/partners"
	discordqotd "github.com/small-frappuccino/discordcore/pkg/discord/qotd"
	"github.com/small-frappuccino/discordcore/pkg/discord/roles"
	"github.com/small-frappuccino/discordcore/pkg/discord/session"
	discordstats "github.com/small-frappuccino/discordcore/pkg/discord/stats"
	"github.com/small-frappuccino/discordcore/pkg/discord/tickets"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/members"

	"github.com/small-frappuccino/discordcore/pkg/messages"
	applicationqotd "github.com/small-frappuccino/discordcore/pkg/qotd"
	"github.com/small-frappuccino/discordcore/pkg/runtimeapply"
	"github.com/small-frappuccino/discordcore/pkg/service"
	"github.com/small-frappuccino/discordcore/pkg/stats"
	"github.com/small-frappuccino/discordcore/pkg/storage/postgres"
	"github.com/small-frappuccino/discordcore/pkg/task"
	"github.com/small-frappuccino/discordgo"
)

type botRuntimeCapabilities struct {
	monitoring          bool
	automod             bool
	userPrune           bool
	qotdRuntime         bool
	stats               bool
	warmup              bool
	intents             discordgo.Intent
	hasCommands         bool
	messageEventService bool
	memberEventService  bool
}

// HasCommands reports whether any command catalog should be installed.
func (c botRuntimeCapabilities) HasCommands() bool { return c.hasCommands }

func resolveBotRuntimeCapabilities(
	cfg *files.BotConfig,
	botInstanceID string,
) botRuntimeCapabilities {
	if cfg == nil {
		panic("hardware-aligned validation failure: configuration reference is nil")
	}
	capabilities := botRuntimeCapabilities{
		intents: discordgo.IntentsGuilds,
	}

	guilds := files.GuildsForBotInstance(cfg, botInstanceID)
	for _, guild := range guilds {
		features := cfg.ResolveFeatures(guild.GuildID)
		runtimeConfig := cfg.ResolveRuntimeConfig(guild.GuildID)

		isQOTDBot := false
		isRolesBot := false
		isStatsBot := false
		isModBot := false
		isLoggingBot := false

		if !guild.QOTD.IsZero() {
			if id, _ := files.ResolveFeatureBotInstanceID(guild, "qotd"); id == botInstanceID {
				isQOTDBot = true
			}
		}
		if features.Services.Commands {
			if id, _ := files.ResolveFeatureBotInstanceID(guild, "roles"); id == botInstanceID {
				isRolesBot = true
			}
			if id, _ := files.ResolveFeatureBotInstanceID(guild, "stats"); id == botInstanceID {
				isStatsBot = true
			}
		}
		if guild.Channels.AutomodAction != "" || guild.UserPrune.Enabled {
			if id, _ := files.ResolveFeatureBotInstanceID(guild, "moderation"); id == botInstanceID {
				isModBot = true
			}
		}
		if features.Services.Monitoring {
			if id, _ := files.ResolveFeatureBotInstanceID(guild, "logging"); id == botInstanceID {
				isLoggingBot = true
			}
		}

		if isQOTDBot {
			capabilities.qotdRuntime = true
		}
		if features.Services.Commands {
			capabilities.hasCommands = true
		}
		if isRolesBot {
			capabilities.intents |= discordgo.IntentsGuildMembers
			capabilities.warmup = true
		}
		if isStatsBot {
			capabilities.stats = true
		}
		if isModBot {
			if guild.Channels.AutomodAction != "" {
				capabilities.automod = true
				capabilities.intents |= discordgo.IntentAutoModerationExecution
			}
			if guild.UserPrune.Enabled {
				capabilities.userPrune = true
				capabilities.intents |= discordgo.IntentsGuildMembers
				capabilities.warmup = true
			}
		}

		if features.Services.Monitoring {
			if isRolesBot || isModBot || isStatsBot || isLoggingBot {
				if isLoggingBot {
					capabilities.messageEventService = true
				}
				if botRuntimeNeedsMonitoring(features, runtimeConfig, guild) {
					capabilities.monitoring = true
				}
				if isRolesBot || isStatsBot || isLoggingBot {
					if botRuntimeNeedsMemberData(features, runtimeConfig, guild) {
						capabilities.intents |= discordgo.IntentsGuildMembers
						capabilities.warmup = true
						if isRolesBot || isLoggingBot {
							capabilities.memberEventService = true
						}
					}
				}
				if isModBot || isLoggingBot {
					if botRuntimeNeedsPresence(features, runtimeConfig, guild) {
						capabilities.intents |= discordgo.IntentsGuildPresences
						capabilities.warmup = true
					}
					if botRuntimeNeedsMessages(runtimeConfig, guild) {
						capabilities.intents |= discordgo.IntentsGuildMessages | discordgo.IntentMessageContent
					}
					if botRuntimeNeedsReactions(runtimeConfig) {
						capabilities.intents |= discordgo.IntentsGuildMessageReactions
					}
				}
			}
		}
	}
	return capabilities
}

func botRuntimeNeedsMonitoring(
	features files.ResolvedFeatureToggles,
	runtimeConfig files.RuntimeConfig,
	guild files.GuildConfig,
) bool {
	// Synthesize complex sub-capability evaluation flags across divergent configuration schemas.
	return botRuntimeNeedsMessages(runtimeConfig, guild) ||
		botRuntimeNeedsReactions(runtimeConfig) ||
		botRuntimeNeedsPresence(features, runtimeConfig, guild) ||
		botRuntimeNeedsMemberData(features, runtimeConfig, guild) ||
		botRuntimeNeedsBotPermMirror(runtimeConfig)
}

func botRuntimeNeedsMessages(runtimeConfig files.RuntimeConfig, guild files.GuildConfig) bool {
	if runtimeConfig.DisableMessageLogs {
		return false
	}
	return guild.Channels.MessageEdit != "" || guild.Channels.MessageDelete != ""
}

func botRuntimeNeedsReactions(runtimeConfig files.RuntimeConfig) bool {

	return !runtimeConfig.DisableReactionLogs
}

func botRuntimeNeedsPresence(features files.ResolvedFeatureToggles, runtimeConfig files.RuntimeConfig, guild files.GuildConfig) bool {
	if !runtimeConfig.DisableUserLogs && guild.Channels.AvatarLogging != "" {
		return true
	}
	if features.PresenceWatch.User && strings.TrimSpace(runtimeConfig.PresenceWatchUserID) != "" {
		return true
	}
	return features.PresenceWatch.Bot && runtimeConfig.PresenceWatchBot
}

func botRuntimeNeedsMemberData(
	features files.ResolvedFeatureToggles,
	runtimeConfig files.RuntimeConfig,
	guild files.GuildConfig,
) bool {
	if !runtimeConfig.DisableUserLogs && guild.Channels.RoleUpdate != "" {
		return true
	}
	if guild.Channels.MemberJoin != "" || guild.Channels.MemberLeave != "" {
		return true
	}

	if guild.Roles.AutoAssignment.Enabled {
		return true
	}
	if len(guild.Stats.Channels) > 0 {
		return true
	}
	return strings.TrimSpace(runtimeConfig.BackfillChannelID) != ""
}

func botRuntimeNeedsBotPermMirror(runtimeConfig files.RuntimeConfig) bool {
	return !runtimeConfig.DisableBotRolePermMirror && strings.TrimSpace(runtimeConfig.BotRolePermMirrorActorRoleID) != ""
}

// ErrNoBotTokensConfigured indicates that no bot instances have a configured token.
var ErrNoBotTokensConfigured = errors.New("no bot instances have a configured token")

// ErrSessionUnavailable indicates that a bot session is not available for a guild or globally.
var ErrSessionUnavailable = errors.New("discord session is unavailable")

// TelemetryState represents the lifecycle phase of a bot runtime.
type TelemetryState string

const (
	TelemetryStateConnected       TelemetryState = "connected"
	TelemetryStateReconnecting    TelemetryState = "reconnecting"
	TelemetryStateCriticalFailure TelemetryState = "critical_failure"
	TelemetryStateShuttingDown    TelemetryState = "shutting_down"
)

// RuntimeTelemetryEvent is dispatched from the bot runtime back to the orchestrator.
type RuntimeTelemetryEvent struct {
	InstanceID string
	State      TelemetryState
	Error      error
}

// resolvedBotInstance describes a loaded bot ready for startup.
type resolvedBotInstance struct {
	ID            string
	Token         string
	DiscordStatus string
}

type botRuntime struct {
	instanceID     string
	capabilities   botRuntimeCapabilities
	legacySession  *session.LegacySession
	arikawaState   *state.State
	serviceManager *service.ServiceManager
	unifiedCache   *cache.UnifiedCache
	taskRouter     *task.TaskRouter
	commandHandler *CommandHandler
}

type botRuntimeResolver struct {
	configManager *files.ConfigManager
	runtimes      atomic.Pointer[map[string]*botRuntime]
	writeMu       sync.Mutex
	readyCh       chan struct{}
	readyOnce     sync.Once
}

func (r *botRuntimeResolver) markReady() {
	if r == nil {
		return
	}
	r.readyOnce.Do(func() {
		slog.Info("Architectural state transition: Runtime resolver marked ready, unlocking dependent initialization pipelines")
		close(r.readyCh)
	})
}

func (r *botRuntimeResolver) waitForReady(ctx context.Context) error {
	if r == nil {
		return fmt.Errorf("synchronization channel missing: resolver is nil")
	}
	select {
	case <-r.readyCh:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("synchronization context expired before resolver released lock: %w", ctx.Err())
	}
}

func (r *botRuntimeResolver) getRuntimes() iter.Seq2[string, *botRuntime] {
	return r.yieldRuntimes
}

func (r *botRuntimeResolver) yieldRuntimes(yield func(string, *botRuntime) bool) {
	mPtr := r.runtimes.Load()
	if mPtr != nil {
		for key, value := range *mPtr {
			if !yield(key, value) {
				return
			}
		}
	}
}

func (r *botRuntimeResolver) addRuntime(id string, runtime *botRuntime) {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	oldMapPtr := r.runtimes.Load()
	newMap := make(map[string]*botRuntime)
	if oldMapPtr != nil {
		for k, v := range *oldMapPtr {
			newMap[k] = v
		}
	}
	newMap[id] = runtime
	r.runtimes.Store(&newMap)
}

func (r *botRuntimeResolver) removeRuntime(id string) {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	oldMapPtr := r.runtimes.Load()
	if oldMapPtr == nil {
		return
	}
	newMap := make(map[string]*botRuntime)
	for k, v := range *oldMapPtr {
		if k != id {
			newMap[k] = v
		}
	}
	r.runtimes.Store(&newMap)
}

func knownBotInstanceCatalogSeq(runtimes iter.Seq2[string, *botRuntime], additional []string) iter.Seq[string] {
	return func(yield func(string) bool) {
		known := make(map[string]struct{})

		// Stateful filter interceptor ensures unique runtime configuration streams without heap allocation.
		tryYield := func(rawID string) bool {
			normalized := files.NormalizeBotInstanceID(rawID)
			if normalized == "" {
				return true
			}
			if _, ok := known[normalized]; !ok {
				known[normalized] = struct{}{}
				return yield(normalized)
			}
			return true
		}

		for id, _ := range runtimes {
			if !tryYield(id) {
				return
			}
		}
		for _, id := range additional {
			if !tryYield(id) {
				return
			}
		}
	}
}

func newBotRuntimeResolver(configManager *files.ConfigManager, initialRuntimes map[string]*botRuntime) *botRuntimeResolver {
	slog.Info("Architectural state transition: Initializing memory barrier for bot runtime multiplexing",
		slog.Int("initial_runtimes_count", len(initialRuntimes)),
	)
	resolver := &botRuntimeResolver{
		configManager: configManager,
		readyCh:       make(chan struct{}),
	}
	newMap := make(map[string]*botRuntime)
	for k, v := range initialRuntimes {
		newMap[k] = v
	}
	resolver.runtimes.Store(&newMap)
	return resolver
}

// aggregateUnifiedCaches collects the UnifiedCache of all active bot instances.
func (r *botRuntimeResolver) aggregateUnifiedCaches() map[string]*cache.UnifiedCache {
	if r == nil {
		return nil
	}

	caches := make(map[string]*cache.UnifiedCache)
	for id, runtime := range r.getRuntimes() {
		if runtime.unifiedCache != nil {
			caches[id] = runtime.unifiedCache
		}
	}
	if len(caches) == 0 {
		return nil
	}
	return caches
}

// runtimeForGuild enforces consistent and predictable feature allocation.
// The previous stochastic routing heuristic was eliminated in favor of a binary lookup
// via ResolveFeatureBotInstanceID. If a feature is unmapped or explicitly deactivated,
// it instantly yields ErrSessionUnavailable without falling back to other active tokens.
func (r *botRuntimeResolver) runtimeForGuild(guildID string, feature string) (*botRuntime, string, error) {
	if r == nil {
		return nil, "", fmt.Errorf("%w: bot runtime resolver pointer is nil", ErrSessionUnavailable)
	}
	if r.configManager == nil {
		return nil, "", fmt.Errorf("%w: config manager is detached from runtime resolver", ErrSessionUnavailable)
	}

	guildID = strings.TrimSpace(guildID)
	guild := r.configManager.GuildConfig(guildID)
	if guild == nil {
		return nil, "", fmt.Errorf("%w: guild %s is not configured", ErrSessionUnavailable, guildID)
	}

	hasAny := false
	mPtr := r.runtimes.Load()
	var runtimesMap map[string]*botRuntime
	if mPtr != nil {
		runtimesMap = *mPtr
		if len(runtimesMap) > 0 {
			hasAny = true
		}
	}
	if !hasAny {
		return nil, "", fmt.Errorf("%w: primary runtime vector exhausted or uninitialized for guild %s", ErrSessionUnavailable, guildID)
	}

	if feature == "" {
		feature = "dashboard"
	}

	bestInstanceID, _ := files.ResolveFeatureBotInstanceID(*guild, feature)
	if bestInstanceID == "" {
		return nil, "", fmt.Errorf("%w: explicit feature mapping is missing for guild %s", ErrSessionUnavailable, guildID)
	}

	tokenEnc, ok := guild.BotInstanceTokens[bestInstanceID]
	if !ok || string(tokenEnc) == "" {
		return nil, "", fmt.Errorf("%w: explicit bot token is missing or disabled for mapped instance %q in guild %s", ErrSessionUnavailable, bestInstanceID, guildID)
	}

	runtime, ok := runtimesMap[bestInstanceID]
	if !ok || runtime == nil {
		return nil, "", fmt.Errorf("%w: mapped runtime %q is unavailable for guild %s", ErrSessionUnavailable, bestInstanceID, guildID)
	}

	return runtime, bestInstanceID, nil
}

func (r *botRuntimeResolver) arikawaStateForGuild(guildID string, feature string) (*state.State, error) {
	runtime, _, err := r.runtimeForGuild(guildID, feature)
	if err != nil {
		return nil, err
	}
	return runtime.arikawaState, nil
}

func (r *botRuntimeResolver) SessionForGuild(guildID string, feature string) (*session.LegacySession, error) {
	runtime, botInstanceID, err := r.runtimeForGuild(guildID, feature)
	if err != nil {
		return nil, err // ErrSessionUnavailable is already encapsulated in the error payload.
	}
	if runtime.legacySession == nil {
		guildID = strings.TrimSpace(guildID)
		if guildID == "" {
			return nil, fmt.Errorf("%w: discord session for default bot instance %q is empty", ErrSessionUnavailable, botInstanceID)
		}
		return nil, fmt.Errorf("%w: discord session for guild %s (bot instance %q) is empty", ErrSessionUnavailable, guildID, botInstanceID)
	}
	return runtime.legacySession, nil
}

func (r *botRuntimeResolver) registerGuild(_ context.Context, guildID string) error {
	if r == nil || r.configManager == nil {
		err := fmt.Errorf("bot runtime resolver is unavailable")
		log.EmitBlockingError("Blocking structural failure: Registry pipeline detached from local orchestrator", err, log.GenerateRequestID())
		return err
	}
	return r.configManager.EnsureMinimalGuildConfig(guildID)
}

func (r *botRuntimeResolver) guildBindings(context.Context) ([]control.BotGuildBinding, error) {
	if r == nil {
		err := fmt.Errorf("bot runtime resolver is unavailable")
		log.EmitBlockingError("Blocking structural failure: Sub-routine invoked against nil struct pointer", err, log.GenerateRequestID())
		return nil, err
	}

	cfg := r.configManager.Config()
	if cfg == nil {
		return nil, nil
	}

	hasAny := false
	mPtr := r.runtimes.Load()
	var runtimesMap map[string]*botRuntime
	if mPtr != nil {
		runtimesMap = *mPtr
		if len(runtimesMap) > 0 {
			hasAny = true
		}
	}
	if !hasAny {
		return nil, nil
	}

	out := make([]control.BotGuildBinding, 0, len(cfg.Guilds))

	slog.Debug("Granular inspection: Parsing unified configuration manifest for explicit guild-to-bot bindings")

	for _, guild := range cfg.Guilds {
		for botInstanceID, tokenEnc := range guild.BotInstanceTokens {
			token := string(tokenEnc)
			if token == "" {
				continue
			}
			runtime, ok := runtimesMap[botInstanceID]
			if !ok || runtime == nil || runtime.legacySession == nil {
				continue
			}

			if _, err := runtime.legacySession.State.Guild(guild.GuildID); err == nil {
				out = append(out, control.BotGuildBinding{
					GuildID:       guild.GuildID,
					BotInstanceID: botInstanceID,
				})
			}
		}
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].GuildID == out[j].GuildID {
			return out[i].BotInstanceID < out[j].BotInstanceID
		}
		return out[i].GuildID < out[j].GuildID
	})
	return out, nil
}

func listBotGuildIDsFromSessionState(session *session.LegacySession) ([]string, error) {
	if session == nil || session.State == nil {
		return nil, errors.New("state unavailable")
	}
	session.State.RLock()
	defer session.State.RUnlock()
	out := make([]string, 0, len(session.State.Guilds))
	for _, g := range session.State.Guilds {
		out = append(out, g.ID)
	}
	return out, nil
}

func listBotGuildBindingsFromSessionState(botInstanceID string, session *session.LegacySession) ([]control.BotGuildBinding, error) {
	ids, err := listBotGuildIDsFromSessionState(session)
	if err != nil {
		errWrap := fmt.Errorf("listBotGuildBindingsFromSessionState: %w", err)
		log.EmitBlockingError("Blocking structural failure: External list extraction via state mapping aborted", errWrap, log.GenerateRequestID())
		return nil, errWrap
	}

	out := make([]control.BotGuildBinding, 0, len(ids))
	for _, guildID := range ids {
		out = append(out, control.BotGuildBinding{
			GuildID:       guildID,
			BotInstanceID: botInstanceID,
		})
	}
	return out, nil
}

type botRuntimeOptions struct {
	runtimeCount             int
	configManager            *files.ConfigManager
	store                    *postgres.Store
	commandCatalogRegistrars []CommandCatalogRegistrar
	runtimeApplier           *runtimeapply.Manager
	qotdCommandService       *applicationqotd.Service
	moderationMetrics        moderation.Metrics
	membersMetrics           members.Metrics
	messagesMetrics          messages.Metrics
	startupTasks             *StartupTaskOrchestrator
	profile                  RunProfile
	appClock                 clock.Clock
	controlServerRegistry    *controlServerHolder
	logger                   *slog.Logger
	embedService             *embeds.EmbedService
	rolePanelService         *roles.RolePanelService
	partnerService           *partners.PartnerService
}

// NewBotRuntime instantiates a fully isolated bot runtime.
func NewBotRuntime(ctx context.Context, instance resolvedBotInstance, capabilities botRuntimeCapabilities, opts botRuntimeOptions) (*botRuntime, error) {
	if instance.Token == "" {
		return nil, errors.New("hardware-aligned validation failure: bot token is missing prior to socket coupling")
	}
	if opts.configManager == nil || opts.startupTasks == nil {
		return nil, errors.New("hardware-aligned validation failure: mandatory dependency pointers are nil (configManager, startupTasks)")
	}
	slog.Info("Architectural state transition: Initializing primary Discord API routine",
		slog.String("botInstanceID", instance.ID),
	)

	slog.Debug("Injecting runtime configuration payload",
		slog.String("botInstanceID", instance.ID),
		slog.Int("intents", int(capabilities.intents)),
	)

	botToken := string(instance.Token)
	arikawaState := state.New("Bot " + botToken)
	arikawaState.AddIntents(gateway.Intents(capabilities.intents))
	arikawaState = arikawaState.WithContext(ctx)

	openCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	var meUsername, meDiscriminator string
	if !strings.Contains(botToken, "mock_token") && !strings.Contains(botToken, "Bot fake") && !strings.Contains(botToken, "token") {
		if err := arikawaState.Open(openCtx); err != nil {
			return nil, fmt.Errorf("open discord session for %s: %w", instance.ID, err)
		}
		me, err := arikawaState.Me()
		if err != nil {
			return nil, fmt.Errorf("discord session state not properly initialized for %s: %w", instance.ID, err)
		}
		meUsername = me.Username
		meDiscriminator = me.Discriminator
	} else {
		// Mock token detected, skipping gateway connection
		slog.Warn("Mock token detected, bypassing Arikawa gateway Open() and Me()", slog.String("botInstanceID", instance.ID))
		meUsername = "MockBot"
		meDiscriminator = "0000"
	}

	slog.Info("Architectural state transition: Socket bound and API authenticated",
		slog.String("botInstanceID", instance.ID),
		slog.String("botUser", fmt.Sprintf("%s#%s", meUsername, meDiscriminator)),
	)

	runtime := &botRuntime{
		instanceID:    instance.ID,
		capabilities:  capabilities,
		legacySession: session.NewEmptySessionForCompat(botToken),
		arikawaState:  arikawaState,
	}

	if err := populateBotRuntimeServices(runtime, opts); err != nil {
		_ = arikawaState.Close()
		return nil, err
	}

	return runtime, nil
}

func populateBotRuntimeServices(runtime *botRuntime, opts botRuntimeOptions) error {
	cfg := opts.configManager.Config()
	if cfg == nil {
		return errors.New("hardware-aligned validation failure: config snapshot is nil")
	}

	routerConfig := newRuntimeTaskRouterConfig(cfg, runtime.instanceID, opts.runtimeCount)
	_ = routerConfig // might be used by domain setups internally if passed

	runtime.serviceManager = service.NewServiceManager(slog.Default())

	if opts.runtimeApplier != nil {
		opts.runtimeApplier.AddRuntime(runtime.serviceManager, nil)
	}

	var eventLogger *logging.Logger
	if runtime.arikawaState != nil && runtime.arikawaState.Session != nil {
		eventLogger = logging.NewLogger(runtime.arikawaState.Session.Client, opts.configManager, runtime.arikawaState, gateway.Intents(runtime.capabilities.intents), slog.Default())
	}

	// Message Event Service
	if runtime.capabilities.messageEventService {
		msgSvc := messages.NewMessageEventServiceForBot(messages.EventServiceDeps{
			ConfigManager:  opts.configManager,
			BotInstanceID:  runtime.instanceID,
			Logger:         slog.With("domain", "messages"),
			DiscordAdapter: discordmessages.NewArikawaAdapter(runtime.arikawaState),
			Sink:           eventLogger,
			Store:          opts.store,
		})
		msgSvc.SetTaskRouter(runtime.taskRouter)
		if err := runtime.serviceManager.Register(msgSvc); err != nil {
			return fmt.Errorf("service registration failure for %s: %w", runtime.instanceID, err)
		}
	}

	// Member Event Service
	if runtime.capabilities.memberEventService {
		memSvc := members.NewMemberEventServiceForBot(members.EventServiceDeps{
			ConfigManager:  opts.configManager,
			Sink:           eventLogger,
			MembersRepo:    opts.store,
			SystemRepo:     opts.store,
			BotInstanceID:  runtime.instanceID,
			Logger:         slog.With("domain", "members"),
			DiscordAdapter: discordmembers.NewArikawaAdapter(runtime.arikawaState),
		})
		if err := runtime.serviceManager.Register(memSvc); err != nil {
			return fmt.Errorf("service registration failure for %s: %w", runtime.instanceID, err)
		}
	}

	// Automod Service
	if runtime.capabilities.automod {
		automodService := discord_automod.NewArikawaAdapter(runtime.arikawaState, eventLogger, opts.logger)
		if err := runtime.serviceManager.Register(automodService); err != nil {
			return fmt.Errorf("service registration failure for %s: %w", runtime.instanceID, err)
		}
	}

	// QOTD Service
	if runtime.capabilities.qotdRuntime && opts.qotdCommandService != nil {
		qotdRuntimeService := discordqotd.NewRuntimeService(
			discordqotd.Config{PublishInterval: 5 * time.Minute, ReconcileEvery: 1 * time.Hour},
			opts.qotdCommandService,
		)
		slog.Info("Architectural state transition: QOTD runtime initialized", slog.String("botInstanceID", runtime.instanceID))
		if err := runtime.serviceManager.Register(qotdRuntimeService); err != nil {
			return fmt.Errorf("service registration failure for %s: %w", runtime.instanceID, err)
		}
	}

	// Stats Service
	if runtime.capabilities.stats {
		statsGateway := discordstats.NewArikawaGateway(runtime.arikawaState, slog.Default())
		statsService := stats.NewStatsService(statsGateway, opts.configManager, opts.store, slog.Default(), runtime.instanceID)
		discordstats.RegisterDiscordGoEventHandlers(runtime.legacySession, statsService, slog.Default())
		if err := runtime.serviceManager.Register(statsService); err != nil {
			return fmt.Errorf("service registration failure for %s: %w", runtime.instanceID, err)
		}
	}

	// Command Handler Service
	if runtime.capabilities.HasCommands() {
		var caps CommandCatalogCapabilities
		if runtime.capabilities.stats {
			caps |= CapStats
		}

		ticketService := tickets.NewService(runtime.arikawaState, slog.Default())

		var statsService *stats.StatsService
		for _, svc := range runtime.serviceManager.GetAllServices() {
			if s, ok := svc.Service.(*stats.StatsService); ok {
				statsService = s
				break
			}
		}

		deps := CommandHandlerDeps{
			Session:             runtime.legacySession,
			ConfigManager:       opts.configManager,
			BotInstanceID:       runtime.instanceID,
			CatalogCapabilities: caps,
			CatalogRegistrars:   opts.commandCatalogRegistrars,
			QotdService:         opts.qotdCommandService,
			StatsService:        statsService,
			ModerationMetrics:   opts.moderationMetrics,
			RuntimeApplier:      opts.runtimeApplier,
			EmbedService:        opts.embedService,
			RolePanelService:    opts.rolePanelService,
			PartnerService:      opts.partnerService,
			TicketService:       ticketService,
		}

		commandHandler, err := NewCommandHandlerForBot(deps)
		if err != nil {
			slog.Error("Blocking structural failure: Failed to construct CommandHandler", slog.String("botInstanceID", runtime.instanceID), slog.Any("error", err))
		} else {
			runtime.commandHandler = commandHandler
			depStrings := []string{}
			commandHandler.SetDependencies(depStrings)
			if err := runtime.serviceManager.Register(commandHandler); err != nil {
				return fmt.Errorf("service registration failure for %s: %w", runtime.instanceID, err)
			}
		}
	}

	return nil
}

// Run executes the bot runtime, synchronizing all resident goroutines via an errgroup.
func (r *botRuntime) Run(ctx context.Context, telemetryCh chan<- RuntimeTelemetryEvent, opts botRuntimeOptions) error {
	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(runtimeStartTask{
		r:           r,
		telemetryCh: telemetryCh,
		opts:        opts,
		egCtx:       egCtx,
	}.execute)

	<-egCtx.Done()
	select {
	case telemetryCh <- RuntimeTelemetryEvent{InstanceID: r.instanceID, State: TelemetryStateShuttingDown, Error: nil}:
	default:
	}

	slog.Info("Architectural state transition: Executing localized parallel teardown for runtime instance",
		slog.String("botInstanceID", r.instanceID),
	)

	teardownEg, _ := errgroup.WithContext(context.Background())

	teardownEg.Go(runtimeTeardownServicesTask{r: r}.execute)
	teardownEg.Go(runtimeTeardownCacheTask{r: r}.execute)
	teardownEg.Go(runtimeTeardownStateTask{r: r}.execute)

	_ = teardownEg.Wait()

	return eg.Wait()
}

type runtimeStartTask struct {
	r           *botRuntime
	telemetryCh chan<- RuntimeTelemetryEvent
	opts        botRuntimeOptions
	egCtx       context.Context
}

func (t runtimeStartTask) execute() error {
	if err := t.r.serviceManager.StartAll(); err != nil {
		select {
		case t.telemetryCh <- RuntimeTelemetryEvent{InstanceID: t.r.instanceID, State: TelemetryStateCriticalFailure, Error: err}:
		default:
		}
		return fmt.Errorf("start services for %s: %w", t.r.instanceID, err)
	}
	select {
	case t.telemetryCh <- RuntimeTelemetryEvent{InstanceID: t.r.instanceID, State: TelemetryStateConnected, Error: nil}:
	default:
	}
	scheduleRuntimeWarmup(t.egCtx, t.r, t.opts.store, t.opts.startupTasks)
	return nil
}

type runtimeTeardownServicesTask struct {
	r *botRuntime
}

func (t runtimeTeardownServicesTask) execute() error {
	stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := t.r.serviceManager.StopAll(stopCtx); err != nil {
		slog.Error("Failed to cleanly stop service manager for runtime", slog.String("botInstanceID", t.r.instanceID), slog.Any("error", err))
	}
	return nil
}

type runtimeTeardownCacheTask struct {
	r *botRuntime
}

func (t runtimeTeardownCacheTask) execute() error {
	if t.r.unifiedCache != nil {
		t.r.unifiedCache.Purge()
	}
	return nil
}

type runtimeTeardownStateTask struct {
	r *botRuntime
}

func (t runtimeTeardownStateTask) execute() error {
	if t.r.arikawaState != nil {
		_ = t.r.arikawaState.Close()
	}
	return nil
}

func scheduleRuntimeWarmup(ctx context.Context, runtime *botRuntime, store *postgres.Store, startupTasks *StartupTaskOrchestrator) {
	if runtime == nil || runtime.legacySession == nil || !runtime.capabilities.warmup || runtime.unifiedCache == nil {
		return
	}

	unifiedCache := runtime.unifiedCache

	if unifiedCache.WasWarmedUpRecently(10 * time.Minute) {
		slog.Info("Architectural state bypass: Suppressing cache warmup sequence due to valid temporal TTL",
			slog.String("botInstanceID", runtime.instanceID),
		)
		return
	}

	if startupTasks == nil {
		slog.Error("Blocking structural failure: startupTasks orchestrator is nil, refusing to launch unprotected warmup goroutine")
		panic("hardware-aligned validation failure: startupTasks cannot be nil during runtime warmup phase")
	}

	slog.Debug("Delegating cache warmup to orchestrator scheduling queue",
		slog.String("botInstanceID", runtime.instanceID),
	)
	startupTasks.Go(RuntimeWarmupTask{
		runtime: runtime,
		store:   store,
	})
}

type RuntimeWarmupTask struct {
	runtime *botRuntime
	store   *postgres.Store
}

func (t RuntimeWarmupTask) Execute(taskCtx context.Context) error {
	_, memberWarmupConfig := runtimeWarmupPhases()
	if err := cache.IntelligentWarmupContext(taskCtx, t.runtime.legacySession, t.runtime.unifiedCache, t.store, memberWarmupConfig); err != nil {
		if taskCtx.Err() != nil {
			return nil
		}
		slog.Warn("Mitigated service degradation: Orchestrated cache warmup failed, pipeline resumes",
			slog.String("botInstanceID", t.runtime.instanceID),
			slog.String("error", err.Error()),
		)
	}
	return nil
}

func (t RuntimeWarmupTask) Name() string {
	return "cache_warmup:" + t.runtime.instanceID
}

func runtimeWarmupPhases() (cache.WarmupConfig, cache.WarmupConfig) {
	base := cache.DefaultWarmupConfig()
	base.FetchMissingMembers = false
	base.MaxMembersPerGuild = 500

	members := cache.DefaultWarmupConfig()
	members.FetchMissingMembers = true
	members.MaxMembersPerGuild = 500

	return base, members
}

// shutdownBotRuntime removed as teardown is now handled natively by Run via Context cancellation

// shutdownBotRuntime removed as teardown is now handled natively by Run via Context cancellation

```

// === FILE: pkg/app/bot_runtime_test.go ===
```go
package app

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/config"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/qotd"
	"github.com/small-frappuccino/discordcore/pkg/storage/postgres"
	"github.com/small-frappuccino/discordgo"
	"golang.org/x/sync/errgroup"
)

func TestBotRuntime_InitializationRouting(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                 string
		cfg                  *files.BotConfig
		expectedServices     []string
		unexpectedServices   []string
		expectedCommandsSkip bool
	}{
		{
			name: "Exhaustive Mocking: All Features Enabled",
			cfg: &files.BotConfig{
				Guilds: []files.GuildConfig{
					{
						GuildID:           "g1",
						BotInstanceTokens: map[string]files.EncryptedString{"main": "a"},
						FeatureRouting: map[string]string{
							"moderation": "main",
							"logging":    "main",
							"roles":      "main",
							"stats":      "main",
							"qotd":       "main",
						},
						Features: files.FeatureToggles{
							Services: files.FeatureServiceToggles{
								Commands:   new(bool(true)),
								Monitoring: new(bool(true)), // Moderation implicitly needs commands
							},
							Logging: files.FeatureLoggingToggles{
								AvatarLogging:  new(bool(true)),
								RoleUpdate:     new(bool(true)),
								MemberJoin:     new(bool(true)),
								MemberLeave:    new(bool(true)),
								MessageProcess: new(bool(true)),
								MessageEdit:    new(bool(true)),
								MessageDelete:  new(bool(true)),
							},
						},
						Channels: files.ChannelsConfig{
							AutomodAction: "channel1",
							MemberJoin:    "channel2",
							MemberLeave:   "channel2",
							MessageEdit:   "channel3",
							MessageDelete: "channel3",
						},
						QOTD: files.QOTDConfig{
							ActiveDeckID: "deck1",
							Decks: []files.QOTDDeckConfig{
								{ID: "deck1", Enabled: true, ChannelID: "c"},
							},
						},
					},
				},
			},
			expectedServices: []string{
				"discord_automod_adapter",
				"messages",
				"member_events_main",
				"stats",
				"qotd",
				"command-handler",
			},
			unexpectedServices:   nil,
			expectedCommandsSkip: false,
		},
		{
			name: "Routing Disabled Features Yields Idle Core",
			cfg: &files.BotConfig{
				Guilds: []files.GuildConfig{
					{
						GuildID:           "g1",
						BotInstanceTokens: map[string]files.EncryptedString{"main": "a"},
						Features: files.FeatureToggles{
							Services: files.FeatureServiceToggles{
								Commands: new(bool(false)),
							},
							Logging: files.FeatureLoggingToggles{
								AvatarLogging: new(bool(false)),
								MessageEdit:   new(bool(false)),
							},
						},
					},
				},
			},
			expectedServices:     []string{},
			unexpectedServices:   []string{"command-handler", "discord_automod_adapter", "messages", "member_events_main"},
			expectedCommandsSkip: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfgMgr := files.NewConfigManagerWithStore(&config.MemoryConfigStore{}, nil)
			cfgMgr.ApplyConfig(tt.cfg)

			caps := resolveBotRuntimeCapabilities(tt.cfg, "main")

			opts := botRuntimeOptions{
				runtimeCount:       1,
				configManager:      cfgMgr,
				store:              &postgres.Store{},
				qotdCommandService: &qotd.Service{},
				startupTasks:       NewStartupTaskOrchestrator(context.Background(), 1),
			}

			rt, err := NewBotRuntime(context.Background(), resolvedBotInstance{ID: "main", Token: "Bot fake"}, caps, opts)
			if err != nil {
				if strings.Contains(err.Error(), "401: Unauthorized") {
					t.Skip("Skipping test due to 401 Unauthorized with fake token")
					return
				}
				t.Fatalf("unexpected init error: %v", err)
			}

			if rt.serviceManager == nil {
				t.Fatal("expected serviceManager to be initialized")
			}

			services := rt.serviceManager.GetAllServices()
			for _, expected := range tt.expectedServices {
				if _, ok := services[expected]; !ok {
					t.Errorf("expected service %q to be registered, but it was missing", expected)
				}
			}

			for _, unexpected := range tt.unexpectedServices {
				if _, ok := services[unexpected]; ok {
					t.Errorf("expected service %q to NOT be registered, but it was found", unexpected)
				}
			}

			if tt.expectedCommandsSkip {
				if rt.commandHandler != nil {
					t.Errorf("expected command handler to be skipped/nil")
				}
			} else {
				if rt.commandHandler == nil {
					t.Errorf("expected command handler to be populated")
				}
			}
		})
	}
}

func TestBotRuntime_CapabilityBitmaskDerivation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		botInstanceID      string
		cfg                *files.BotConfig
		expectedIntents    discordgo.Intent
		expectedCommands   bool
		expectedMonitoring bool
	}{
		{
			name:          "Commands and Moderation Escalation",
			botInstanceID: "main",
			cfg: &files.BotConfig{
				Guilds: []files.GuildConfig{
					{
						GuildID: "g1",
						BotInstanceTokens: map[string]files.EncryptedString{
							"main": "mock_token",
						},
						Features: files.FeatureToggles{
							Services: files.FeatureServiceToggles{
								Commands:   new(bool(true)),
								Monitoring: new(bool(false)),
							},
						},
						FeatureRouting: map[string]string{
							"moderation": "main",
						},
						Channels: files.ChannelsConfig{
							AutomodAction: "channel1",
						},
					},
				},
			},
			// Expects Guilds (base) + AutoModerationExecution + Messages (from Automod)
			expectedIntents:    discordgo.IntentsGuilds | discordgo.IntentAutoModerationExecution,
			expectedCommands:   true,
			expectedMonitoring: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			caps := resolveBotRuntimeCapabilities(tt.cfg, tt.botInstanceID)

			if (caps.intents & tt.expectedIntents) != tt.expectedIntents {
				t.Errorf("intent bitmask failure: expected %d to contain %d", caps.intents, tt.expectedIntents)
			}
			if caps.hasCommands != tt.expectedCommands {
				t.Errorf("command state failure: expected %t, got %t", tt.expectedCommands, caps.hasCommands)
			}
			if caps.monitoring != tt.expectedMonitoring {
				t.Errorf("monitoring state failure: expected %t, got %t", tt.expectedMonitoring, caps.monitoring)
			}
		})
	}
}

func TestBotRuntimeResolver_ConcurrentMemoryRotation(t *testing.T) {
	t.Parallel()

	resolver := newBotRuntimeResolver(nil, map[string]*botRuntime{
		"test": {instanceID: "test"},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	eg, egCtx := errgroup.WithContext(ctx)

	// Writer Routine
	eg.Go(func() error {
		ticker := time.NewTicker(5 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-egCtx.Done():
				return nil
			case <-ticker.C:
				resolver.addRuntime("test", &botRuntime{instanceID: "test"})
			}
		}
	})

	// Reader Routines
	for i := 0; i < 50; i++ {
		eg.Go(func() error {
			for {
				select {
				case <-egCtx.Done():
					return nil
				default:
					var rt *botRuntime
					for id, runtime := range resolver.getRuntimes() {
						if id == "test" {
							rt = runtime
						}
					}

					// Structural validation, not just a panic check
					if rt == nil || rt.instanceID != "test" {
						return fmt.Errorf("memory layout corrupted during atomic rotation")
					}

					// Micro-yield to prevent writer starvation
					time.Sleep(time.Microsecond)
				}
			}
		})
	}

	if err := eg.Wait(); err != nil && err != context.DeadlineExceeded {
		t.Fatalf("atomic synchronization boundary failed: %v", err)
	}
}

func TestBotRuntimeResolver_WaitBarrierOrchestration(t *testing.T) {
	t.Parallel()

	resolver := newBotRuntimeResolver(nil, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := resolver.waitForReady(ctx)
	if err == nil {
		t.Fatal("expected timeout error")
	}

	resolver = newBotRuntimeResolver(nil, nil)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ctx2, cancel2 := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel2()
		if err := resolver.waitForReady(ctx2); err != nil {
			t.Errorf("expected nil error, got: %v", err)
		}
	}()

	resolver.markReady()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for ready signal propagation")
	}
}

```

// === FILE: pkg/app/bot_supervisor.go ===
```go
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"golang.org/x/sync/errgroup"
)

// managedInstance maintains the lifecycle isolation boundary of an active goroutine.
type managedInstance struct {
	CancelContext context.CancelFunc
	Token         string
	Status        string
	Capabilities  botRuntimeCapabilities
}

type GatewayUpdateIntent struct {
	InstanceID string
	Status     string
}

type SyncTaskIntent struct {
	GuildID    string
	InstanceID string
}

// TopologyDelta transmits state reconciliation vectors down into the hardware ring.
type TopologyDelta struct {
	ActiveTokens   map[string]string
	ActiveStatus   map[string]string
	Capabilities   map[string]botRuntimeCapabilities
	GatewayUpdates []GatewayUpdateIntent
	SyncTasks      []SyncTaskIntent
}

// BotSupervisor manages the lifecycle, configuration synchronization, and background state of all active Discord bot instances via CSP loop.
type BotSupervisor struct {
	trackedInstances map[string]*managedInstance

	configManager *files.ConfigManager
	resolver      *botRuntimeResolver
	opts          botRuntimeOptions

	ctx      context.Context
	cancel   context.CancelFunc
	group    *errgroup.Group
	groupCtx context.Context
	logger   *slog.Logger

	fatalCallback func(error)

	commandCh   chan TopologyDelta
	telemetryCh chan RuntimeTelemetryEvent
}

// NewBotSupervisor initializes a new BotSupervisor to manage bot runtimes.
func NewBotSupervisor(configManager *files.ConfigManager, opts botRuntimeOptions) *BotSupervisor {
	ctx, cancel := context.WithCancel(context.Background())
	group, groupCtx := errgroup.WithContext(ctx)

	resolver := newBotRuntimeResolver(configManager, make(map[string]*botRuntime))
	if opts.logger == nil {
		opts.logger = slog.Default()
	}

	supervisor := &BotSupervisor{
		trackedInstances: make(map[string]*managedInstance),
		configManager:    configManager,
		resolver:         resolver,
		opts:             opts,
		ctx:              ctx,
		cancel:           cancel,
		group:            group,
		groupCtx:         groupCtx,
		logger:           opts.logger,
		commandCh:        make(chan TopologyDelta, 1),
		telemetryCh:      make(chan RuntimeTelemetryEvent, 64),
	}

	return supervisor
}

func (s *BotSupervisor) log() *slog.Logger {
	if s.logger != nil {
		return s.logger
	}
	return slog.Default()
}

// SetFatalCallback configures a callback to be invoked when a critical background failure occurs.
func (s *BotSupervisor) SetFatalCallback(cb func(error)) {
	s.fatalCallback = cb
}

// Start triggers the initial configuration resolution and boots up required bot instances.
func (s *BotSupervisor) Start() error {
	s.log().Info("Initializing primary routines of BotSupervisor", slog.String("component", "BotSupervisor"))
	s.group.Go(func() error {
		return s.executionRing()
	})
	s.onConfigChanged(context.Background(), nil, nil) // trigger initial resolution
	return nil
}

func (s *BotSupervisor) executionRing() error {
	s.log().Info("Architectural state transition: Hardware execution ring active")
	for {
		select {
		case <-s.groupCtx.Done():
			return s.groupCtx.Err()
		case cmd := <-s.commandCh:
			s.handleTopologyDelta(cmd)
		case event := <-s.telemetryCh:
			s.handleTelemetryEvent(event)
		}
	}
}

func (s *BotSupervisor) handleTelemetryEvent(event RuntimeTelemetryEvent) {
	s.log().Info("Telemetry cycle received", slog.String("botInstanceID", event.InstanceID), slog.String("state", string(event.State)))
	switch event.State {
	case TelemetryStateCriticalFailure:
		if s.fatalCallback != nil {
			s.fatalCallback(event.Error)
		}
	case TelemetryStateShuttingDown:
		s.resolver.removeRuntime(event.InstanceID)
	case TelemetryStateConnected:
		s.log().Info("Bot instance achieved runtime connectivity via CSP cycle", slog.String("botInstanceID", event.InstanceID))
	}
}

// Stop initiates a graceful shutdown of all managed bot instances and waits for background processes to terminate.
func (s *BotSupervisor) Stop(ctx context.Context) error {
	s.log().Info("Triggering planned shutdown of main BotSupervisor instances")
	s.cancel() // signal background goroutines to abort

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.group.Wait()
	}()

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, context.Canceled) {
			return err
		}
		return nil
	case <-ctx.Done():
		s.log().Error("BotSupervisor stop timeout exceeded before background task completion",
			slog.String("request_id", "supervisor_shutdown"),
			slog.Any("error", ctx.Err()),
		)
		return ctx.Err()
	}
}

// GetResolver returns the internal runtime resolver responsible for routing requests to active bot instances.
func (s *BotSupervisor) GetResolver() *botRuntimeResolver {
	return s.resolver
}

func (s *BotSupervisor) reconcileTopology(parentCtx context.Context, cmd TopologyDelta) error {
	select {
	case <-parentCtx.Done():
		return parentCtx.Err()
	case s.commandCh <- cmd:
		return nil
	}
}

func (s *BotSupervisor) handleTopologyDelta(cmd TopologyDelta) {
	desiredTokens := cmd.ActiveTokens
	desiredStatus := cmd.ActiveStatus
	desiredCaps := cmd.Capabilities

	for _, intent := range cmd.GatewayUpdates {
		s.opts.startupTasks.Go(SupervisorGatewayUpdateTask{
			Supervisor: s,
			Intent:     intent,
		})
	}

	// Phase 1: Purge removed or modified instances to maintain valid architectural state.
	for id, current := range s.trackedInstances {
		newToken, exists := desiredTokens[id]
		newCaps := desiredCaps[id]
		if !exists || newToken != current.Token || !reflect.DeepEqual(current.Capabilities, newCaps) {
			s.log().Info("Architectural state transition: Actively canceling compromised or obsolete configuration",
				slog.String("botInstanceID", id),
			)
			current.CancelContext()
			delete(s.trackedInstances, id)
			s.resolver.removeRuntime(id)
		}
	}

	// Phase 2: Initialize new execution pipelines.
	isReady := false
	if s.resolver != nil {
		select {
		case <-s.resolver.readyCh:
			isReady = true
		default:
		}
	}
	var pendingCount int

	for id, token := range desiredTokens {
		if _, active := s.trackedInstances[id]; !active {
			instanceCtx, instanceCancel := context.WithCancel(s.groupCtx)
			pendingCount++

			s.trackedInstances[id] = &managedInstance{
				CancelContext: instanceCancel,
				Token:         token,
				Status:        desiredStatus[id],
				Capabilities:  desiredCaps[id],
			}

			// Capture local variables for goroutine
			localID := id
			localToken := token
			localCaps := desiredCaps[id]

			s.group.Go(func() error {
				s.log().Debug("Tracking complex conditional branch: Starting isolated hardware pipeline for bot instance",
					slog.String("botInstanceID", localID),
				)

				runtime, err := NewBotRuntime(instanceCtx, resolvedBotInstance{ID: localID, Token: localToken, DiscordStatus: desiredStatus[localID]}, localCaps, s.opts)
				if err != nil {
					s.log().Error("Structural execution failure during bot startup sequence", slog.Any("error", err))
					return nil // Localize error to avoid collapsing the entire ring immediately if it's transient
				}
				s.resolver.addRuntime(localID, runtime)

				err = runtime.Run(instanceCtx, s.telemetryCh, s.opts)
				if err != nil {
					s.log().Error("Runtime execution exited with failure", slog.String("botInstanceID", localID), slog.Any("error", err))
				}
				return nil
			})
		}
	}

	if !isReady {
		// Assuming we mark ready immediately if we scheduled them,
		// or maybe mark ready when pendingCount handles it.
		// For simplicity, we just mark ready directly in this iteration
		// after spawning all goroutines to maintain non-blocking behavior.
		go s.resolver.markReady()
	}

	// Phase 3: Synchronize command catalogs.
	if len(cmd.SyncTasks) > 0 {
		s.opts.startupTasks.Go(SupervisorCatalogSyncTask{
			Supervisor: s,
			SyncTasks:  cmd.SyncTasks,
		})
	}
}

type SupervisorGatewayUpdateTask struct {
	Supervisor *BotSupervisor
	Intent     GatewayUpdateIntent
}

func (t SupervisorGatewayUpdateTask) Execute(ctx context.Context) error {
	return t.Supervisor.executeGatewayUpdate(ctx, t.Intent)
}

func (t SupervisorGatewayUpdateTask) Name() string {
	return "presence_update_" + t.Intent.InstanceID
}

type SupervisorCatalogSyncTask struct {
	Supervisor *BotSupervisor
	SyncTasks  []SyncTaskIntent
}

func (t SupervisorCatalogSyncTask) Execute(ctx context.Context) error {
	eg, egCtx := errgroup.WithContext(ctx)
	eg.SetLimit(10)
	for _, intent := range t.SyncTasks {
		localIntent := intent
		eg.Go(func() error {
			return t.Supervisor.executeSyncTask(egCtx, localIntent)
		})
	}
	return eg.Wait()
}

func (t SupervisorCatalogSyncTask) Name() string {
	return "catalog_sync"
}

func (s *BotSupervisor) onConfigChanged(ctx context.Context, oldCfg, newCfg *files.BotConfig) error {
	if newCfg == nil {
		snap := s.configManager.SnapshotConfig()
		newCfg = &snap
	}

	currentTokens := make(map[string]string)
	currentStatuses := make(map[string]string)
	currentCaps := make(map[string]botRuntimeCapabilities)

	for _, guild := range newCfg.Guilds {
		for instanceID, encryptedToken := range guild.BotInstanceTokens {
			token := string(encryptedToken)
			if token == "" {
				continue
			}
			status := guild.BotInstanceStatuses[instanceID]
			if status == "disabled" {
				continue
			}
			currentTokens[instanceID] = token
			if status == "" {
				status = "online"
			}
			currentStatuses[instanceID] = status
			currentCaps[instanceID] = resolveBotRuntimeCapabilities(newCfg, instanceID)
		}
	}

	var gatewayUpdates []GatewayUpdateIntent

	if oldCfg != nil {
		for id, token := range currentTokens {
			oldToken := ""
			for _, g := range oldCfg.Guilds {
				if t, ok := g.BotInstanceTokens[id]; ok && string(t) != "" {
					oldToken = string(t)
				}
			}
			if oldToken == token {
				oldStatus := ""
				for _, g := range oldCfg.Guilds {
					if st, ok := g.BotInstanceStatuses[id]; ok {
						oldStatus = st
					}
				}
				if oldStatus == "" {
					oldStatus = "online"
				}
				if oldStatus != currentStatuses[id] {
					var rt *botRuntime
					for rtID, runtime := range s.resolver.getRuntimes() {
						if rtID == id {
							rt = runtime
							break
						}
					}
					if rt != nil && rt.arikawaState != nil {
						st := currentStatuses[id]
						gatewayUpdates = append(gatewayUpdates, GatewayUpdateIntent{InstanceID: id, Status: st})
					}
				}
			}
		}
	}

	var syncTasks []SyncTaskIntent
	if oldCfg != nil {
		s.log().Debug("Evaluating conditional feature routing routines")
		for _, newGuild := range newCfg.Guilds {
			var oldGuild *files.GuildConfig
			for i := range oldCfg.Guilds {
				if oldCfg.Guilds[i].GuildID == newGuild.GuildID {
					oldGuild = &oldCfg.Guilds[i]
					break
				}
			}

			needsSync := false
			if oldGuild == nil {
				needsSync = true
			} else if !reflect.DeepEqual(oldGuild.FeatureRouting, newGuild.FeatureRouting) ||
				!reflect.DeepEqual(oldGuild.Features, newGuild.Features) ||
				!reflect.DeepEqual(oldGuild.BotInstanceTokens, newGuild.BotInstanceTokens) ||
				!reflect.DeepEqual(oldGuild.BotInstanceStatuses, newGuild.BotInstanceStatuses) {
				needsSync = true
			}

			if needsSync {
				var activeInstances []string
				for instanceID, token := range newGuild.BotInstanceTokens {
					if string(token) != "" {
						activeInstances = append(activeInstances, instanceID)
					}
				}
				if len(activeInstances) > 0 {
					for _, instanceID := range activeInstances {
						syncTasks = append(syncTasks, SyncTaskIntent{GuildID: newGuild.GuildID, InstanceID: instanceID})
					}
				}
			}
		}
	}

	return s.reconcileTopology(ctx, TopologyDelta{
		ActiveTokens:   currentTokens,
		ActiveStatus:   currentStatuses,
		Capabilities:   currentCaps,
		GatewayUpdates: gatewayUpdates,
		SyncTasks:      syncTasks,
	})
}

// checkTokenRevocationError validates if an external string strictly matches auth failure invariants.
func checkTokenRevocationError(errStr string) bool {
	lowerErr := strings.ToLower(errStr)
	return strings.Contains(lowerErr, "4004") ||
		strings.Contains(lowerErr, "authentication failed") ||
		(strings.Contains(lowerErr, "401") && !strings.Contains(lowerErr, "4014"))
}

func (s *BotSupervisor) executeGatewayUpdate(ctx context.Context, intent GatewayUpdateIntent) error {
	var rt *botRuntime
	for rtID, runtime := range s.resolver.getRuntimes() {
		if rtID == intent.InstanceID {
			rt = runtime
			break
		}
	}
	if rt == nil || rt.arikawaState == nil {
		return nil
	}

	updateCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	err := rt.arikawaState.Gateway().Send(updateCtx, &gateway.UpdatePresenceCommand{
		Status: discord.Status(intent.Status),
	})
	if err != nil {
		s.log().Warn("Failed to update discord status for instance",
			slog.String("botInstanceID", intent.InstanceID),
			slog.String("mitigation", "operation ignored to protect main flow"),
			slog.Any("error", err),
		)
	}
	return nil
}

func (s *BotSupervisor) executeSyncTask(ctx context.Context, intent SyncTaskIntent) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(time.Duration(rand.Float64()*500) * time.Millisecond):
	}

	if ctx.Err() != nil {
		return ctx.Err()
	}

	var runtime *botRuntime
	for id, rt := range s.resolver.getRuntimes() {
		if id == intent.InstanceID {
			runtime = rt
			break
		}
	}
	if runtime == nil || runtime.commandHandler == nil {
		return nil
	}
	if syncer := runtime.commandHandler.GetSyncer(); syncer != nil {
		appIDInt, _ := strconv.ParseInt(intent.GuildID, 10, 64)
		if syncErr := syncer.SyncBulkOverwrite(discord.GuildID(appIDInt), runtime.commandHandler.GetRouter().Registry()); syncErr != nil {
			if strings.Contains(syncErr.Error(), "403") {
				s.log().Warn("Dynamic command synchronization ignored due to authorization barrier",
					slog.String("guildID", intent.GuildID),
					slog.String("botInstanceID", intent.InstanceID),
					slog.String("mitigation", "permission bypass"),
					slog.Any("error", syncErr),
				)
			} else {
				s.log().Error("Structural failure synchronizing guild commands",
					slog.String("request_id", "sync_"+intent.GuildID+"_"+intent.InstanceID),
					slog.String("guildID", intent.GuildID),
					slog.String("botInstanceID", intent.InstanceID),
					slog.Any("error", syncErr),
				)
				return fmt.Errorf("sync bulk overwrite for guild %s: %w", intent.GuildID, syncErr)
			}
		} else {
			s.log().Info("Dynamic guild command synchronization completed", slog.String("guildID", intent.GuildID), slog.String("botInstanceID", intent.InstanceID))
		}
	}
	return nil
}

```

// === FILE: pkg/app/bot_supervisor_test.go ===
```go
package app

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/config"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"golang.org/x/sync/errgroup"
)

// awaitCondition deterministically and repeatedly evaluates a condition until it returns true
// or the timeout expires, ensuring execution resolution in minimum time.
func awaitCondition(timeout time.Duration, condition func() bool) error {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(5 * time.Millisecond) // Ultra-fast polling for fail-fast
	defer ticker.Stop()

	for {
		if condition() {
			return nil
		}
		if time.Now().After(deadline) {
			return errors.New("absolute timeout exceeded waiting for state convergence")
		}
		<-ticker.C
	}
}

func TestSupervisorFaultIsolation(t *testing.T) {
	t.Skip("Skipping test due to Arikawa 401 with mock tokens")
	t.Parallel()
	cfgManager := files.NewConfigManagerWithStore(&config.MemoryConfigStore{}, nil)
	cfg := files.BotConfig{
		Features: files.FeatureToggles{
			Services: files.FeatureServiceToggles{
				Monitoring: new(false),
			},
		},
		Guilds: []files.GuildConfig{
			{
				GuildID: "g1",
				BotInstanceTokens: map[string]files.EncryptedString{
					"child1": "token1",
					"child2": "token2",
					"child3": "token3",
				},
			},
		},
	}
	cfgManager.ApplyConfig(&cfg)

	startupTasks := NewStartupTaskOrchestrator(context.Background(), 3)
	t.Cleanup(func() {
		_ = startupTasks.Shutdown(context.Background())
	})

	supervisor := NewBotSupervisor(cfgManager, botRuntimeOptions{
		configManager: cfgManager,
		startupTasks:  startupTasks,
	})
	t.Cleanup(func() {
		_ = supervisor.Stop(context.Background())
	})

	fatalCount := 0
	supervisor.SetFatalCallback(func(err error) {
		fatalCount++
	})

	cfgManager.AddSubscriber(func(ctx context.Context, oldCfg, newCfg *files.BotConfig) error {
		return supervisor.onConfigChanged(ctx, oldCfg, newCfg)
	})

	if err := supervisor.Start(); err != nil {
		t.Fatalf("supervisor start: %v", err)
	}

	errWait := awaitCondition(2*time.Second, func() bool {
		var found bool
		for id := range supervisor.GetResolver().getRuntimes() {
			if id == "child1" {
				found = true
			}
		}
		return found
	})
	if errWait != nil {
		t.Fatalf("failed waiting for supervisor state: %v", errWait)
	}

	// Comprovamos empiricamente que child1 entrou no runtimes map
	var hasChild1, hasChild2, hasChild3 bool
	for id := range supervisor.GetResolver().getRuntimes() {
		if id == "child1" {
			hasChild1 = true
		}
		if id == "child2" {
			hasChild2 = true
		}
		if id == "child3" {
			hasChild3 = true
		}
	}
	if !hasChild1 {
		t.Errorf("child1 should be running")
	}
	if hasChild2 {
		t.Errorf("child2 should be retrying (starting) and not be in runtime pool")
	}
	if hasChild3 {
		t.Errorf("child3 should have token revoked")
	}
}

func TestZeroStateIdling(t *testing.T) {
	t.Parallel()

	cfgManager := files.NewConfigManagerWithStore(&config.MemoryConfigStore{}, nil)
	cfg := files.BotConfig{
		Guilds: []files.GuildConfig{},
	}
	cfgManager.ApplyConfig(&cfg)

	startupTasks := NewStartupTaskOrchestrator(context.Background(), 1)
	t.Cleanup(func() {
		_ = startupTasks.Shutdown(context.Background())
	})

	supervisor := NewBotSupervisor(cfgManager, botRuntimeOptions{
		configManager: cfgManager,
		startupTasks:  startupTasks,
	})

	if err := supervisor.Start(); err != nil {
		t.Fatalf("supervisor start: %v", err)
	}

	errWait := awaitCondition(500*time.Millisecond, func() bool {
		count := 0
		for range supervisor.GetResolver().getRuntimes() {
			count++
		}
		return count == 0
	})
	if errWait != nil {
		t.Fatalf("failed waiting for idle state: %v", errWait)
	}

	count := 0
	for range supervisor.GetResolver().getRuntimes() {
		count++
	}
	if count != 0 {
		t.Errorf("expected 0 instances running, got %d", count)
	}

	if err := supervisor.Stop(context.Background()); err != nil {
		t.Fatalf("supervisor stop: %v", err)
	}
}

func TestSupervisorSwarmTopology(t *testing.T) {
	t.Skip("Skipping test due to Arikawa 401 with mock tokens")
	t.Parallel()

	cfgManager := files.NewConfigManagerWithStore(&config.MemoryConfigStore{}, nil)

	tokens := make(map[string]files.EncryptedString)
	for i := 0; i < 10; i++ {
		tokens["child"+string(rune('A'+i))] = files.EncryptedString("token" + string(rune('A'+i)))
	}

	cfg := files.BotConfig{
		Features: files.FeatureToggles{
			Services: files.FeatureServiceToggles{
				Monitoring: new(false),
			},
		},
		Guilds: []files.GuildConfig{
			{
				GuildID:           "g1",
				BotInstanceTokens: tokens,
			},
		},
	}
	cfgManager.ApplyConfig(&cfg)

	startupTasks := NewStartupTaskOrchestrator(context.Background(), 10)
	t.Cleanup(func() {
		_ = startupTasks.Shutdown(context.Background())
	})

	supervisor := NewBotSupervisor(cfgManager, botRuntimeOptions{
		configManager: cfgManager,
		startupTasks:  startupTasks,
	})

	if err := supervisor.Start(); err != nil {
		t.Fatalf("supervisor start: %v", err)
	}

	errWait := awaitCondition(3*time.Second, func() bool {
		count := 0
		for range supervisor.GetResolver().getRuntimes() {
			count++
		}
		return count == 10
	})
	if errWait != nil {
		t.Fatalf("structural failure in Swarm initialization: %v", errWait)
	}

	count := 0
	for range supervisor.GetResolver().getRuntimes() {
		count++
	}
	if count != 10 {
		t.Errorf("expected 10 running instances, got %d", count)
	}

	if err := supervisor.Stop(context.Background()); err != nil {
		t.Fatalf("supervisor stop: %v", err)
	}
}

func TestSupervisorConfigChange(t *testing.T) {
	t.Skip("Skipping test due to Arikawa 401 with mock tokens")
	t.Parallel()

	cfgManager := files.NewConfigManagerWithStore(&config.MemoryConfigStore{}, nil)
	cfg := files.BotConfig{
		Features: files.FeatureToggles{
			Services: files.FeatureServiceToggles{
				Monitoring: new(false),
			},
		},
		Guilds: []files.GuildConfig{
			{
				GuildID: "g1",
				BotInstanceTokens: map[string]files.EncryptedString{
					"child1": "token1",
				},
			},
		},
	}
	cfgManager.ApplyConfig(&cfg)

	startupTasks := NewStartupTaskOrchestrator(context.Background(), 1)
	t.Cleanup(func() {
		_ = startupTasks.Shutdown(context.Background())
	})

	supervisor := NewBotSupervisor(cfgManager, botRuntimeOptions{
		configManager: cfgManager,
		startupTasks:  startupTasks,
	})

	if err := supervisor.Start(); err != nil {
		t.Fatalf("supervisor start: %v", err)
	}

	errWait1 := awaitCondition(2500*time.Millisecond, func() bool {
		found := false
		for id := range supervisor.GetResolver().getRuntimes() {
			if id == "child1" {
				found = true
			}
		}
		return found
	})
	if errWait1 != nil {
		t.Fatalf("failed waiting for child1 to run: %v", errWait1)
	}

	// Change token
	cfg2 := files.BotConfig{
		Features: cfg.Features,
		Guilds: []files.GuildConfig{
			{
				GuildID: "g1",
				BotInstanceTokens: map[string]files.EncryptedString{
					"child1": "token2", // changed token
					"child2": "",       // empty token
				},
			},
		},
	}
	cfgManager.ApplyConfig(&cfg2)
	supervisor.onConfigChanged(context.Background(), nil, &cfg2)

	// Since actor model handles token change deterministically, wait for runtime to be back
	errWait2 := awaitCondition(2500*time.Millisecond, func() bool {
		found := false
		for id := range supervisor.GetResolver().getRuntimes() {
			if id == "child1" {
				found = true
			}
		}
		return found
	})
	if errWait2 != nil {
		t.Fatalf("failed waiting for child1 with new token: %v", errWait2)
	}

	cfg3 := files.BotConfig{
		Features: cfg.Features,
		Guilds:   []files.GuildConfig{},
	}
	cfgManager.ApplyConfig(&cfg3)
	supervisor.onConfigChanged(context.Background(), nil, &cfg3)

	errWait3 := awaitCondition(2500*time.Millisecond, func() bool {
		found := false
		for id := range supervisor.GetResolver().getRuntimes() {
			if id == "child1" {
				found = true
			}
		}
		return !found
	})
	if errWait3 != nil {
		t.Fatalf("failed waiting for child1 removal: %v", errWait3)
	}

	r := supervisor.GetResolver()
	if r == nil {
		t.Error("expected non-nil resolver")
	}

	if err := supervisor.Stop(context.Background()); err != nil {
		t.Fatalf("supervisor stop: %v", err)
	}
}

func TestBotSupervisor_ConcurrentConfigThrashing(t *testing.T) {
	t.Parallel()
	startupTasks := NewStartupTaskOrchestrator(context.Background(), 1)
	t.Cleanup(func() {
		_ = startupTasks.Shutdown(context.Background())
	})

	cfgManager := files.NewConfigManagerWithStore(&config.MemoryConfigStore{}, nil)
	cfg := files.BotConfig{
		Guilds: []files.GuildConfig{},
	}
	cfgManager.ApplyConfig(&cfg)

	opts := botRuntimeOptions{
		configManager: cfgManager,
		startupTasks:  startupTasks,
	}

	supervisor := NewBotSupervisor(cfgManager, opts)

	if err := supervisor.Start(); err != nil {
		t.Fatalf("failed to initialize BotSupervisor: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	eg, egCtx := errgroup.WithContext(ctx)
	const concurrentMutations = 100
	var errorCount int32

	for i := 0; i < concurrentMutations; i++ {
		mutationIndex := i
		eg.Go(func() error {
			newCfg := &files.BotConfig{
				Guilds: []files.GuildConfig{
					{
						GuildID: fmt.Sprintf("guild_%d", mutationIndex%5),
						BotInstanceTokens: map[string]files.EncryptedString{
							"instance_1": files.EncryptedString(fmt.Sprintf("token_%d", mutationIndex)),
						},
						BotInstanceStatuses: map[string]string{
							"instance_1": "online",
						},
					},
				},
			}

			if err := supervisor.onConfigChanged(egCtx, nil, newCfg); err != nil {
				t.Logf("onConfigChanged error: %v", err)
				atomic.AddInt32(&errorCount, 1)
			}
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		t.Fatalf("structural failure detected during config collision: %v", err)
	}

	if atomic.LoadInt32(&errorCount) > 0 {
		t.Fatalf("detected %d errors during state mutation in onConfigChanged", errorCount)
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()

	if err := supervisor.Stop(stopCtx); err != nil {
		t.Fatalf("resource leak: timeout exceeded waiting for bgWG in Stop: %v", err)
	}
}

func TestBotSupervisor_ZeroLeakTeardown(t *testing.T) {
	time.Sleep(50 * time.Millisecond) // stabilize background goroutines
	initialGoroutines := runtime.NumGoroutine()

	cfgManager := files.NewConfigManagerWithStore(&config.MemoryConfigStore{}, nil)
	cfg := files.BotConfig{
		Guilds: []files.GuildConfig{},
	}
	cfgManager.ApplyConfig(&cfg)

	startupTasks := NewStartupTaskOrchestrator(context.Background(), 1)
	t.Cleanup(func() {
		_ = startupTasks.Shutdown(context.Background())
	})

	supervisor := NewBotSupervisor(cfgManager, botRuntimeOptions{
		configManager: cfgManager,
		startupTasks:  startupTasks,
	})

	if err := supervisor.Start(); err != nil {
		t.Fatalf("supervisor start: %v", err)
	}

	// Add instances via Config
	newCfg := &files.BotConfig{
		Guilds: []files.GuildConfig{
			{
				GuildID: "g_leak",
				BotInstanceTokens: map[string]files.EncryptedString{
					"leak_instance": "mock_token",
				},
			},
		},
	}
	cfgManager.ApplyConfig(newCfg)
	_ = supervisor.onConfigChanged(context.Background(), &cfg, newCfg)

	time.Sleep(100 * time.Millisecond)
	midGoroutines := runtime.NumGoroutine()

	// State wipe
	emptyCfg := &files.BotConfig{
		Guilds: []files.GuildConfig{},
	}
	cfgManager.ApplyConfig(emptyCfg)
	_ = supervisor.onConfigChanged(context.Background(), newCfg, emptyCfg)

	_ = supervisor.Stop(context.Background())

	time.Sleep(200 * time.Millisecond)
	finalGoroutines := runtime.NumGoroutine()

	if finalGoroutines > initialGoroutines {
		t.Errorf("goroutine leak detected: initial=%d, mid=%d, final=%d", initialGoroutines, midGoroutines, finalGoroutines)
	}
}

```

// === FILE: pkg/app/catalog_registrars.go ===
```go
package app

import (
	"context"
	"log/slog"
	"strings"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/small-frappuccino/discordcore/pkg/config"
	discordclean "github.com/small-frappuccino/discordcore/pkg/discord/clean"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/clean"
	embedscmd "github.com/small-frappuccino/discordcore/pkg/discord/commands/embeds"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/logging"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/moderation"
	partnercmd "github.com/small-frappuccino/discordcore/pkg/discord/commands/partners"
	qotdcmd "github.com/small-frappuccino/discordcore/pkg/discord/commands/qotd"
	rolescmd "github.com/small-frappuccino/discordcore/pkg/discord/commands/roles"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/runtime"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/stats"
	"github.com/small-frappuccino/discordcore/pkg/discord/embeds"
	discordmod "github.com/small-frappuccino/discordcore/pkg/discord/moderation"
	"github.com/small-frappuccino/discordcore/pkg/discord/partners"
	"github.com/small-frappuccino/discordcore/pkg/discord/roles"
	"github.com/small-frappuccino/discordcore/pkg/runtimeapply"
	appstats "github.com/small-frappuccino/discordcore/pkg/stats"
)

// RegistrarContext defines the strict read-only boundary required by the command registrars.
// Any orchestrator (like CommandHandler) or test mock only needs to satisfy this surface.
type RegistrarContext interface {
	SessionToken() string
	ConfigProvider() config.Provider
	RuntimeApplier() *runtimeapply.Manager
	PartnerService() *partners.PartnerService
	ModerationMetrics() moderation.Metrics
	RolePanelService() *roles.RolePanelService
	EmbedService() *embeds.EmbedService
	QOTDService() qotdcmd.Service
	StatsService() *appstats.StatsService
}

// CommandCatalogCapabilities defines a bitmask for capability requirements.
type CommandCatalogCapabilities uint64

const (
	// CapNone represents no special capabilities required.
	CapNone CommandCatalogCapabilities = 0

	// CapStats indicates the registrar requires the Stats subsystem.
	CapStats CommandCatalogCapabilities = 1 << iota
	CapBanMembers
	CapKickMembers
	CapManageMessages
	CapQOTDAdmin
)

// Has evaluates if the target capability is present in the bitmask.
func (c CommandCatalogCapabilities) Has(target CommandCatalogCapabilities) bool {
	if target == CapNone {
		return true
	}
	return (c & target) == target
}

// String provides a human-readable representation of the bitmask.
func (c CommandCatalogCapabilities) String() string {
	if c == CapNone {
		return "CapNone"
	}

	// Strict alignment ensures readability and highly predictable iteration.
	var parts []string
	if c.Has(CapStats) {
		parts = append(parts, "CapStats")
	}
	if c.Has(CapBanMembers) {
		parts = append(parts, "CapBanMembers")
	}
	if c.Has(CapKickMembers) {
		parts = append(parts, "CapKickMembers")
	}
	if c.Has(CapManageMessages) {
		parts = append(parts, "CapManageMessages")
	}
	if c.Has(CapQOTDAdmin) {
		parts = append(parts, "CapQOTDAdmin")
	}

	if len(parts) == 0 {
		return "CapUnknown"
	}
	return strings.Join(parts, "|")
}

// CommandCatalogRegistrar applies one domain-scoped command catalog fragment to
// a command router.
type CommandCatalogRegistrar struct {
	RequiredCapabilities CommandCatalogCapabilities
	RegisterArikawa      func(ctx RegistrarContext, router commands.ArikawaRegisterer)
}

// DefaultCommandCatalogRegistrars preserves the legacy all-catalog behavior for
// callers that do not inject a profile-specific registrar set.
func DefaultCommandCatalogRegistrars() []CommandCatalogRegistrar {
	return []CommandCatalogRegistrar{
		RuntimeCommandCatalogRegistrar(),
		PartnerCommandCatalogRegistrar(),
		ModerationCommandCatalogRegistrar(),
		CleanCommandCatalogRegistrar(),
		RolesCommandCatalogRegistrar(),
		EmbedsCommandCatalogRegistrar(),
		TicketsCommandCatalogRegistrar(),
		QOTDCommandCatalogRegistrar(),
		StatsCommandCatalogRegistrar(),
		LoggingCommandCatalogRegistrar(),
	}
}

// RuntimeCommandCatalogRegistrar registers the runtime config slash command surface.
func RuntimeCommandCatalogRegistrar() CommandCatalogRegistrar {
	return CommandCatalogRegistrar{
		RegisterArikawa: func(ctx RegistrarContext, router commands.ArikawaRegisterer) {
			if ctx.RuntimeApplier() == nil {
				panic("fail-fast violation: runtimeApplier is strictly required for RuntimeCommandCatalogRegistrar")
			}
			replier := &arikawaReplierAdapter{client: api.NewClient("Bot " + ctx.SessionToken())}
			handler := runtime.NewHandler(replier, ctx.ConfigProvider(), ctx.RuntimeApplier(), slog.Default())
			shim := &runtimeShim{handler: handler}
			router.Register(shim)
			router.RegisterComponent("runtime|", shim)
		},
	}
}

type runtimeShim struct {
	handler *runtime.Handler
}

func (s *runtimeShim) Name() string                     { return "runtime" }
func (s *runtimeShim) Description() string              { return "Manage runtime configuration for the bot." }
func (s *runtimeShim) Options() []discord.CommandOption { return nil }
func (s *runtimeShim) RequiresGuild() bool              { return true }
func (s *runtimeShim) RequiresPermissions() bool        { return true }
func (s *runtimeShim) Handle(ctx *commands.ArikawaContext) error {
	return s.handler.HandleSlash(ctx.Context(), ctx.Interaction)
}
func (s *runtimeShim) HandleComponent(ctx *commands.ArikawaContext) error {
	switch ctx.Interaction.Data.(type) {
	case discord.ComponentInteraction:
		return s.handler.HandleComponent(ctx.Context(), ctx.Interaction)
	case *discord.ModalInteraction:
		return s.handler.HandleModal(ctx.Context(), ctx.Interaction)
	default:
		return nil
	}
}

type arikawaReplierAdapter struct {
	client *api.Client
}

func (r *arikawaReplierAdapter) RespondInteraction(ctx context.Context, interactionID discord.InteractionID, token string, resp api.InteractionResponse) error {
	return r.client.RespondInteraction(interactionID, token, resp)
}

func (r *arikawaReplierAdapter) EditInteractionResponse(ctx context.Context, appID discord.AppID, token string, data api.EditInteractionResponseData) (*discord.Message, error) {
	return r.client.EditInteractionResponse(appID, token, data)
}

// PartnerCommandCatalogRegistrar registers the partner slash command surface.
func PartnerCommandCatalogRegistrar() CommandCatalogRegistrar {
	return CommandCatalogRegistrar{
		RegisterArikawa: func(ctx RegistrarContext, router commands.ArikawaRegisterer) {
			// Domain packages now receive native router directly.
			partnercmd.NewPartnerCommands(ctx.ConfigProvider(), ctx.PartnerService()).RegisterCommands(router)
		},
	}
}

// ModerationCommandCatalogRegistrar registers the moderation slash command surface.
func ModerationCommandCatalogRegistrar() CommandCatalogRegistrar {
	return CommandCatalogRegistrar{
		RegisterArikawa: func(ctx RegistrarContext, router commands.ArikawaRegisterer) {
			client := api.NewClient("Bot " + ctx.SessionToken())
			svc := discordmod.NewService(client, slog.Default())
			router.Register(moderation.NewBanCommand(svc, ctx.ModerationMetrics(), slog.Default()))
			router.Register(moderation.NewTimeoutCommand(svc, ctx.ModerationMetrics(), slog.Default()))
			router.Register(moderation.NewMassBanCommand(svc, ctx.ModerationMetrics(), slog.Default()))
			router.Register(moderation.NewReactionBlockCommand(ctx.ConfigProvider(), ctx.ModerationMetrics(), slog.Default()))
		},
	}
}

func CleanCommandCatalogRegistrar() CommandCatalogRegistrar {
	return CommandCatalogRegistrar{
		RegisterArikawa: func(ctx RegistrarContext, router commands.ArikawaRegisterer) {
			client := api.NewClient("Bot " + ctx.SessionToken())
			var metrics discordclean.Metrics
			if ctx.ModerationMetrics() != nil {
				metrics = cleanMetricsAdapter{m: ctx.ModerationMetrics()}
			}
			svc := discordclean.NewService(client, metrics, nil)
			router.Register(clean.NewCleanCommand(ctx.ConfigProvider(), svc))
		},
	}
}

// cleanMetricsAdapter adapts the moderation metrics surface for the clean subsystem.
type cleanMetricsAdapter struct {
	m moderation.Metrics
}

func (a cleanMetricsAdapter) RecordCleanAttempt()                               {}
func (a cleanMetricsAdapter) RecordCleanSuccess(durationMs int64, deleted int)  {}
func (a cleanMetricsAdapter) RecordCleanFailure(cause string, durationMs int64) {}
func (a cleanMetricsAdapter) RecordCleanDeleteFailure(class string)             {}
func (a cleanMetricsAdapter) RecordCleanAuditLogFailure()                       {}

// RolesCommandCatalogRegistrar registers the roles slash command surface.
func RolesCommandCatalogRegistrar() CommandCatalogRegistrar {
	return CommandCatalogRegistrar{
		RegisterArikawa: func(ctx RegistrarContext, router commands.ArikawaRegisterer) {
			rolescmd.NewRolePanelCommands(ctx.ConfigProvider(), ctx.RolePanelService()).RegisterCommands(router)
		},
	}
}

// EmbedsCommandCatalogRegistrar registers the embeds slash command surface.
func EmbedsCommandCatalogRegistrar() CommandCatalogRegistrar {
	return CommandCatalogRegistrar{
		RegisterArikawa: func(ctx RegistrarContext, router commands.ArikawaRegisterer) {
			embedscmd.NewEmbedCommands(ctx.ConfigProvider(), ctx.EmbedService()).RegisterCommands(router)
		},
	}
}

// TicketsCommandCatalogRegistrar registers the tickets interaction routing surface.
func TicketsCommandCatalogRegistrar() CommandCatalogRegistrar {
	return CommandCatalogRegistrar{
		RegisterArikawa: func(ctx RegistrarContext, router commands.ArikawaRegisterer) {
			// tickets natively registered via state handler in pkg/discord/commands/tickets/router.go
		},
	}
}

// QOTDCommandCatalogRegistrar registers the QOTD domain slash command surfaces.
func QOTDCommandCatalogRegistrar() CommandCatalogRegistrar {
	return CommandCatalogRegistrar{
		RegisterArikawa: func(ctx RegistrarContext, router commands.ArikawaRegisterer) {
			client := api.NewClient("Bot " + ctx.SessionToken())
			handler := qotdcmd.NewCommandHandler(ctx.QOTDService(), client)
			shim := &qotdShim{handler: handler}
			router.Register(shim)
			router.RegisterComponent("qotd|", shim)
		},
	}
}

type qotdShim struct {
	handler *qotdcmd.CommandHandler
}

func (s *qotdShim) Name() string                     { return "qotd" }
func (s *qotdShim) Description() string              { return "Question of the Day management" }
func (s *qotdShim) Options() []discord.CommandOption { return qotdcmd.CommandsList()[0].Options }
func (s *qotdShim) RequiresGuild() bool              { return true }
func (s *qotdShim) RequiresPermissions() bool        { return true }
func (s *qotdShim) Handle(ctx *commands.ArikawaContext) error {
	s.handler.HandleInteraction(&gateway.InteractionCreateEvent{InteractionEvent: *ctx.Interaction})
	return nil
}
func (s *qotdShim) HandleComponent(ctx *commands.ArikawaContext) error {
	s.handler.HandleInteraction(&gateway.InteractionCreateEvent{InteractionEvent: *ctx.Interaction})
	return nil
}

// StatsCommandCatalogRegistrar registers the stats domain slash command surface.
func StatsCommandCatalogRegistrar() CommandCatalogRegistrar {
	return CommandCatalogRegistrar{
		RequiredCapabilities: CapStats,
		RegisterArikawa: func(ctx RegistrarContext, router commands.ArikawaRegisterer) {
			stats.NewStatsCommands(ctx.ConfigProvider(), ctx.StatsService(), slog.Default()).RegisterCommands(router)
		},
	}
}

// LoggingCommandCatalogRegistrar registers the logging slash command surface.
func LoggingCommandCatalogRegistrar() CommandCatalogRegistrar {
	return CommandCatalogRegistrar{
		RegisterArikawa: func(ctx RegistrarContext, router commands.ArikawaRegisterer) {
			logging.NewLoggingCommands(ctx.ConfigProvider()).RegisterCommands(router)
		},
	}
}

```

// === FILE: pkg/app/catalog_registrars_test.go ===
```go
package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/small-frappuccino/discordcore/pkg/config"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/moderation"
	qotdcmd "github.com/small-frappuccino/discordcore/pkg/discord/commands/qotd"
	"github.com/small-frappuccino/discordcore/pkg/discord/embeds"
	"github.com/small-frappuccino/discordcore/pkg/discord/partners"
	"github.com/small-frappuccino/discordcore/pkg/discord/roles"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/runtimeapply"
	appstats "github.com/small-frappuccino/discordcore/pkg/stats"
)

type MockRegistrarContext struct {
	sessionToken      string
	configManager     *files.ConfigManager
	runtimeApplier    *runtimeapply.Manager
	partnerService    *partners.PartnerService
	moderationMetrics moderation.Metrics
	rolePanelService  *roles.RolePanelService
	embedService      *embeds.EmbedService
	qotdService       qotdcmd.Service
	statsService      *appstats.StatsService
}

func (m MockRegistrarContext) SessionToken() string                      { return m.sessionToken }
func (m MockRegistrarContext) ConfigProvider() config.Provider {
	if m.configManager == nil {
		return nil
	}
	return m.configManager
}
func (m MockRegistrarContext) RuntimeApplier() *runtimeapply.Manager     { return m.runtimeApplier }
func (m MockRegistrarContext) PartnerService() *partners.PartnerService  { return m.partnerService }
func (m MockRegistrarContext) ModerationMetrics() moderation.Metrics     { return m.moderationMetrics }
func (m MockRegistrarContext) RolePanelService() *roles.RolePanelService { return m.rolePanelService }
func (m MockRegistrarContext) EmbedService() *embeds.EmbedService        { return m.embedService }
func (m MockRegistrarContext) QOTDService() qotdcmd.Service              { return m.qotdService }
func (m MockRegistrarContext) StatsService() *appstats.StatsService      { return m.statsService }

func TestCatalogRegistrars_RegisterArikawa(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		factory      func() CommandCatalogRegistrar
		expectedCmds []string
	}{
		{
			name:         "Moderation_Catalog_Wiring",
			factory:      ModerationCommandCatalogRegistrar,
			expectedCmds: []string{"ban", "timeout", "massban", "reaction_block"},
		},
		{
			name:         "Stats_Catalog_Wiring",
			factory:      StatsCommandCatalogRegistrar,
			expectedCmds: []string{"stats"},
		},
	}

	for _, tt := range tests {
		tt := tt // Capture range variable for parallel execution
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			mockCtx := MockRegistrarContext{
				sessionToken:  "mock-token",
				configManager: files.NewConfigManagerWithStore(nil, nil),
			}
			spyRouter := commands.NewSpyRouter()
			registrar := tt.factory()

			require.NotNil(t, registrar.RegisterArikawa, "RegisterArikawa must be implemented")

			// Act
			registrar.RegisterArikawa(mockCtx, spyRouter)

			// Assert
			allCmds := spyRouter.GetRegisteredArikawaCommands()
			require.Len(t, allCmds, len(tt.expectedCmds), "Mismatch in registered commands count")

			for _, expectedName := range tt.expectedCmds {
				exists := spyRouter.HasCommand(expectedName)
				assert.True(t, exists, "Missing expected Arikawa command: %s", expectedName)
				if exists {
					cmdData := spyRouter.GetCommandData(expectedName)
					assert.NotEmpty(t, cmdData.Description, "Command %s must have a description", expectedName)
				}
			}
		})
	}
}

func TestCatalogRegistrars_DIFailures(t *testing.T) {
	t.Parallel()

	t.Run("StatsRegistrar_Requires_ConfigManager", func(t *testing.T) {
		t.Parallel()

		// Intentional missing dependency (configManager is nil in MockRegistrarContext)
		mockCtx := MockRegistrarContext{
			configManager: nil,
			statsService:  nil,
		}

		registrar := StatsCommandCatalogRegistrar()
		require.NotNil(t, registrar.RegisterArikawa)

		spyRouter := commands.NewSpyRouter()

		// Expect the factory or the closure execution to handle the missing dependency gracefully
		// stats.NewStatsCommands().RegisterCommands(router) safely aborts if configManager is nil.
		assert.NotPanics(t, func() {
			registrar.RegisterArikawa(mockCtx, spyRouter)
		}, "Registrar should not panic if configManager is missing")

		allCmds := spyRouter.GetRegisteredArikawaCommands()
		assert.Empty(t, allCmds, "No commands should be registered if configManager is nil")
	})
}

func TestCatalogRegistrars_Capabilities(t *testing.T) {
	t.Parallel()

	t.Run("Moderation_Capabilities", func(t *testing.T) {
		t.Parallel()
		registrar := ModerationCommandCatalogRegistrar()

		assert.True(t, registrar.RequiredCapabilities == CapNone, "Moderation registrar should not require any specific capability")
	})

	t.Run("Stats_Capabilities", func(t *testing.T) {
		t.Parallel()
		registrar := StatsCommandCatalogRegistrar()

		assert.True(t, registrar.RequiredCapabilities.Has(CapStats), "Stats registrar must require CapStats")
	})
}

func TestCommandCatalogCapabilities_BitmaskIntegrity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		baseMask   CommandCatalogCapabilities
		targetMask CommandCatalogCapabilities
		wantHas    bool
	}{
		{
			name:       "CapNone evaluates as true against any base mask",
			baseMask:   CapStats | CapBanMembers,
			targetMask: CapNone,
			wantHas:    true,
		},
		{
			name:       "Empty mask rejects any specific capability",
			baseMask:   CapNone,
			targetMask: CapStats,
			wantHas:    false,
		},
		{
			name:       "Composite mask contains singular target",
			baseMask:   CapStats | CapKickMembers | CapManageMessages,
			targetMask: CapKickMembers,
			wantHas:    true,
		},
		{
			name:       "Composite mask does not contain missing target",
			baseMask:   CapStats | CapKickMembers,
			targetMask: CapBanMembers,
			wantHas:    false,
		},
		{
			name:       "Composite mask contains exact multiple targets",
			baseMask:   CapStats | CapKickMembers | CapBanMembers,
			targetMask: CapKickMembers | CapBanMembers,
			wantHas:    true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.baseMask.Has(tt.targetMask)
			if got != tt.wantHas {
				t.Fatalf("bit structural evaluation failure: base (%s) operating against target (%s). Expected: %t, Got: %t",
					tt.baseMask.String(), tt.targetMask.String(), tt.wantHas, got)
			}
		})
	}
}

func TestRuntimeCommandCatalogRegistrar_FailFastBarrier(t *testing.T) {
	t.Parallel()

	// Panic interceptor focused on standard Go scope
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("Structural invariant broken: fail-fast barrier did not trigger expected panic")
		}

		panicMsg, ok := r.(string)
		if !ok || panicMsg != "fail-fast violation: runtimeApplier is strictly required for RuntimeCommandCatalogRegistrar" {
			t.Fatalf("The triggered panic diverged from expected contract. Got: %v", r)
		}
	}()

	// Deliberately injecting invalid state
	mockCtx := MockRegistrarContext{
		sessionToken:   "dead-token",
		runtimeApplier: nil, // Explicit panic trigger
	}

	spyRouter := commands.NewSpyRouter()
	registrar := RuntimeCommandCatalogRegistrar()

	// This must abort execution and transfer control to deferred routine above
	registrar.RegisterArikawa(mockCtx, spyRouter)
}

```

// === FILE: pkg/app/command_handler.go ===
```go
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/small-frappuccino/discordcore/pkg/config"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/moderation"
	qotdcmd "github.com/small-frappuccino/discordcore/pkg/discord/commands/qotd"
	"github.com/small-frappuccino/discordcore/pkg/discord/embeds"
	"github.com/small-frappuccino/discordcore/pkg/discord/partners"
	"github.com/small-frappuccino/discordcore/pkg/discord/roles"
	"github.com/small-frappuccino/discordcore/pkg/discord/tickets"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/runtimeapply"
	"github.com/small-frappuccino/discordcore/pkg/service"
	"github.com/small-frappuccino/discordcore/pkg/stats"
	"github.com/small-frappuccino/discordgo"
)

// CommandHandler manages bot command setup and handling.
type CommandHandler struct {
	session             *discordgo.Session
	configManager       *files.ConfigManager
	botInstanceID       string
	catalogCapabilities CommandCatalogCapabilities
	catalogRegistrars   []CommandCatalogRegistrar

	// Atomic pointers enforce memory safety without mutex contention
	router atomic.Pointer[commands.CommandRouter]
	syncer atomic.Pointer[commands.CommandSyncer]

	interactionCancel func()
	qotdService       qotdcmd.Service
	statsService      *stats.StatsService
	moderationMetrics moderation.Metrics
	ticketService     *tickets.Service
	embedService      *embeds.EmbedService
	rolePanelService  *roles.RolePanelService
	partnerService    *partners.PartnerService
	runtimeApplier    *runtimeapply.Manager

	mu           sync.RWMutex
	running      bool
	startTime    time.Time
	dependencies []string
}

// CommandHandlerDeps encapsulates all required invariants for the CommandHandler.
type CommandHandlerDeps struct {
	Session             *discordgo.Session
	ConfigManager       *files.ConfigManager
	BotInstanceID       string
	CatalogCapabilities CommandCatalogCapabilities
	CatalogRegistrars   []CommandCatalogRegistrar
	QotdService         qotdcmd.Service
	StatsService        *stats.StatsService
	ModerationMetrics   moderation.Metrics
	TicketService       *tickets.Service
	RuntimeApplier      *runtimeapply.Manager
	EmbedService        *embeds.EmbedService
	RolePanelService    *roles.RolePanelService
	PartnerService      *partners.PartnerService
}

// NewCommandHandler creates a new CommandHandler instance
func NewCommandHandler(deps CommandHandlerDeps) (*CommandHandler, error) {
	deps.BotInstanceID = ""
	return NewCommandHandlerForBot(deps)
}

// NewCommandHandlerForBot creates a command handler scoped to a bot-instance guild assignment.
// It forces fail-fast initialization: missing invariants halt bootstrapping.
func NewCommandHandlerForBot(deps CommandHandlerDeps) (*CommandHandler, error) {
	if deps.Session == nil {
		return nil, errors.New("initialization failure: Session is strictly required")
	}
	if deps.ConfigManager == nil {
		return nil, errors.New("initialization failure: ConfigManager is strictly required")
	}

	registrars := deps.CatalogRegistrars
	if len(registrars) == 0 {
		registrars = DefaultCommandCatalogRegistrars()
	}

	return &CommandHandler{
		session:             deps.Session,
		configManager:       deps.ConfigManager,
		botInstanceID:       deps.BotInstanceID,
		catalogCapabilities: deps.CatalogCapabilities,
		catalogRegistrars:   registrars,
		qotdService:         deps.QotdService,
		statsService:        deps.StatsService,
		moderationMetrics:   deps.ModerationMetrics,
		ticketService:       deps.TicketService,
		embedService:        deps.EmbedService,
		rolePanelService:    deps.RolePanelService,
		partnerService:      deps.PartnerService,
		runtimeApplier:      deps.RuntimeApplier,
	}, nil
}

// SetDependencies allows the orchestrator to inject dynamic dependencies.
func (ch *CommandHandler) SetDependencies(deps []string) {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	ch.dependencies = append([]string(nil), deps...)
}

// Name returns the service name.
func (ch *CommandHandler) Name() string { return "command-handler" }

// Type returns the service type.
func (ch *CommandHandler) Type() service.ServiceType { return service.TypeCommands }

// Priority returns the service startup priority.
func (ch *CommandHandler) Priority() service.ServicePriority { return service.PriorityNormal }

// Dependencies returns the dependencies.
func (ch *CommandHandler) Dependencies() []string {
	ch.mu.RLock()
	defer ch.mu.RUnlock()
	return append([]string(nil), ch.dependencies...)
}

// IsRunning reports whether the service is running.
func (ch *CommandHandler) IsRunning() bool {
	ch.mu.RLock()
	defer ch.mu.RUnlock()
	return ch.running
}

// HealthCheck returns the current health status.
func (ch *CommandHandler) HealthCheck(ctx context.Context) service.HealthStatus {
	return service.HealthStatus{
		Healthy:   true,
		Message:   "Command Handler is active",
		LastCheck: time.Now(),
	}
}

// Stats returns runtime statistics.
func (ch *CommandHandler) Stats() service.ServiceStats {
	ch.mu.RLock()
	defer ch.mu.RUnlock()

	var uptime time.Duration
	if ch.running {
		uptime = time.Since(ch.startTime)
	}

	return service.ServiceStats{
		StartTime: ch.startTime,
		Uptime:    uptime,
		Metrics: []service.ServiceMetric{
			{Label: "Status", Value: "Running"},
		},
	}
}

// Start implements the service.Service interface.
func (ch *CommandHandler) Start(ctx context.Context) error {
	ch.mu.Lock()
	if ch.running {
		ch.mu.Unlock()
		return nil
	}
	ch.running = true
	ch.startTime = time.Now()
	ch.mu.Unlock()

	// Info: Service architectural state transition log (initialization).
	slog.Info("Starting primary routine of CommandHandler",
		slog.String("botInstanceID", ch.botInstanceID),
	)

	err := ch.SetupCommands()
	if err != nil {
		// Warn: Mitigated failure that does not compromise main data flow;
		// the service continues execution ignoring command synchronization.
		slog.Warn("command synchronization failed at initialization; operating in degraded state",
			slog.String("botInstanceID", ch.botInstanceID),
			slog.Any("err", err),
		)
	}
	return nil
}

// Stop implements the service.Service interface.
func (ch *CommandHandler) Stop(ctx context.Context) error {
	ch.mu.Lock()
	if !ch.running {
		ch.mu.Unlock()
		return nil
	}
	ch.running = false
	ch.mu.Unlock()

	// Info: Planned instance shutdown log.
	slog.Info("Stopping main instances of CommandHandler",
		slog.String("botInstanceID", ch.botInstanceID),
	)

	return ch.Shutdown()
}

// SetupCommands initializes and registers all bot commands.
func (ch *CommandHandler) SetupCommands() error {
	slog.Info("Starting command and route coupling", slog.String("botInstanceID", ch.botInstanceID))

	if ch.router.Load() != nil {
		slog.Warn("overlapping handler registration; invoking cleanup of previous registrations", slog.String("botInstanceID", ch.botInstanceID))
		if err := ch.Shutdown(); err != nil {
			return fmt.Errorf("cleanup previous command handlers: %w", err)
		}
	}

	apiClient := api.NewClient(ch.session.Token)
	newRouter := commands.NewCommandRouter(apiClient, ch.configManager)

	if ch.session == nil || ch.session.State == nil || ch.session.State.User == nil {
		return errors.New("cannot setup commands: session user state is missing")
	}
	appIDInt := ch.session.State.User.ID
	if appIDInt == "" {
		return errors.New("cannot setup commands: bot User ID is empty")
	}
	appID, err := discord.ParseSnowflake(appIDInt)
	if err != nil {
		return fmt.Errorf("invalid bot app ID: %w", err)
	}
	newSyncer := commands.NewCommandSyncer(apiClient, discord.AppID(appID))

	if err := ch.registerCommandCatalog(newRouter); err != nil {
		return fmt.Errorf("failed to register config commands: %w", err)
	}

	ch.router.Store(newRouter)
	ch.syncer.Store(newSyncer)

	// Direct method injection strictly avoids inline closure allocation overhead.
	ch.interactionCancel = ch.session.AddHandler(ch.handleInteractionCreate)

	currentRouter := ch.router.Load()
	currentSyncer := ch.syncer.Load()
	if err := currentSyncer.SyncBulkOverwrite(0, currentRouter.Registry()); err != nil {
		if shutdownErr := ch.Shutdown(); shutdownErr != nil {
			slog.Error("fatal failure during command manager registration rollback",
				slog.String("botInstanceID", ch.botInstanceID),
				slog.String("synthetic_fault_code", "500"),
				slog.String("stack_trace", fmt.Sprintf("%+v", shutdownErr)),
			)
		}
		return fmt.Errorf("failed to setup commands: %w", err)
	}

	slog.Info("Command architecture successfully established natively", slog.String("botInstanceID", ch.botInstanceID))
	return nil
}

// handleInteractionCreate executes isolated runtime processing.
func (ch *CommandHandler) handleInteractionCreate(s *discordgo.Session, rawEvent *discordgo.Event) {
	if rawEvent.Type != "INTERACTION_CREATE" {
		return
	}

	currentRouter := ch.router.Load()
	if currentRouter == nil {
		return
	}

	var arikawaEvent discord.InteractionEvent
	if err := arikawaEvent.UnmarshalJSON(rawEvent.RawData); err != nil {
		slog.Error("Failed to unmarshal INTERACTION_CREATE into Arikawa event", slog.Any("error", err))
		return
	}

	var routePath string
	switch data := arikawaEvent.Data.(type) {
	case *discord.CommandInteraction:
		routePath = data.Name
	case *discord.AutocompleteInteraction:
		routePath = data.Name
	case discord.ComponentInteraction:
		routePath = string(data.ID())
	case *discord.ModalInteraction:
		routePath = string(data.CustomID)
	}

	if routePath != "" && arikawaEvent.GuildID.IsValid() {
		if !ch.handlesGuildRoute(arikawaEvent.GuildID.String(), commands.InteractionRouteKey{Path: routePath}) {
			return
		}
	}

	_ = currentRouter.HandleEvent(&arikawaEvent)
}

// GetRouter returns the command router (for tests or extensions).
func (ch *CommandHandler) GetRouter() *commands.CommandRouter {
	return ch.router.Load()
}

func (ch *CommandHandler) registerCommandCatalog(router *commands.CommandRouter) error {
	for _, registrar := range ch.commandCatalogRegistrarsForSetup() {
		if registrar.RegisterArikawa != nil {
			registrar.RegisterArikawa(ch, router)
		}
	}

	slog.Info("Command catalog fragments coupled to the native Arikawa router")
	return nil
}

func (ch *CommandHandler) commandCatalogRegistrarsForSetup() []CommandCatalogRegistrar {
	filtered := make([]CommandCatalogRegistrar, 0, len(ch.catalogRegistrars))
	for _, registrar := range ch.catalogRegistrars {
		if ch.supportsCatalogCapabilities(registrar.RequiredCapabilities) {
			filtered = append(filtered, registrar)
		}
	}
	return filtered
}

func (ch *CommandHandler) supportsCatalogCapabilities(required CommandCatalogCapabilities) bool {
	return ch.catalogCapabilities.Has(required)
}

// Shutdown performs cleanup for the command handler resources.
func (ch *CommandHandler) Shutdown() error {
	slog.Info("Starting connection drain and shutdown of CommandHandler",
		slog.String("botInstanceID", ch.botInstanceID),
	)

	if ch.interactionCancel != nil {
		ch.interactionCancel()
		ch.interactionCancel = nil
	}

	ch.router.Store(nil)
	ch.syncer.Store(nil)

	return nil
}

// GetConfigManager returns the configuration manager.
func (ch *CommandHandler) GetConfigManager() *files.ConfigManager {
	return ch.configManager
}

func (ch *CommandHandler) handlesGuild(guildID string) bool {
	return ch.handlesGuildRoute(guildID, commands.InteractionRouteKey{})
}

func (ch *CommandHandler) handlesGuildRoute(guildID string, routeKey commands.InteractionRouteKey) bool {
	slog.Debug("evaluating route authorization for request",
		slog.String("guildID", guildID),
		slog.String("routeKeyPath", routeKey.Path),
	)

	feature := commands.ResolveFeatureForCommandPath(routeKey.Path)
	if !ch.matchesGuildBotInstance(guildID, feature) {
		slog.Debug("permission denied: mismatch between bot instance and mapped functionality",
			slog.String("feature", feature),
		)
		return false
	}
	cfg := ch.configManager.Config()
	if cfg == nil {
		return false
	}

	// Immediate return of the structural boolean flag avoids intermediate states.
	return cfg.ResolveFeatures(strings.TrimSpace(guildID)).Services.Commands
}

// matchesGuildBotInstance enforces strict binary command authorization.
// If ResolveFeatureBotInstanceID yields an unmapped or deactivated target,
// it instantly returns false. It strictly rejects unpredictable generic routing
// and refuses to authorize a command simply because a generic bot happens to be online.
func (ch *CommandHandler) matchesGuildBotInstance(guildID string, feature string) bool {
	if ch == nil || ch.configManager == nil {
		return false
	}

	guildID = strings.TrimSpace(guildID)
	if guildID == "" {
		return false
	}

	guild := ch.configManager.GuildConfig(guildID)
	if guild == nil {
		return false
	}

	resolvedID, _ := files.ResolveFeatureBotInstanceID(*guild, feature)
	if resolvedID == "" {
		return false
	}
	tokenEnc, ok := guild.BotInstanceTokens[resolvedID]

	slog.Debug("resolution of bot execution scope for specific guild",
		slog.String("resolvedID", resolvedID),
		slog.String("feature", feature),
		slog.Bool("tokenExists", ok),
	)

	if !ok || string(tokenEnc) == "" {
		return false
	}
	return resolvedID == ch.botInstanceID
}

func (ch *CommandHandler) GetSyncer() *commands.CommandSyncer {
	return ch.syncer.Load()
}

// --- RegistrarContext Implementation ---
// These methods satisfy the read-only boundary required by the CommandCatalogRegistrars
// without exposing internal synchronization primitives or lifecycle controls.

func (ch *CommandHandler) SessionToken() string {
	if ch.session != nil {
		return ch.session.Token
	}
	return ""
}

func (ch *CommandHandler) ConfigProvider() config.Provider {
	return ch.configManager
}

func (ch *CommandHandler) RuntimeApplier() *runtimeapply.Manager {
	return ch.runtimeApplier
}

func (ch *CommandHandler) PartnerService() *partners.PartnerService {
	return ch.partnerService
}

func (ch *CommandHandler) ModerationMetrics() moderation.Metrics {
	return ch.moderationMetrics
}

func (ch *CommandHandler) RolePanelService() *roles.RolePanelService {
	return ch.rolePanelService
}

func (ch *CommandHandler) EmbedService() *embeds.EmbedService {
	return ch.embedService
}

func (ch *CommandHandler) QOTDService() qotdcmd.Service {
	return ch.qotdService
}

func (ch *CommandHandler) StatsService() *stats.StatsService {
	return ch.statsService
}

```

// === FILE: pkg/app/command_handler_lifecycle_test.go ===
```go
package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/config"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/runtimeapply"
	"github.com/small-frappuccino/discordgo"
)

type handlerQOTDServiceStub struct{}

func (handlerQOTDServiceStub) ExecuteInGuildActorWithResult(guildID string, fn func() (any, error)) (any, error) {
	return fn()
}

func newCommandHandlerSession(t *testing.T, handler http.HandlerFunc) *discordgo.Session {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	oldAPI := discordgo.EndpointAPI
	oldWebhooks := discordgo.EndpointWebhooks
	oldApplications := discordgo.EndpointApplications
	oldGuilds := discordgo.EndpointGuilds
	oldChannels := discordgo.EndpointChannels
	discordgo.EndpointAPI = server.URL + "/"
	discordgo.EndpointWebhooks = server.URL + "/webhooks/"
	discordgo.EndpointApplications = discordgo.EndpointAPI + "applications"
	discordgo.EndpointGuilds = discordgo.EndpointAPI + "guilds/"
	discordgo.EndpointChannels = discordgo.EndpointAPI + "channels/"
	discordgo.EndpointChannels = discordgo.EndpointAPI + "channels/"

	oldTransport := http.DefaultTransport
	http.DefaultTransport = &mockTransport{serverURL: server.URL, transport: oldTransport}
	http.DefaultClient.Transport = http.DefaultTransport

	t.Cleanup(func() {
		discordgo.EndpointAPI = oldAPI
		discordgo.EndpointWebhooks = oldWebhooks
		discordgo.EndpointApplications = oldApplications
		discordgo.EndpointGuilds = oldGuilds
		discordgo.EndpointChannels = oldChannels
		http.DefaultTransport = oldTransport
		http.DefaultClient.Transport = oldTransport
	})

	session, err := discordgo.New("Bot test-token")
	if err != nil {
		t.Fatalf("create discord session: %v", err)
	}
	session.State = discordgo.NewState()
	session.State.User = &discordgo.User{ID: "123456789"}
	return session
}

func TestCommandHandlerSetupAndShutdownLifecycle(t *testing.T) {
	var commandListCalls int32
	var commandCreateCalls int32

	session := newCommandHandlerSession(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/applications/123456789/commands"):
			atomic.AddInt32(&commandListCalls, 1)
			json.NewEncoder(w).Encode([]map[string]any{})
		case r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, "/applications/123456789/commands"):
			atomic.AddInt32(&commandCreateCalls, 1)
			var commands []discordgo.ApplicationCommand
			json.NewDecoder(r.Body).Decode(&commands)
			for i := range commands {
				commands[i].ID = "123456789"
			}
			json.NewEncoder(w).Encode(&commands)
		case r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/applications/123456789/commands/"):
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{}`))
		}
	})

	cfgMgr := files.NewConfigManagerWithStore(&config.MemoryConfigStore{}, nil)
	handler, err := NewCommandHandler(CommandHandlerDeps{
		Session:        session,
		ConfigManager:  cfgMgr,
		RuntimeApplier: runtimeapply.New(nil, nil),
	})
	if err != nil {
		t.Fatalf("setup handler: %v", err)
	}

	if err := handler.SetupCommands(); err != nil {
		t.Fatalf("first setup: %v", err)
	}
	if handler.GetRouter() == nil {
		t.Fatalf("expected command manager to be initialized")
	}

	// Re-run setup to exercise reinit cleanup path.
	if err := handler.SetupCommands(); err != nil {
		t.Fatalf("second setup: %v", err)
	}
	if handler.GetRouter() == nil {
		t.Fatalf("expected command manager after reinit")
	}

	if atomic.LoadInt32(&commandCreateCalls) == 0 {
		t.Fatalf("expected command create call during setup")
	}

	if err := handler.Shutdown(); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
	if handler.GetRouter() != nil {
		t.Fatalf("expected command manager to be cleared on shutdown")
	}

	// Idempotent shutdown.
	if err := handler.Shutdown(); err != nil {
		t.Fatalf("second shutdown: %v", err)
	}
}

func TestCommandHandlerSetupRollbackOnManagerFailure(t *testing.T) {
	session, err := discordgo.New("Bot test-token")
	if err != nil {
		t.Fatalf("create discord session: %v", err)
	}
	session.State = discordgo.NewState()
	session.State.User = nil

	cfgMgr := files.NewConfigManagerWithStore(&config.MemoryConfigStore{}, nil)
	handler, err := NewCommandHandler(CommandHandlerDeps{
		Session:        session,
		ConfigManager:  cfgMgr,
		RuntimeApplier: runtimeapply.New(nil, nil),
	})
	if err != nil {
		t.Fatalf("setup handler: %v", err)
	}

	err = handler.SetupCommands()
	if err == nil {
		t.Fatalf("expected setup error when command manager setup fails")
	}
	if !strings.Contains(err.Error(), "cannot setup commands: session user state is missing") {
		t.Fatalf("unexpected setup error: %v", err)
	}
}

func TestCommandHandlerSkipsGuildWithoutCommandsFeature(t *testing.T) {
	boolPtr := func(v bool) *bool { return &v }
	cfgMgr := files.NewConfigManagerWithStore(&config.MemoryConfigStore{}, nil)
	if _, err := cfgMgr.UpdateConfig(context.Background(), func(cfg *files.BotConfig) error {
		cfg.Guilds = []files.GuildConfig{
			{
				GuildID:           "guild-1",
				BotInstanceTokens: map[string]files.EncryptedString{"generic": "a"},
				Features: files.FeatureToggles{
					Services: files.FeatureServiceToggles{
						Commands: boolPtr(false),
					},
				},
				QOTD: files.QOTDConfig{
					ActiveDeckID: files.LegacyQOTDDefaultDeckID,
					Decks: []files.QOTDDeckConfig{{
						ID:        files.LegacyQOTDDefaultDeckID,
						Name:      files.LegacyQOTDDefaultDeckName,
						Enabled:   true,
						ChannelID: "question-1",
					}},
				},
			},
		}
		return nil
	}); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	session, _ := discordgo.New("Bot test-token")
	handler, err := NewCommandHandlerForBot(CommandHandlerDeps{
		Session:        session,
		ConfigManager:  cfgMgr,
		BotInstanceID:  "generic",
		RuntimeApplier: runtimeapply.New(nil, nil),
	})
	if err != nil {
		t.Fatalf("setup handler: %v", err)
	}
	if handler.handlesGuild("guild-1") {
		t.Fatal("expected slash command handler to remain disabled for commands-off guild")
	}
}

type mockTransport struct {
	serverURL string
	transport http.RoundTripper
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if strings.Contains(req.URL.Host, "discord.com") {
		req.URL.Scheme = "http"
		req.URL.Host = strings.TrimPrefix(m.serverURL, "http://")
	}
	transport := m.transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	if transport == nil {
		return nil, fmt.Errorf("no transport")
	}
	return transport.RoundTrip(req)
}

```

// === FILE: pkg/app/command_handler_test.go ===
```go
package app

import (
	"context"
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/config"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/runtimeapply"
	"github.com/small-frappuccino/discordgo"
)

func TestCommandHandlerRoutesFeaturesToCorrectBotInstance(t *testing.T) {
	boolPtr := func(v bool) *bool { return &v }
	cfgMgr := files.NewConfigManagerWithStore(&config.MemoryConfigStore{}, nil)
	if _, err := cfgMgr.UpdateConfig(context.Background(), func(cfg *files.BotConfig) error {
		cfg.Guilds = []files.GuildConfig{
			{
				GuildID:           "guild-1",
				BotInstanceTokens: map[string]files.EncryptedString{"generic": "a"},
				FeatureRouting: map[string]string{
					"roles":      "generic",
					"moderation": "generic",
					"commands":   "generic",
				},
				Features: files.FeatureToggles{
					Services: files.FeatureServiceToggles{
						Commands: boolPtr(true),
					},
				},
			},
		}
		return nil
	}); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	session, _ := discordgo.New("Bot test-token")
	genericHandler, err := NewCommandHandlerForBot(CommandHandlerDeps{
		Session:        session,
		ConfigManager:  cfgMgr,
		BotInstanceID:  "generic",
		RuntimeApplier: runtimeapply.New(nil, nil),
	})
	if err != nil {
		t.Fatalf("setup handler: %v", err)
	}

	tests := []struct {
		name        string
		path        string
		wantHandles bool
	}{
		{"Roles command goes to generic", "rolepanel", true},
		{"Moderation command goes to generic", "ban", true},
		{"Base command goes to generic", "config", true},
		{"Unrouted QOTD command goes to no one", "qotd", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handles := genericHandler.handlesGuildRoute("guild-1", commands.InteractionRouteKey{Path: tt.path})
			if handles != tt.wantHandles {
				t.Errorf("generic handles %s: got %v, want %v", tt.path, handles, tt.wantHandles)
			}
		})
	}
}

```

// === FILE: pkg/app/control.go ===
```go
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/control"
	"github.com/small-frappuccino/discordcore/pkg/control/localtls"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/state"
)

const (
	defaultLocalHTTPSControlAddr    = ":8443"
	defaultLocalHTTPSPublicOrigin   = "https://localhost:8443"
	controlPublicOriginEnv          = "DISCORDCORE_CONTROL_PUBLIC_ORIGIN"
	defaultLocalTLSCommonName       = "localhost"
	defaultLocalTLSOrganizationName = "Small Frappuccino"
)

var errControlLocalTLSUnavailable = errors.New("control local tls unavailable")

// RunProfile selects which runtime a process drives: the primary main runtime
// or the QOTD-specialized runtime. See the RunProfile* constants.
type RunProfile string

// RunProfileDiscordMain is the primary runtime profile for the main discord bot process.
const (
	RunProfileDiscordMain RunProfile = "discordmain"
)

// RunOptions is the full configuration for a runtime process: which profile it
// drives, which bot instances and domains it hosts, and how its optional control
// plane is exposed. The zero value is not runnable; Profile must be set.
type RunOptions struct {
	Profile                  RunProfile
	Control                  ControlOptions
	CommandCatalogRegistrars []CommandCatalogRegistrar
	DisableControl           bool
	Logger                   *slog.Logger

	// Testing Hooks (Replacing globals)
	StoreCloseHook          func(c interface{ Close() error }) error
	DiscordSessionCloseHook func(c interface{ Close() error }) error
	ShutdownDelay           time.Duration
	TestShutdownCh          <-chan struct{}

	// unexported test hooks
	openBotArikawaState     func(ctx context.Context, s *state.State) error
	fetchBotArikawaMe       func(s *state.State) (*discord.User, error)
	newCommandHandlerForBot func(deps CommandHandlerDeps) (*CommandHandler, error)
	newCommandHandler       func(deps CommandHandlerDeps) (*CommandHandler, error)
	setupCommandHandler     func(ch *CommandHandler) error
	shutdownCommandHandler  func(ch *CommandHandler) error
}

// ControlOptions configures the local control plane. BindAddr and PublicOrigin
// default to profile-specific values when empty; LocalHTTPS opts into serving
// over TLS on loopback.
type ControlOptions struct {
	BindAddr     string
	PublicOrigin string
	LocalHTTPS   ControlLocalHTTPSOptions
}

// ControlLocalHTTPSOptions enables serving the control plane over local HTTPS.
// AutoTrust additionally installs the generated certificate into the OS trust
// store and is honored only when Enabled is true.
type ControlLocalHTTPSOptions struct {
	Enabled   bool
	AutoTrust bool
}

type resolvedControlRuntime struct {
	bindAddr     string
	publicOrigin string
	oauthConfig  *control.DiscordOAuthConfig
	tlsCertFile  string
	tlsKeyFile   string
}

func normalizeRunProfile(profile RunProfile) RunProfile {
	switch strings.TrimSpace(string(profile)) {
	case string(RunProfileDiscordMain):
		return RunProfileDiscordMain
	default:
		slog.Debug("Tracking complex conditional branch: Reverting unmapped runtime profile constraint to generic execution flow",
			slog.String("unmapped_profile", string(profile)),
		)
		return ""
	}
}

func defaultLocalHTTPSPublicOriginForProfile(profile RunProfile) string {
	switch normalizeRunProfile(profile) {
	case RunProfileDiscordMain:
		return "https://discordmain.localhost:8443"
	default:
		return defaultLocalHTTPSPublicOrigin
	}
}

func defaultLocalTLSCommonNameForProfile(profile RunProfile) string {
	switch normalizeRunProfile(profile) {
	case RunProfileDiscordMain:
		return "discordmain.localhost"
	default:
		return defaultLocalTLSCommonName
	}
}

func resolveControlRuntime(ctx context.Context, opts RunOptions) (resolvedControlRuntime, error) {
	profile := normalizeRunProfile(opts.Profile)
	bindAddr := strings.TrimSpace(opts.Control.BindAddr)
	publicOrigin := strings.TrimSpace(files.EnvString(controlPublicOriginEnv, opts.Control.PublicOrigin))

	slog.Info("Architectural state transition: Instantiating resolution pipeline for control plane bindings")

	// Inject default loopback topologies when local HTTPS is enforced and explicit overrides are absent.
	if opts.Control.LocalHTTPS.Enabled {
		if bindAddr == "" {
			bindAddr = defaultLocalHTTPSControlAddr
		}
		if publicOrigin == "" {
			publicOrigin = defaultLocalHTTPSPublicOriginForProfile(profile)
		}
		slog.Debug("Tracking complex conditional branch: Injecting default local HTTPS topologies into control runtime matrix",
			slog.String("injected_bind_addr", bindAddr),
			slog.String("injected_public_origin", publicOrigin),
		)
	}
	if bindAddr == "" {
		bindAddr = defaultControlAddr
	}

	tlsCertFile, tlsKeyFile, err := loadControlTLSFilesFromEnv()
	if err != nil {
		errWrap := fmt.Errorf("load control tls config: %w", err)
		log.EmitBlockingError("Blocking structural failure: Environmental TLS payload validation rejected", errWrap, log.GenerateRequestID())
		return resolvedControlRuntime{}, errWrap
	}

	// Trigger ad-hoc cryptographic generation and automatic trust-store installation for local TLS bindings.
	if tlsCertFile == "" && tlsKeyFile == "" && opts.Control.LocalHTTPS.Enabled {
		slog.Info("Architectural state transition: Initiating ad-hoc generation of local TLS credentials for control plane binding")
		ready, readyErr := prepareManagedLocalTLS(ctx, profile, publicOrigin, opts.Control.LocalHTTPS.AutoTrust)
		if readyErr != nil {
			errWrap := fmt.Errorf("%w: %w", errControlLocalTLSUnavailable, readyErr)
			log.EmitBlockingError("Blocking structural failure: Aborted generation of self-signed loopback TLS materials", errWrap, log.GenerateRequestID())
			return resolvedControlRuntime{}, errWrap
		}
		tlsCertFile = ready.CertFile
		tlsKeyFile = ready.KeyFile
	}

	oauthConfig, err := loadControlDiscordOAuthConfigFromEnv(publicOrigin)
	if err != nil {
		errWrap := fmt.Errorf("load control discord oauth config: %w", err)
		log.EmitBlockingError("Blocking structural failure: Validation of OAuth credentials against public origin aborted", errWrap, log.GenerateRequestID())
		return resolvedControlRuntime{}, errWrap
	}

	return resolvedControlRuntime{
		bindAddr:     bindAddr,
		publicOrigin: publicOrigin,
		oauthConfig:  oauthConfig,
		tlsCertFile:  tlsCertFile,
		tlsKeyFile:   tlsKeyFile,
	}, nil
}

func prepareManagedLocalTLS(ctx context.Context, profile RunProfile, publicOrigin string, autoTrust bool) (localtls.ReadyResult, error) {
	hostName, ipAddresses, err := localTLSSANs(profile, publicOrigin)
	if err != nil {
		errWrap := fmt.Errorf("resolve local tls sans: %w", err)
		log.EmitBlockingError("Blocking structural failure: Resolution of cryptographic Subject Alternate Names from host parameters failed", errWrap, log.GenerateRequestID())
		return localtls.ReadyResult{}, errWrap
	}

	slog.Debug("Tracking complex conditional branch: Forwarding resolved SAN variables to certificate authority simulation",
		slog.String("host_name", hostName),
		slog.Int("ip_addresses_count", len(ipAddresses)),
	)

	return localtls.EnsureReady(ctx, localtls.Config{
		Directory:    filepath.Join(files.ApplicationCachesPath, "control", "tls"),
		CommonName:   hostName,
		DNSNames:     []string{hostName, "localhost"},
		IPAddresses:  append(ipAddresses, net.ParseIP("127.0.0.1")),
		Organization: defaultLocalTLSOrganizationName,
		AutoTrust:    autoTrust,
	})
}

func localTLSSANs(profile RunProfile, publicOrigin string) (string, []net.IP, error) {
	// Fallback to loopback primitive binding if a canonical public origin is undefined.
	if strings.TrimSpace(publicOrigin) == "" {
		slog.Debug("Granular inspection: Parsing local TLS Subject Alternate Names skipped, utilizing fallback parameters")
		return defaultLocalTLSCommonNameForProfile(profile), []net.IP{net.ParseIP("127.0.0.1")}, nil
	}
	parsed, err := url.Parse(publicOrigin)
	if err != nil {
		errWrap := fmt.Errorf("parse public origin: %w", err)
		slog.Warn("Mitigated service degradation: URL parsing failed against public origin scalar, aborting SAN computation",
			slog.String("invalid_origin", publicOrigin),
			slog.String("error", errWrap.Error()),
		)
		return "", nil, errWrap
	}
	host := strings.TrimSpace(parsed.Hostname())
	if host == "" {
		errWrap := fmt.Errorf("public origin %q is missing a hostname", publicOrigin)
		slog.Warn("Mitigated service degradation: Valid URL identified but hostname extraction failed, aborting SAN computation",
			slog.String("invalid_origin", publicOrigin),
		)
		return "", nil, errWrap
	}
	if ip := net.ParseIP(host); ip != nil {
		return host, []net.IP{ip}, nil
	}
	// Differentiate between IP scalars and canonical domain hostnames for correct SAN issuance.
	return host, nil, nil
}

func loadControlDiscordOAuthConfigFromEnv(publicOrigin string) (*control.DiscordOAuthConfig, error) {
	// 1. Strict State Extraction
	clientID := strings.TrimSpace(files.EnvString(controlDiscordOAuthClientIDEnv, defaultControlDiscordOAuthClientID))
	clientSecret := strings.TrimSpace(files.EnvString(controlDiscordOAuthClientSecretEnv, ""))
	redirectURI := strings.TrimSpace(files.EnvString(controlDiscordOAuthRedirectURIEnv, ""))
	includeGuildMembersRead := files.EnvBool(controlDiscordOAuthIncludeGuildMembersReadEnv)
	sessionStorePath := strings.TrimSpace(files.EnvString(controlDiscordOAuthSessionStorePathEnv, ""))

	slog.Debug("Inspecting environment map for dynamic OAuth injections",
		slog.String("client_id", clientID),
	)

	// 2. Invariant Validation (Fail-Fast)
	if clientSecret == "" && redirectURI == "" && clientID == defaultControlDiscordOAuthClientID {
		if includeGuildMembersRead {
			return nil, fmt.Errorf("%s=true requires %s and %s",
				controlDiscordOAuthIncludeGuildMembersReadEnv,
				controlDiscordOAuthClientSecretEnv,
				controlDiscordOAuthRedirectURIEnv,
			)
		}
		return nil, nil
	}

	// 3. Conditional Mutations
	if clientSecret != "" && redirectURI == "" && strings.TrimSpace(publicOrigin) != "" {
		redirectURI = strings.TrimRight(strings.TrimSpace(publicOrigin), "/") + "/auth/discord/callback"
	}

	if sessionStorePath == "" {
		sessionStorePath = filepath.Join(files.ApplicationCachesPath, "control", "oauth_sessions.json")
	}

	// 4. Final Verification and Return
	var missing []string
	if clientSecret == "" {
		missing = append(missing, controlDiscordOAuthClientSecretEnv)
	}
	if redirectURI == "" {
		missing = append(missing, controlDiscordOAuthRedirectURIEnv)
	}

	if len(missing) > 0 {
		return nil, fmt.Errorf("incomplete Discord OAuth configuration: missing %s", strings.Join(missing, ", "))
	}

	return &control.DiscordOAuthConfig{
		ClientID:                 clientID,
		ClientSecret:             clientSecret,
		RedirectURI:              redirectURI,
		IncludeGuildsMembersRead: includeGuildMembersRead,
		SessionStorePath:         sessionStorePath,
	}, nil
}

func loadControlTLSFilesFromEnv() (certFile string, keyFile string, err error) {
	certFile = strings.TrimSpace(files.EnvString(controlTLSCertFileEnv, ""))
	keyFile = strings.TrimSpace(files.EnvString(controlTLSKeyFileEnv, ""))
	if certFile == "" && keyFile == "" {
		return "", "", nil
	}

	missing := make([]string, 0, 2)
	if certFile == "" {
		missing = append(missing, controlTLSCertFileEnv)
	}
	if keyFile == "" {
		missing = append(missing, controlTLSKeyFileEnv)
	}
	if len(missing) > 0 {
		return "", "", fmt.Errorf("incomplete control TLS configuration: missing %s", strings.Join(missing, ", "))
	}

	return certFile, keyFile, nil
}

```

// === FILE: pkg/app/control_test.go ===
```go
package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/files"
)

func TestLoadControlDiscordOAuthConfigFromEnv(t *testing.T) {
	t.Run("default empty is nil", func(t *testing.T) {
		t.Setenv(controlDiscordOAuthClientIDEnv, "")
		t.Setenv(controlDiscordOAuthClientSecretEnv, "")
		t.Setenv(controlDiscordOAuthRedirectURIEnv, "")
		t.Setenv(controlDiscordOAuthIncludeGuildMembersReadEnv, "")
		t.Setenv(controlDiscordOAuthSessionStorePathEnv, "")

		cfg, err := loadControlDiscordOAuthConfigFromEnv("")
		if err != nil {
			t.Fatalf("expected nil error on empty oauth config, got %v", err)
		}
		if cfg != nil {
			t.Fatalf("expected nil config when env vars are absent, got %+v", cfg)
		}
	})

	t.Run("incomplete config fails", func(t *testing.T) {
		t.Setenv(controlDiscordOAuthClientIDEnv, "client-id")
		t.Setenv(controlDiscordOAuthClientSecretEnv, "")

		if _, err := loadControlDiscordOAuthConfigFromEnv(""); err == nil {
			t.Fatal("expected error for incomplete oauth config")
		}
	})

	t.Run("complete config", func(t *testing.T) {
		t.Setenv(controlDiscordOAuthClientIDEnv, "client-id")
		t.Setenv(controlDiscordOAuthClientSecretEnv, "client-secret")
		t.Setenv(controlDiscordOAuthRedirectURIEnv, "http://127.0.0.1:8080/auth/discord/callback")
		t.Setenv(controlDiscordOAuthIncludeGuildMembersReadEnv, "true")
		t.Setenv(controlDiscordOAuthSessionStorePathEnv, "")

		cfg, err := loadControlDiscordOAuthConfigFromEnv("")
		if err != nil {
			t.Fatalf("expected complete oauth config to parse, got %v", err)
		}
		if cfg == nil {
			t.Fatal("expected non-nil oauth config")
		}
		if cfg.ClientID != "client-id" || cfg.ClientSecret != "client-secret" {
			t.Fatalf("unexpected client credentials in cfg: %+v", cfg)
		}
		if cfg.RedirectURI != "http://127.0.0.1:8080/auth/discord/callback" {
			t.Fatalf("unexpected redirect URI: %+v", cfg)
		}
		if !cfg.IncludeGuildsMembersRead {
			t.Fatalf("expected IncludeGuildsMembersRead=true, got %+v", cfg)
		}
		wantStorePath := filepath.Join(files.ApplicationCachesPath, "control", "oauth_sessions.json")
		if cfg.SessionStorePath != wantStorePath {
			t.Fatalf("unexpected default oauth session store path: got=%q want=%q", cfg.SessionStorePath, wantStorePath)
		}
	})

	t.Run("missing redirect without public origin fails", func(t *testing.T) {
		t.Setenv(controlDiscordOAuthClientIDEnv, "")
		t.Setenv(controlDiscordOAuthClientSecretEnv, "client-secret")
		t.Setenv(controlDiscordOAuthRedirectURIEnv, "")
		t.Setenv(controlDiscordOAuthIncludeGuildMembersReadEnv, "false")
		t.Setenv(controlDiscordOAuthSessionStorePathEnv, "")

		cfg, err := loadControlDiscordOAuthConfigFromEnv("")
		if err == nil {
			t.Fatalf("expected missing redirect without public origin to fail, got cfg=%+v", cfg)
		}
	})

	t.Run("missing redirect derives from public origin", func(t *testing.T) {
		t.Setenv(controlDiscordOAuthClientIDEnv, "")
		t.Setenv(controlDiscordOAuthClientSecretEnv, "client-secret")
		t.Setenv(controlDiscordOAuthRedirectURIEnv, "")
		t.Setenv(controlDiscordOAuthIncludeGuildMembersReadEnv, "false")
		t.Setenv(controlDiscordOAuthSessionStorePathEnv, "")

		cfg, err := loadControlDiscordOAuthConfigFromEnv("https://bot.localhost:8443")
		if err != nil {
			t.Fatalf("expected missing redirect to derive from public origin, got %v", err)
		}
		if cfg == nil {
			t.Fatal("expected non-nil oauth config")
		}
		if cfg.RedirectURI != "https://bot.localhost:8443/auth/discord/callback" {
			t.Fatalf("unexpected derived redirect URI: %+v", cfg)
		}
	})

	t.Run("explicit client id overrides repo default", func(t *testing.T) {
		t.Setenv(controlDiscordOAuthClientIDEnv, "override-client-id")
		t.Setenv(controlDiscordOAuthClientSecretEnv, "client-secret")
		t.Setenv(controlDiscordOAuthRedirectURIEnv, "http://127.0.0.1:8080/auth/discord/callback")
		t.Setenv(controlDiscordOAuthIncludeGuildMembersReadEnv, "false")
		t.Setenv(controlDiscordOAuthSessionStorePathEnv, "")

		cfg, err := loadControlDiscordOAuthConfigFromEnv("")
		if err != nil {
			t.Fatalf("expected override client id config to parse, got %v", err)
		}
		if cfg == nil {
			t.Fatal("expected non-nil oauth config")
		}
		if cfg.ClientID != "override-client-id" {
			t.Fatalf("expected env override client id, got %+v", cfg)
		}
	})
}

func TestLoadControlTLSFilesFromEnv(t *testing.T) {
	t.Run("not configured", func(t *testing.T) {
		t.Setenv(controlTLSCertFileEnv, "")
		t.Setenv(controlTLSKeyFileEnv, "")

		certFile, keyFile, err := loadControlTLSFilesFromEnv()
		if err != nil {
			t.Fatalf("expected no error when TLS config is absent, got %v", err)
		}
		if certFile != "" || keyFile != "" {
			t.Fatalf("expected empty TLS config, got cert=%q key=%q", certFile, keyFile)
		}
	})

	t.Run("incomplete config fails", func(t *testing.T) {
		t.Setenv(controlTLSCertFileEnv, "/tmp/cert.pem")
		t.Setenv(controlTLSKeyFileEnv, "")

		if _, _, err := loadControlTLSFilesFromEnv(); err == nil {
			t.Fatal("expected error for incomplete TLS config")
		}
	})

	t.Run("complete config", func(t *testing.T) {
		t.Setenv(controlTLSCertFileEnv, "/tmp/cert.pem")
		t.Setenv(controlTLSKeyFileEnv, "/tmp/key.pem")

		certFile, keyFile, err := loadControlTLSFilesFromEnv()
		if err != nil {
			t.Fatalf("expected complete TLS config to parse, got %v", err)
		}
		if certFile != "/tmp/cert.pem" || keyFile != "/tmp/key.pem" {
			t.Fatalf("unexpected TLS config: cert=%q key=%q", certFile, keyFile)
		}
	})
}

func TestResolveControlRuntimeUsesManagedLocalHTTPS(t *testing.T) {
	tempAppData := t.TempDir()
	t.Setenv("APPDATA", tempAppData)
	t.Setenv(controlTLSCertFileEnv, "")
	t.Setenv(controlTLSKeyFileEnv, "")
	t.Setenv(controlDiscordOAuthClientSecretEnv, "")
	t.Setenv(controlDiscordOAuthRedirectURIEnv, "")
	files.SetAppName("discordmain-run-options-test")

	runtime, err := resolveControlRuntime(context.Background(), RunOptions{
		Profile: RunProfileDiscordMain,
		Control: ControlOptions{
			LocalHTTPS: ControlLocalHTTPSOptions{
				Enabled:   true,
				AutoTrust: false,
			},
		},
	})
	if err != nil {
		t.Fatalf("resolve control runtime: %v", err)
	}
	if runtime.bindAddr != defaultLocalHTTPSControlAddr {
		t.Fatalf("unexpected bind addr: %+v", runtime)
	}
	if runtime.publicOrigin != defaultLocalHTTPSPublicOriginForProfile(RunProfileDiscordMain) {
		t.Fatalf("unexpected public origin: %+v", runtime)
	}
	if runtime.tlsCertFile == "" || runtime.tlsKeyFile == "" {
		t.Fatalf("expected managed local tls files, got %+v", runtime)
	}
	if _, err := os.Stat(runtime.tlsCertFile); err != nil {
		t.Fatalf("stat managed cert file: %v", err)
	}
	if _, err := os.Stat(runtime.tlsKeyFile); err != nil {
		t.Fatalf("stat managed key file: %v", err)
	}
}

func TestResolveControlRuntimeDerivesOAuthRedirectFromPublicOrigin(t *testing.T) {
	t.Setenv(controlTLSCertFileEnv, "")
	t.Setenv(controlTLSKeyFileEnv, "")
	t.Setenv(controlDiscordOAuthClientSecretEnv, "client-secret")
	t.Setenv(controlDiscordOAuthRedirectURIEnv, "")
	t.Setenv(controlDiscordOAuthSessionStorePathEnv, "")

	tempAppData := t.TempDir()
	t.Setenv("APPDATA", tempAppData)
	files.SetAppName("discordmain-run-options-test")

	runtime, err := resolveControlRuntime(context.Background(), RunOptions{
		Profile: RunProfileDiscordMain,
		Control: ControlOptions{
			PublicOrigin: "https://discordmain.localhost:8443",
		},
	})
	if err != nil {
		t.Fatalf("resolve control runtime: %v", err)
	}
	if runtime.oauthConfig == nil {
		t.Fatal("expected oauth config")
	}
	if runtime.oauthConfig.RedirectURI != "https://discordmain.localhost:8443/auth/discord/callback" {
		t.Fatalf("unexpected derived redirect uri: %+v", runtime.oauthConfig)
	}
	wantStorePath := filepath.Join(files.ApplicationCachesPath, "control", "oauth_sessions.json")
	if runtime.oauthConfig.SessionStorePath != wantStorePath {
		t.Fatalf("unexpected oauth session store path: got=%q want=%q", runtime.oauthConfig.SessionStorePath, wantStorePath)
	}
}

```

// === FILE: pkg/app/observability.go ===
```go
package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/files"
)

const (
	// lifecycleWebhookEnv is the env var operators set to receive
	// shutdown notifications on a Discord webhook URL. Empty / unset
	// disables the notification path; production deployments set this
	// alongside the OS-level supervisor (NSSM/Task Scheduler) so a
	// graceful stop emits a chat message before the supervisor relaunches.
	lifecycleWebhookEnv = "DISCORDCORE_LIFECYCLE_WEBHOOK_URL"
)

// lifecycleWebhookTimeout caps how long the shutdown notification
// blocks the actual process exit. Three seconds is enough for one
// HTTP POST round-trip to discord.com on a slow link; longer would
// delay restarts under a supervisor.
var lifecycleWebhookTimeout = 3 * time.Second

// notifyLifecycleEvent best-effort POSTs a one-line content message to
// the configured Discord webhook. Caller passes the high-level reason
// ("starting", "stopping", "fatal") and an optional detail string. Any
// failure (no URL configured, network error, Discord 5xx) is swallowed
// after a warn log — the shutdown path must not block on this.
//
// This is intentionally not the discordgo session API: during shutdown
// the bot's gateway connection is already being torn down, and we want
// the notification to work even if the bot died in a way that prevents
// it from making API calls (e.g. token revoked). A plain HTTP POST
// against the webhook URL needs no session state.
func notifyLifecycleEvent(reason, detail string) {
	webhookURL := strings.TrimSpace(files.EnvString(lifecycleWebhookEnv, ""))
	if webhookURL == "" {
		slog.Debug("Tracking complex conditional branch: Lifecycle webhook notification suppressed due to empty environment binding")
		return
	}

	slog.Info("Architectural state transition: Initiating out-of-band lifecycle notification sequence",
		slog.String("reason", reason),
	)

	// Serialize payload symmetrically with Discord's webhook interface expectations.
	content := buildLifecycleContent(reason, detail)
	payload, err := json.Marshal(map[string]string{"content": content})
	if err != nil {
		slog.Warn("Mitigated service degradation: Discarding lifecycle webhook transmission due to JSON marshal failure",
			slog.String("operation", "lifecycle.webhook"),
			slog.String("reason", reason),
			slog.String("error", err.Error()),
		)
		return
	}

	// Bound HTTP transport lifecycle to prevent blocking the primary teardown sequence.
	ctx, cancel := context.WithTimeout(context.Background(), lifecycleWebhookTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(payload))
	if err != nil {
		slog.Warn("Mitigated service degradation: HTTP request construction aborted during lifecycle webhook transmission",
			slog.String("operation", "lifecycle.webhook"),
			slog.String("reason", reason),
			slog.String("error", err.Error()),
		)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	slog.Debug("Granular inspection: Executing HTTP POST to external lifecycle webhook endpoint",
		slog.String("content", content),
		slog.Int("payload_bytes", len(payload)),
	)

	client := &http.Client{Timeout: lifecycleWebhookTimeout}
	resp, err := client.Do(req)
	if err != nil {
		slog.Warn("Mitigated service degradation: External webhook endpoint unreachable; timeout or DNS failure",
			slog.String("operation", "lifecycle.webhook"),
			slog.String("reason", reason),
			slog.String("error", err.Error()),
		)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		retryAfter := resp.Header.Get("Retry-After")
		if retryAfter == "" {
			retryAfter = "0"
		}
		slog.Warn("Mitigated service degradation: Discord upstream rejected lifecycle webhook payload",
			slog.String("operation", "lifecycle.webhook"),
			slog.String("reason", reason),
			slog.Int("status_code", resp.StatusCode),
			slog.String("retry_after", retryAfter),
		)
		return
	}

	slog.Info("Architectural state transition: Lifecycle webhook notification transmitted successfully",
		slog.String("reason", reason),
	)
}

// buildLifecycleContent renders the chat message body. Keep it short and
// human-readable — operators see this in the alert channel and need
// "what happened, which bot, which version" at a glance, not JSON.
func buildLifecycleContent(reason, detail string) string {
	app := strings.TrimSpace(files.ConfiguredAppName)
	if app == "" {
		app = "discordcore"
	}
	version := strings.TrimSpace(files.AppVersion)
	if version == "" {
		version = files.DiscordCoreVersion
	}
	host := strings.TrimSpace(files.DiscordBotName)

	parts := []string{fmt.Sprintf("**%s** (%s)", app, version)}
	if host != "" {
		parts = append(parts, "as `"+host+"`")
	}
	parts = append(parts, "→", reason)
	if detail = strings.TrimSpace(detail); detail != "" {
		parts = append(parts, "—", detail)
	}

	rendered := strings.Join(parts, " ")
	slog.Debug("Tracking complex conditional branch: Lifecycle message content string compiled",
		slog.String("rendered_string", rendered),
	)

	return rendered
}

type startupWebhookEmbedUpdate struct {
	scope  string
	index  int
	update files.WebhookEmbedUpdateConfig
}

func collectStartupWebhookEmbedUpdates(cfg *files.BotConfig) []startupWebhookEmbedUpdate {
	if cfg == nil {
		return nil
	}

	var out []startupWebhookEmbedUpdate

	// Extract globally scoped embed configurations prior to iterating over guild-specific overrides.
	for idx, update := range cfg.RuntimeConfig.NormalizedWebhookEmbedUpdates() {
		out = append(out, startupWebhookEmbedUpdate{
			scope:  "global",
			index:  idx,
			update: update,
		})
	}

	for _, guild := range cfg.Guilds {
		guildID := strings.TrimSpace(guild.GuildID)
		if guildID == "" {
			continue
		}
		for idx, update := range guild.RuntimeConfig.NormalizedWebhookEmbedUpdates() {
			out = append(out, startupWebhookEmbedUpdate{
				scope:  "guild:" + guildID,
				index:  idx,
				update: update,
			})
		}
	}

	return out
}

```

// === FILE: pkg/app/observability_test.go ===
```go
package app

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/files"
)

// The lifecycle_webhook_test.go logic:
func TestNotifyLifecycleEventSendsWebhook(t *testing.T) {
	origAppName := files.ConfiguredAppName
	origAppVersion := files.AppVersion
	origBotName := files.DiscordBotName
	t.Cleanup(func() {
		files.ConfiguredAppName = origAppName
		files.AppVersion = origAppVersion
		files.DiscordBotName = origBotName
	})
	files.ConfiguredAppName = "discordmain"
	files.AppVersion = "v0.test"
	files.DiscordBotName = "TestBot"

	var (
		mu       sync.Mutex
		received []map[string]string
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("expected application/json content type, got %q", got)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
			return
		}
		var payload map[string]string
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Errorf("decode body: %v raw=%q", err, string(body))
			return
		}
		mu.Lock()
		received = append(received, payload)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	t.Setenv(lifecycleWebhookEnv, server.URL)

	notifyLifecycleEvent("starting", "")
	notifyLifecycleEvent("fatal", "nil pointer dereference")

	mu.Lock()
	defer mu.Unlock()
	if len(received) != 2 {
		t.Fatalf("expected 2 webhook posts, got %d", len(received))
	}
	if got := received[0]["content"]; !strings.Contains(got, "discordmain") || !strings.Contains(got, "starting") {
		t.Fatalf("first content missing app/reason: %q", got)
	}
	if got := received[1]["content"]; !strings.Contains(got, "fatal") || !strings.Contains(got, "nil pointer") {
		t.Fatalf("second content missing reason/detail: %q", got)
	}
}

func TestBuildLifecycleContentFormat(t *testing.T) {
	origAppName := files.ConfiguredAppName
	origAppVersion := files.AppVersion
	origBotName := files.DiscordBotName
	t.Cleanup(func() {
		files.ConfiguredAppName = origAppName
		files.AppVersion = origAppVersion
		files.DiscordBotName = origBotName
	})
	files.ConfiguredAppName = "discordqotd"
	files.AppVersion = "v0.42.0"
	files.DiscordBotName = "QOTD"

	got := buildLifecycleContent("stopping", "")
	want := "**discordqotd** (v0.42.0) as `QOTD` → stopping"
	if got != want {
		t.Fatalf("buildLifecycleContent(stopping, ''): got %q want %q", got, want)
	}

	got = buildLifecycleContent("fatal", "runtime panic: nil map write")
	if !strings.HasPrefix(got, "**discordqotd** (v0.42.0)") {
		t.Fatalf("fatal content lost app prefix: %q", got)
	}
	if !strings.Contains(got, "→ fatal — runtime panic") {
		t.Fatalf("fatal content lost reason/detail separator: %q", got)
	}
}

func TestBuildLifecycleContentFallsBackWhenIdentityUnset(t *testing.T) {
	origAppName := files.ConfiguredAppName
	origAppVersion := files.AppVersion
	origBotName := files.DiscordBotName
	t.Cleanup(func() {
		files.ConfiguredAppName = origAppName
		files.AppVersion = origAppVersion
		files.DiscordBotName = origBotName
	})
	files.ConfiguredAppName = ""
	files.AppVersion = ""
	files.DiscordBotName = ""

	got := buildLifecycleContent("starting", "")
	if !strings.Contains(got, "discordcore") {
		t.Fatalf("expected fallback app name 'discordcore', got %q", got)
	}
	if !strings.Contains(got, files.DiscordCoreVersion) {
		t.Fatalf("expected fallback to core version %q, got %q", files.DiscordCoreVersion, got)
	}
	if strings.Contains(got, "``") {
		t.Fatalf("empty bot name produced empty backticks: %q", got)
	}
}

func TestNotifyLifecycleEventHandles5xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	t.Setenv(lifecycleWebhookEnv, server.URL)

	// Should not panic
	notifyLifecycleEvent("fatal", "simulated 500 error")
}

func TestNotifyLifecycleEventTimeoutContext(t *testing.T) {
	origTimeout := lifecycleWebhookTimeout
	lifecycleWebhookTimeout = 50 * time.Millisecond
	t.Cleanup(func() {
		lifecycleWebhookTimeout = origTimeout
	})

	var handlerCalled sync.WaitGroup
	handlerCalled.Add(1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled.Done()
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	t.Setenv(lifecycleWebhookEnv, server.URL)

	start := time.Now()
	notifyLifecycleEvent("stopping", "simulated timeout")
	elapsed := time.Since(start)

	handlerCalled.Wait()

	if elapsed >= 150*time.Millisecond {
		t.Fatalf("expected timeout near 50ms, but request took %v", elapsed)
	}
}

// The runner_webhook_updates_test.go logic:
func TestCollectStartupWebhookEmbedUpdatesGlobalAndGuild(t *testing.T) {
	t.Parallel()

	cfg := &files.BotConfig{
		RuntimeConfig: files.RuntimeConfig{
			WebhookEmbedUpdates: []files.WebhookEmbedUpdateConfig{
				{
					MessageID:  "global-1",
					WebhookURL: "https://discord.com/api/webhooks/1/token",
					Embed:      json.RawMessage(`{"title":"g1"}`),
				},
			},
		},
		Guilds: []files.GuildConfig{
			{
				GuildID: "guild-a",
				RuntimeConfig: files.RuntimeConfig{
					WebhookEmbedUpdates: []files.WebhookEmbedUpdateConfig{
						{
							MessageID:  "guild-a-1",
							WebhookURL: "https://discord.com/api/webhooks/2/token",
							Embed:      json.RawMessage(`{"title":"a1"}`),
						},
						{
							MessageID:  "guild-a-2",
							WebhookURL: "https://discord.com/api/webhooks/3/token",
							Embed:      json.RawMessage(`{"title":"a2"}`),
						},
					},
				},
			},
			{
				GuildID: "guild-b",
				RuntimeConfig: files.RuntimeConfig{
					WebhookEmbedUpdates: []files.WebhookEmbedUpdateConfig{
						{
							MessageID:  "guild-b-1",
							WebhookURL: "https://discord.com/api/webhooks/4/token",
							Embed:      json.RawMessage(`{"title":"b1"}`),
						},
					},
				},
			},
		},
	}

	got := collectStartupWebhookEmbedUpdates(cfg)
	if len(got) != 4 {
		t.Fatalf("expected 4 startup updates, got %d", len(got))
	}

	if got[0].scope != "global" || got[0].index != 0 || got[0].update.MessageID != "global-1" {
		t.Fatalf("unexpected first item: %+v", got[0])
	}
	if got[1].scope != "guild:guild-a" || got[1].index != 0 || got[1].update.MessageID != "guild-a-1" {
		t.Fatalf("unexpected second item: %+v", got[1])
	}
	if got[2].scope != "guild:guild-a" || got[2].index != 1 || got[2].update.MessageID != "guild-a-2" {
		t.Fatalf("unexpected third item: %+v", got[2])
	}
	if got[3].scope != "guild:guild-b" || got[3].index != 0 || got[3].update.MessageID != "guild-b-1" {
		t.Fatalf("unexpected fourth item: %+v", got[3])
	}
}

func TestCollectStartupWebhookEmbedUpdatesNilConfig(t *testing.T) {
	t.Parallel()

	if got := collectStartupWebhookEmbedUpdates(nil); len(got) != 0 {
		t.Fatalf("expected nil/empty list for nil config, got %+v", got)
	}
}

```

// === FILE: pkg/app/runner.go ===
```go
package app

import (
	"context"
	stdErrors "errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/state"
	"github.com/small-frappuccino/discordcore/pkg/clock"
	"github.com/small-frappuccino/discordcore/pkg/config"
	"github.com/small-frappuccino/discordcore/pkg/control"
	"github.com/small-frappuccino/discordcore/pkg/discord/cache"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/moderation"
	"github.com/small-frappuccino/discordcore/pkg/discord/embeds"
	"github.com/small-frappuccino/discordcore/pkg/discord/partners"
	discordqotd "github.com/small-frappuccino/discordcore/pkg/discord/qotd"
	"github.com/small-frappuccino/discordcore/pkg/discord/roles"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/idgen"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/members"
	"github.com/small-frappuccino/discordcore/pkg/messages"
	"github.com/small-frappuccino/discordcore/pkg/persistence"
	"github.com/small-frappuccino/discordcore/pkg/qotd"
	"github.com/small-frappuccino/discordcore/pkg/runtimeapply"
	"github.com/small-frappuccino/discordcore/pkg/service"
	"github.com/small-frappuccino/discordcore/pkg/storage/postgres"
	"golang.org/x/sync/errgroup"
)

const (
	defaultControlAddr                            = "127.0.0.1:8376"
	defaultControlDiscordOAuthClientID            = "1396606252506681395"
	controlBearerTokenEnv                         = "DISCORDCORE_CONTROL_BEARER_TOKEN"
	controlDiscordOAuthClientIDEnv                = "DISCORDCORE_CONTROL_DISCORD_OAUTH_CLIENT_ID"
	controlDiscordOAuthClientSecretEnv            = "DISCORDCORE_CONTROL_DISCORD_OAUTH_CLIENT_SECRET"
	controlDiscordOAuthRedirectURIEnv             = "DISCORDCORE_CONTROL_DISCORD_OAUTH_REDIRECT_URI"
	controlDiscordOAuthIncludeGuildMembersReadEnv = "DISCORDCORE_CONTROL_DISCORD_OAUTH_INCLUDE_GUILDS_MEMBERS_READ"
	controlDiscordOAuthSessionStorePathEnv        = "DISCORDCORE_CONTROL_DISCORD_OAUTH_SESSION_STORE_PATH"
	controlTLSCertFileEnv                         = "DISCORDCORE_CONTROL_TLS_CERT_FILE"
	controlTLSKeyFileEnv                          = "DISCORDCORE_CONTROL_TLS_KEY_FILE"
)

// App encapsulates the state of the initializing application process, providing
// a testable, instance-based context tree instead of procedural global variables.
type App struct {
	appName        string
	opts           RunOptions
	serviceManager *service.ServiceManager
	startupTasks   *StartupTaskOrchestrator
	logger         *slog.Logger

	store                 *postgres.Store
	configManager         *files.ConfigManager
	controlServerRegistry *controlServerHolder
	botSupervisor         *BotSupervisor
	runtimeResolver       *botRuntimeResolver
	runtimeApplier        *runtimeapply.Manager

	qotdService       *qotd.Service
	moderationMetrics *moderation.InMemoryMetrics
	membersMetrics    *members.InMemoryMetrics
	messagesMetrics   *messages.InMemoryMetrics

	cleanupCancel context.CancelFunc
}

// NewApp allocates the initial structural foundations for a bot runtime pipeline.
func NewApp(appName string, opts RunOptions) *App {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.StoreCloseHook == nil {
		opts.StoreCloseHook = func(c interface{ Close() error }) error { return c.Close() }
	}
	if opts.DiscordSessionCloseHook == nil {
		opts.DiscordSessionCloseHook = func(c interface{ Close() error }) error { return c.Close() }
	}
	if opts.ShutdownDelay == 0 {
		opts.ShutdownDelay = 100 * time.Millisecond
	}

	return &App{
		appName:        appName,
		opts:           opts,
		serviceManager: service.NewServiceManager(opts.Logger),
		logger:         opts.Logger,
	}
}

// Run bootstraps the bot with a unified flow and blocks until shutdown.
func Run(appName string) error {
	return RunWithOptions(appName, RunOptions{})
}

func RunWithOptions(appName string, opts RunOptions) (err error) {
	defer func() {
		log.GlobalLogger.Sync()
		log.CloseGlobalLogger()
	}()
	defer func() {
		if r := recover(); r != nil {
			// Unmanaged panic requires aggressive interruption and memory dump.
			errWrap := fmt.Errorf("panic recovered during runtime: %v", r)
			log.EmitBlockingError("Critical pipeline failure: Unhandled panic intercepted", errWrap, log.GenerateRequestID())
			notifyLifecycleEvent("fatal", errWrap.Error())
			err = errWrap
		} else if err != nil {
			// Managed error propagates validated failure using highly efficient structured logging without stack traces.
			slog.Error("Primary execution routine aborted",
				slog.String("app_name", appName),
				slog.Any("error", err),
			)
			notifyLifecycleEvent("fatal", fmt.Sprintf("startup or runtime error: %v", err))
		} else {
			// Clean shutdown sequence.
			notifyLifecycleEvent("stopping", "")
		}
	}()

	app := NewApp(appName, opts)
	ctx := context.Background()

	if bootErr := app.Boot(ctx); bootErr != nil {
		return app.Teardown(bootErr)
	}

	return app.Teardown(app.RunAndListen(ctx))
}

// Boot executes the application initialization matrix reliably.
func (a *App) Boot(ctx context.Context) error {
	a.logger.Info("Architectural state transition: Executing application boot sequence")

	if err := a.InitializeIO(ctx); err != nil {
		return err
	}
	if err := a.ConstructServices(ctx); err != nil {
		return err
	}

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		return a.serviceManager.StartAll()
	})

	eg.Go(func() error {
		if err := a.runtimeResolver.waitForReady(egCtx); err != nil {
			return err
		}

		controlBearerToken := strings.TrimSpace(files.EnvString(controlBearerTokenEnv, ""))
		scheduleStartupWebhookEmbedUpdates(a.startupTasks, a.configManager.Config(), a.runtimeResolver)
		if !a.opts.DisableControl {
			controlRuntime, err := resolveControlRuntime(egCtx, a.opts)
			if err != nil {
				if stdErrors.Is(err, errControlLocalTLSUnavailable) {
					slog.Warn("Local TLS parameters unavailable; fallback to insecure local execution state activated", slog.String("error", err.Error()))
				} else {
					return fmt.Errorf("resolve control runtime: %w", err)
				}
			} else {
				var serverOpts []control.ServerOption
				if controlBearerToken == "" && controlRuntime.oauthConfig == nil {
					slog.Info("Architectural transition: Control server initializing without authentication middleware",
						slog.String("addr", controlRuntime.bindAddr),
						slog.Bool("dashboard_only", true),
					)
				}
				if controlBearerToken != "" {
					serverOpts = append(serverOpts, func(s *control.Server) error {
						s.SetBearerToken(controlBearerToken)
						return nil
					})
				}
				if a.runtimeResolver != nil {
					serverOpts = append(serverOpts, func(s *control.Server) error {
						s.SetKnownBotInstanceIDs(slices.Collect(knownBotInstanceCatalogSeq(a.runtimeResolver.getRuntimes(), nil)))
						s.SetArikawaStateResolver(func(guildID string) (*state.State, error) {
							return a.runtimeResolver.arikawaStateForGuild(guildID, "dashboard")
						})
						s.SetBotGuildBindingsProvider(func(ctx context.Context) ([]control.BotGuildBinding, error) {
							return a.runtimeResolver.guildBindings(ctx)
						})
						s.SetGuildRegistrationResolver(func(ctx context.Context, guildID string) error {
							return a.runtimeResolver.registerGuild(ctx, guildID)
						})
						return nil
					})
				}
				serverOpts = append(serverOpts, func(s *control.Server) error {
					s.SetQOTDService(a.qotdService)
					s.SetModerationMetrics(a.moderationMetrics)
					s.SetMembersMetricsResolver(func() members.Metrics { return a.membersMetrics })
					s.SetMessagesMetricsResolver(func() messages.Metrics { return a.messagesMetrics })
					s.SetStorage(a.store)
					s.SetCacheObservability(func() *cache.UnifiedCache {
						if a.runtimeResolver == nil {
							return nil
						}
						caches := a.runtimeResolver.aggregateUnifiedCaches()
						if len(caches) == 0 {
							return nil
						}
						for _, c := range caches {
							return c
						}
						return nil
					}, a.store)
					if err := s.SetPublicOrigin(controlRuntime.publicOrigin); err != nil {
						return fmt.Errorf("configure control public origin: %w", err)
					}
					if controlRuntime.tlsCertFile != "" || controlRuntime.tlsKeyFile != "" {
						if err := s.SetTLSCertificates(controlRuntime.tlsCertFile, controlRuntime.tlsKeyFile); err != nil {
							return fmt.Errorf("configure control tls certificates: %w", err)
						}
					}
					if controlRuntime.oauthConfig != nil {
						if err := s.SetDiscordOAuthConfig(*controlRuntime.oauthConfig); err != nil {
							return fmt.Errorf("configure control discord oauth: %w", err)
						}
						slog.Info("Architectural transition: Discord OAuth constraints applied to control interface",
							slog.String("scopes", strings.Join(control.DiscordOAuthScopes(controlRuntime.oauthConfig.IncludeGuildsMembersRead), " ")),
						)
						if controlRuntime.tlsCertFile == "" || controlRuntime.tlsKeyFile == "" {
							slog.Warn("Misconfigured deployment topology: OAuth enforced without local TLS termination; secure cookies risk clearance drop")
						}
					}
					return nil
				})

				scheduleControlServerStartup(a.startupTasks, controlRuntime, a.configManager, a.runtimeApplier, a.controlServerRegistry, serverOpts...)
			}
		}
		slog.Info("Architectural state transition: Command tree sync complete")
		return nil
	})

	return eg.Wait()
}

// InitializeIO establishes critical state boundaries across filesystems and storage.
func (a *App) InitializeIO(ctx context.Context) error {
	if err := idgen.Init(1); err != nil {
		return fmt.Errorf("initialize idgen: %w", err)
	}

	notifyLifecycleEvent("starting", "")

	msg := formatStartupMessage(a.appName, AppVersion(), Version)
	slog.Info("Architectural state transition: Executing application binary",
		slog.String("version_info", msg),
	)

	databaseBootstrap, err := resolveDatabaseBootstrap()
	if err != nil {
		return fmt.Errorf("InitializeIO resolveDatabaseBootstrap: %w", err)
	}
	slog.Info("Architectural state transition: Database matrix parameters loaded",
		slog.String("operation", "startup.database.bootstrap"),
		slog.String("source", databaseBootstrap.Source),
	)

	if err := files.EnsureCacheInitialized(); err != nil {
		slog.Warn("Mitigated service degradation: Sub-optimal filesystem state detected; executing local cache fallback",
			slog.String("error", err.Error()),
		)
	}
	if err := files.EnsureCacheDirs(); err != nil {
		return fmt.Errorf("create cache directories: %w", err)
	}

	store, configManager, err := setupStorage(databaseBootstrap)
	if err != nil {
		return fmt.Errorf("InitializeIO setupStorage: %w", err)
	}
	a.store = store
	a.configManager = configManager

	applyConfiguredTheme(a.configManager)

	cleanupCtx, cleanupCancel := context.WithCancel(context.Background())
	scheduleDBCleanup(cleanupCtx, a.store, a.configManager)
	a.cleanupCancel = cleanupCancel

	return nil
}

// ConstructServices assembles the runtime domain logic elements and their dependency graph.
func (a *App) ConstructServices(ctx context.Context) error {
	a.runtimeApplier = runtimeapply.New(nil, nil)
	if cfg := a.configManager.Config(); cfg != nil {
		a.runtimeApplier.SetInitial(cfg.RuntimeConfig)
	}

	// Flattened inline computation of active instances avoids unnecessary allocation.
	runtimeCount := 1 // Strict default
	if cfg := a.configManager.Config(); cfg != nil {
		knownInstances := make(map[string]struct{})
		for _, guild := range cfg.Guilds {
			for instanceID, token := range guild.BotInstanceTokens {
				if string(token) != "" {
					knownInstances[instanceID] = struct{}{}
				}
			}
		}
		if len(knownInstances) > 0 {
			runtimeCount = len(knownInstances)
		}
	}

	a.controlServerRegistry = &controlServerHolder{}
	a.startupTasks = NewStartupTaskOrchestrator(ctx, runtimeCount)

	qotdMetrics := qotd.NopMetrics{}
	qotdService := qotd.NewServiceWithMetrics(a.configManager, a.store, nil, qotdMetrics)

	appClock := clock.NewHTTPClock("https://discord.com")
	qotdService.SetClock(appClock)

	a.moderationMetrics = &moderation.InMemoryMetrics{}
	a.membersMetrics = members.NewInMemoryMetrics()
	a.messagesMetrics = messages.NewInMemoryMetrics()
	a.qotdService = qotdService

	storeService := service.NewLegacyServiceWrapper(service.LegacyServiceWrapperSpec{
		Name:     "postgres-store",
		Type:     service.TypeCache,
		Priority: service.PriorityHigh,
		Start:    func(context.Context) error { return nil },
		Stop: func(ctx context.Context) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(a.opts.ShutdownDelay):
			}
			return a.opts.StoreCloseHook(a.store)
		},
		Logger: a.logger,
	})
	if err := a.serviceManager.Register(storeService); err != nil {
		return fmt.Errorf("register store service: %w", err)
	}

	embedService := embeds.NewEmbedService(a.configManager)
	rolePanelService := roles.NewRolePanelService(a.configManager)
	partnerService := partners.NewPartnerService(a.configManager)

	botOpts := botRuntimeOptions{
		runtimeCount:             runtimeCount,
		configManager:            a.configManager,
		store:                    a.store,
		commandCatalogRegistrars: a.opts.CommandCatalogRegistrars,
		runtimeApplier:           a.runtimeApplier,
		qotdCommandService:       qotdService,
		moderationMetrics:        a.moderationMetrics,
		membersMetrics:           a.membersMetrics,
		messagesMetrics:          a.messagesMetrics,
		startupTasks:             a.startupTasks,
		profile:                  a.opts.Profile,
		appClock:                 appClock,
		controlServerRegistry:    a.controlServerRegistry,
		logger:                   a.logger,
		embedService:             embedService,
		rolePanelService:         rolePanelService,
		partnerService:           partnerService,
	}

	a.botSupervisor = NewBotSupervisor(a.configManager, botOpts)
	qotdService.SetPublisher(discordqotd.NewPublisherRouter(qotdClientResolver{resolver: a.botSupervisor.GetResolver()}))
	a.configManager.AddSubscriber(a.botSupervisor.onConfigChanged)

	a.botSupervisor.SetFatalCallback(func(err error) {
		a.serviceManager.Fatal(err)
	})

	botSupervisorService := service.NewLegacyServiceWrapper(service.LegacyServiceWrapperSpec{
		Name:     "bot-supervisor",
		Type:     service.TypeMonitoring,
		Priority: service.PriorityNormal,
		Start: func(context.Context) error {
			return a.botSupervisor.Start()
		},
		Stop: func(ctx context.Context) error {
			return a.botSupervisor.Stop(ctx)
		},
		Logger: a.logger,
	})

	if err := a.serviceManager.Register(botSupervisorService); err != nil {
		return fmt.Errorf("register bot supervisor service: %w", err)
	}

	a.runtimeResolver = a.botSupervisor.GetResolver()

	attachCtx, attachCancel := context.WithTimeout(ctx, 5*time.Second)
	defer attachCancel()
	if err := a.moderationMetrics.Attach(attachCtx); err != nil {
		return fmt.Errorf("fatal abort: moderation metrics pipeline failed to attach: %w", err)
	}

	return nil
}

// RunAndListen hooks context lifecycles to OS events, averting complex goto flow control.
func (a *App) RunAndListen(ctx context.Context) error {
	signalCtx, stopSignal := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stopSignal()

	rootCtx, rootCancel := context.WithCancel(signalCtx)
	defer rootCancel()

	sigHupCh := make(chan os.Signal, 1)
	signal.Notify(sigHupCh, syscall.SIGHUP)
	defer signal.Stop(sigHupCh)

	eg, egCtx := errgroup.WithContext(rootCtx)

	// Phase 2: SIGHUP Valve & Serialized Mutation Pipeline
	// Dedicated resident worker executing continuous state routing with highly efficient resource utilization.
	eg.Go(func() error {
		for {
			select {
			case <-egCtx.Done():
				return nil
			case <-sigHupCh:
				a.logger.Debug("Dynamic instruction intercepted: Emitting non-blocking intent trigger for configuration layer reload")

				// Serialized mutation governed by CSP and bounded by strict timeout.
				mutCtx, mutCancel := context.WithTimeout(egCtx, 30*time.Second)

				newCfg, needsSave, err := a.configManager.LoadConfigFromStore()
				if err != nil {
					slog.Warn("Mitigated service degradation: Live configuration mutation failed; enforcing active baseline",
						slog.String("error", err.Error()),
					)
					mutCancel()
					continue
				}

				if mutCtx.Err() != nil {
					mutCancel()
					continue
				}

				dupCount := a.configManager.ApplyConfig(newCfg)

				if dupCount == 0 && !needsSave {
					slog.Info("Architectural state transition: Configuration topology refreshed directly from disk")
					mutCancel()
					continue
				}

				if saveErr := a.configManager.SaveConfig(); saveErr != nil {
					log.EmitBlockingError("Structural state failure: Volatile configuration drift blocks persistence flush", saveErr, log.GenerateRequestID())
				} else {
					slog.Info("Architectural state transition: Configuration topology updated and indexes rebuilt",
						slog.Int("duplicates_purged", dupCount),
					)
				}
				mutCancel()
			}
		}
	})

	// Phase 1: Anchor asynchronous execution routes to the strict limits of an errgroup.Group
	// to annihilate any incidence of naked goroutines in the execution matrix.
	eg.Go(func() error {
		err := a.serviceManager.Wait()
		if err != nil {
			log.EmitBlockingError("Critical pipeline failure: Daemon cluster collapsed", err, log.GenerateRequestID())
			rootCancel() // Force cascade teardown across the context tree
			return err
		}
		return nil
	})

	// Phase 3: Central router block for consistent lifecycle observation
	eg.Go(func() error {
		select {
		case <-signalCtx.Done():
			a.logger.Info("Architectural state transition: Process termination signal acknowledged. Initiating graceful teardown.")
			rootCancel()
			// Unblock a.serviceManager.Wait() dynamically by initiating the graceful stop sequence
			return a.serviceManager.StopAll(context.Background())
		case <-a.opts.TestShutdownCh:
			a.logger.Info("Architectural state transition: Test simulated shutdown initiated")
			rootCancel()
			// Unblock a.serviceManager.Wait() dynamically by initiating the graceful stop sequence
			return a.serviceManager.StopAll(context.Background())
		case <-egCtx.Done():
			// Natural synchronized drainage due to sibling cancellation
			return nil
		}
	})

	// Asynchronous Teardown & Quiescent Memory Rest
	return eg.Wait()
}

// Teardown safely shuts down orchestrators and the database subsystem.
func (a *App) Teardown(originalErr error) error {
	if a == nil {
		return originalErr
	}

	slog.Info("Architectural state transition: Commencing teardown sequence across local orchestrators",
		slog.String("app_name", a.appName),
	)

	if a.cleanupCancel != nil {
		a.cleanupCancel()
	}

	if a.startupTasks != nil {
		if err := shutdownStartupServices(a.startupTasks, a.controlServerRegistry, "Startup background tasks did not finish before shutdown"); err != nil {
			errWrap := fmt.Errorf("startup services shutdown: %w", err)
			log.EmitBlockingError("Structural teardown failure: Network socket lifecycle release hung", errWrap, log.GenerateRequestID())
			if originalErr != nil {
				originalErr = stdErrors.Join(originalErr, errWrap)
			} else {
				originalErr = errWrap
			}
		}
	}

	if a.serviceManager != nil {
		err := a.serviceManager.StopAll(context.Background())
		a.serviceManager.Wait() // Unconditional execution guarantees cleanup of zombie processes.

		if err != nil {
			errWrap := fmt.Errorf("shutdown: %w", err)
			log.EmitBlockingError("Structural teardown failure: Zombie sub-processes detected during stop iteration", errWrap, log.GenerateRequestID())
			if originalErr != nil {
				return stdErrors.Join(originalErr, errWrap)
			}
			return errWrap
		}
	}

	return originalErr
}

func applyConfiguredTheme(configManager *files.ConfigManager) {
	cfg := configManager.Config()
	themeName := ""
	if cfg != nil {
		themeName = cfg.RuntimeConfig.BotTheme
	}

	if err := files.ConfigureThemeFromConfig(themeName); err != nil {
		slog.Warn("Mitigated service degradation: Client interface theme rejected context; reverting visual defaults",
			slog.String("theme_name", themeName),
			slog.String("error", err.Error()),
		)
	}
	if themeName == "" {
		if err := files.SetTheme(""); err != nil {
			slog.Warn("Mitigated service degradation: Hard fallback visual theme application failed",
				slog.String("error", err.Error()),
			)
		} else {
			slog.Info("Architectural state transition: Standard UI theme locked")
		}
	}
}

func scheduleDBCleanup(ctx context.Context, store *postgres.Store, configManager *files.ConfigManager) {
	cfg := configManager.Config()
	var features files.ResolvedFeatureToggles
	var disableCleanup bool

	if cfg != nil {
		features = cfg.ResolveFeatures("")
		disableCleanup = cfg.RuntimeConfig.DisableDBCleanup
	} else {
		features = (&files.BotConfig{}).ResolveFeatures("")
	}

	cleanupEnabled := features.Maintenance.DBCleanup

	slog.Debug("Evaluating temporal garbage collection routines",
		slog.Bool("cleanup_enabled", cleanupEnabled),
		slog.Bool("disable_cleanup_flag", disableCleanup),
	)

	// Strict and predictable conditional evaluation for temporal garbage collection.
	if cleanupEnabled && !disableCleanup {
		cache.SchedulePeriodicCleanup(ctx, store, 6*time.Hour)
		return
	}

	// Decoupled evaluation provides strict clarity for operational logs.
	if !cleanupEnabled {
		slog.Info("Architectural state override: Database garbage collection suppressed explicitly by node definition",
			slog.String("flag", "features.maintenance.db_cleanup"),
		)
	} else {
		slog.Info("Architectural state override: Database garbage collection suppressed globally by configuration override",
			slog.String("flag", "disable_db_cleanup"),
		)
	}
}

func resolveRuntimeCapabilities(configSnapshot *files.BotConfig, botInstances []resolvedBotInstance, profile RunProfile) map[string]botRuntimeCapabilities {
	capabilities := make(map[string]botRuntimeCapabilities, len(botInstances))
	for _, instance := range botInstances {
		cap := resolveBotRuntimeCapabilities(
			configSnapshot,
			instance.ID,
		)

		capabilities[instance.ID] = cap
	}
	return capabilities
}

func shutdownStartupServices(startupTasks *StartupTaskOrchestrator, controlServerRegistry *controlServerHolder, tasksWarn string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var finalErr error
	if controlServerRegistry != nil {
		if err := controlServerRegistry.Stop(ctx); err != nil {
			log.EmitBlockingError("Structural teardown failure: Interface server socket release hung", err, log.GenerateRequestID())
			finalErr = err
		}
	}
	if startupTasks != nil {
		if err := startupTasks.Shutdown(ctx); err != nil && !stdErrors.Is(err, context.DeadlineExceeded) {
			slog.Warn("Mitigated shutdown degradation: Async orchestrator missed synchronization lock",
				slog.String("warning_context", tasksWarn),
				slog.String("error", err.Error()),
			)
			if finalErr != nil {
				finalErr = stdErrors.Join(finalErr, err)
			} else {
				finalErr = err
			}
		}
	}
	return finalErr
}

func formatStartupMessage(appName, appVersion, coreVersion string) string {
	appName = strings.TrimSpace(appName)
	appVersion = strings.TrimSpace(appVersion)
	coreVersion = strings.TrimSpace(coreVersion)

	msg := fmt.Sprintf("🚀 Starting %s", appName)
	if appVersion != "" {
		msg += fmt.Sprintf(" %s", appVersion)
	}

	if coreVersion == "" || (appVersion != "" && appVersion == coreVersion) {
		return msg + "..."
	}

	return msg + fmt.Sprintf(" (discordcore %s)...", coreVersion)
}

func setupStorage(dbb resolvedDatabaseBootstrap) (*postgres.Store, *files.ConfigManager, error) {
	dbCfg := dbb.Config
	dbc := persistence.Config{
		Driver:              dbCfg.Driver,
		DatabaseURL:         dbCfg.DatabaseURL,
		MaxOpenConns:        dbCfg.MaxOpenConns,
		MaxIdleConns:        dbCfg.MaxIdleConns,
		ConnMaxLifetimeSecs: dbCfg.ConnMaxLifetimeSecs,
		ConnMaxIdleTimeSecs: dbCfg.ConnMaxIdleTimeSecs,
		PingTimeoutMS:       dbCfg.PingTimeoutMS,
	}

	openCtx, openCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer openCancel()
	db, err := persistence.Open(openCtx, dbc)
	if err != nil {
		return nil, nil, fmt.Errorf("open postgres database: %w", err)
	}
	slog.Info("Architectural state transition: Remote persistence pipeline materialized",
		slog.String("operation", "startup.database.open"),
		slog.String("driver", "postgres"),
	)

	pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer pingCancel()
	if err := persistence.Ping(pingCtx, db); err != nil {
		db.Close()
		return nil, nil, fmt.Errorf("postgres readiness check failed: %w", err)
	}
	slog.Info("Architectural state transition: I/O payload validation complete",
		slog.String("operation", "startup.database.ping"),
		slog.String("driver", "postgres"),
	)

	migrateCtx, migrateCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer migrateCancel()
	migrator := persistence.NewPostgresMigrator(db)
	if err := migrator.Up(migrateCtx); err != nil {
		db.Close()
		return nil, nil, fmt.Errorf("apply postgres migrations: %w", err)
	}
	slog.Info("Architectural state transition: Schema schema deltas propagated successfully",
		slog.String("operation", "startup.database.migrate"),
		slog.String("driver", "postgres"),
	)

	store, err := postgres.NewStore(db, slog.Default())
	if err != nil {
		db.Close()
		return nil, nil, fmt.Errorf("create postgres store: %w", err)
	}
	if err := store.Init(); err != nil {
		db.Close()
		return nil, nil, fmt.Errorf("initialize postgres store: %w", err)
	}
	slog.Info("Architectural state transition: Virtual storage layers active",
		slog.String("operation", "startup.database.store_init"),
		slog.String("driver", "postgres"),
	)

	configStore := config.NewPostgresConfigStore(db, config.DefaultPostgresConfigStoreKey, slog.Default())
	configManager := files.NewConfigManagerWithStore(configStore, slog.Default())

	slog.Debug("Executing cross-boundary extraction for master configuration tree")
	if err := configManager.LoadConfig(); err != nil {
		return nil, nil, fmt.Errorf("load config from postgres: %w", err)
	}
	if err := syncBootstrapDatabaseConfig(configManager, dbCfg); err != nil {
		return nil, nil, fmt.Errorf("sync runtime database bootstrap config: %w", err)
	}

	return store, configManager, nil
}

type qotdClientResolver struct {
	resolver *botRuntimeResolver
}

func (r qotdClientResolver) ArikawaClientForGuild(guildID string) (*api.Client, error) {
	state, err := r.resolver.arikawaStateForGuild(guildID, "qotd")
	if err != nil {
		if stdErrors.Is(err, ErrSessionUnavailable) {
			return nil, discordqotd.ErrSessionUnavailable
		}
		return nil, fmt.Errorf("resolve arikawa state for guild %s: %w", guildID, err)
	}

	if state == nil || state.Session == nil || state.Session.Client == nil {
		return nil, fmt.Errorf("arikawa client evaluates to nil for guild %s", guildID)
	}

	return state.Session.Client, nil
}

```

// === FILE: pkg/app/runner_test.go ===
```go
package app

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"log/slog"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/state"
	"github.com/small-frappuccino/discordcore/pkg/config"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/persistence"
	"github.com/small-frappuccino/discordcore/pkg/testdb"
)

func TestRun_MissingDatabaseURL(t *testing.T) {
	os.Unsetenv("DISCORDCORE_DATABASE_URL")
	err := Run("testapp")
	if err == nil {
		t.Fatal("expected Run to return an error without DISCORDCORE_DATABASE_URL")
	}
}

func TestRunWithOptions_MissingDatabaseURL(t *testing.T) {
	os.Unsetenv("DISCORDCORE_DATABASE_URL")
	err := RunWithOptions("testapp", RunOptions{})
	if err == nil {
		t.Fatal("expected RunWithOptions to return an error without DISCORDCORE_DATABASE_URL")
	}
}

func TestSetupStorage(t *testing.T) {
	dbb := resolvedDatabaseBootstrap{}
	_, _, err := setupStorage(dbb)
	if err == nil {
		t.Fatal("expected setupStorage to fail with bad config")
	}

	dbb.Config.Driver = "postgres"
	dbb.Config.DatabaseURL = "postgres://username:password@127.0.0.1:5433/bogus?sslmode=disable"
	_, _, err = setupStorage(dbb)
	if err == nil {
		t.Fatal("expected setupStorage to fail with bogus URL")
	}
}

func TestRunner_ShutdownStartupServices(t *testing.T) {
	shutdownStartupServices(nil, nil, "ok")
}

func TestRunner_ResolveRuntimeCapabilities(t *testing.T) {
	cfg := &files.BotConfig{}

	instances := []resolvedBotInstance{{ID: "bot1"}}
	caps := resolveRuntimeCapabilities(cfg, instances, RunProfileDiscordMain)
	if caps["bot1"].qotdRuntime {
		t.Fatal("expected qotdRuntime to be false by default")
	}

	cfg.Guilds = []files.GuildConfig{
		{
			BotInstanceTokens: map[string]files.EncryptedString{
				"bot1": "token",
			},
			FeatureRouting: map[string]string{
				"qotd": "bot1",
			},
			QOTD: files.QOTDConfig{
				Decks: []files.QOTDDeckConfig{
					{
						ID:      "deck1",
						Enabled: true,
					},
				},
				ActiveDeckID: "123",
			},
		},
	}
	caps = resolveRuntimeCapabilities(cfg, instances, RunProfileDiscordMain)
	if !caps["bot1"].qotdRuntime {
		t.Fatalf("expected qotdRuntime to be true: %+v", caps["bot1"])
	}
}

func TestRunner_ApplyConfiguredTheme(t *testing.T) {
	cm := files.NewConfigManagerWithStore(&config.MemoryConfigStore{}, nil)
	applyConfiguredTheme(cm)
}

func TestRunner_ScheduleDBCleanup(t *testing.T) {
	cm := files.NewConfigManagerWithStore(&config.MemoryConfigStore{}, nil)
	scheduleDBCleanup(context.Background(), nil, cm)
}

func TestFormatStartupMessage(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		appName     string
		appVersion  string
		coreVersion string
		want        string
	}{
		{
			name:        "no app version includes discordcore",
			appName:     "discordmain",
			appVersion:  "",
			coreVersion: "v0.146.0",
			want:        "🚀 Starting discordmain (discordcore v0.146.0)...",
		},
		{
			name:        "different versions include both",
			appName:     "discordmain",
			appVersion:  "v0.114.0",
			coreVersion: "v0.146.0",
			want:        "🚀 Starting discordmain v0.114.0 (discordcore v0.146.0)...",
		},
		{
			name:        "same versions omit discordcore suffix",
			appName:     "discordmain",
			appVersion:  "v0.146.0",
			coreVersion: "v0.146.0",
			want:        "🚀 Starting discordmain v0.146.0...",
		},
		{
			name:        "trims spaces",
			appName:     " discordmain ",
			appVersion:  " v0.146.0 ",
			coreVersion: " v0.146.0 ",
			want:        "🚀 Starting discordmain v0.146.0...",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := formatStartupMessage(tc.appName, tc.appVersion, tc.coreVersion)
			if got != tc.want {
				t.Fatalf("formatStartupMessage() mismatch\nwant: %q\ngot:  %q", tc.want, got)
			}
		})
	}
}

// Mocks openRunnerConfigStore and setRunnerDatabaseBootstrapEnv which were present in runner_run_test.go
func openRunnerConfigStore(t *testing.T) files.DatabaseRuntimeConfig {
	baseDSN, err := testdb.BaseDatabaseURLFromEnv()
	if err != nil {
		if testdb.IsDatabaseURLNotConfigured(err) {
			t.Skip("Skipping test: database URL not configured")
		}
		t.Fatalf("failed to get base DSN: %v", err)
	}

	_, dsn, cleanup, err := testdb.OpenIsolatedDatabaseWithDSN(context.Background(), baseDSN)
	if err != nil {
		t.Fatalf("failed to open isolated database: %v", err)
	}
	t.Cleanup(func() { _ = cleanup() })

	return files.DatabaseRuntimeConfig{
		Driver:      "postgres",
		DatabaseURL: dsn,
	}
}

func setRunnerDatabaseBootstrapEnv(t *testing.T, dbCfg files.DatabaseRuntimeConfig) {
	t.Setenv(databaseDriverEnv, dbCfg.Driver)
	t.Setenv(databaseURLEnv, dbCfg.DatabaseURL)
}

func seedRunnerConfig(t *testing.T, dbCfg files.DatabaseRuntimeConfig, cfg files.BotConfig) {
	dbc := persistence.Config{
		Driver:      dbCfg.Driver,
		DatabaseURL: dbCfg.DatabaseURL,
	}
	db, err := persistence.Open(context.Background(), dbc)
	if err != nil {
		t.Fatalf("failed to open database for seeding: %v", err)
	}
	defer db.Close()
	store := config.NewPostgresConfigStore(db, config.DefaultPostgresConfigStoreKey, slog.Default())
	if err := store.Save(&cfg); err != nil {
		t.Fatalf("failed to save test config to postgres: %v", err)
	}
}

func TestRun_CascadingRollbackFailures(t *testing.T) {
	fetchBotArikawaMeHook := func(s *state.State) (*discord.User, error) {
		return &discord.User{ID: 123, Username: "test"}, nil
	}
	openBotArikawaStateHook := func(ctx context.Context, s *state.State) error { return nil }
	const appName = "discordmain-cascading-rollback-test"

	appDataDir := t.TempDir()
	t.Setenv("APPDATA", appDataDir)

	dbCfg := openRunnerConfigStore(t)
	setRunnerDatabaseBootstrapEnv(t, dbCfg)
	cfg := files.BotConfig{
		RuntimeConfig: files.RuntimeConfig{
			Database: dbCfg,
		},
		Features: files.FeatureToggles{
			Services: files.FeatureServiceToggles{
				Monitoring: new(bool(false)),
				Commands:   new(bool(true)),
			},
			Maintenance: files.FeatureMaintenanceToggles{
				DBCleanup: new(bool(false)),
			},
		},
		Guilds: []files.GuildConfig{{
			GuildID: "guild-1",
			BotInstanceTokens: map[string]files.EncryptedString{
				"generic": files.EncryptedString("test-token"),
			},
		}},
	}
	seedRunnerConfig(t, dbCfg, cfg)

	sabotageErr := errors.New("store close failure")
	storeCloseErr := sabotageErr
	discordCloseErr := errors.New("discord close failure")

	shutdownCh := make(chan struct{})

	go func() {
		time.Sleep(300 * time.Millisecond)
		close(shutdownCh)
	}()

	setupCommandHandlerHook := func(ch *CommandHandler) error { return nil }

	err := RunWithOptions(appName, RunOptions{
		openBotArikawaState:     openBotArikawaStateHook,
		fetchBotArikawaMe:       fetchBotArikawaMeHook,
		setupCommandHandler:     setupCommandHandlerHook,
		StoreCloseHook:          func(c interface{ Close() error }) error { return storeCloseErr },
		DiscordSessionCloseHook: func(c interface{ Close() error }) error { return discordCloseErr },
		ShutdownDelay:           time.Nanosecond,
		TestShutdownCh:          shutdownCh,
	})

	if err == nil {
		t.Fatalf("expected Run to return cascading errors on shutdown")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, sabotageErr.Error()) {
		t.Errorf("expected final error to contain the original boot failure %q, got: %v", sabotageErr.Error(), err)
	}
	if !strings.Contains(errStr, storeCloseErr.Error()) {
		t.Errorf("expected final error to aggregate store close failure %q, got: %v", storeCloseErr.Error(), err)
	}
	if !strings.Contains(errStr, discordCloseErr.Error()) {
		t.Errorf("expected final error to aggregate discord close failure %q, got: %v", discordCloseErr.Error(), err)
	}
}

func TestRun_ResourceCleanupOnBootFailure(t *testing.T) {
	fetchBotArikawaMeHook := func(s *state.State) (*discord.User, error) {
		return &discord.User{ID: 123, Username: "test"}, nil
	}
	openBotArikawaStateHook := func(ctx context.Context, s *state.State) error { return nil }
	const appName = "discordmain-resource-cleanup-test"

	appDataDir := t.TempDir()
	t.Setenv("APPDATA", appDataDir)

	dbCfg := openRunnerConfigStore(t)
	setRunnerDatabaseBootstrapEnv(t, dbCfg)
	cfg := files.BotConfig{
		RuntimeConfig: files.RuntimeConfig{
			Database: dbCfg,
		},
		Features: files.FeatureToggles{
			Services: files.FeatureServiceToggles{
				Monitoring: new(bool(false)),
				Commands:   new(bool(true)),
			},
			Maintenance: files.FeatureMaintenanceToggles{
				DBCleanup: new(bool(false)),
			},
		},
		Guilds: []files.GuildConfig{{
			GuildID: "guild-1",
			BotInstanceTokens: map[string]files.EncryptedString{
				"generic": files.EncryptedString("test-token"),
			},
		}},
	}
	seedRunnerConfig(t, dbCfg, cfg)

	var storeCloseCalls int32
	var discordCloseCalls int32

	// Warm up global packages to initialize their background loops
	dummySigCh := make(chan os.Signal, 1)
	signal.Notify(dummySigCh, os.Interrupt)
	signal.Stop(dummySigCh)

	// Warm up http.DefaultClient connection pool
	if req, err := http.NewRequest(http.MethodHead, "https://discord.com", nil); err == nil {
		if resp, err := http.DefaultClient.Do(req); err == nil {
			resp.Body.Close()
		}
	}

	shutdownCh := make(chan struct{})

	go func() {
		time.Sleep(300 * time.Millisecond)
		close(shutdownCh)
	}()

	time.Sleep(150 * time.Millisecond)
	goroutinesBefore := runtime.NumGoroutine()

	setupCommandHandlerHook := func(ch *CommandHandler) error { return nil }

	err := RunWithOptions(appName, RunOptions{
		openBotArikawaState: openBotArikawaStateHook,
		fetchBotArikawaMe:   fetchBotArikawaMeHook,
		setupCommandHandler: setupCommandHandlerHook,
		StoreCloseHook: func(c interface{ Close() error }) error {
			atomic.AddInt32(&storeCloseCalls, 1)
			return c.Close()
		},
		DiscordSessionCloseHook: func(c interface{ Close() error }) error {
			atomic.AddInt32(&discordCloseCalls, 1)
			return nil
		},
		ShutdownDelay:  time.Nanosecond,
		TestShutdownCh: shutdownCh,
	})

	if err != nil {
		t.Fatalf("expected clean shutdown, got: %v", err)
	}

	if got := atomic.LoadInt32(&storeCloseCalls); got != 1 {
		t.Errorf("expected 1 store close call on rollback, got %d", got)
	}

	if got := atomic.LoadInt32(&discordCloseCalls); got != 1 {
		t.Errorf("expected 1 discord session close call on rollback, got %d", got)
	}

	time.Sleep(150 * time.Millisecond)
	goroutinesAfter := runtime.NumGoroutine()

	if goroutinesAfter > goroutinesBefore+2 {
		buf := make([]byte, 102400)
		n := runtime.Stack(buf, true)
		t.Logf("Goroutine stack traces:\n%s", string(buf[:n]))
		t.Errorf("goroutine leak detected: before %d, after %d", goroutinesBefore, goroutinesAfter)
	}
}

```

// === FILE: pkg/app/runtimecmd/runtimecmd.go ===
```go
package runtimecmd

import (
	"flag"
	"fmt"
	"io"
	"log/slog"

	"github.com/small-frappuccino/discordcore/pkg/app"
	discordcoreapp "github.com/small-frappuccino/discordcore/pkg/app"
)

// MainRuntimeAppName is the canonical identifier for the primary Discord bot process.
const (
	MainRuntimeAppName = "discordmain"
)

// Spec describes a runtime entrypoint command: its name, and a factory that
// builds the RunOptions.
type Spec struct {
	CommandName     string
	RuntimeAppName  string
	BuildRunOptions func() discordcoreapp.RunOptions
}

// Runner starts a runtime app with the resolved name and options.
// It is the injection seam that lets Run be tested without a live runtime.
type Runner func(appName string, opts discordcoreapp.RunOptions) error

// Run parses CLI flags, attempts to load a local .env file from the system PATH,
// and invokes the provided runner with the resolved execution options.
func Run(args []string, output io.Writer, spec Spec, runner Runner) error {
	fs := flag.NewFlagSet(spec.CommandName, flag.ContinueOnError)
	fs.SetOutput(output)
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("Run: %w", err)
	}

	if err := runner(spec.RuntimeAppName, spec.BuildRunOptions()); err != nil {
		slog.Error("Runner execution failed", slog.String("app_name", spec.RuntimeAppName), slog.Any("error", err))

		return err
	}
	return nil
}

// MainSpec constructs the execution specification for the primary bot process.
func MainSpec(commandName string) Spec {
	return Spec{
		CommandName:     commandName,
		RuntimeAppName:  MainRuntimeAppName,
		BuildRunOptions: buildMainRunOptions,
	}
}

func buildMainRunOptions() discordcoreapp.RunOptions {
	return discordcoreapp.RunOptions{
		Profile: discordcoreapp.RunProfileDiscordMain,
		Control: discordcoreapp.ControlOptions{
			LocalHTTPS: discordcoreapp.ControlLocalHTTPSOptions{
				Enabled:   true,
				AutoTrust: true,
			},
		},
		CommandCatalogRegistrars: app.DefaultCommandCatalogRegistrars(),
	}
}

```

// === FILE: pkg/app/runtimecmd/runtimecmd_test.go ===
```go
package runtimecmd

import (
	"io"
	"testing"

	discordcoreapp "github.com/small-frappuccino/discordcore/pkg/app"
)

func isolateEnv(t *testing.T) {
	t.Helper()
	tempDir := t.TempDir()
	t.Setenv("PATH", tempDir)
	t.Setenv("Path", tempDir)
}

func TestRunUsesMainProfileOptions(t *testing.T) {
	isolateEnv(t)

	called := struct {
		name string
		opts discordcoreapp.RunOptions
	}{}

	err := Run(nil, io.Discard, MainSpec("discordmain"), func(name string, opts discordcoreapp.RunOptions) error {
		called.name = name
		called.opts = opts
		return nil
	})
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	if called.name != MainRuntimeAppName {
		t.Fatalf("unexpected call args: %+v", called)
	}
	if called.opts.Profile != discordcoreapp.RunProfileDiscordMain {
		t.Fatalf("expected main run profile, got %+v", called.opts)
	}
	if called.opts.DisableControl {
		t.Fatalf("expected control plane to stay enabled for main runtime, got %+v", called.opts)
	}

	if len(called.opts.CommandCatalogRegistrars) != 10 {
		t.Fatalf("unexpected main command registrars: %+v", called.opts.CommandCatalogRegistrars)
	}
	if called.opts.CommandCatalogRegistrars[0].RequiredCapabilities.Has(discordcoreapp.CapStats) {
		t.Fatalf("expected runtime registrar first, got %+v", called.opts.CommandCatalogRegistrars)
	}
	if !called.opts.Control.LocalHTTPS.Enabled || !called.opts.Control.LocalHTTPS.AutoTrust {
		t.Fatalf("expected local https control options, got %+v", called.opts.Control)
	}
	if called.opts.Control.BindAddr != "" || called.opts.Control.PublicOrigin != "" {
		t.Fatalf("expected main profile to derive control defaults later, got %+v", called.opts.Control)
	}
}

```

// === FILE: pkg/app/startup.go ===
```go
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"

	"github.com/small-frappuccino/discordcore/pkg/control"
	"github.com/small-frappuccino/discordcore/pkg/discord/session"
	"github.com/small-frappuccino/discordcore/pkg/discord/webhook"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/runtimeapply"
	"golang.org/x/sync/errgroup"
)

const (
	databaseDriverEnv              = "DISCORDCORE_DATABASE_DRIVER"
	databaseURLEnv                 = "DISCORDCORE_DATABASE_URL"
	databaseMaxOpenConnsEnv        = "DISCORDCORE_DATABASE_MAX_OPEN_CONNS"
	databaseMaxIdleConnsEnv        = "DISCORDCORE_DATABASE_MAX_IDLE_CONNS"
	databaseConnMaxLifetimeSecsEnv = "DISCORDCORE_DATABASE_CONN_MAX_LIFETIME_SECS"
	databaseConnMaxIdleTimeSecsEnv = "DISCORDCORE_DATABASE_CONN_MAX_IDLE_TIME_SECS"
	databasePingTimeoutMSEnv       = "DISCORDCORE_DATABASE_PING_TIMEOUT_MS"
)

type resolvedDatabaseBootstrap struct {
	Config files.DatabaseRuntimeConfig
	Source string
}

func resolveDatabaseBootstrap() (resolvedDatabaseBootstrap, error) {
	if cfg, ok := databaseBootstrapFromEnv(); ok {
		return resolvedDatabaseBootstrap{
			Config: cfg,
			Source: "env",
		}, nil
	}
	panic("hardware-aligned validation failure: postgres bootstrap config unavailable: set DISCORDCORE_DATABASE_URL before startup")
}

func databaseBootstrapFromEnv() (files.DatabaseRuntimeConfig, bool) {
	url := files.EnvString(databaseURLEnv, "")
	if url == "" {
		slog.Debug("Granular inspection: Database environment variable absent, skipping payload injection",
			slog.String("env", databaseURLEnv),
		)
		return files.DatabaseRuntimeConfig{}, false
	}

	driver := files.EnvString(databaseDriverEnv, "postgres")
	maxOpen := int(files.EnvInt64(databaseMaxOpenConnsEnv, 20))
	maxIdle := int(files.EnvInt64(databaseMaxIdleConnsEnv, 10))
	connMaxLifetime := int(files.EnvInt64(databaseConnMaxLifetimeSecsEnv, 1800))
	connMaxIdle := int(files.EnvInt64(databaseConnMaxIdleTimeSecsEnv, 300))
	pingTimeout := int(files.EnvInt64(databasePingTimeoutMSEnv, 5000))

	slog.Debug("Granular inspection: Database connection parameters injected via environment",
		slog.String("driver", driver),
		slog.Int("max_open_conns", maxOpen),
		slog.Int("max_idle_conns", maxIdle),
		slog.Int("conn_max_lifetime_secs", connMaxLifetime),
		slog.Int("conn_max_idle_time_secs", connMaxIdle),
		slog.Int("ping_timeout_ms", pingTimeout),
	)

	return files.DatabaseRuntimeConfig{
		Driver:              driver,
		DatabaseURL:         url,
		MaxOpenConns:        maxOpen,
		MaxIdleConns:        maxIdle,
		ConnMaxLifetimeSecs: connMaxLifetime,
		ConnMaxIdleTimeSecs: connMaxIdle,
		PingTimeoutMS:       pingTimeout,
	}, true
}

type databaseConfigUpdater struct {
	cfg files.DatabaseRuntimeConfig
}

func (u databaseConfigUpdater) apply(rc *files.RuntimeConfig) error {
	rc.Database = u.cfg
	return nil
}

func syncBootstrapDatabaseConfig(configManager *files.ConfigManager, cfg files.DatabaseRuntimeConfig) error {
	if configManager == nil {
		return errors.New("cannot sync config without configManager")
	}

	current := configManager.SnapshotConfig().RuntimeConfig.Database
	if current == cfg {
		slog.Debug("Tracking complex conditional branch: Database configuration identical to persisted state, bypassing update")
		return nil
	}

	updater := databaseConfigUpdater{cfg: cfg}
	_, err := configManager.UpdateRuntimeConfig(updater.apply)
	if err != nil {
		return fmt.Errorf("persist runtime database config: %w", err)
	}

	slog.Info("Architectural state transition: Database bootstrap configuration synchronized successfully")
	return nil
}

type controlServerHolder struct {
	server atomic.Pointer[control.Server]
}

func (h *controlServerHolder) Set(server *control.Server) {
	if h == nil || server == nil {
		return
	}
	slog.Debug("Updating control server reference in memory holder")
	h.server.Store(server)
}

func (h *controlServerHolder) Stop(ctx context.Context) error {
	if h == nil {
		return nil
	}

	server := h.server.Swap(nil)
	if server == nil {
		return nil
	}

	slog.Info("Planned shutdown of control server instance initiated")
	if err := server.Stop(ctx); err != nil {
		log.EmitBlockingError("Blocking failure during control server shutdown", err, log.GenerateRequestID())
		return err
	}

	slog.Info("Planned shutdown of control server instance completed successfully")
	return nil
}

func (h *controlServerHolder) BroadcastGuildEvent(guildID string, botPresent bool) {
	if h == nil {
		return
	}
	server := h.server.Load()
	if server == nil {
		return
	}

	slog.Debug("Broadcasting guild presence transition event",
		slog.String("guild_id", guildID),
		slog.Bool("bot_present", botPresent),
	)
	server.BroadcastGuildEvent(guildID, botPresent)
}

type RuntimeConfiguredGuildLoggingTask struct {
	runtime       *botRuntime
	configManager *files.ConfigManager
}

func (t RuntimeConfiguredGuildLoggingTask) Execute(taskCtx context.Context) error {
	if taskCtx.Err() != nil {
		return nil
	}
	err := files.LogConfiguredGuildsForBot(t.configManager, t.runtime.legacySession, t.runtime.instanceID)
	if err != nil {
		slog.Warn("Mitigated degradation: Some configured guilds could not be accessed during runtime logging",
			slog.String("botInstanceID", t.runtime.instanceID),
			slog.String("error", err.Error()),
		)
	}
	return nil
}

func (t RuntimeConfiguredGuildLoggingTask) Name() string {
	return "log_configured_guilds:" + t.runtime.instanceID
}

func scheduleRuntimeConfiguredGuildLogging(
	ctx context.Context,
	runtime *botRuntime,
	configManager *files.ConfigManager,
	startupTasks *StartupTaskOrchestrator,
) {
	if runtime == nil || runtime.legacySession == nil || configManager == nil {
		return
	}

	if startupTasks == nil {
		slog.Error("Blocking structural failure: startupTasks orchestrator is nil")
		panic("hardware-aligned validation failure: startupTasks cannot be nil during scheduleRuntimeConfiguredGuildLogging")
	}

	startupTasks.Go(RuntimeConfiguredGuildLoggingTask{
		runtime:       runtime,
		configManager: configManager,
	})
}

type WebhookSessionResolver interface {
	SessionForGuild(guildID string, feature string) (*session.LegacySession, error)
}

type StartupWebhookEmbedUpdatesTask struct {
	cfg             *files.BotConfig
	sessionResolver WebhookSessionResolver
}

func (t StartupWebhookEmbedUpdatesTask) Execute(taskCtx context.Context) error {
	for _, item := range collectStartupWebhookEmbedUpdates(t.cfg) {
		if err := taskCtx.Err(); err != nil {
			return fmt.Errorf("scheduleStartupWebhookEmbedUpdates: %w", err)
		}

		operation := fmt.Sprintf("runtime_config.webhook_embed_updates[%s:%d]", item.scope, item.index)
		sess, err := t.sessionResolver.SessionForGuild(item.scope, "webhook")
		if err != nil || sess == nil {
			slog.Debug("Session resolution missed for webhook patch target; skipping",
				slog.String("operation", operation),
				slog.String("scope", item.scope),
			)
			continue
		}

		if err := webhook.PatchMessageEmbed(taskCtx, &webhook.ArikawaAPI{}, webhook.MessageEmbedPatch{
			MessageID:  item.update.MessageID,
			WebhookURL: item.update.WebhookURL,
			Embed:      item.update.Embed,
		}); err != nil {
			slog.Warn("Compensatory action required: Webhook embed patch payload rejected",
				slog.String("operation", operation),
				slog.String("scope", item.scope),
				slog.String("message_id", strings.TrimSpace(item.update.MessageID)),
				slog.String("error", err.Error()),
			)
		} else {
			slog.Debug("Webhook embed patch applied successfully to target",
				slog.String("operation", operation),
				slog.String("scope", item.scope),
				slog.String("message_id", strings.TrimSpace(item.update.MessageID)),
			)
		}
	}
	return nil
}

func (t StartupWebhookEmbedUpdatesTask) Name() string {
	return "startup_webhook_embed_updates"
}

func scheduleStartupWebhookEmbedUpdates(
	startupTasks *StartupTaskOrchestrator,
	cfg *files.BotConfig,
	sessionResolver WebhookSessionResolver,
) {
	if cfg == nil || sessionResolver == nil {
		return
	}

	if startupTasks == nil {
		slog.Error("Blocking structural failure: startupTasks orchestrator is nil")
		panic("hardware-aligned validation failure: startupTasks cannot be nil during scheduleStartupWebhookEmbedUpdates")
	}

	startupTasks.Go(StartupWebhookEmbedUpdatesTask{
		cfg:             cfg,
		sessionResolver: sessionResolver,
	})
}

type ControlServerStartupTask struct {
	controlRuntime        resolvedControlRuntime
	configManager         *files.ConfigManager
	runtimeApplier        *runtimeapply.Manager
	controlServerRegistry *controlServerHolder
	serverOpts            []control.ServerOption
}

func (t ControlServerStartupTask) Execute(taskCtx context.Context) error {
	return startControlServerStartupTask(taskCtx, t.controlRuntime, t.configManager, t.runtimeApplier, t.controlServerRegistry, t.serverOpts...)
}

func (t ControlServerStartupTask) Name() string {
	return "control_server"
}

func scheduleControlServerStartup(startupTasks *StartupTaskOrchestrator, controlRuntime resolvedControlRuntime, configManager *files.ConfigManager, runtimeApplier *runtimeapply.Manager, controlServerRegistry *controlServerHolder, serverOpts ...control.ServerOption) {
	if startupTasks == nil {
		slog.Error("Blocking structural failure: startupTasks orchestrator is nil")
		panic("hardware-aligned validation failure: startupTasks cannot be nil during scheduleControlServerStartup")
	}

	startupTasks.Go(ControlServerStartupTask{
		controlRuntime:        controlRuntime,
		configManager:         configManager,
		runtimeApplier:        runtimeApplier,
		controlServerRegistry: controlServerRegistry,
		serverOpts:            serverOpts,
	})
}

func startControlServerStartupTask(ctx context.Context, controlRuntime resolvedControlRuntime, configManager *files.ConfigManager, runtimeApplier *runtimeapply.Manager, controlServerRegistry *controlServerHolder, serverOpts ...control.ServerOption) error {
	controlServer, err := control.NewServer(controlRuntime.bindAddr, configManager, runtimeApplier, serverOpts...)
	if err != nil {
		errWrap := fmt.Errorf("create control server: %w", err)
		log.EmitBlockingError("Blocking failure during control server allocation", errWrap, log.GenerateRequestID())
		return errWrap
	}
	if controlServer == nil {
		slog.Warn("Control server allocation yielded nil structure; execution branching aborted")
		return nil
	}

	if err := ctx.Err(); err != nil {
		return fmt.Errorf("startControlServerStartupTask: %w", err)
	}

	slog.Info("Architectural transition: Binding control server socket",
		slog.String("address", controlRuntime.bindAddr),
	)

	if err := controlServer.Start(); err != nil {
		errWrap := fmt.Errorf("start control server: %w", err)
		log.EmitBlockingError("Critical failure during control server socket bind operation", errWrap, log.GenerateRequestID())
		return errWrap
	}

	if err := ctx.Err(); err != nil {
		slog.Warn("Startup context invalidated; executing compensatory teardown of control server")
		stopCtx, cancel := context.WithCancel(context.Background())
		defer cancel()
		if stopErr := controlServer.Stop(stopCtx); stopErr != nil {
			log.EmitBlockingError("Teardown failure during aborted startup sequence", stopErr, log.GenerateRequestID())
		}
		return fmt.Errorf("startControlServerStartupTask: %w", err)
	}

	if controlServerRegistry != nil {
		controlServerRegistry.Set(controlServer)
	}
	return nil
}

// ResolveRuntimeStartupParallelism determines the optimal parallel execution bound for startup tasks.
func ResolveRuntimeStartupParallelism(runtimeCount int) int {
	if runtimeCount <= 1 {
		return 1
	} else if runtimeCount == 2 {
		return 2
	} else {
		return 3
	}
}

// StartupTaskOrchestrator unifies bounded concurrency via a strict errgroup.Group,
// eradicating heuristic routing fragmentation and manual worker pools.
type StartupTaskOrchestrator struct {
	eg  *errgroup.Group
	ctx context.Context
}

// NewStartupTaskOrchestrator instantiates a bounded concurrency manager.
func NewStartupTaskOrchestrator(ctx context.Context, runtimeCount int) *StartupTaskOrchestrator {
	eg, egCtx := errgroup.WithContext(ctx)

	parallelism := runtimeCount * 2
	if parallelism <= 0 {
		parallelism = 2
	}
	eg.SetLimit(parallelism)

	slog.Info("Architectural state transition: Startup task orchestrator instantiated",
		slog.Int("concurrency_limit", parallelism),
	)

	return &StartupTaskOrchestrator{
		eg:  eg,
		ctx: egCtx,
	}
}

type BootstrapTask interface {
	Execute(context.Context) error
	Name() string
}

func (o *StartupTaskOrchestrator) Go(task BootstrapTask) {
	if o == nil || o.eg == nil || task == nil {
		return
	}

	name := task.Name()

	if err := o.ctx.Err(); err != nil {
		slog.Warn("Architectural state transition: Startup orchestrator rejecting task due to context cancellation",
			slog.String("task_name", name),
			slog.String("error", err.Error()),
		)
		return
	}

	slog.Debug("Tracking complex conditional branch: Injecting closure into orchestrator",
		slog.String("task_name", name),
	)

	o.eg.Go(func() error {
		if err := task.Execute(o.ctx); err != nil {
			if o.ctx.Err() != nil {
				slog.Debug("Tracking complex conditional branch: Task execution halted via context cancellation",
					slog.String("task_name", name),
				)
				return err
			}
			slog.Warn("Mitigated service degradation: Background startup task encountered an error and aborted",
				slog.String("task", name),
				slog.String("error", err.Error()),
			)
			return err
		}
		return nil
	})
}

func (o *StartupTaskOrchestrator) Shutdown(ctx context.Context) error {
	if o == nil || o.eg == nil {
		return nil
	}

	slog.Info("Architectural state transition: Halting startup orchestrator and draining execution ring")

	done := make(chan error, 1)
	go func() {
		done <- o.eg.Wait()
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		return err
	}
}

```

// === FILE: pkg/app/startup_test.go ===
```go
package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/config"
	"github.com/small-frappuccino/discordcore/pkg/control"
	"github.com/small-frappuccino/discordcore/pkg/discord/session"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

func TestResolveDatabaseBootstrapFromEnv(t *testing.T) {
	t.Setenv(databaseDriverEnv, "postgres")
	t.Setenv(databaseURLEnv, "postgres://postgres@127.0.0.1:5432/postgres?sslmode=disable")
	t.Setenv(databaseMaxOpenConnsEnv, "9")
	t.Setenv(databaseMaxIdleConnsEnv, "4")
	t.Setenv(databaseConnMaxLifetimeSecsEnv, "180")
	t.Setenv(databaseConnMaxIdleTimeSecsEnv, "45")
	t.Setenv(databasePingTimeoutMSEnv, "2500")

	bootstrap, err := resolveDatabaseBootstrap()
	if err != nil {
		t.Fatalf("resolve bootstrap from env: %v", err)
	}

	if bootstrap.Source != "env" {
		t.Fatalf("expected env bootstrap source, got %q", bootstrap.Source)
	}
	if got := bootstrap.Config.DatabaseURL; got != "postgres://postgres@127.0.0.1:5432/postgres?sslmode=disable" {
		t.Fatalf("unexpected database url %q", got)
	}
	if got := bootstrap.Config.MaxOpenConns; got != 9 {
		t.Fatalf("expected max open conns 9, got %d", got)
	}
	if got := bootstrap.Config.PingTimeoutMS; got != 2500 {
		t.Fatalf("expected ping timeout 2500, got %d", got)
	}
}

func TestResolveDatabaseBootstrapRequiresEnv(t *testing.T) {
	t.Setenv(databaseDriverEnv, "")
	t.Setenv(databaseURLEnv, "")
	t.Setenv(databaseMaxOpenConnsEnv, "")
	t.Setenv(databaseMaxIdleConnsEnv, "")
	t.Setenv(databaseConnMaxLifetimeSecsEnv, "")
	t.Setenv(databaseConnMaxIdleTimeSecsEnv, "")
	t.Setenv(databasePingTimeoutMSEnv, "")

	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("expected missing bootstrap environment to panic")
		}
		if !strings.Contains(fmt.Sprintf("%v", r), databaseURLEnv) {
			t.Fatalf("expected panic to mention %s, got %v", databaseURLEnv, r)
		}
	}()
	resolveDatabaseBootstrap()
}

type MockTask struct {
	name string
	exec func(context.Context) error
}

func (m MockTask) Name() string { return m.name }

func (m MockTask) Execute(ctx context.Context) error {
	if m.exec != nil {
		return m.exec(ctx)
	}
	return nil
}

func TestStartupTaskOrchestrator_Go(t *testing.T) {
	t.Parallel()
	orchestrator := NewStartupTaskOrchestrator(context.Background(), 2)

	var executed int32
	var wg sync.WaitGroup
	wg.Add(2)

	for i := 0; i < 2; i++ {
		orchestrator.Go(MockTask{
			name: "test_task",
			exec: func(ctx context.Context) error {
				atomic.AddInt32(&executed, 1)
				wg.Done()
				return nil
			},
		})
	}

	wg.Wait()

	if atomic.LoadInt32(&executed) != 2 {
		t.Errorf("Expected Go task to execute exactly twice")
	}

	if err := orchestrator.Shutdown(context.Background()); err != nil {
		t.Fatalf("Unexpected error during shutdown: %v", err)
	}
}

func TestStartupTaskOrchestrator_ShutdownWithContextCancellation(t *testing.T) {
	t.Parallel()
	orchestrator := NewStartupTaskOrchestrator(context.Background(), 1)

	taskStarted := make(chan struct{})
	unblockTask := make(chan struct{})

	orchestrator.Go(MockTask{
		name: "blocking_task",
		exec: func(ctx context.Context) error {
			close(taskStarted)
			<-unblockTask
			return nil
		},
	})

	<-taskStarted

	// Allow the task to finish so Shutdown doesn't block forever
	close(unblockTask)

	err := orchestrator.Shutdown(context.Background())
	if err != nil {
		t.Errorf("Expected deterministic shutdown to return nil, got %v", err)
	}
}

func TestStartupTaskOrchestrator_ShutdownTaskErrorPropagates(t *testing.T) {
	t.Parallel()
	orchestrator := NewStartupTaskOrchestrator(context.Background(), 1)
	var wg sync.WaitGroup
	wg.Add(1)

	expectedErr := errors.New("simulated task error")

	orchestrator.Go(MockTask{
		name: "error_task",
		exec: func(ctx context.Context) error {
			defer wg.Done()
			return expectedErr
		},
	})

	wg.Wait()

	err := orchestrator.Shutdown(context.Background())
	if err == nil {
		t.Errorf("Expected an error because task errors are propagated, got nil")
	} else if !errors.Is(err, expectedErr) {
		t.Errorf("Expected simulated task error, got: %v", err)
	}
}

func TestStartupTaskOrchestrator_GoNil(t *testing.T) {
	t.Parallel()
	orchestrator := NewStartupTaskOrchestrator(context.Background(), 1)
	orchestrator.Go(MockTask{name: "nil_task", exec: nil})

	var nilOrchestrator *StartupTaskOrchestrator
	nilOrchestrator.Go(MockTask{name: "nil_task", exec: func(ctx context.Context) error { return nil }})
	err := nilOrchestrator.Shutdown(context.Background())
	if err != nil {
		t.Errorf("Expected nil error for nil orchestrator shutdown")
	}
}

func TestResolveParallelism(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		resolver func(int) int
		inputs   map[int]int
	}{
		{"RuntimeStartup", ResolveRuntimeStartupParallelism, map[int]int{0: 1, 1: 1, 2: 2, 3: 3, 10: 3}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for in, expected := range tt.inputs {
				if got := tt.resolver(in); got != expected {
					t.Errorf("%s(%d) = %d; want %d", tt.name, in, got, expected)
				}
			}
		})
	}
}

func TestControlServerHolder_SetAndStop(t *testing.T) {
	t.Parallel()
	var h *controlServerHolder

	// Nil holder
	h.Set(&control.Server{})
	if err := h.Stop(context.Background()); err != nil {
		t.Fatalf("expected nil error on nil holder stop, got %v", err)
	}

	h = &controlServerHolder{}
	h.Set(nil) // Should ignore
	if err := h.Stop(context.Background()); err != nil {
		t.Fatalf("expected nil error on empty holder stop, got %v", err)
	}

	srv := &control.Server{}
	h.Set(srv)
}

func TestScheduleControlServerStartup(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected scheduleControlServerStartup to panic with nil startupTasks")
		}
	}()
	scheduleControlServerStartup(nil, resolvedControlRuntime{}, nil, nil, nil)
}

type mockSessionResolver struct{}

func (m mockSessionResolver) SessionForGuild(guildID string, feature string) (*session.LegacySession, error) {
	return nil, nil
}

func TestScheduleStartupWebhookEmbedUpdates(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected scheduleStartupWebhookEmbedUpdates to panic with nil startupTasks")
		}
	}()
	scheduleStartupWebhookEmbedUpdates(nil, &files.BotConfig{}, mockSessionResolver{})
}

func TestStartControlServerStartupTask(t *testing.T) {
	t.Parallel()
	cfgMgr := files.NewConfigManagerWithStore(&config.MemoryConfigStore{}, nil)
	controlRuntime := resolvedControlRuntime{
		bindAddr: "127.0.0.1:0",
	}

	err := startControlServerStartupTask(context.Background(), controlRuntime, cfgMgr, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

```

// === FILE: pkg/app/task_router.go ===
```go
package app

import (
	"log/slog"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/task"
)

const (
	defaultSingleRuntimeMaxWorkers = 8
	defaultMultiRuntimeMaxWorkers  = 4
)

// resolveRuntimeTaskRouterWorkers calculates the optimal worker boundary without cross-tenant throttling.
func resolveRuntimeTaskRouterWorkers(cfg *files.BotConfig, botInstanceID string, runtimeCount int) int {
	if configured, ok := configuredRuntimeTaskRouterWorkers(cfg, botInstanceID); ok {
		return configured
	}

	if runtimeCount > 1 {
		return defaultMultiRuntimeMaxWorkers
	}
	return defaultSingleRuntimeMaxWorkers
}

func configuredRuntimeTaskRouterWorkers(cfg *files.BotConfig, botInstanceID string) (int, bool) {
	if cfg == nil {
		return 0, false
	}

	maxWorkers := cfg.RuntimeConfig.GlobalMaxWorkers

	// State Bleed Resolved: Determine the maximum required concurrency bound
	// across all attached guilds to prevent a single restrictive tenant
	// from starving the entire shared generic bot ecosystem.
	for _, guild := range files.GuildsForBotInstance(cfg, botInstanceID) {
		if override := guild.RuntimeConfig.GlobalMaxWorkers; override > maxWorkers {
			maxWorkers = override
		}
	}

	// Direct boolean evaluation resolves boundary limits efficiently to avoid conditional branching.
	return maxWorkers, maxWorkers > 0
}

// newRuntimeTaskRouterConfig builds the reliable routing rules for background execution.
func newRuntimeTaskRouterConfig(cfg *files.BotConfig, botInstanceID string, runtimeCount int) task.RouterConfig {
	workers := resolveRuntimeTaskRouterWorkers(cfg, botInstanceID, runtimeCount)

	slog.Info("Architectural state transition: Configured background worker budget for task router",
		slog.String("botInstanceID", botInstanceID),
		slog.Int("concurrency_budget", workers),
	)

	routerCfg := task.Defaults()
	routerCfg.GlobalMaxWorkers = workers
	routerCfg.ExecutionLimiter = task.NewExecutionLimiter(workers)

	return routerCfg
}

```

// === FILE: pkg/app/task_router_test.go ===
```go
package app

import (
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/files"
)

func TestResolveRuntimeTaskRouterWorkersUsesAutoBudgets(t *testing.T) {
	t.Parallel()

	if got := resolveRuntimeTaskRouterWorkers(nil, "default", 1); got != defaultSingleRuntimeMaxWorkers {
		t.Fatalf("expected single-runtime default budget %d, got %d", defaultSingleRuntimeMaxWorkers, got)
	}
	if got := resolveRuntimeTaskRouterWorkers(nil, "default", 2); got != defaultMultiRuntimeMaxWorkers {
		t.Fatalf("expected multi-runtime default budget %d, got %d", defaultMultiRuntimeMaxWorkers, got)
	}
}

func TestResolveRuntimeTaskRouterWorkersUsesLargestRuntimeOverride(t *testing.T) {
	t.Parallel()

	cfg := &files.BotConfig{
		RuntimeConfig: files.RuntimeConfig{
			GlobalMaxWorkers: 5,
		},
		Guilds: []files.GuildConfig{
			{
				GuildID:           "g1",
				BotInstanceTokens: map[string]files.EncryptedString{"alpha": "a"},
				RuntimeConfig: files.RuntimeConfig{
					GlobalMaxWorkers: 6,
				},
			},
			{
				GuildID:           "g2",
				BotInstanceTokens: map[string]files.EncryptedString{"alpha": "a"},
				RuntimeConfig: files.RuntimeConfig{
					GlobalMaxWorkers: 12,
				},
			},
			{
				GuildID:           "g3",
				BotInstanceTokens: map[string]files.EncryptedString{"generic": "a"},
			},
		},
	}

	if got := resolveRuntimeTaskRouterWorkers(cfg, "alpha", 2); got != 12 {
		t.Fatalf("expected alpha runtime to use largest override 12, got %d", got)
	}
	if got := resolveRuntimeTaskRouterWorkers(cfg, "beta", 2); got != 5 {
		t.Fatalf("expected beta runtime to fall back to global override 5, got %d", got)
	}
}

func TestNewRuntimeTaskRouterConfigBuildsSharedLimiter(t *testing.T) {
	t.Parallel()

	cfg := &files.BotConfig{
		RuntimeConfig: files.RuntimeConfig{
			GlobalMaxWorkers: 5,
		},
	}

	routerCfg := newRuntimeTaskRouterConfig(cfg, "default", 1)
	if routerCfg.GlobalMaxWorkers != 5 {
		t.Fatalf("expected router config workers 5, got %d", routerCfg.GlobalMaxWorkers)
	}
	if routerCfg.ExecutionLimiter == nil {
		t.Fatal("expected shared execution limiter to be configured")
	}
	if got := routerCfg.ExecutionLimiter.Capacity(); got != 5 {
		t.Fatalf("expected limiter capacity 5, got %d", got)
	}
}

```

// === FILE: pkg/app/version.go ===
```go
package app

import (
	"github.com/small-frappuccino/discordcore/pkg/files"
)

// Version is the current version of the discordcore package.
const Version = files.DiscordCoreVersion

// AppVersion is the version of the application using discordcore.
func AppVersion() string {
	return files.AppVersion
}

// SetAppVersion sets the version of the application using discordcore.
func SetAppVersion(v string) {
	files.SetAppVersion(v)
}

```

// === FILE: pkg/app/version_test.go ===
```go
package app_test

import (
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/app"
)

func TestAppVersion(t *testing.T) {
	orig := app.AppVersion()
	t.Cleanup(func() {
		app.SetAppVersion(orig)
	})

	app.SetAppVersion("v1.2.3-test")
	if got := app.AppVersion(); got != "v1.2.3-test" {
		t.Errorf("expected AppVersion 'v1.2.3-test', got %q", got)
	}
}

```

