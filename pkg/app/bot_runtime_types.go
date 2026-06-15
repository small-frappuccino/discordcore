package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"

	"github.com/small-frappuccino/discordgo"

	"github.com/small-frappuccino/discordcore/pkg/control"
	"github.com/small-frappuccino/discordcore/pkg/discord/cache"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/monitoring"

	"github.com/small-frappuccino/discordcore/pkg/service"
)

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
	instanceID        string
	capabilities      botRuntimeCapabilities
	session           *discordgo.Session
	serviceManager    *service.ServiceManager
	monitoringService *monitoring.MonitoringService
	commandHandler    *commands.CommandHandler
}

type botRuntimeResolver struct {
	mu            sync.RWMutex
	configManager *files.ConfigManager
	runtimes      map[string]*botRuntime
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
		err := fmt.Errorf("resolver is nil")
		log.EmitBlockingError("Blocking structural failure: Synchronization channel missing from resolver matrix", err, log.GenerateRequestID())
		return err
	}
	select {
	case <-r.readyCh:
		slog.Debug("Tracking complex conditional branch: Wait lock released across runtime resolver")
		return nil
	case <-ctx.Done():
		errWrap := ctx.Err()
		slog.Warn("Mitigated service degradation: Synchronization context expired before runtime resolver released the wait lock",
			slog.String("error", errWrap.Error()),
		)
		return errWrap
	}
}

func (r *botRuntimeResolver) getRuntimes() map[string]*botRuntime {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.runtimes
}

func (r *botRuntimeResolver) swapRuntimes(newMap map[string]*botRuntime) {
	r.mu.Lock()
	defer r.mu.Unlock()

	slog.Debug("Granular inspection: Executing atomic pointer rotation for active runtimes map",
		slog.Int("new_map_size", len(newMap)),
	)

	r.runtimes = newMap
}

func knownBotInstanceCatalog(runtimes map[string]*botRuntime, additional []string) map[string]struct{} {
	known := make(map[string]struct{}, len(runtimes)+len(additional))
	for botInstanceID := range runtimes {
		normalizedBotInstanceID := files.NormalizeBotInstanceID(botInstanceID)
		if normalizedBotInstanceID == "" {
			continue
		}
		known[normalizedBotInstanceID] = struct{}{}
	}
	for _, botInstanceID := range additional {
		normalizedBotInstanceID := files.NormalizeBotInstanceID(botInstanceID)
		if normalizedBotInstanceID == "" {
			continue
		}
		known[normalizedBotInstanceID] = struct{}{}
	}

	return known
}

func knownBotInstanceCatalogSlice(catalog map[string]struct{}) []string {
	if len(catalog) == 0 {
		return nil
	}
	out := make([]string, 0, len(catalog))
	for botInstanceID := range catalog {
		out = append(out, botInstanceID)
	}
	sort.Strings(out)
	return out
}

func newBotRuntimeResolver(configManager *files.ConfigManager, initialRuntimes map[string]*botRuntime) *botRuntimeResolver {
	slog.Info("Architectural state transition: Initializing memory barrier for bot runtime multiplexing",
		slog.Int("initial_runtimes_count", len(initialRuntimes)),
	)
	return &botRuntimeResolver{
		configManager: configManager,
		runtimes:      initialRuntimes,
		readyCh:       make(chan struct{}),
	}
}

// aggregateUnifiedCaches collects the UnifiedCache of all active bot instances.
func (r *botRuntimeResolver) aggregateUnifiedCaches() map[string]*cache.UnifiedCache {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()

	caches := make(map[string]*cache.UnifiedCache)
	for id, runtime := range r.runtimes {
		if runtime.monitoringService != nil && runtime.monitoringService.GetUnifiedCache() != nil {
			caches[id] = runtime.monitoringService.GetUnifiedCache()
		}
	}
	if len(caches) == 0 {
		return nil
	}
	return caches
}

// aggregateMonitoringMetrics collects the monitoring.Metrics of all active bot instances.
func (r *botRuntimeResolver) aggregateMonitoringMetrics() map[string]monitoring.Metrics {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()

	metrics := make(map[string]monitoring.Metrics)
	for id, runtime := range r.runtimes {
		if runtime.monitoringService != nil {
			m := runtime.monitoringService.Metrics()
			if m != nil {
				metrics[id] = m
			}
		}
	}
	if len(metrics) == 0 {
		return nil
	}
	return metrics
}

