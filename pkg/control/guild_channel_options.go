package control

import (
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/bwmarrin/discordgo"
)

type guildChannelOption struct {
	ID                   string `json:"id"`
	Name                 string `json:"name"`
	DisplayName          string `json:"display_name"`
	Kind                 string `json:"kind"`
	ParentID             string `json:"parent_id,omitempty"`
	ParentName           string `json:"parent_name,omitempty"`
	SupportsMessageRoute bool   `json:"supports_message_route"`
	position             int    `json:"-"`
	parentPosition       int    `json:"-"`
}

func (s *Server) handleGuildChannelOptionsGet(w http.ResponseWriter, guildID string) {
	session, err := s.discordSessionForGuild(guildID)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to resolve guild channel options: %v", err), http.StatusServiceUnavailable)
		return
	}

	options, err := buildGuildChannelOptions(session, guildID)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to build guild channel options: %v", err), http.StatusServiceUnavailable)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"guild_id": guildID,
		"channels": options,
	})
}

func buildGuildChannelOptions(session *discordgo.Session, guildID string) ([]guildChannelOption, error) {
	guild, err := resolveGuildFromDiscordSession(session, guildID)
	if err != nil {
		return nil, err
	}

	parentByID := make(map[string]*discordgo.Channel, len(guild.Channels))
	for _, channel := range guild.Channels {
		if channel == nil {
			continue
		}
		parentByID[strings.TrimSpace(channel.ID)] = channel
	}

	options := make([]guildChannelOption, 0, len(guild.Channels))
	for _, channel := range guild.Channels {
		if channel == nil {
			continue
		}

		id := strings.TrimSpace(channel.ID)
		name := strings.TrimSpace(channel.Name)
		if id == "" || name == "" {
			continue
		}

		parentID := strings.TrimSpace(channel.ParentID)
		parentName := ""
		parentPosition := -1
		if parent := parentByID[parentID]; parent != nil {
			parentName = strings.TrimSpace(parent.Name)
			parentPosition = parent.Position
		}

		kind := guildChannelKind(channel.Type)
		options = append(options, guildChannelOption{
			ID:                   id,
			Name:                 name,
			DisplayName:          formatGuildChannelDisplayName(name, parentName, kind),
			Kind:                 kind,
			ParentID:             parentID,
			ParentName:           parentName,
			SupportsMessageRoute: channelSupportsMessageRoute(channel.Type),
			position:             channel.Position,
			parentPosition:       parentPosition,
		})
	}

	slices.SortFunc(options, compareGuildChannelOptions)
	return options, nil
}

func compareGuildChannelOptions(left, right guildChannelOption) int {
	leftGroupPosition := left.position
	if left.Kind != "category" && left.parentPosition >= 0 {
		leftGroupPosition = left.parentPosition
	}
	rightGroupPosition := right.position
	if right.Kind != "category" && right.parentPosition >= 0 {
		rightGroupPosition = right.parentPosition
	}

	if leftGroupPosition != rightGroupPosition {
		if leftGroupPosition > rightGroupPosition {
			return -1
		}
		return 1
	}

	if left.Kind == "category" && right.Kind != "category" {
		return -1
	}
	if right.Kind == "category" && left.Kind != "category" {
		return 1
	}

	leftParent := strings.ToLower(strings.TrimSpace(left.ParentName))
	rightParent := strings.ToLower(strings.TrimSpace(right.ParentName))
	if leftParent != rightParent {
		return strings.Compare(leftParent, rightParent)
	}

	leftRank := guildChannelKindRank(left.Kind)
	rightRank := guildChannelKindRank(right.Kind)
	if leftRank != rightRank {
		if leftRank < rightRank {
			return -1
		}
		return 1
	}

	if left.position != right.position {
		if left.position > right.position {
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

func guildChannelKind(channelType discordgo.ChannelType) string {
	switch channelType {
	case discordgo.ChannelTypeGuildText:
		return "text"
	case discordgo.ChannelTypeGuildVoice:
		return "voice"
	case discordgo.ChannelTypeGuildCategory:
		return "category"
	case discordgo.ChannelTypeGuildNews:
		return "announcement"
	case discordgo.ChannelTypeGuildStageVoice:
		return "stage"
	case discordgo.ChannelTypeGuildForum:
		return "forum"
	case discordgo.ChannelTypeGuildNewsThread:
		return "announcement_thread"
	case discordgo.ChannelTypeGuildPublicThread:
		return "public_thread"
	case discordgo.ChannelTypeGuildPrivateThread:
		return "private_thread"
	case discordgo.ChannelTypeGuildDirectory:
		return "directory"
	case discordgo.ChannelTypeGuildMedia:
		return "media"
	default:
		return "other"
	}
}

func guildChannelKindRank(kind string) int {
	switch kind {
	case "category":
		return 0
	case "text":
		return 1
	case "announcement":
		return 2
	case "forum":
		return 3
	case "voice":
		return 4
	case "stage":
		return 5
	default:
		return 6
	}
}

func channelSupportsMessageRoute(channelType discordgo.ChannelType) bool {
	switch channelType {
	case discordgo.ChannelTypeGuildText, discordgo.ChannelTypeGuildNews:
		return true
	default:
		return false
	}
}

func formatGuildChannelDisplayName(name, parentName, kind string) string {
	trimmedName := strings.TrimSpace(name)
	trimmedParent := strings.TrimSpace(parentName)
	if trimmedName == "" {
		return ""
	}

	label := trimmedName
	if kind == "text" || kind == "announcement" {
		label = "#" + trimmedName
	}
	if trimmedParent == "" {
		return label
	}
	return trimmedParent + " / " + label
}
