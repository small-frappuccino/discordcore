package moderation

import (
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
)

// RegisterModerationCommands registers the moderation slash commands as
// top-level commands without observability wiring. Equivalent to passing nil
// to RegisterModerationCommandsWithMetrics; the clean command will fall back
// to NopMetrics.
func RegisterModerationCommands(router *core.CommandRouter) {
	RegisterModerationCommandsWithMetrics(router, nil)
}

// RegisterModerationCommandsWithMetrics registers each moderation command as a
// top-level slash command. It is the canonical entry point when the host
// runtime wires a moderation Metrics sink (production startup wires the
// in-memory implementation so /v1/health/moderation has counters to expose).
// Passing a nil metrics value falls back to NopMetrics so library tests that
// don't care about observability stay clean.
func RegisterModerationCommandsWithMetrics(router *core.CommandRouter, metrics Metrics) {
	if metrics == nil {
		metrics = NopMetrics{}
	}
	configManager := router.GetConfigManager()

	router.RegisterSlashCommand(newBanCommand())
	router.RegisterSlashCommand(newMassBanCommand())
	router.RegisterSlashCommand(newCleanCommand(metrics))
	router.RegisterSlashCommand(newKickCommand())
	router.RegisterSlashCommand(newMuteCommand())
	router.RegisterSlashCommand(newReactionBlockCommand(configManager))
	router.RegisterSlashCommand(newTimeoutCommand())
	router.RegisterSlashCommand(newWarnCommand())
	router.RegisterSlashCommand(newWarningsCommand())
}
