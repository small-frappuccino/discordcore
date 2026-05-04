package control

import "github.com/bwmarrin/discordgo"

func (s *Server) discordSessionForGuild(guildID string) (*discordgo.Session, error) {
	return s.discordSessionForGuildDomain(guildID, "")
}

func (s *Server) discordSessionForGuildDomain(guildID, domain string) (*discordgo.Session, error) {
	if s == nil || s.discordSession == nil {
		return nil, nil
	}
	return s.discordSession(guildID, domain)
}
