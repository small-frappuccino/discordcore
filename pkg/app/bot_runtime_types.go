package app

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/control"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands"
	"github.com/small-frappuccino/discordcore/pkg/discord/logging"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/service"
	"github.com/small-frappuccino/discordcore/pkg/util"
)

const DefaultBotInstanceID = "default"

var ErrNoBotTokensConfigured = errors.New("no bot instances have a configured token")

// BotInstanceDefinition describes one Discord bot instance managed by the host
// runtime. Tokens remain host-owned and are referenced by environment variable.
type BotInstanceDefinition struct {
	ID       string
	TokenEnv string
	Optional bool
}

type resolvedBotInstance struct {
	ID       string
	TokenEnv string
	Token    string
}

type botRuntime struct {
	instanceID        string
	capabilities      botRuntimeCapabilities
	session           *discordgo.Session
	serviceManager    *service.ServiceManager
	monitoringService *logging.MonitoringService
	commandHandler    *commands.CommandHandler
	persistStop       chan struct{}
}

type botRuntimeResolver struct {
	configManager        *files.ConfigManager
	runtimes             map[string]*botRuntime
	defaultBotInstanceID string
}

func resolveBotInstances(primaryTokenEnv string, opts RunOptions) ([]resolvedBotInstance, string, error) {
	catalog := opts.BotCatalog
	defaultOwnerBotInstanceID := strings.TrimSpace(opts.DefaultOwnerBotInstanceID)
	domainSupport := newRuntimeDomainSupport(opts.SupportedDomains)
	if len(catalog) == 0 {
		primaryTokenEnv = strings.TrimSpace(primaryTokenEnv)
		if primaryTokenEnv == "" {
			return nil, "", fmt.Errorf("token environment variable is required")
		}
		catalog = []BotInstanceDefinition{{
			ID:       DefaultBotInstanceID,
			TokenEnv: primaryTokenEnv,
		}}
	}

	resolved := make([]resolvedBotInstance, 0, len(catalog))
	seenIDs := make(map[string]struct{}, len(catalog))
	resolvedIDs := make(map[string]struct{}, len(catalog))

	for _, item := range catalog {
		botInstanceID := strings.TrimSpace(item.ID)
		if botInstanceID == "" {
			return nil, "", fmt.Errorf("bot instance id is required")
		}
		if _, exists := seenIDs[botInstanceID]; exists {
			return nil, "", fmt.Errorf("duplicate bot instance id: %s", botInstanceID)
		}
		seenIDs[botInstanceID] = struct{}{}

		tokenEnv := strings.TrimSpace(item.TokenEnv)
		if tokenEnv == "" {
			return nil, "", fmt.Errorf("bot instance %s is missing token env", botInstanceID)
		}

		token := resolveBotToken(tokenEnv)
		if token == "" {
			if item.Optional {
				log.ApplicationLogger().Info(
					"Skipping optional bot instance without token",
					"botInstanceID", botInstanceID,
					"tokenEnv", tokenEnv,
				)
				continue
			}
			return nil, "", fmt.Errorf("%s not set in environment or .env file", tokenEnv)
		}

		resolved = append(resolved, resolvedBotInstance{
			ID:       botInstanceID,
			TokenEnv: tokenEnv,
			Token:    token,
		})
		resolvedIDs[botInstanceID] = struct{}{}
	}

	if len(resolved) == 0 {
		return nil, "", ErrNoBotTokensConfigured
	}
	if defaultOwnerBotInstanceID == "" && len(resolved) > 0 {
		defaultOwnerBotInstanceID = resolved[0].ID
	}
	if domainSupport.supportsDefaultDomain() {
		if _, ok := resolvedIDs[defaultOwnerBotInstanceID]; !ok {
			return nil, "", fmt.Errorf("default bot instance %q is not present in the runtime catalog", defaultOwnerBotInstanceID)
		}
	}

	return resolved, defaultOwnerBotInstanceID, nil
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

func resolveBotToken(tokenEnv string) string {
	tokenEnv = strings.TrimSpace(tokenEnv)
	if tokenEnv == "" {
		return ""
	}

	token := strings.TrimSpace(util.EnvString(tokenEnv, ""))
	if token != "" {
		return token
	}

	if _, err := util.LoadEnvWithLocalBinFallback(tokenEnv); err != nil {
		return ""
	}

	return strings.TrimSpace(util.EnvString(tokenEnv, ""))
}

func newBotRuntimeResolver(
	configManager *files.ConfigManager,
	runtimes map[string]*botRuntime,
	defaultBotInstanceID string,
) *botRuntimeResolver {
	return &botRuntimeResolver{
		configManager:        configManager,
		runtimes:             runtimes,
		defaultBotInstanceID: strings.TrimSpace(defaultBotInstanceID),
	}
}

func (r *botRuntimeResolver) defaultRuntime() (*botRuntime, string, error) {
	if r == nil {
		return nil, "", fmt.Errorf("bot runtime resolver is unavailable")
	}
	botInstanceID := strings.TrimSpace(r.defaultBotInstanceID)
	if botInstanceID == "" {
		return nil, "", fmt.Errorf("default bot instance is not configured")
	}
	runtime := r.runtimes[botInstanceID]
	if runtime == nil {
		return nil, botInstanceID, fmt.Errorf("default bot instance %q is unavailable", botInstanceID)
	}
	return runtime, botInstanceID, nil
}

func (r *botRuntimeResolver) runtimeForGuild(guildID string) (*botRuntime, string, error) {
	return r.runtimeForGuildDomain(guildID, "")
}

func (r *botRuntimeResolver) runtimeForGuildDomain(guildID, domain string) (*botRuntime, string, error) {
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
	botInstanceID := guild.EffectiveBotInstanceIDForDomain(domain, r.defaultBotInstanceID)
	if botInstanceID == "" {
		return nil, "", fmt.Errorf("guild %s does not resolve to a bot instance", guildID)
	}
	runtime := r.runtimes[botInstanceID]
	if runtime == nil {
		return nil, botInstanceID, fmt.Errorf("bot instance %q is unavailable for guild %s", botInstanceID, guildID)
	}
	return runtime, botInstanceID, nil
}

func (r *botRuntimeResolver) sessionForGuild(guildID string) (*discordgo.Session, error) {
	return r.sessionForGuildDomain(guildID, "")
}

func (r *botRuntimeResolver) sessionForGuildDomain(guildID, domain string) (*discordgo.Session, error) {
	runtime, botInstanceID, err := r.runtimeForGuildDomain(guildID, domain)
	if err != nil {
		return nil, err
	}
	if runtime.session == nil {
		guildID = strings.TrimSpace(guildID)
		if guildID == "" {
			return nil, fmt.Errorf("discord session for default bot instance %q is unavailable", botInstanceID)
		}
		return nil, fmt.Errorf("discord session for guild %s (bot instance %q) is unavailable", guildID, botInstanceID)
	}
	return runtime.session, nil
}

func (r *botRuntimeResolver) registerGuild(_ context.Context, guildID, botInstanceID string) error {
	if r == nil || r.configManager == nil {
		return fmt.Errorf("bot runtime resolver is unavailable")
	}
	selectedBotInstanceID := strings.TrimSpace(botInstanceID)
	if selectedBotInstanceID == "" {
		selectedBotInstanceID = r.defaultBotInstanceID
	}
	runtime := r.runtimes[selectedBotInstanceID]
	if runtime == nil || runtime.session == nil {
		return fmt.Errorf("bot instance %q is unavailable", selectedBotInstanceID)
	}
	return r.configManager.EnsureMinimalGuildConfigForBot(guildID, selectedBotInstanceID)
}

func (r *botRuntimeResolver) guildBindings(context.Context) ([]control.BotGuildBinding, error) {
	if r == nil {
		return nil, fmt.Errorf("bot runtime resolver is unavailable")
	}

	out := make([]control.BotGuildBinding, 0)
	for botInstanceID, runtime := range r.runtimes {
		if runtime == nil || runtime.session == nil {
			continue
		}
		bindings, err := listBotGuildBindingsFromSessionState(botInstanceID, runtime.session)
		if err != nil {
			return nil, err
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
		return nil, err
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
		botInstanceID := guild.EffectiveBotInstanceID(defaultBotInstanceID)
		if botInstanceID == "" {
			return fmt.Errorf("guild %s does not resolve to a bot instance", guild.GuildID)
		}
		if _, ok := knownBotInstanceIDs[botInstanceID]; !ok {
			return fmt.Errorf("guild %s references unknown bot instance %q", guild.GuildID, botInstanceID)
		}
		for domain, explicitBotInstanceID := range guild.DomainBotInstanceIDs {
			normalizedDomain := files.NormalizeBotDomain(domain)
			if normalizedDomain == "" {
				return fmt.Errorf("guild %s has an empty domain bot binding key", guild.GuildID)
			}
			switch normalizedDomain {
			case "core", "default":
				return fmt.Errorf("guild %s uses reserved domain %q; use bot_instance_id instead", guild.GuildID, normalizedDomain)
			}

			normalizedBotInstanceID := files.NormalizeBotInstanceID(explicitBotInstanceID)
			if normalizedBotInstanceID == "" {
				return fmt.Errorf("guild %s domain %q does not resolve to a bot instance", guild.GuildID, normalizedDomain)
			}
			if _, ok := knownBotInstanceIDs[normalizedBotInstanceID]; !ok {
				return fmt.Errorf("guild %s domain %q references unknown bot instance %q", guild.GuildID, normalizedDomain, normalizedBotInstanceID)
			}
		}
	}
	return nil
}
