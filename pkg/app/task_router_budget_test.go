package app

import (
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/files"
)

func TestResolveRuntimeTaskRouterWorkersUsesAutoBudgets(t *testing.T) {
	t.Parallel()

	if got := resolveRuntimeTaskRouterWorkers(nil, "default", "default", 1); got != defaultSingleRuntimeMaxWorkers {
		t.Fatalf("expected single-runtime default budget %d, got %d", defaultSingleRuntimeMaxWorkers, got)
	}
	if got := resolveRuntimeTaskRouterWorkers(nil, "default", "default", 2); got != defaultMultiRuntimeMaxWorkers {
		t.Fatalf("expected multi-runtime default budget %d, got %d", defaultMultiRuntimeMaxWorkers, got)
	}
}

func TestResolveRuntimeTaskRouterWorkersUsesSmallestRuntimeOverride(t *testing.T) {
	t.Parallel()

	cfg := &files.BotConfig{
		RuntimeConfig: files.RuntimeConfig{
			GlobalMaxWorkers: 10,
		},
		Guilds: []files.GuildConfig{
			{
				GuildID:       "g1",
				BotInstanceID: "alpha",
				RuntimeConfig: files.RuntimeConfig{
					GlobalMaxWorkers: 6,
				},
			},
			{
				GuildID:       "g2",
				BotInstanceID: "alpha",
				RuntimeConfig: files.RuntimeConfig{
					GlobalMaxWorkers: 3,
				},
			},
			{
				GuildID:       "g3",
				BotInstanceID: "beta",
			},
		},
	}

	if got := resolveRuntimeTaskRouterWorkers(cfg, "alpha", "default", 2); got != 3 {
		t.Fatalf("expected alpha runtime to use smallest non-zero override 3, got %d", got)
	}
	if got := resolveRuntimeTaskRouterWorkers(cfg, "beta", "default", 2); got != 10 {
		t.Fatalf("expected beta runtime to fall back to global override 10, got %d", got)
	}
}

func TestNewRuntimeTaskRouterConfigBuildsSharedLimiter(t *testing.T) {
	t.Parallel()

	cfg := &files.BotConfig{
		RuntimeConfig: files.RuntimeConfig{
			GlobalMaxWorkers: 5,
		},
	}

	routerCfg := newRuntimeTaskRouterConfig(cfg, "default", "default", 1)
	if routerCfg.GlobalMaxWorkers != 5 {
		t.Fatalf("expected router config workers 5, got %d", routerCfg.GlobalMaxWorkers)
	}
	if routerCfg.ExecutionLimiter == nil {
		t.Fatal("expected shared execution limiter to be configured")
	}
	if got := routerCfg.ExecutionLimiter.Capacity(); got != 5 {
		t.Fatalf("expected limiter capacity 5, got %d", got)
	}
}
