# Domain Architecture: app

## Layout Topology
```text
app/
├── runtimecmd
│   └── runtimecmd.go
├── bot_runtime.go
├── bot_supervisor.go
├── catalog_registrars.go
├── command_handler.go
├── contracts.go
├── control.go
├── observability.go
├── runner.go
├── startup.go
├── task_router.go
└── version.go
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

// === FILE: pkg/app/contracts.go ===
```go
package app

import (
	"context"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

// CommandGroup standardizes the delivery of Guild & Bot-Profile isolated slash commands to the gateway registrar.
// Allocation Footprint: Negligible. Typically returns pre-allocated static slices and maps.
// Preemption Rules: None. These methods are pure accessors and must not block or perform I/O.
type CommandGroup interface {
	// Register exposes the slice of Discord Application Command blueprints generated by the vertical's internal constructor, isolated by Guild and Bot Profile.
	// Allocation Footprint: O(1) if returning a pre-allocated slice, O(N) if generating on the fly.
	// Preemption Rules: Must return immediately without blocking.
	Register(guildID string, botProfileID string) []api.CreateCommandData

	// Handle exposes the O(1) routing dictionary binding the unique command invocation string directly to its procedural execution lane, isolated by Guild and Bot Profile.
	// Allocation Footprint: O(1) if returning a pre-allocated map.
	// Preemption Rules: Must return immediately without blocking.
	Handle(guildID string, botProfileID string) map[string]core.CommandHandler
}

// FeatureService defines the passive watcher triggered by state changes.
// Allocation Footprint: Minimal overhead. Goroutine spawned for Start.
// Preemption Rules: Must respect context cancellation boundaries strictly.
type FeatureService interface {
	// Start traps execution inside its spawned goroutine and yields control exclusively to errgroup.
	// It must gracefully unwind its stack the exact clock cycle ctx.Done() returns.
	// Allocation Footprint: Small stack allocation for goroutine. No sustained heap growth.
	// Preemption Rules: Blocking call. Yields completely upon context cancellation.
	Start(ctx context.Context) error

	// Stop signals the feature service to shutdown and release any held resources gracefully.
	// Allocation Footprint: Minimal. No heap allocations expected.
	// Preemption Rules: Must return immediately or block only briefly for teardown.
	Stop() error

	// WatchConfig is the passive Pub/Sub reactive receiver for configuration events.
	// It receives a ConfigEvent containing the GuildID and the read-only configuration snapshot.
	// The implementer must compute feature toggles (Enable/Disable) deterministically in O(1) or O(N) local memory without acquiring global write locks.
	// Allocation Footprint: O(1) or local O(N) for evaluation. Must not escape variables to the heap unnecessarily.
	// Preemption Rules: Must execute synchronously and quickly. Should preempt if context is canceled during complex evaluation.
	WatchConfig(ctx context.Context, event files.ConfigEvent)
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
	qotdService := qotd.NewService(a.configManager, qotd.WithRepository(a.store), qotd.WithMetrics(qotdMetrics))

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

