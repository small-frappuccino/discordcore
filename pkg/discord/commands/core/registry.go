package core

import (
	"fmt"
	"iter"
	"sync"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
)

type CommandHandler func(ctx *InteractionContext) error

type Command struct {
	Name        string
	Description string
	Handler     CommandHandler
}

type CommandRegistry struct {
	mu       sync.RWMutex
	commands map[string]*Command
	sealed   bool
}

func NewCommandRegistry() *CommandRegistry {
	return &CommandRegistry{
		commands: make(map[string]*Command),
	}
}

func (r *CommandRegistry) Register(cmd *Command) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.sealed {
		return fmt.Errorf("registry is sealed")
	}
	r.commands[cmd.Name] = cmd
	return nil
}

func (r *CommandRegistry) Seal() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sealed = true
}

func (r *CommandRegistry) All() iter.Seq[*Command] {
	return func(yield func(*Command) bool) {
		r.mu.RLock()
		defer r.mu.RUnlock()
		for _, cmd := range r.commands {
			if !yield(cmd) {
				return
			}
		}
	}
}

func (r *CommandRegistry) Get(name string) (*Command, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	cmd, ok := r.commands[name]
	return cmd, ok
}

type BulkOverwriteClient interface {
	BulkOverwriteCommands(appID discord.AppID, commands []api.CreateCommandData) ([]discord.Command, error)
}

func (r *CommandRegistry) Sync(client BulkOverwriteClient, appID discord.AppID) error {
	var createData []api.CreateCommandData
	r.mu.RLock()
	for _, cmd := range r.commands {
		createData = append(createData, api.CreateCommandData{
			Name:        cmd.Name,
			Description: cmd.Description,
		})
	}
	r.mu.RUnlock()

	_, err := client.BulkOverwriteCommands(appID, createData)
	return err
}
