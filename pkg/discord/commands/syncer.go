package commands

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
)

// CommandSyncer orchestrates the state alignment between the local AST
// (CommandRegistry) and the Discord API via Arikawa.
type CommandSyncer struct {
	client *api.Client
	appID  discord.AppID
	logger *slog.Logger
}

// NewCommandSyncer allocates a new native API syncer.
func NewCommandSyncer(client *api.Client, appID discord.AppID) *CommandSyncer {
	return &CommandSyncer{
		client: client,
		appID:  appID,
	}
}

// SetLogger injects a logger into the syncer.
func (s *CommandSyncer) SetLogger(logger *slog.Logger) {
	s.logger = logger
}

func (s *CommandSyncer) log() *slog.Logger {
	if s.logger != nil {
		return s.logger
	}
	return slog.Default()
}

// BuildCreateData maps internal ArikawaCommand interfaces into the exact
// payload structure demanded by Discord's Bulk Overwrite endpoint.
func (s *CommandSyncer) BuildCreateData(registry *CommandRegistry) []api.CreateCommandData {
	data := make([]api.CreateCommandData, 0, registry.Len())

	for _, cmd := range registry.All() {
		createData := api.CreateCommandData{
			Name:        cmd.Name(),
			Description: cmd.Description(),
			Options:     cmd.Options(),
		}

		if provider, ok := cmd.(DefaultMemberPermissionsProvider); ok {
			perms := provider.DefaultMemberPermissions()
			createData.DefaultMemberPermissions = &perms
		}

		data = append(data, createData)
	}

	return data
}

// SyncBulkOverwrite performs a destructive overwrite of the current Discord
// application commands, mapping local registry state exactly 1:1 to the gateway.
func (s *CommandSyncer) SyncBulkOverwrite(guildID discord.GuildID, registry *CommandRegistry) error {
	data := s.BuildCreateData(registry)

	// Operational Annotation: We rely on BulkOverwriteCommands to atomically
	// insert, update, and delete all commands. This avoids complex diffing logic
	// natively while delegating the heavy lifting to Discord's backend.
	var err error
	if guildID.IsValid() {
		_, err = s.client.BulkOverwriteGuildCommands(s.appID, guildID, data)
	} else {
		_, err = s.client.BulkOverwriteCommands(s.appID, data)
	}

	if err != nil {
		s.log().Error("Bulk command synchronization failed",
			slog.String("guild_id", guildID.String()),
			slog.Any("error", err),
		)
		return fmt.Errorf("bulk overwrite failed: %w", err)
	}

	s.log().Info("Successfully synchronized commands via BulkOverwrite",
		slog.String("guild_id", guildID.String()),
		slog.Int("total_commands", len(data)),
	)
	return nil
}

// Diff identifies mutations by comparing local registry state against remote Discord commands.
func (s *CommandSyncer) Diff(ctx context.Context, guildID discord.GuildID, registry *CommandRegistry) (added, updated, deleted int, err error) {
	var remoteCmds []discord.Command
	if guildID.IsValid() {
		remoteCmds, err = s.client.GuildCommands(s.appID, guildID)
	} else {
		remoteCmds, err = s.client.Commands(s.appID)
	}

	if err != nil {
		return 0, 0, 0, err
	}

	remoteMap := make(map[string]discord.Command, len(remoteCmds))
	for _, cmd := range remoteCmds {
		remoteMap[cmd.Name] = cmd
	}

	for name := range registry.All() {
		if _, exists := remoteMap[name]; !exists {
			added++
		} else {
			// A deep semantic comparison of options/permissions would go here.
			// For architectural purity, we rely on bulk overwrites. This diff
			// is purely for observability/telemetry.
			updated++
		}
	}

	for name := range remoteMap {
		if _, exists := registry.GetCommand(name); !exists {
			deleted++
		}
	}

	return added, updated, deleted, nil
}
