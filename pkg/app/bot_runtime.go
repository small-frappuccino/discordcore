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
	"time"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/state"
	"github.com/small-frappuccino/discordcore/pkg/clock"
	"github.com/small-frappuccino/discordcore/pkg/control"
	discord_automod "github.com/small-frappuccino/discordcore/pkg/discord/automod"
	"github.com/small-frappuccino/discordcore/pkg/discord/cache"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/moderation"
	"github.com/small-frappuccino/discordcore/pkg/discord/embeds"
	"github.com/small-frappuccino/discordcore/pkg/discord/logging"
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

// hasCommands reports whether any command catalog should be installed.
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

	guilds := cfg.GuildsForBotInstance(botInstanceID)
	for _, guild := range guilds {
		features := cfg.ResolveFeatures(guild.GuildID)
		runtimeConfig := cfg.ResolveRuntimeConfig(guild.GuildID)

		isQOTDBot := false
		isRolesBot := false
		isStatsBot := false
		isModBot := false
		isLoggingBot := false

		if !guild.QOTD.IsZero() {
			if id, _ := guild.ResolveFeatureBotInstanceID("qotd"); id == botInstanceID {
				isQOTDBot = true
			}
		}
		if features.Services.Commands {
			if id, _ := guild.ResolveFeatureBotInstanceID("roles"); id == botInstanceID {
				isRolesBot = true
			}
			if id, _ := guild.ResolveFeatureBotInstanceID("stats"); id == botInstanceID {
				isStatsBot = true
			}
		}
		if guild.Channels.AutomodAction != "" || guild.UserPrune.Enabled {
			if id, _ := guild.ResolveFeatureBotInstanceID("moderation"); id == botInstanceID {
				isModBot = true
			}
		}
		if features.Services.Monitoring {
			if id, _ := guild.ResolveFeatureBotInstanceID("logging"); id == botInstanceID {
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

// ErrNoBotTokensConfigured defines err no bot tokens configured.
var ErrNoBotTokensConfigured = errors.New("no bot instances have a configured token")

// ErrSessionUnavailable defines err when a bot session is not available for a guild or globally.
var ErrSessionUnavailable = errors.New("discord session is unavailable")

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
	runtimes      sync.Map // key: string, value: *botRuntime
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
	return func(yield func(string, *botRuntime) bool) {
		r.runtimes.Range(func(key, value any) bool {
			return yield(key.(string), value.(*botRuntime))
		})
	}
}

func (r *botRuntimeResolver) addRuntime(id string, runtime *botRuntime) {
	r.runtimes.Store(id, runtime)
}

func (r *botRuntimeResolver) removeRuntime(id string) {
	r.runtimes.Delete(id)
}

func knownBotInstanceCatalogSeq(runtimes iter.Seq2[string, *botRuntime], additional []string) iter.Seq[string] {
	return func(yield func(string) bool) {
		known := make(map[string]struct{})

		// Closure atua como interceptador de filtro stateful
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
	for k, v := range initialRuntimes {
		resolver.runtimes.Store(k, v)
	}
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
	r.runtimes.Range(func(_, _ any) bool { hasAny = true; return false })
	if !hasAny {
		return nil, "", fmt.Errorf("%w: primary runtime vector exhausted or uninitialized for guild %s", ErrSessionUnavailable, guildID)
	}

	if feature == "" {
		feature = "dashboard"
	}

	// 1. Prioridade Estrita: Resolução Específica de Feature
	bestInstanceID, _ := guild.ResolveFeatureBotInstanceID(feature)
	if bestInstanceID != "" {
		if tokenEnc, ok := guild.BotInstanceTokens[bestInstanceID]; ok && string(tokenEnc) != "" {
			if val, ok := r.runtimes.Load(bestInstanceID); ok {
				if runtime := val.(*botRuntime); runtime != nil {
					return runtime, bestInstanceID, nil
				}
			}
		}
	}

	// 2. Degradação Graciosa: Qualquer Instância Ativa na Guild
	for id, tokenEnc := range guild.BotInstanceTokens {
		if string(tokenEnc) == "" {
			continue
		}
		if val, ok := r.runtimes.Load(id); ok {
			if runtime := val.(*botRuntime); runtime != nil {
				return runtime, id, nil
			}
		}
	}

	// 3. Fallback de Último Recurso: Instância Global/Default
	if val, ok := r.runtimes.Load(""); ok {
		if runtime := val.(*botRuntime); runtime != nil {
			return runtime, "", nil
		}
	}

	return nil, "", fmt.Errorf("%w: orchestrator failed to couple guild %s to an active port", ErrSessionUnavailable, guildID)
}

func (r *botRuntimeResolver) arikawaStateForGuild(guildID string, feature string) (*state.State, error) {
	runtime, _, err := r.runtimeForGuild(guildID, feature)
	if err != nil {
		return nil, err
	}
	return runtime.arikawaState, nil
}

func (r *botRuntimeResolver) sessionForGuild(guildID string, feature string) (*session.LegacySession, error) {
	runtime, botInstanceID, err := r.runtimeForGuild(guildID, feature)
	if err != nil {
		return nil, err // O erro já encapsula ErrSessionUnavailable
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
	r.runtimes.Range(func(_, _ any) bool { hasAny = true; return false })
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
			val, ok := r.runtimes.Load(botInstanceID)
			if !ok {
				continue
			}
			runtime := val.(*botRuntime)
			if runtime == nil || runtime.legacySession == nil {
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
	openBotArikawaState      func(ctx context.Context, s *state.State) error
	fetchBotArikawaMe        func(s *state.State) (*discord.User, error)
	discordSessionCloseHook  func(c interface{ Close() error }) error
	newCommandHandlerForBot  func(deps CommandHandlerDeps) (*CommandHandler, error)
	newCommandHandler        func(deps CommandHandlerDeps) (*CommandHandler, error)
	setupCommandHandler      func(ch *CommandHandler) error
	shutdownCommandHandler   func(ch *CommandHandler) error
}

type memberSinkWrapper struct {
	logger *logging.Logger
}

func (w memberSinkWrapper) OnMemberJoin(ctx context.Context, e *gateway.GuildMemberAddEvent, accountAge time.Duration) {
	if w.logger != nil {
		w.logger.OnMemberJoin(ctx, e.GuildID.String(), e.Member)
	}
}

func (w memberSinkWrapper) OnMemberLeave(ctx context.Context, e *gateway.GuildMemberRemoveEvent, serverTime time.Duration, botTime time.Duration) {
	if w.logger != nil {
		w.logger.OnMemberLeave(ctx, e.GuildID.String(), e.User)
	}
}

func (w memberSinkWrapper) OnRoleUpdate(ctx context.Context, guildID string, user discord.User, addedRoles, removedRoles []discord.RoleID) {
	if w.logger != nil {
		w.logger.OnRoleUpdate(ctx, guildID, user, addedRoles, removedRoles)
	}
}

func (w memberSinkWrapper) OnAvatarUpdate(ctx context.Context, guildID string, user discord.User, oldAvatarHash, newAvatarHash string) {
	if w.logger != nil {
		w.logger.OnAvatarUpdate(ctx, guildID, user, oldAvatarHash, newAvatarHash)
	}
}

func (w memberSinkWrapper) OnModerationAction(ctx context.Context, guildID string, actionType string, targetUser discord.User, reason string, moderator discord.User) {
	if w.logger != nil {
		w.logger.OnModerationAction(ctx, guildID, actionType, targetUser, reason, moderator)
	}
}

func openBotRuntime(instance resolvedBotInstance, capabilities botRuntimeCapabilities, opts botRuntimeOptions) (*botRuntime, error) {
	if instance.Token == "" {
		panic("hardware-aligned validation failure: bot token is missing prior to socket coupling")
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

	// Enforce hard execution deadlines on gateway socket binding to prevent invisible deadlocks.
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := opts.openBotArikawaState(ctx, arikawaState); err != nil {
		return nil, fmt.Errorf("open discord session for %s: %w", instance.ID, err)
	}

	me, err := opts.fetchBotArikawaMe(arikawaState)
	if err != nil {
		return nil, fmt.Errorf("discord session state not properly initialized for %s: %w", instance.ID, err)
	}

	slog.Info("Architectural state transition: Socket bound and API authenticated",
		slog.String("botInstanceID", instance.ID),
		slog.String("botUser", fmt.Sprintf("%s#%s", me.Username, me.Discriminator)),
	)

	return &botRuntime{
		instanceID:    instance.ID,
		capabilities:  capabilities,
		legacySession: session.NewEmptySessionForCompat(botToken),
		arikawaState:  arikawaState,
	}, nil
}

// Assinatura removida por diretiva de refatoração para procedural flattening.

func initializeBotRuntime(ctx context.Context, runtime *botRuntime, opts botRuntimeOptions) error {
	slog.Debug("Iniciando rotina de alocação de runtime", slog.String("instance_id", runtime.instanceID))

	if runtime == nil || runtime.legacySession == nil {
		panic("hardware-aligned validation failure: runtime or legacy session is nil during initialization")
	}

	cfg := opts.configManager.Config()
	if cfg == nil {
		panic("hardware-aligned validation failure: config snapshot is nil")
	}

	routerConfig := newRuntimeTaskRouterConfig(cfg, runtime.instanceID, opts.runtimeCount)
	_ = routerConfig // might be used by domain setups internally if passed, but currently they aren't taking it here

	runtime.serviceManager = service.NewServiceManager(slog.Default())

	token := runtime.legacySession.Token
	if !strings.HasPrefix(token, "Bot ") {
		token = "Bot " + token
	}
	runtime.arikawaState = state.New(token)

	if opts.runtimeApplier != nil {
		opts.runtimeApplier.AddRuntime(runtime.serviceManager, nil)
	}

	// Message Event Service
	if runtime.capabilities.messageEventService {
		mes := messages.NewMessageEventServiceForBot(messages.EventServiceDeps{
			ArikawaState:  runtime.arikawaState,
			ConfigManager: opts.configManager,
			Sink:          resolveEventLogger(runtime, opts.configManager),
			Store:         opts.store,
			BotInstanceID: runtime.instanceID,
			Logger:        slog.Default(),
		})
		mes.SetTaskRouter(runtime.taskRouter)
		if err := runtime.serviceManager.Register(mes); err != nil {
			return fmt.Errorf("service registration failure for %s: %w", runtime.instanceID, err)
		}
	}

	// Member Event Service
	if runtime.capabilities.memberEventService {
		memSvc := members.NewMemberEventServiceForBot(members.EventServiceDeps{
			ArikawaState:  runtime.arikawaState,
			ConfigManager: opts.configManager,
			Sink:          memberSinkWrapper{logger: resolveEventLogger(runtime, opts.configManager)},
			MembersRepo:   opts.store,
			SystemRepo:    opts.store,
			BotInstanceID: runtime.instanceID,
			Logger:        slog.Default(),
		})
		if err := runtime.serviceManager.Register(memSvc); err != nil {
			return fmt.Errorf("service registration failure for %s: %w", runtime.instanceID, err)
		}
	}

	// Automod Service
	if runtime.capabilities.automod {
		var eventLogger *logging.Logger
		if runtime.arikawaState != nil && runtime.arikawaState.Session != nil {
			eventLogger = logging.NewLogger(runtime.arikawaState.Session.Client, opts.configManager, runtime.arikawaState, gateway.Intents(runtime.capabilities.intents), slog.Default())
		}
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

		commandHandler, err := opts.newCommandHandlerForBot(deps)
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

var intelligentWarmupFn = cache.IntelligentWarmupContext

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

	_, memberWarmupConfig := runtimeWarmupPhases()

	if startupTasks == nil {
		slog.Error("Blocking structural failure: startupTasks orchestrator is nil, refusing to launch unprotected warmup goroutine")
		panic("hardware-aligned validation failure: startupTasks cannot be nil during runtime warmup phase")
	}

	slog.Debug("Delegating cache warmup to orchestrator scheduling queue",
		slog.String("botInstanceID", runtime.instanceID),
	)
	startupTasks.GoHeavy("cache_warmup:"+runtime.instanceID, &RuntimeWarmupTask{
		Runtime:      runtime,
		UnifiedCache: unifiedCache,
		Store:        store,
		WarmupConfig: memberWarmupConfig,
		InstanceCtx:  ctx,
	})
}

type RuntimeWarmupTask struct {
	Runtime      *botRuntime
	UnifiedCache *cache.UnifiedCache
	Store        *postgres.Store
	WarmupConfig cache.WarmupConfig
	InstanceCtx  context.Context
}

func (t *RuntimeWarmupTask) Execute(_ context.Context) error {
	ctx := t.InstanceCtx
	if err := intelligentWarmupFn(ctx, t.Runtime.legacySession, t.UnifiedCache, t.Store, t.WarmupConfig); err != nil {
		if ctx.Err() != nil {
			return nil
		}
		slog.Warn("Mitigated service degradation: Orchestrated cache warmup failed, pipeline resumes",
			slog.String("botInstanceID", t.Runtime.instanceID),
			slog.String("error", err.Error()),
		)
	}
	return nil
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

func shutdownBotRuntime(runtime *botRuntime, ctx context.Context) []error {
	if runtime == nil {
		return nil
	}

	slog.Info("Architectural state transition: Executing planned shutdown across main runtime instances",
		slog.String("botInstanceID", runtime.instanceID),
	)

	var errs []error
	if runtime.serviceManager != nil {
		if err := runtime.serviceManager.StopAll(ctx); err != nil {
			errs = append(errs, fmt.Errorf("stop services for %s: %w", runtime.instanceID, err))
		}
	}
	return errs
}

// resolveEventLogger centraliza a injeção do sink de logs para serviços de auditoria.
func resolveEventLogger(runtime *botRuntime, configManager *files.ConfigManager) *logging.Logger {
	if runtime.arikawaState == nil || runtime.arikawaState.Session == nil {
		return nil
	}
	return logging.NewLogger(
		runtime.arikawaState.Session.Client,
		configManager,
		runtime.arikawaState,
		gateway.Intents(runtime.capabilities.intents),
		slog.Default(),
	)
}
