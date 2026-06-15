package files

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/small-frappuccino/discordcore/pkg/log"
)

// NewMinimalGuildConfig returns a dormant guild record for automatic discovery.
// Newly listed guilds keep all feature overrides explicitly disabled until an
// operator configures them.
func NewMinimalGuildConfig(guildID string) GuildConfig {
	disabled := false

	features := FeatureToggles{}
	for _, spec := range featureRegistry {
		// Do not forcefully disable the core command router. If we disable it, the bot
		// strips its own command list out of Discord entirely when joining a new guild.
		if spec.ID == "services.commands" {
			continue
		}
		features.SetToggle(spec.ID, boolPtr(disabled))
	}

	slog.Debug("Granular inspection: Dormant guild configuration structure materialized in memory",
		slog.String("guild_id", guildID),
	)

	return GuildConfig{
		GuildID:  strings.TrimSpace(guildID),
		Features: features,
	}
}

// EnsureMinimalGuildConfigForBot persists a dormant guild record if it does not
// exist yet. Existing guild settings are preserved.
func (mgr *ConfigManager) EnsureMinimalGuildConfig(guildID string) error {
	guildID = strings.TrimSpace(guildID)
	if guildID == "" {
		err := fmt.Errorf("guild id is required")
		log.EmitBlockingError("Blocking structural failure: Guild configuration enforcement aborted due to null identifier", err, log.GenerateRequestID())
		return err
	}

	_, err := mgr.UpdateConfig(func(cfg *BotConfig) error {
		for idx := range cfg.Guilds {
			if cfg.Guilds[idx].GuildID != guildID {
				continue
			}

			slog.Debug("Granular inspection: Guild configuration already resident in operational matrix",
				slog.String("guild_id", guildID),
				slog.Int("matrix_index", idx),
			)
			return nil
		}

		cfg.Guilds = append(cfg.Guilds, NewMinimalGuildConfig(guildID))

		slog.Info("Architectural state transition: Dormant guild node appended to global configuration tree",
			slog.String("guild_id", guildID),
		)

		return nil
	})

	if err != nil {
		errWrap := fmt.Errorf("ensure minimal guild config for %s: %w", guildID, err)
		log.EmitBlockingError("Blocking structural failure: State mutation transaction rejected during guild enforcement", errWrap, log.GenerateRequestID())
		return errWrap
	}
	return nil
}
