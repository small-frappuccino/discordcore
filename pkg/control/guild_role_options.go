package control

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
)

type guildRoleOption struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Position  int    `json:"position"`
	Managed   bool   `json:"managed"`
	IsDefault bool   `json:"is_default"`
}

func (s *Server) handleGuildRoleOptionsGet(w http.ResponseWriter, guildID string) {
	session, err := s.discordServiceForGuild(guildID)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to resolve guild role options: %v", err), http.StatusServiceUnavailable)
		return
	}

	options, err := buildGuildRoleOptions(session, guildID)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to build guild role options: %v", err), http.StatusServiceUnavailable)
		return
	}

	writeJSON(w, http.StatusOK, GuildRolesResponse{
		Status:  "ok",
		GuildID: guildID,
		Roles:   options,
	})
}

func buildGuildRoleOptions(session DiscordService, guildID string) ([]guildRoleOption, error) {
	guild, err := resolveGuildFromDiscordSession(session, guildID)
	if err != nil {
		return nil, fmt.Errorf("buildGuildRoleOptions: %w", err)
	}

	options := make([]guildRoleOption, 0, len(guild.Roles))
	for _, role := range guild.Roles {
		if role == nil {
			continue
		}
		options = append(options, guildRoleOption{
			ID:        strings.TrimSpace(role.ID),
			Name:      strings.TrimSpace(role.Name),
			Position:  role.Position,
			Managed:   role.Managed,
			IsDefault: strings.TrimSpace(role.ID) == strings.TrimSpace(guildID),
		})
	}

	slices.SortFunc(options, compareGuildRoleOptions)
	return options, nil
}

func guildRoleOptionIndex(session DiscordService, guildID string) (map[string]guildRoleOption, error) {
	options, err := buildGuildRoleOptions(session, guildID)
	if err != nil {
		return nil, fmt.Errorf("guildRoleOptionIndex: %w", err)
	}

	index := make(map[string]guildRoleOption, len(options))
	for _, option := range options {
		index[option.ID] = option
	}
	return index, nil
}

func resolveGuildFromDiscordSession(session DiscordService, guildID string) (*Guild, error) {
	if session == nil {
		return nil, fmt.Errorf("discord service unavailable")
	}

	guild, err := session.Guild(guildID)
	if err != nil {
		return nil, fmt.Errorf("load guild %s from discord service: %w", guildID, err)
	}
	if guild == nil {
		return nil, fmt.Errorf("guild %s unavailable in discord service", guildID)
	}
	return guild, nil
}

func compareGuildRoleOptions(left, right guildRoleOption) int {
	if left.Position != right.Position {
		if left.Position > right.Position {
			return -1
		}
		return 1
	}

	leftName := strings.ToLower(strings.TrimSpace(left.Name))
	rightName := strings.ToLower(strings.TrimSpace(right.Name))
	if leftName != rightName {
		return strings.Compare(leftName, rightName)
	}

	return strings.Compare(left.ID, right.ID)
}
