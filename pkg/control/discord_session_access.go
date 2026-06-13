package control

func (s *Server) discordServiceForGuild(guildID string) (DiscordService, error) {
	if s == nil || s.discordService == nil {
		return nil, nil
	}
	return s.discordService(guildID)
}
