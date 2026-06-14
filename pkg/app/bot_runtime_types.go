package app

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/small-frappuccino/discordgo"

	"github.com/small-frappuccino/discordcore/pkg/control"
	"github.com/small-frappuccino/discordcore/pkg/discord/cache"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands"
	"github.com/small-frappuccino/discordcore/pkg/files"
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
	r.readyOnce.Do(func() { close(r.readyCh) })
}

func (r *botRuntimeResolver) waitForReady(ctx context.Context) error {
	if r == nil {
		return fmt.Errorf("resolver is nil")
	}
	select {
	case <-r.readyCh:
		return nil
	case <-ctx.Done():
		return ctx.Err()
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
		return nil, "", fmt.Errorf("bot runtime resolver is unavailable")
	}
	guildID = strings.TrimSpace(guildID)
	if r.configManager == nil {
		return nil, "", fmt.Errorf("bot runtime resolver config manager is unavailable")
	}
	guild := r.configManager.GuildConfig(guildID)
	if guild == nil {
		return nil, "", fmt.Errorf("guild %s is not configured", guildID)
	}

	if feature == "" {
		feature = "dashboard"
	}
	bestInstanceID, _ := guild.ResolveFeatureBotInstanceID(feature)

	r.mu.RLock()
	defer r.mu.RUnlock()

	runtimes := r.runtimes
	if len(runtimes) == 0 {
		return nil, "", ErrSessionUnavailable
	}

	if bestInstanceID != "" {
		tokenEnc, ok := guild.BotInstanceTokens[bestInstanceID]
		if ok && string(tokenEnc) != "" {
			th := files.TokenHash(string(tokenEnc))
			if runtime, ok := runtimes[th]; ok && runtime != nil {
				return runtime, bestInstanceID, nil
			}
		}
	}

	if len(guild.BotInstanceTokens) > 0 {
		for id, tokenEnc := range guild.BotInstanceTokens {
			if string(tokenEnc) == "" {
				continue
			}
			th := files.TokenHash(string(tokenEnc))
			if runtime, ok := runtimes[th]; ok && runtime != nil {
				return runtime, id, nil
			}
		}
	} else {
		// Gracefully fallback to the magic blank instance if the guild has no configured tokens.
		if runtime, ok := runtimes[""]; ok && runtime != nil {
			return runtime, "", nil
		}
	}

	return nil, "", fmt.Errorf("guild %s does not resolve to a running bot instance", guildID)
}

func (r *botRuntimeResolver) sessionForGuild(guildID string, feature string) (*discordgo.Session, error) {
	runtime, botInstanceID, err := r.runtimeForGuild(guildID, feature)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSessionUnavailable, err)
	}
	if runtime.session == nil {
		guildID = strings.TrimSpace(guildID)
		if guildID == "" {
			return nil, fmt.Errorf("%w: discord session for default bot instance %q is empty", ErrSessionUnavailable, botInstanceID)
		}
		return nil, fmt.Errorf("%w: discord session for guild %s (bot instance %q) is empty", ErrSessionUnavailable, guildID, botInstanceID)
	}
	return runtime.session, nil
}

func (r *botRuntimeResolver) registerGuild(_ context.Context, guildID string) error {
	if r == nil || r.configManager == nil {
		return fmt.Errorf("bot runtime resolver is unavailable")
	}
	return r.configManager.EnsureMinimalGuildConfig(guildID)
}

func (r *botRuntimeResolver) guildBindings(context.Context) ([]control.BotGuildBinding, error) {
	if r == nil {
		return nil, fmt.Errorf("bot runtime resolver is unavailable")
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

	for _, guild := range cfg.Guilds {
		for botInstanceID, tokenEnc := range guild.BotInstanceTokens {
			token := string(tokenEnc)
			if token == "" {
				continue
			}
			th := files.TokenHash(token)
			runtime, ok := runtimes[th]
			if !ok || runtime == nil || runtime.session == nil {
				continue
			}

			// Check if this token's runtime actually sees this guild
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
		return nil, fmt.Errorf("listBotGuildBindingsFromSessionState: %w", err)
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

// Ptr is a generic helper for inline pointer allocations.
