package legacycore

import "github.com/diamondburned/arikawa/v3/discord"

// ArikawaOptionList is a helper for extracting options from Arikawa interactions.
type ArikawaOptionList []discord.CommandInteractionOption

// String gets a string option.
func (l ArikawaOptionList) String(name string) string {
	for _, opt := range l {
		if opt.Name == name {
			return opt.String()
		}
	}
	return ""
}

// ChannelID gets a channel ID option.
func (l ArikawaOptionList) ChannelID(name string) string {
	for _, opt := range l {
		if opt.Name == name {
			chID, _ := opt.SnowflakeValue()
			if chID != 0 {
				return chID.String()
			}
		}
	}
	return ""
}

// RoleID gets a role ID option.
func (l ArikawaOptionList) RoleID(name string) string {
	for _, opt := range l {
		if opt.Name == name {
			rID, _ := opt.SnowflakeValue()
			if rID != 0 {
				return rID.String()
			}
		}
	}
	return ""
}

// Float gets a float option.
func (l ArikawaOptionList) Float(name string) float64 {
	for _, opt := range l {
		if opt.Name == name {
			f, _ := opt.FloatValue()
			return f
		}
	}
	return 0
}

// HasOption checks if an option is present.
func (l ArikawaOptionList) HasOption(name string) bool {
	for _, opt := range l {
		if opt.Name == name {
			return true
		}
	}
	return false
}

// Bool gets a boolean option.
func (l ArikawaOptionList) Bool(name string) bool {
	for _, opt := range l {
		if opt.Name == name {
			b, _ := opt.BoolValue()
			return b
		}
	}
	return false
}

// Int gets an integer option.
func (l ArikawaOptionList) Int(name string) int64 {
	for _, opt := range l {
		if opt.Name == name {
			i, _ := opt.IntValue()
			return i
		}
	}
	return 0
}

// GetArikawaSubCommandOptions extracts options considering subcommand nesting.
func GetArikawaSubCommandOptions(i *discord.InteractionEvent) []discord.CommandInteractionOption {
	if i == nil {
		return nil
	}
	data, ok := i.Data.(*discord.CommandInteraction)
	if !ok || len(data.Options) == 0 {
		return nil
	}

	opt := data.Options[0]
	if opt.Type == discord.SubcommandOptionType {
		return opt.Options
	}
	if opt.Type == discord.SubcommandGroupOptionType && len(opt.Options) > 0 {
		// Subcommand inside a group
		return opt.Options[0].Options
	}

	return data.Options
}
