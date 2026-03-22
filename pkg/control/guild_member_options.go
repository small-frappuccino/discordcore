package control

import (
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
)

const (
	defaultGuildMemberOptionsLimit = 25
	maxGuildMemberOptionsLimit     = 100
)

type guildMemberOption struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Username    string `json:"username"`
	Bot         bool   `json:"bot"`
}

func (s *Server) handleGuildMemberOptionsGet(w http.ResponseWriter, r *http.Request, guildID string) {
	session, err := s.discordSessionForGuild(guildID)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to resolve guild member options: %v", err), http.StatusServiceUnavailable)
		return
	}

	query := ""
	selectedID := ""
	limit := defaultGuildMemberOptionsLimit
	if r != nil {
		query = strings.TrimSpace(r.URL.Query().Get("query"))
		selectedID = strings.TrimSpace(r.URL.Query().Get("selected_id"))
		limit = parseGuildMemberOptionsLimit(r.URL.Query())
	}

	options, err := buildGuildMemberOptions(session, guildID, query, selectedID, limit)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to build guild member options: %v", err), http.StatusServiceUnavailable)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"guild_id": guildID,
		"members":  options,
	})
}

func buildGuildMemberOptions(session *discordgo.Session, guildID, query, selectedID string, limit int) ([]guildMemberOption, error) {
	if session == nil {
		return nil, fmt.Errorf("discord session unavailable")
	}

	normalizedLimit := clampGuildMemberOptionsLimit(limit)
	normalizedQuery := strings.TrimSpace(query)
	normalizedSelectedID := strings.TrimSpace(selectedID)

	options := make([]guildMemberOption, 0, normalizedLimit+1)
	seen := make(map[string]struct{}, normalizedLimit+1)

	if normalizedSelectedID != "" {
		selectedMember, err := resolveGuildMemberFromDiscordSession(session, guildID, normalizedSelectedID)
		if err == nil {
			appendGuildMemberOption(&options, seen, selectedMember)
		}
	}

	members, err := lookupGuildMembers(session, guildID, normalizedQuery, normalizedLimit)
	if err != nil {
		return nil, err
	}

	for _, member := range members {
		appendGuildMemberOption(&options, seen, member)
		if len(options) >= normalizedLimit {
			break
		}
	}

	if normalizedSelectedID != "" && len(options) > 0 && options[0].ID != normalizedSelectedID {
		if selectedMember, err := resolveGuildMemberFromDiscordSession(session, guildID, normalizedSelectedID); err == nil {
			selectedOption, ok := buildGuildMemberOption(selectedMember)
			if ok {
				options = prependGuildMemberOption(options, selectedOption)
			}
		}
	}

	if len(options) > normalizedLimit {
		options = options[:normalizedLimit]
	}
	return options, nil
}

