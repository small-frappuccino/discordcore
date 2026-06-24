package core

import (
	"fmt"
	"iter"
	"log/slog"
	"sync"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
)

// CommandHandler defines the canonical function signature for executing a slash command.
type CommandHandler func(ctx *InteractionContext) error

// Command models a single executable Discord slash command mapping.
// It binds the Discord API metadata with the Go execution handler.
type Command struct {
	Name        string
	Description string
	Handler     CommandHandler
}

// CommandRegistry manages the lifecycle and retrieval of all registered slash commands.
// It leverages a read-write mutex to serialize initialization phases against concurrent access.
type CommandRegistry struct {
	mu       sync.RWMutex
	commands map[string]*Command
	sealed   bool
}

// NewCommandRegistry instantiates a mutable, empty command registry.
func NewCommandRegistry() *CommandRegistry {
	return &CommandRegistry{
		commands: make(map[string]*Command),
	}
}

// Register injects a new command into the registry.
// It rejects mutations if the registry has been explicitly sealed post-initialization
// to guarantee deterministic routing behaviors during the application lifecycle.
func (r *CommandRegistry) Register(cmd *Command) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.sealed {
		return fmt.Errorf("registry is sealed")
	}
	r.commands[cmd.Name] = cmd
	return nil
}

// Seal finalizes the registry state, blocking any subsequent calls to Register.
// Executing this transition post-initialization elides lock contention costs on pure reads.
func (r *CommandRegistry) Seal() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sealed = true
}

// All yields an iterator sequence of all registered commands.
// The read lock is held exclusively during iterator traversal to prevent
// race conditions if the caller iterates before the registry seals.
func (r *CommandRegistry) All() iter.Seq[*Command] {
	return func(yield func(*Command) bool) {
		r.mu.RLock()
		cmds := make([]*Command, 0, len(r.commands))
		for _, cmd := range r.commands {
			cmds = append(cmds, cmd)
		}
		r.mu.RUnlock()

		for _, cmd := range cmds {
			if !yield(cmd) {
				return
			}
		}
	}
}

// Get resolves a registered command by its exact string identifier.
// It returns the target command and a boolean flag indicating presence.
func (r *CommandRegistry) Get(name string) (*Command, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	cmd, ok := r.commands[name]
	return cmd, ok
}

// BulkOverwriteClient exposes the minimal Arikawa API surface necessary
// to synchronize local command configurations with the Discord API.
type BulkOverwriteClient interface {
	BulkOverwriteCommands(appID discord.AppID, commands []api.CreateCommandData) ([]discord.Command, error)
}

// Sync overwrites the upstream Discord application command state with the local registry.
// This operation is highly destructive to the remote state and should execute
// exclusively during primary orchestration startup to prevent split-brain conflicts.
func (r *CommandRegistry) Sync(client BulkOverwriteClient, appID discord.AppID) error {
	var createData []api.CreateCommandData

	// Isolate the registry read-lock to the immediate snapshot phase.
	// Holding the lock during the high-latency network call is prohibited
	// as it stalls the primary dispatcher routines processing gateway events.
	r.mu.RLock()
	for _, cmd := range r.commands {
		createData = append(createData, api.CreateCommandData{
			Name:        cmd.Name,
			Description: cmd.Description,
		})
	}
	count := len(createData)
	r.mu.RUnlock()

	slog.Info("Syncing commands to Discord",
		slog.String("operation", "registry.sync"),
		slog.String("appID", appID.String()),
		slog.Int("count", count),
	)

	_, err := client.BulkOverwriteCommands(appID, createData)
	if err != nil {
		slog.Error("Failed to sync commands to Discord",
			slog.String("operation", "registry.sync_failed"),
			slog.String("appID", appID.String()),
			slog.String("error", err.Error()),
			slog.String("syntheticFailure", "500"),
		)
	}
	return err
}
