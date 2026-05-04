package control

import (
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

type guildChannelOption struct {
	ID                   string `json:"id"`
	Name                 string `json:"name"`
	DisplayName          string `json:"display_name"`
	Kind                 string `json:"kind"`
	SupportsMessageRoute bool   `json:"supports_message_route"`
	position             int    `json:"-"`
}

func (s *Server) handleGuildChannelOptionsGet(w http.ResponseWriter, r *http.Request, guildID string) {
	domain := ""
	if r != nil {
		domain = files.NormalizeBotDomain(r.URL.Query().Get("domain"))
	}

	session, err := s.discordSessionForGuildDomain(guildID, domain)
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

		if channel.Type == discordgo.ChannelTypeGuildCategory {
			continue
		}

		kind := guildChannelKind(channel.Type)
		options = append(options, guildChannelOption{
			ID:                   id,
			Name:                 name,
			DisplayName:          formatGuildChannelDisplayName(name, kind),
			Kind:                 kind,
			SupportsMessageRoute: channelSupportsMessageRoute(channel.Type),
			position:             channel.Position,
		})
	}

	slices.SortFunc(options, compareGuildChannelOptions)
	return options, nil
}

func compareGuildChannelOptions(left, right guildChannelOption) int {
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
	case "text":
		return 0
	case "announcement":
		return 1
	case "forum":
		return 2
	case "media":
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

func formatGuildChannelDisplayName(name, kind string) string {
	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		return ""
	}

	if kind == "text" || kind == "announcement" {
		return "#" + trimmedName
	}

	return trimmedName
}
