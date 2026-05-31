package core

import (
	"strings"

	"github.com/bwmarrin/discordgo"
)

// FilterChoices filters choices based on user input
func FilterChoices(choices []*discordgo.ApplicationCommandOptionChoice, input string) []*discordgo.ApplicationCommandOptionChoice {
	if input == "" {
		return choices
	}

	input = strings.ToLower(input)
	filtered := make([]*discordgo.ApplicationCommandOptionChoice, 0)

	for _, choice := range choices {
		if strings.Contains(strings.ToLower(choice.Name), input) {
			filtered = append(filtered, choice)
		}
	}

	return filtered
}

// CreateChoice creates a choice for autocomplete
func CreateChoice(name, value string) *discordgo.ApplicationCommandOptionChoice {
	return &discordgo.ApplicationCommandOptionChoice{
		Name:  name,
		Value: value,
	}
}

// CreateChoicesFromStrings creates choices from a slice of strings
func CreateChoicesFromStrings(items []string) []*discordgo.ApplicationCommandOptionChoice {
	choices := make([]*discordgo.ApplicationCommandOptionChoice, len(items))
	for i, item := range items {
		choices[i] = &discordgo.ApplicationCommandOptionChoice{
			Name:  item,
			Value: item,
		}
	}
	return choices
}
