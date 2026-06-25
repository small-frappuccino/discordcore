package commands

import (
	"github.com/diamondburned/arikawa/v3/api"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/cmd"
)

// LegacyAdapter bridges old ArikawaCommand instances to the new cmd.CommandGroup interface.
type LegacyAdapter struct {
	commands []ArikawaCommand
}

// NewLegacyAdapter constructs a CommandGroup from legacy Arikawa commands.
func NewLegacyAdapter(cmds ...ArikawaCommand) cmd.CommandGroup {
	return &LegacyAdapter{commands: cmds}
}

// Register returns the O(1) creation data.
func (la *LegacyAdapter) Register(guildID string, botProfileID string) []api.CreateCommandData {
	var data []api.CreateCommandData
	for _, c := range la.commands {
		d := api.CreateCommandData{
			Name:        c.Name(),
			Description: c.Description(),
			Options:     c.Options(),
		}
		if p, ok := c.(DefaultMemberPermissionsProvider); ok {
			perm := p.DefaultMemberPermissions()
			d.DefaultMemberPermissions = &perm
		}
		data = append(data, d)
	}
	return data
}

// Handle exposes the O(1) routing dictionary.
func (la *LegacyAdapter) Handle(guildID string, botProfileID string) map[string]cmd.CommandHandler {
	m := make(map[string]cmd.CommandHandler)
	for _, c := range la.commands {
		localCmd := c
		m[localCmd.Name()] = func(ctx *cmd.Context) error {
			legacyCtx, err := NewArikawaContext(*ctx.Event, ctx.DI.ConfigProvider())
			if err != nil {
				return err
			}
			legacyCtx.SetClient(ctx.Client)
			legacyCtx.WithContext(ctx.Context)

			// Propagate custom guild ID override if valid (used by legacy commands)
			if ctx.GuildID.IsValid() {
				legacyCtx.GuildID = ctx.GuildID
			}

			return localCmd.Handle(legacyCtx)
		}
	}
	return m
}

// ArikawaComponentAdapter bridges old ComponentHandlers.
type ArikawaComponentAdapter struct {
	customIDPrefix string
	handler        ComponentHandler
}

func NewArikawaComponentAdapter(prefix string, h ComponentHandler) *ArikawaComponentAdapter {
	return &ArikawaComponentAdapter{customIDPrefix: prefix, handler: h}
}

// NewArikawaContextFromCmd is a helper.
func NewArikawaContextFromCmd(ctx *cmd.Context) (*ArikawaContext, error) {
	legacyCtx, err := NewArikawaContext(*ctx.Event, ctx.DI.ConfigProvider())
	if err != nil {
		return nil, err
	}
	legacyCtx.SetClient(ctx.Client)
	if ctx.Context != nil {
		legacyCtx.WithContext(ctx.Context)
	}
	if ctx.GuildID.IsValid() {
		legacyCtx.GuildID = ctx.GuildID
	}
	return legacyCtx, nil
}
