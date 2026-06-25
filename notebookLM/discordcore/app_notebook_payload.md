# Domain Architecture: app

## Layout Topology
```text
app/
├── bot_runtime.go
├── command_handler.go
├── contracts.go
└── startup.go
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
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/cmd"
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
	runtimeCount          int
	configManager         *files.ConfigManager
	store                 *postgres.Store
	commandGroups         []cmd.CommandGroup
	runtimeApplier        *runtimeapply.Manager
	qotdCommandService    *applicationqotd.Service
	moderationMetrics     moderation.Metrics
	membersMetrics        members.Metrics
	messagesMetrics       messages.Metrics
	startupTasks          *StartupTaskOrchestrator
	profile               RunProfile
	appClock              clock.Clock
	controlServerRegistry *controlServerHolder
	logger                *slog.Logger
	embedService          *embeds.EmbedService
	rolePanelService      *roles.RolePanelService
	partnerService        *partners.PartnerService
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

		cg := opts.commandGroups
		deps := CommandHandlerDeps{
			Session:             runtime.legacySession,
			ConfigManager:       opts.configManager,
			BotInstanceID:       runtime.instanceID,
			CatalogCapabilities: caps,
			CommandGroups:       cg,
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
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/cmd"
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
	commandGroups       []cmd.CommandGroup

	// Atomic pointers enforce memory safety without mutex contention
	routerMap atomic.Pointer[map[string]cmd.CommandHandler]
	registrar *CommandRegistrar

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
	CommandGroups       []cmd.CommandGroup
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

	return &CommandHandler{
		session:             deps.Session,
		configManager:       deps.ConfigManager,
		botInstanceID:       deps.BotInstanceID,
		catalogCapabilities: deps.CatalogCapabilities,
		commandGroups:       deps.CommandGroups,
		registrar:           NewCommandRegistrar(),
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

	if ch.routerMap.Load() != nil {
		slog.Warn("overlapping handler registration; invoking cleanup of previous registrations", slog.String("botInstanceID", ch.botInstanceID))
		if err := ch.Shutdown(); err != nil {
			return fmt.Errorf("cleanup previous command handlers: %w", err)
		}
	}

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

	apiClient := api.NewClient(ch.session.Token)

	// Assume no explicit guildID or botProfileID is passed to CompileAndSync for global or default setup, or we use botInstanceID
	// Compile the O(1) map and conditionally sync
	routerMap, err := ch.registrar.CompileAndSync(apiClient, discord.AppID(appID), "", ch.botInstanceID, ch.commandGroups)
	if err != nil {
		if shutdownErr := ch.Shutdown(); shutdownErr != nil {
			slog.Error("fatal failure during command manager registration rollback",
				slog.String("botInstanceID", ch.botInstanceID),
				slog.String("synthetic_fault_code", "500"),
				slog.String("stack_trace", fmt.Sprintf("%+v", shutdownErr)),
			)
		}
		return fmt.Errorf("failed to setup commands: %w", err)
	}

	ch.routerMap.Store(&routerMap)

	// Direct method injection strictly avoids inline closure allocation overhead.
	ch.interactionCancel = ch.session.AddHandler(ch.handleInteractionCreate)

	// Explicitly drop linear array of command groups to free heap memory
	ch.commandGroups = nil

	slog.Info("Command architecture successfully established natively", slog.String("botInstanceID", ch.botInstanceID))
	return nil
}

// handleInteractionCreate executes isolated runtime processing.
func (ch *CommandHandler) handleInteractionCreate(s *discordgo.Session, rawEvent *discordgo.Event) {
	if rawEvent.Type != "INTERACTION_CREATE" {
		return
	}

	routerMapPtr := ch.routerMap.Load()
	if routerMapPtr == nil {
		return
	}
	routerMap := *routerMapPtr

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
		// Component IDs might contain additional metadata separated by |
		routePath = string(data.ID())
		if idx := strings.Index(routePath, "|"); idx != -1 {
			routePath = routePath[:idx+1]
		}
	case *discord.ModalInteraction:
		routePath = string(data.CustomID)
		if idx := strings.Index(routePath, "|"); idx != -1 {
			routePath = routePath[:idx+1]
		}
	}

	if routePath != "" && arikawaEvent.GuildID.IsValid() {
		if !ch.handlesGuildRoute(arikawaEvent.GuildID.String(), commands.InteractionRouteKey{Path: routePath}) {
			return
		}
	}

	// O(1) Routing
	handler, exists := routerMap[routePath]
	if !exists {
		slog.Debug("No handler found for route", slog.String("routePath", routePath))
		return
	}

	// Inject custom cmd.Context
	apiClient := api.NewClient(ch.session.Token)
	logger := slog.With("guildID", arikawaEvent.GuildID.String(), "routePath", routePath)

	// Create context with DI
	cmdCtx := cmd.NewContext(context.Background(), apiClient, &arikawaEvent, logger, ch, nil) // nil for Tx for now

	// Wrap handler with Middleware
	feature := commands.ResolveFeatureForCommandPath(routePath)
	wrappedHandler := Chain(handler, RateLimitMiddleware(), PermissionsMiddleware(feature))

	// Execute handler
	if err := wrappedHandler(cmdCtx); err != nil {
		slog.Error("Command handler failed", slog.Any("error", err), slog.String("routePath", routePath))
	}
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

	ch.routerMap.Store(nil)

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

	"github.com/small-frappuccino/discordcore/pkg/files"
)

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