func parseGuildMemberOptionsLimit(values url.Values) int {
	if values == nil {
		return defaultGuildMemberOptionsLimit
	}

	raw := strings.TrimSpace(values.Get("limit"))
	if raw == "" {
		return defaultGuildMemberOptionsLimit
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil {
		return defaultGuildMemberOptionsLimit
	}
	return clampGuildMemberOptionsLimit(parsed)
}

func clampGuildMemberOptionsLimit(limit int) int {
	if limit <= 0 {
		return defaultGuildMemberOptionsLimit
	}
	if limit > maxGuildMemberOptionsLimit {
		return maxGuildMemberOptionsLimit
	}
	return limit
}

func lookupGuildMembers(session *discordgo.Session, guildID, query string, limit int) ([]*discordgo.Member, error) {
	normalizedLimit := clampGuildMemberOptionsLimit(limit)
	normalizedQuery := strings.TrimSpace(query)

	if normalizedQuery != "" {
		if stateMatches := guildMembersFromState(session, guildID, normalizedQuery, normalizedLimit); len(stateMatches) > 0 {
			return stateMatches, nil
		}

		members, err := session.GuildMembersSearch(guildID, normalizedQuery, normalizedLimit)
		if err != nil {
			return nil, fmt.Errorf("search guild members for %s: %w", guildID, err)
		}
		return members, nil
	}

	if stateMembers := guildMembersFromState(session, guildID, "", normalizedLimit); len(stateMembers) > 0 {
		return stateMembers, nil
	}

	members, err := session.GuildMembers(guildID, "", normalizedLimit)
	if err != nil {
		return nil, fmt.Errorf("list guild members for %s: %w", guildID, err)
	}
	return members, nil
}

func guildMembersFromState(session *discordgo.Session, guildID, query string, limit int) []*discordgo.Member {
	guild, err := resolveGuildFromDiscordSession(session, guildID)
	if err != nil || guild == nil || len(guild.Members) == 0 {
		return nil
	}

	normalizedQuery := strings.ToLower(strings.TrimSpace(query))
	matches := make([]*discordgo.Member, 0, len(guild.Members))
	for _, member := range guild.Members {
		if member == nil || member.User == nil {
			continue
		}
		if normalizedQuery != "" && !guildMemberMatchesQuery(member, normalizedQuery) {
			continue
		}
		matches = append(matches, member)
	}

	slices.SortFunc(matches, compareGuildMembers)
	if len(matches) > limit {
		matches = matches[:limit]
	}
	return matches
}

func resolveGuildMemberFromDiscordSession(session *discordgo.Session, guildID, userID string) (*discordgo.Member, error) {
	if session == nil {
		return nil, fmt.Errorf("discord session unavailable")
	}

	normalizedGuildID := strings.TrimSpace(guildID)
	normalizedUserID := strings.TrimSpace(userID)
	if normalizedGuildID == "" || normalizedUserID == "" {
		return nil, fmt.Errorf("guild member lookup requires guild_id and user_id")
	}

	if session.State != nil {
		member, err := session.State.Member(normalizedGuildID, normalizedUserID)
		if err == nil && member != nil {
			return member, nil
		}
	}

	member, err := session.GuildMember(normalizedGuildID, normalizedUserID)
	if err != nil {
		return nil, fmt.Errorf("load member %s for guild %s: %w", normalizedUserID, normalizedGuildID, err)
	}
	if member == nil {
		return nil, fmt.Errorf("member %s unavailable in guild %s", normalizedUserID, normalizedGuildID)
	}
	return member, nil
}

func appendGuildMemberOption(options *[]guildMemberOption, seen map[string]struct{}, member *discordgo.Member) {
	option, ok := buildGuildMemberOption(member)
	if !ok {
		return
	}
	if _, exists := seen[option.ID]; exists {
		return
	}
	seen[option.ID] = struct{}{}
	*options = append(*options, option)
}

func prependGuildMemberOption(options []guildMemberOption, selected guildMemberOption) []guildMemberOption {
	filtered := make([]guildMemberOption, 0, len(options)+1)
	filtered = append(filtered, selected)
	for _, option := range options {
		if option.ID == selected.ID {
			continue
		}
		filtered = append(filtered, option)
	}
	return filtered
}

func buildGuildMemberOption(member *discordgo.Member) (guildMemberOption, bool) {
	if member == nil || member.User == nil {
		return guildMemberOption{}, false
	}

	id := strings.TrimSpace(member.User.ID)
	username := strings.TrimSpace(member.User.Username)
	displayName := strings.TrimSpace(member.DisplayName())
	if displayName == "" {
		displayName = username
	}
	if id == "" || displayName == "" {
		return guildMemberOption{}, false
	}

	return guildMemberOption{
		ID:          id,
		DisplayName: displayName,
		Username:    username,
		Bot:         member.User.Bot,
	}, true
}

func guildMemberMatchesQuery(member *discordgo.Member, query string) bool {
	if member == nil || member.User == nil {
		return false
	}

	normalizedQuery := strings.ToLower(strings.TrimSpace(query))
	if normalizedQuery == "" {
		return true
	}

	candidates := []string{
		strings.ToLower(strings.TrimSpace(member.DisplayName())),
		strings.ToLower(strings.TrimSpace(member.Nick)),
		strings.ToLower(strings.TrimSpace(member.User.Username)),
		strings.ToLower(strings.TrimSpace(member.User.GlobalName)),
		strings.ToLower(strings.TrimSpace(member.User.ID)),
	}
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if strings.HasPrefix(candidate, normalizedQuery) {
			return true
		}
	}
	return false
}

func compareGuildMembers(left, right *discordgo.Member) int {
	leftOption, _ := buildGuildMemberOption(left)
	rightOption, _ := buildGuildMemberOption(right)

	leftDisplay := strings.ToLower(strings.TrimSpace(leftOption.DisplayName))
	rightDisplay := strings.ToLower(strings.TrimSpace(rightOption.DisplayName))
	if leftDisplay != rightDisplay {
		return strings.Compare(leftDisplay, rightDisplay)
	}

	leftUsername := strings.ToLower(strings.TrimSpace(leftOption.Username))
	rightUsername := strings.ToLower(strings.TrimSpace(rightOption.Username))
	if leftUsername != rightUsername {
		return strings.Compare(leftUsername, rightUsername)
	}

	return strings.Compare(leftOption.ID, rightOption.ID)
}
