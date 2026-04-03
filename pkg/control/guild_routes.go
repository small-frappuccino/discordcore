package control

import (
	"net/http"
	"strings"
)

func (s *Server) handleGuildConfigRoutes(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.authorizeRequest(w, r)
	if !ok {
		return
	}

	if s.configManager == nil {
		http.Error(w, "config manager unavailable", http.StatusInternalServerError)
		return
	}

	guildID, tail, ok := splitGuildRoute(r.URL.Path)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if guildID == "" {
		http.Error(w, "guild_id is required", http.StatusBadRequest)
		return
	}
	if !s.authorizeGuildAccess(w, r, auth, guildID) {
		return
	}

	switch {
	case len(tail) == 1 && tail[0] == "settings":
		switch r.Method {
		case http.MethodGet:
			s.handleGuildSettingsGet(w, r, guildID)
		case http.MethodPut:
			s.handleGuildSettingsPut(w, r, guildID)
		case http.MethodDelete:
			s.handleGuildSettingsDelete(w, r, guildID)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
		return
	case len(tail) == 1 && tail[0] == "features":
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleGuildFeaturesList(w, guildID)
		return
	case len(tail) == 1 && tail[0] == "role-options":
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleGuildRoleOptionsGet(w, guildID)
		return
	case len(tail) == 1 && tail[0] == "channel-options":
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleGuildChannelOptionsGet(w, guildID)
		return
	case len(tail) == 1 && tail[0] == "member-options":
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleGuildMemberOptionsGet(w, r, guildID)
		return
	case len(tail) == 2 && tail[0] == "features":
		switch r.Method {
		case http.MethodGet:
			s.handleGuildFeatureGet(w, guildID, tail[1])
		case http.MethodPatch:
			s.handleGuildFeaturePatch(w, r, guildID, tail[1])
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
		return
	case len(tail) >= 1 && tail[0] == "partner-board":
		s.handleGuildPartnerBoardRoutes(w, r, guildID, tail)
		return
	case len(tail) >= 1 && tail[0] == "qotd":
		s.handleGuildQOTDRoutes(w, r, guildID, tail, auth)
		return
	default:
		http.NotFound(w, r)
		return
	}
}

func (s *Server) handleGuildPartnerBoardRoutes(w http.ResponseWriter, r *http.Request, guildID string, tail []string) {
	if !s.requirePartnerBoardService(w) {
		return
	}

	switch {
	case len(tail) == 1:
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handlePartnerBoardGet(w, r, guildID)
		return
	case len(tail) == 2 && tail[1] == "target":
		switch r.Method {
		case http.MethodGet:
			s.handlePartnerBoardTargetGet(w, r, guildID)
		case http.MethodPut:
			s.handlePartnerBoardTargetPut(w, r, guildID)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
		return
	case len(tail) == 2 && tail[1] == "template":
		switch r.Method {
		case http.MethodGet:
			s.handlePartnerBoardTemplateGet(w, r, guildID)
		case http.MethodPut:
			s.handlePartnerBoardTemplatePut(w, r, guildID)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
		return
	case len(tail) == 2 && tail[1] == "partners":
		switch r.Method {
		case http.MethodGet:
			s.handlePartnerBoardPartnersList(w, r, guildID)
		case http.MethodPost:
			s.handlePartnerBoardPartnersCreate(w, r, guildID)
		case http.MethodPut:
			s.handlePartnerBoardPartnersUpdate(w, r, guildID)
		case http.MethodDelete:
			s.handlePartnerBoardPartnersDelete(w, r, guildID)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
		return
	case len(tail) == 2 && tail[1] == "sync":
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handlePartnerBoardSyncPost(w, r, guildID)
		return
	default:
		http.NotFound(w, r)
		return
	}
}

func splitGuildRoute(path string) (string, []string, bool) {
	const prefix = "/v1/guilds/"
	if !strings.HasPrefix(path, prefix) {
		return "", nil, false
	}

	trimmed := strings.Trim(path[len(prefix):], "/")
	if trimmed == "" {
		return "", nil, false
	}
	parts := strings.Split(trimmed, "/")
	if len(parts) == 0 {
		return "", nil, false
	}

	guildID := strings.TrimSpace(parts[0])
	tail := []string{}
	if len(parts) > 1 {
		tail = parts[1:]
	}
	return guildID, tail, true
}

func (s *Server) requirePartnerBoardService(w http.ResponseWriter) bool {
	if s.partnerBoardService != nil {
		return true
	}

	http.Error(w, "partner board service unavailable", http.StatusInternalServerError)
	return false
}

func (s *Server) requireQOTDService(w http.ResponseWriter) bool {
	if s.qotdService != nil {
		return true
	}

	http.Error(w, "qotd service unavailable", http.StatusInternalServerError)
	return false
}
