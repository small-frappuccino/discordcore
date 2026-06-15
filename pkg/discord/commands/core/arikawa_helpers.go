package core

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