func (r *botRuntimeResolver) runtimeForGuild(guildID string, feature string) (*botRuntime, string, error) {
	if r == nil {
		err := fmt.Errorf("bot runtime resolver is unavailable")
		log.EmitBlockingError("Blocking structural failure: Pointer to runtime resolver dropped from state matrix", err, log.GenerateRequestID())
		return nil, "", err
	}
	guildID = strings.TrimSpace(guildID)
	if r.configManager == nil {
		err := fmt.Errorf("bot runtime resolver config manager is unavailable")
		log.EmitBlockingError("Blocking structural failure: Config manager detached from runtime resolver", err, log.GenerateRequestID())
		return nil, "", err
	}
	guild := r.configManager.GuildConfig(guildID)
	if guild == nil {
		errWrap := fmt.Errorf("guild %s is not configured", guildID)
		slog.Warn("Mitigated service degradation: Request target resolves to an unconfigured guild parameter, aborting sub-routine",
			slog.String("guild_id", guildID),
			slog.String("error", errWrap.Error()),
		)
		return nil, "", errWrap
	}

	if feature == "" {
		feature = "dashboard"
	}
	bestInstanceID, _ := guild.ResolveFeatureBotInstanceID(feature)

	r.mu.RLock()
	defer r.mu.RUnlock()

	runtimes := r.runtimes
	if len(runtimes) == 0 {
		slog.Warn("Mitigated service degradation: Primary runtime vector exhausted or uninitialized",
			slog.String("guild_id", guildID),
		)
		return nil, "", ErrSessionUnavailable
	}

	if bestInstanceID != "" {
		tokenEnc, ok := guild.BotInstanceTokens[bestInstanceID]
		if ok && string(tokenEnc) != "" {
			if runtime, ok := runtimes[bestInstanceID]; ok && runtime != nil {
				return runtime, bestInstanceID, nil
			}
		}
	}

	slog.Debug("Tracking complex conditional branch: Executing heuristic token scan for orphan guild",
		slog.String("guild_id", guildID),
	)

	if len(guild.BotInstanceTokens) > 0 {
		for id, tokenEnc := range guild.BotInstanceTokens {
			if string(tokenEnc) == "" {
				continue
			}
			if runtime, ok := runtimes[id]; ok && runtime != nil {
				slog.Debug("Tracking complex conditional branch: First valid token matched during scan",
					slog.String("guild_id", guildID),
					slog.String("resolved_id", id),
				)
				return runtime, id, nil
			}
		}
	} else {
		if runtime, ok := runtimes[""]; ok && runtime != nil {
			slog.Debug("Tracking complex conditional branch: Reverting to blank sentinel runtime for tokenless guild",
				slog.String("guild_id", guildID),
			)
			return runtime, "", nil
		}
	}

	err := fmt.Errorf("guild %s does not resolve to a running bot instance", guildID)
	slog.Warn("Mitigated service degradation: Orchestrator failed to couple guild node to an active port",
		slog.String("guild_id", guildID),
		slog.String("error", err.Error()),
	)
	return nil, "", err
}

func (r *botRuntimeResolver) sessionForGuild(guildID string, feature string) (*discordgo.Session, error) {
	runtime, botInstanceID, err := r.runtimeForGuild(guildID, feature)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSessionUnavailable, err)
	}
	if runtime.session == nil {
		guildID = strings.TrimSpace(guildID)
		if guildID == "" {
			errWrap := fmt.Errorf("%w: discord session for default bot instance %q is empty", ErrSessionUnavailable, botInstanceID)
			log.EmitBlockingError("Blocking structural failure: Socket payload evaluates to nil on default instance", errWrap, log.GenerateRequestID())
			return nil, errWrap
		}
		errWrap := fmt.Errorf("%w: discord session for guild %s (bot instance %q) is empty", ErrSessionUnavailable, guildID, botInstanceID)
		log.EmitBlockingError("Blocking structural failure: Socket payload evaluates to nil on specific guild channel", errWrap, log.GenerateRequestID())
		return nil, errWrap
	}
	return runtime.session, nil
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

	out := make([]control.BotGuildBinding, 0)
	runtimes := r.getRuntimes()
	if len(runtimes) == 0 {
		return out, nil
	}

	cfg := r.configManager.Config()
	if cfg == nil {
		return out, nil
	}

	slog.Debug("Granular inspection: Parsing unified configuration manifest for explicit guild-to-bot bindings")

	for _, guild := range cfg.Guilds {
		for botInstanceID, tokenEnc := range guild.BotInstanceTokens {
			token := string(tokenEnc)
			if token == "" {
				continue
			}
			runtime, ok := runtimes[botInstanceID]
			if !ok || runtime == nil || runtime.session == nil {
				continue
			}

			if _, err := runtime.session.State.Guild(guild.GuildID); err == nil {
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

func listBotGuildBindingsFromSessionState(botInstanceID string, session *discordgo.Session) ([]control.BotGuildBinding, error) {
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
