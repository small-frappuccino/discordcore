package app

import (
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/task"
)

const (
	defaultSingleRuntimeMaxWorkers = 8
	defaultMultiRuntimeMaxWorkers  = 4
)

func resolveRuntimeTaskRouterWorkers(
	cfg *files.BotConfig,
	botInstanceID string,
	defaultBotInstanceID string,
	runtimeCount int,
) int {
	if configured, ok := configuredRuntimeTaskRouterWorkers(cfg, botInstanceID, defaultBotInstanceID); ok {
		return configured
	}
	if runtimeCount > 1 {
		return defaultMultiRuntimeMaxWorkers
	}
	return defaultSingleRuntimeMaxWorkers
}

func configuredRuntimeTaskRouterWorkers(
	cfg *files.BotConfig,
	botInstanceID string,
	defaultBotInstanceID string,
) (int, bool) {
	if cfg == nil {
		return 0, false
	}

	selected := 0
	if cfg.RuntimeConfig.GlobalMaxWorkers > 0 {
		selected = cfg.RuntimeConfig.GlobalMaxWorkers
	}

	for _, guild := range cfg.GuildsForBotInstance(botInstanceID, defaultBotInstanceID) {
		override := guild.RuntimeConfig.GlobalMaxWorkers
		if override <= 0 {
			continue
		}
		if selected == 0 || override < selected {
			selected = override
		}
	}

	if selected <= 0 {
		return 0, false
	}
	return selected, true
}

func newRuntimeTaskRouterConfig(
	cfg *files.BotConfig,
	botInstanceID string,
	defaultBotInstanceID string,
	runtimeCount int,
) task.RouterConfig {
	workers := resolveRuntimeTaskRouterWorkers(cfg, botInstanceID, defaultBotInstanceID, runtimeCount)
	limiter := task.NewExecutionLimiter(workers)

	routerCfg := task.Defaults()
	routerCfg.GlobalMaxWorkers = workers
	routerCfg.ExecutionLimiter = limiter
	return routerCfg
}
