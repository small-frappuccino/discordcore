package control

import "github.com/bwmarrin/discordgo"

func (s *Server) currentDiscordSession() (*discordgo.Session, error) {
	if s == nil {
		return nil, nil
	}
	return s.discordSessionForGuild("")
}

func (s *Server) discordSessionForGuild(guildID string) (*discordgo.Session, error) {
	if s == nil || s.discordSession == nil {
		return nil, nil
	}
	return s.discordSession(guildID)
}
