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

	selected := 0
	if cfg.RuntimeConfig.GlobalMaxWorkers > 0 {
		selected = cfg.RuntimeConfig.GlobalMaxWorkers
	}

	// State Bleed Resolved: Determine the maximum required concurrency bound
	// across all attached guilds to prevent a single restrictive tenant
	// from starving the entire shared generic bot ecosystem.
	for _, guild := range cfg.GuildsForBotInstance(botInstanceID) {
		override := guild.RuntimeConfig.GlobalMaxWorkers
		if override > selected {
			selected = override
		}
	}

	if selected <= 0 {
		return 0, false
	}
	return selected, true
}

// newRuntimeTaskRouterConfig builds the deterministic routing rules for background execution.
func newRuntimeTaskRouterConfig(cfg *files.BotConfig, botInstanceID string, runtimeCount int) task.RouterConfig {
	workers := resolveRuntimeTaskRouterWorkers(cfg, botInstanceID, runtimeCount)

	// Initialize execution limiter to enforce strict concurrency boundaries.
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
