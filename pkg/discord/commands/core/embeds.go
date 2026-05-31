package core

import (
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/theme"
)

// SuccessEmbed creates a success embed
func SuccessEmbed(title, description string) *discordgo.MessageEmbed {
	return &discordgo.MessageEmbed{
		Title:       title,
		Description: description,
		Color:       theme.Success(),
		Timestamp:   time.Now().Format(time.RFC3339),
	}
}

// ErrorEmbed creates an error embed
func ErrorEmbed(title, description string) *discordgo.MessageEmbed {
	return &discordgo.MessageEmbed{
		Title:       title,
		Description: description,
		Color:       theme.Error(),
		Timestamp:   time.Now().Format(time.RFC3339),
	}
}

// InfoEmbed creates an informational embed
func InfoEmbed(title, description string) *discordgo.MessageEmbed {
	return &discordgo.MessageEmbed{
		Title:       title,
		Description: description,
		Color:       theme.Info(),
		Timestamp:   time.Now().Format(time.RFC3339),
	}
}

// WarningEmbed creates a warning embed
func WarningEmbed(title, description string) *discordgo.MessageEmbed {
	return &discordgo.MessageEmbed{
		Title:       title,
		Description: description,
		Color:       theme.Warning(),
		Timestamp:   time.Now().Format(time.RFC3339),
	}
}
