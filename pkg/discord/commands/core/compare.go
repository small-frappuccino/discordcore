package core

import (
	"encoding/json"

	"github.com/small-frappuccino/discordgo"
)

// normalizeCommandOptions ensures required options come before optional options,
// recursively for nested option trees, while preserving relative order.
func normalizeCommandOptions(options []*discordgo.ApplicationCommandOption) []*discordgo.ApplicationCommandOption {
	if len(options) == 0 {
		return nil
	}

	required := make([]*discordgo.ApplicationCommandOption, 0, len(options))
	optional := make([]*discordgo.ApplicationCommandOption, 0, len(options))

	for _, opt := range options {
		if opt == nil {
			continue
		}
		cloned := *opt
		cloned.Options = normalizeCommandOptions(opt.Options)

		if len(opt.Choices) > 0 {
			cloned.Choices = append([]*discordgo.ApplicationCommandOptionChoice(nil), opt.Choices...)
		}
		if len(opt.ChannelTypes) > 0 {
			cloned.ChannelTypes = append([]discordgo.ChannelType(nil), opt.ChannelTypes...)
		}

		if cloned.Required {
			required = append(required, &cloned)
		} else {
			optional = append(optional, &cloned)
		}
	}

	out := make([]*discordgo.ApplicationCommandOption, 0, len(required)+len(optional))
	out = append(out, required...)
	out = append(out, optional...)
	return out
}

// CompareCommands compares two commands to check if they are semantically equal.
// Option order is normalized so equivalent command definitions with required-first
// normalization compare as equal. DefaultMemberPermissions is included so the
// sync path does not flag drift when only the permission floor changes — and so
// it does flag drift when Discord-side state diverges from the local declaration.
func CompareCommands(a, b *discordgo.ApplicationCommand) bool {
	if a == nil || b == nil {
		return a == b
	}
	type comparable struct {
		Name                     string                                `json:"name"`
		Description              string                                `json:"description"`
		Options                  []*discordgo.ApplicationCommandOption `json:"options"`
		DefaultMemberPermissions *int64                                `json:"default_member_permissions"`
	}
	ca := comparable{a.Name, a.Description, normalizeCommandOptions(a.Options), a.DefaultMemberPermissions}
	cb := comparable{b.Name, b.Description, normalizeCommandOptions(b.Options), b.DefaultMemberPermissions}
	ba, _ := json.Marshal(ca)
	bb, _ := json.Marshal(cb)
	return string(ba) == string(bb)
}
