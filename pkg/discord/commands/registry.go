package commands

import (
	"iter"
	"log/slog"
	"sync"
)

// CommandRegistry securely manages registered Arikawa commands.
// It leverages a reader-writer mutex to guarantee safe concurrent reads
// during rapid execution intervals and writes during the boot cycle.
type CommandRegistry struct {
	mu       sync.RWMutex
	commands map[string]ArikawaCommand
}

// NewCommandRegistry creates an initialized command registry.
func NewCommandRegistry() *CommandRegistry {
	return &CommandRegistry{
		commands: make(map[string]ArikawaCommand),
	}
}

// Register safely associates a given command with its declared name.
func (r *CommandRegistry) Register(cmd ArikawaCommand) {
	if cmd == nil {
		return
	}

	// Operational Annotation: We enforce a full write lock (mu.Lock) rather than RLock
	// because registration mutates the shared map schema. This mitigates race conditions
	// during multi-module concurrent registration at bot boot sequence.
	r.mu.Lock()
	defer r.mu.Unlock()

	slog.Info("Architectural state transition: Registering native command",
		slog.String("command_name", cmd.Name()),
	)

	r.commands[cmd.Name()] = cmd
}

// GetCommand safely retrieves a previously registered command by its exact name.
func (r *CommandRegistry) GetCommand(name string) (ArikawaCommand, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	cmd, exists := r.commands[name]
	return cmd, exists
}

// Len securely returns the total number of top-level registered commands.
func (r *CommandRegistry) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.commands)
}

// All returns an iterator over all registered commands.
// It acquires a read lock for each iteration step.
func (r *CommandRegistry) All() iter.Seq2[string, ArikawaCommand] {
	return func(yield func(string, ArikawaCommand) bool) {
		r.mu.RLock()
		defer r.mu.RUnlock()
		for name, cmd := range r.commands {
			if !yield(name, cmd) {
				return
			}
		}
	}
}
