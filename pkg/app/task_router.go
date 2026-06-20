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

func resolveRuntimeTaskRouterWorkers(
	cfg *files.BotConfig,
	botInstanceID string,
	runtimeCount int,
) int {
	if configured, ok := configuredRuntimeTaskRouterWorkers(cfg, botInstanceID); ok {
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
) (int, bool) {
	if cfg == nil {
		return 0, false
	}

	selected := 0
	if cfg.RuntimeConfig.GlobalMaxWorkers > 0 {
		selected = cfg.RuntimeConfig.GlobalMaxWorkers
	}

	// Iterate through all attached guilds to identify the most restrictive concurrency limit.
	for _, guild := range cfg.GuildsForBotInstance(botInstanceID) {
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
	runtimeCount int,
) task.RouterConfig {
	workers := resolveRuntimeTaskRouterWorkers(cfg, botInstanceID, runtimeCount)

	// Initialize execution limiter to enforce global concurrency boundaries across the runtime.
	limiter := task.NewExecutionLimiter(workers)

	slog.Info("Architectural state transition: Configured background worker budget for task router",
		slog.String("botInstanceID", botInstanceID),
		slog.Int("concurrency_budget", workers),
	)

	routerCfg := task.Defaults()
	routerCfg.GlobalMaxWorkers = workers
	routerCfg.ExecutionLimiter = limiter
	return routerCfg
}
