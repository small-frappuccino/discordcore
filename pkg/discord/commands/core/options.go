package core

import (
	"fmt"
	"strings"

	"github.com/small-frappuccino/discordgo"
)

// OptionList simplifies extraction of options for Discord commands
type OptionList []*discordgo.ApplicationCommandInteractionDataOption

// String extracts a string option by name
func (e OptionList) String(name string) string {
	for _, opt := range e {
		if opt.Name == name {
			return strings.TrimSpace(opt.StringValue())
		}
	}
	return ""
}

// StringRequired extracts a required string option
func (e OptionList) StringRequired(name string) (string, error) {
	value := e.String(name)
	if value == "" {
		return "", &ValidationError{Field: name, Message: fmt.Sprintf("Option '%s' is required", name)}
	}
	return value, nil
}

// Bool extracts a boolean option by name
func (e OptionList) Bool(name string) bool {
	for _, opt := range e {
		if opt.Name == name {
			return opt.BoolValue()
		}
	}
	return false
}

// Int extracts an integer option by name
func (e OptionList) Int(name string) int64 {
	for _, opt := range e {
		if opt.Name == name {
			return opt.IntValue()
		}
	}
	return 0
}

// Float extracts a float option by name
func (e OptionList) Float(name string) float64 {
	for _, opt := range e {
		if opt.Name == name {
			return opt.FloatValue()
		}
	}
	return 0
}

// HasOption checks whether an option exists
func (e OptionList) HasOption(name string) bool {
	for _, opt := range e {
		if opt.Name == name {
			return true
		}
	}
	return false
}

// GetAllOptions returns all options as a map
func (e OptionList) GetAllOptions() map[string]any {
	result := make(map[string]any)
	for _, opt := range e {
		switch opt.Type {
		case discordgo.ApplicationCommandOptionString:
			result[opt.Name] = opt.StringValue()
		case discordgo.ApplicationCommandOptionInteger:
			result[opt.Name] = opt.IntValue()
		case discordgo.ApplicationCommandOptionBoolean:
			result[opt.Name] = opt.BoolValue()
		case discordgo.ApplicationCommandOptionNumber:
			result[opt.Name] = opt.FloatValue()
		}
	}
	return result
}

// GetStringOption extracts a string option value from command options
func GetStringOption(options []*discordgo.ApplicationCommandInteractionDataOption, name string) string {
	for _, option := range options {
		if option.Name == name && option.Type == discordgo.ApplicationCommandOptionString {
			return option.StringValue()
		}
	}
	return ""
}

// GetIntegerOption extracts an integer option value from command options
func GetIntegerOption(options []*discordgo.ApplicationCommandInteractionDataOption, name string) int64 {
	for _, option := range options {
		if option.Name == name && option.Type == discordgo.ApplicationCommandOptionInteger {
			return option.IntValue()
		}
	}
	return 0
}

// GetBooleanOption extracts a boolean option value from command options
func GetBooleanOption(options []*discordgo.ApplicationCommandInteractionDataOption, name string) bool {
	for _, option := range options {
		if option.Name == name && option.Type == discordgo.ApplicationCommandOptionBoolean {
			return option.BoolValue()
		}
	}
	return false
}

// ChannelID extracts a channel ID option by name
func (e OptionList) ChannelID(name string) string {
	for _, opt := range e {
		if opt.Name == name {
			if s, ok := opt.Value.(string); ok {
				return s
			}
		}
	}
	return ""
}

// RoleID extracts a role ID option by name
func (e OptionList) RoleID(name string) string {
	for _, opt := range e {
		if opt.Name == name {
			if s, ok := opt.Value.(string); ok {
				return s
			}
		}
	}
	return ""
}

// UserID extracts a user ID option by name
func (e OptionList) UserID(name string) string {
	for _, opt := range e {
		if opt.Name == name {
			if s, ok := opt.Value.(string); ok {
				return s
			}
		}
	}
	return ""
}
