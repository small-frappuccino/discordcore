package control

import "github.com/small-frappuccino/discordgo"

func (s *Server) discordSessionForGuild(guildID string) (*discordgo.Session, error) {
	if s == nil || s.discordSession == nil {
		return nil, nil
	}
	return s.discordSession(guildID)
}
