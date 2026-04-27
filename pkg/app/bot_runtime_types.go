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
	defaultBotInstanceID := strings.TrimSpace(opts.DefaultBotInstanceID)
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
	if defaultBotInstanceID == "" && len(resolved) > 0 {
		defaultBotInstanceID = resolved[0].ID
	}
	if _, ok := resolvedIDs[defaultBotInstanceID]; !ok {
		return nil, "", fmt.Errorf("default bot instance %q is not present in the runtime catalog", defaultBotInstanceID)
	}

	return resolved, defaultBotInstanceID, nil
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
	botInstanceID := guild.EffectiveBotInstanceID(r.defaultBotInstanceID)
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
	runtime, botInstanceID, err := r.runtimeForGuild(guildID)
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
	runtimes map[string]*botRuntime,
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
		if _, ok := runtimes[botInstanceID]; !ok {
			return fmt.Errorf("guild %s references unknown bot instance %q", guild.GuildID, botInstanceID)
		}
	}
	return nil
}
