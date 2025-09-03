package commands

import (
	"github.com/bwmarrin/discordgo"
)

type User struct {
	ID       string
	Username string
	Avatar   string
}

type SlashCommand struct {
	Name        string
	Description string
	Options     []*discordgo.ApplicationCommandOption
	Handler     func(s *discordgo.Session, i *discordgo.InteractionCreate)
}

type CommandCategory struct {
	Name        string
	Description string
	Commands    []SlashCommand
}
