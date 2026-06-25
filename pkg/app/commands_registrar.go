package app

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/cmd"
)

// CommandRegistrar compiles command groups and hashes them for O(1) routing and state syncing.
type CommandRegistrar struct {
	mu           sync.RWMutex
	syncedHashes map[discord.AppID]string
}

// CommandCatalogCapabilities defines a bitmask for capability requirements.
type CommandCatalogCapabilities uint64

const (
	// CapNone represents no special capabilities required.
	CapNone CommandCatalogCapabilities = 0

	// CapStats indicates the registrar requires the Stats subsystem.
	CapStats CommandCatalogCapabilities = 1 << iota
	CapBanMembers
	CapKickMembers
	CapManageMessages
	CapQOTDAdmin
)

// Has evaluates if the target capability is present in the bitmask.
func (c CommandCatalogCapabilities) Has(target CommandCatalogCapabilities) bool {
	if target == CapNone {
		return true
	}
	return (c & target) == target
}

// NewCommandRegistrar creates a new CommandRegistrar.
func NewCommandRegistrar() *CommandRegistrar {
	return &CommandRegistrar{
		syncedHashes: make(map[discord.AppID]string),
	}
}

// BulkOverwriteClient exposes the Arikawa API surface for syncing commands.
type BulkOverwriteClient interface {
	BulkOverwriteCommands(appID discord.AppID, commands []api.CreateCommandData) ([]discord.Command, error)
}

// CompileAndSync consumes command groups, compiles an O(1) routing map, and conditionally syncs via hashing.
func (r *CommandRegistrar) CompileAndSync(
	client BulkOverwriteClient,
	appID discord.AppID,
	guildID string,
	botProfileID string,
	groups []cmd.CommandGroup,
) (map[string]cmd.CommandHandler, error) {

	routerMap := make(map[string]cmd.CommandHandler)
	var allCreateData []api.CreateCommandData

	for _, g := range groups {
		// Populate O(1) map
		handlers := g.Handle(guildID, botProfileID)
		for name, handler := range handlers {
			if _, exists := routerMap[name]; exists {
				return nil, fmt.Errorf("duplicate command handler for %s", name)
			}
			routerMap[name] = handler
		}

		// Collect AST tree for hashing
		data := g.Register(guildID, botProfileID)
		allCreateData = append(allCreateData, data...)
	}

	// Compute deterministic hash (SHA-256) of the AST-generated command tree
	bytes, err := json.Marshal(allCreateData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal command tree for hashing: %w", err)
	}
	hash := fmt.Sprintf("%x", sha256.Sum256(bytes))

	// Conditionally sync to Discord
	r.mu.RLock()
	lastHash, exists := r.syncedHashes[appID]
	r.mu.RUnlock()

	if !exists || lastHash != hash {
		slog.Info("Command tree hash mismatch, executing Bulk Overwrite",
			slog.String("appID", appID.String()),
			slog.String("oldHash", lastHash),
			slog.String("newHash", hash),
		)

		_, err := client.BulkOverwriteCommands(appID, allCreateData)
		if err != nil {
			return nil, fmt.Errorf("failed to bulk overwrite commands: %w", err)
		}

		r.mu.Lock()
		r.syncedHashes[appID] = hash
		r.mu.Unlock()
	} else {
		slog.Debug("Command tree hash matches, skipping Bulk Overwrite",
			slog.String("appID", appID.String()),
			slog.String("hash", hash),
		)
	}

	return routerMap, nil
}
