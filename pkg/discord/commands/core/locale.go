package core

import "github.com/bwmarrin/discordgo"

// Locale resolves the best-match supported locale for this interaction.
// Guild locale is preferred (visible to all server members) over the user's
// own client locale. When neither maps to a supported locale, en-US is used.
func (ctx *Context) Locale() discordgo.Locale {
	if ctx == nil || ctx.Interaction == nil {
		return discordgo.EnglishUS
	}
	if ctx.Interaction.GuildLocale != nil {
		if l := mapLocale(*ctx.Interaction.GuildLocale); l != discordgo.Unknown {
			return l
		}
	}
	if l := mapLocale(ctx.Interaction.Locale); l != discordgo.Unknown {
		return l
	}
	return discordgo.EnglishUS
}

// mapLocale maps a Discord locale to the nearest supported catalog locale.
// Unknown is returned when there is no supported mapping.
func mapLocale(l discordgo.Locale) discordgo.Locale {
	switch l {
	case discordgo.PortugueseBR:
		return discordgo.PortugueseBR
	case discordgo.EnglishUS, discordgo.EnglishGB:
		return discordgo.EnglishUS
	default:
		return discordgo.Unknown
	}
}
