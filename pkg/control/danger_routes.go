package control

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
)

func (s *Server) handleDangerPurgeBase(w http.ResponseWriter, r *http.Request, domain string, purgeFunc func(context.Context, string) error) {
	auth, ok := s.authorizeRequest(w, r)
	if !ok {
		return
	}

	guildID := strings.TrimPrefix(r.URL.Path, "/v1/guilds/")
	if strings.Contains(guildID, "/") {
		guildID = strings.Split(guildID, "/")[0]
	}

	if !s.authorizeGuildControlAccess(w, r, auth, guildID, guildAccessLevelWrite) {
		return
	}

	// Wait, the plan calls for strict Administrator checks. authorizeGuildControlAccess
	// only checks if the user has dashboard write access (which could be granted by RolesConfig).
	// For "Danger Zone" we strictly require actual Discord Administrator permissions.

	if auth.mode == requestAuthModeDiscordOAuthSession {
		accessible, err := s.resolveAccessibleGuilds(r.Context(), auth.oauthSession)
		if err != nil {
			http.Error(w, "failed to verify admin rights", http.StatusInternalServerError)
			return
		}
		isAdmin := false
		for _, guild := range accessible {
			if guild.ID == guildID && (guild.Permissions&0x8) != 0 { // 0x8 is Administrator
				isAdmin = true
				break
			}
		}
		if !isAdmin {
			http.Error(w, "danger zone actions require discord administrator permissions", http.StatusForbidden)
			return
		}
	}

	if err := purgeFunc(r.Context(), guildID); err != nil {
		s.log().LogAttrs(r.Context(), slog.LevelError, "failed to purge domain data",
			slog.String("guildID", guildID),
			slog.String("domain", domain),
			slog.Any("err", err))
		http.Error(w, fmt.Sprintf("failed to purge %s data", domain), http.StatusInternalServerError)
		return
	}

	writeJSON(w, s.log(), http.StatusOK, map[string]string{"status": "ok", "purged": domain})
}

func (s *Server) handleDangerPurgeModeration(w http.ResponseWriter, r *http.Request) {
	s.handleDangerPurgeBase(w, r, "moderation", s.storage.PurgeGuildModerationData)
}

func (s *Server) handleDangerPurgeQOTD(w http.ResponseWriter, r *http.Request) {
	s.handleDangerPurgeBase(w, r, "qotd", s.storage.PurgeGuildQOTDData)
}

func (s *Server) handleDangerPurgeEngagement(w http.ResponseWriter, r *http.Request) {
	s.handleDangerPurgeBase(w, r, "engagement", s.storage.PurgeGuildEngagementMetrics)
}

func (s *Server) handleDangerPurgeCache(w http.ResponseWriter, r *http.Request) {
	s.handleDangerPurgeBase(w, r, "cache", s.storage.PurgeGuildCache)
}

func (s *Server) handleDangerWipeGuild(w http.ResponseWriter, r *http.Request) {
	// WIPES DB entirely for the guild, then drops config
	s.handleDangerPurgeBase(w, r, "all", func(ctx context.Context, guildID string) error {
		if err := s.storage.WipeGuildCompletely(ctx, guildID); err != nil {
			return err
		}
		return nil
	})
}
