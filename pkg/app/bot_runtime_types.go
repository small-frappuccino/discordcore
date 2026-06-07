package app

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync/atomic"

	"github.com/bwmarrin/discordgo"

	"github.com/small-frappuccino/discordcore/pkg/control"
	"github.com/small-frappuccino/discordcore/pkg/discord/cache"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands"
	"github.com/small-frappuccino/discordcore/pkg/discord/logging"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/service"
)

// DefaultBotInstanceID defines default bot instance id.
const DefaultBotInstanceID = "default"

// ErrNoBotTokensConfigured defines err no bot tokens configured.
var ErrNoBotTokensConfigured = errors.New("no bot instances have a configured token")

// ErrSessionUnavailable defines err when a bot session is not available for a guild or globally.
var ErrSessionUnavailable = errors.New("discord session is unavailable")

// BotInstanceDefinition describes one Discord bot instance managed by the host
// runtime.
type BotInstanceDefinition struct {
	ID       string
	Optional bool
}

// resolvedBotInstance describes a loaded bot ready for startup.
type resolvedBotInstance struct {
	ID    string
	Token string
}

type botRuntime struct {
	instanceID        string
	capabilities      botRuntimeCapabilities
	session           *discordgo.Session
	serviceManager    *service.ServiceManager
	monitoringService *logging.MonitoringService
	commandHandler    *commands.CommandHandler
}

type botRuntimeResolver struct {
	configManager        *files.ConfigManager
	runtimes             atomic.Value // holds map[string]*botRuntime
	defaultBotInstanceID string
}

func (r *botRuntimeResolver) getRuntimes() map[string]*botRuntime {
	if m := r.runtimes.Load(); m != nil {
		return m.(map[string]*botRuntime)
	}
	return nil
}

func (r *botRuntimeResolver) swapRuntimes(newMap map[string]*botRuntime) {
	r.runtimes.Store(newMap)
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

func newBotRuntimeResolver(
	configManager *files.ConfigManager,
	runtimes map[string]*botRuntime,
	defaultBotInstanceID string,
) *botRuntimeResolver {
	resolver := &botRuntimeResolver{
		configManager:        configManager,
		defaultBotInstanceID: strings.TrimSpace(defaultBotInstanceID),
	}
	resolver.swapRuntimes(runtimes)
	return resolver
}

// defaultUnifiedCache returns the UnifiedCache from the default bot runtime's
// monitoring service, or nil if no runtime is registered, the runtime has no
// monitoring service, or the cache has not been constructed yet. The control
// server's /v1/health/cache route calls this on each request so it sees the
// cache as soon as the runtime layer publishes it.
func (r *botRuntimeResolver) defaultUnifiedCache() *cache.UnifiedCache {
	if r == nil {
		return nil
	}
	runtime, _, err := r.defaultRuntime()
	if err != nil || runtime == nil || runtime.monitoringService == nil {
		return nil
	}
	return runtime.monitoringService.GetUnifiedCache()
}

// defaultMonitoringMetrics returns the monitoring observability sink from the
// default bot runtime's monitoring service, or nil if no runtime is
// registered or monitoring is disabled on that runtime. Mirrors
// defaultUnifiedCache; /v1/health/monitoring scrapes via this accessor.
func (r *botRuntimeResolver) defaultMonitoringMetrics() logging.Metrics {
	if r == nil {
		return nil
	}
	runtime, _, err := r.defaultRuntime()
	if err != nil || runtime == nil || runtime.monitoringService == nil {
		return nil
	}
	return runtime.monitoringService.Metrics()
}

func (r *botRuntimeResolver) defaultRuntime() (*botRuntime, string, error) {
	if r == nil {
		return nil, "", fmt.Errorf("bot runtime resolver is unavailable")
	}
	botInstanceID := strings.TrimSpace(r.defaultBotInstanceID)
	if botInstanceID == "" {
		return nil, "", fmt.Errorf("default bot instance is not configured")
	}
	runtimes := r.getRuntimes()
	runtime := runtimes[botInstanceID]
	if runtime == nil {
		return nil, botInstanceID, fmt.Errorf("default bot instance %q is unavailable", botInstanceID)
	}
	return runtime, botInstanceID, nil
}

func (r *botRuntimeResolver) runtimeForGuild(guildID string) (*botRuntime, string, error) {
	if r == nil {
		return nil, "", fmt.Errorf("bot runtime resolver is unavailable")
	}
	guildID = strings.TrimSpace(guildID)
	if guildID == "" {
		return r.defaultRuntime()
	}
	if r.configManager == nil {
		return nil, "", fmt.Errorf("bot runtime resolver config manager is unavailable")
	}
	guild := r.configManager.GuildConfig(guildID)
	if guild == nil {
		return nil, "", fmt.Errorf("guild %s is not configured", guildID)
	}

	var bestInstanceID string
	var bestRuntime *botRuntime
	runtimes := r.getRuntimes()

	for instanceID, runtime := range runtimes {
		token, ok := guild.BotInstanceTokens[instanceID]
		if ok && token != "" {
			bestInstanceID = instanceID
			bestRuntime = runtime
			if instanceID == r.defaultBotInstanceID {
				break
			}
		}
	}

	if bestRuntime == nil {
		if r.defaultBotInstanceID != "" && runtimes[r.defaultBotInstanceID] != nil {
			if len(guild.BotInstanceTokens) == 0 {
				return runtimes[r.defaultBotInstanceID], r.defaultBotInstanceID, nil
			}
		}
		return nil, "", fmt.Errorf("guild %s does not resolve to a running bot instance", guildID)
	}

	return bestRuntime, bestInstanceID, nil
}

func (r *botRuntimeResolver) sessionForGuild(guildID string) (*discordgo.Session, error) {
	runtime, botInstanceID, err := r.runtimeForGuild(guildID)
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
	for botInstanceID, runtime := range runtimes {
		if runtime == nil || runtime.session == nil {
			continue
		}
		bindings, err := listBotGuildBindingsFromSessionState(botInstanceID, runtime.session)
		if err != nil {
			return nil, fmt.Errorf("botRuntimeResolver.guildBindings: %w", err)
		}
		out = append(out, bindings...)
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

func validateConfiguredBotInstances(
	cfg *files.BotConfig,
	knownBotInstanceIDs map[string]struct{},
	defaultBotInstanceID string,
) error {
	if cfg == nil {
		return nil
	}
	for _, guild := range cfg.Guilds {
		if len(guild.BotInstanceTokens) == 0 {
			if defaultBotInstanceID == "" {
				return fmt.Errorf("guild %s does not resolve to a bot instance", guild.GuildID)
			}
			if _, ok := knownBotInstanceIDs[defaultBotInstanceID]; !ok {
				return fmt.Errorf("guild %s references unknown default bot instance %q", guild.GuildID, defaultBotInstanceID)
			}
		}
		for botInstanceID := range guild.BotInstanceTokens {
			if _, ok := knownBotInstanceIDs[botInstanceID]; !ok {
				return fmt.Errorf("guild %s references unknown bot instance %q", guild.GuildID, botInstanceID)
			}
		}
	}
	return nil
}
