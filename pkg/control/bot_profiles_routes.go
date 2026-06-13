package control

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/log"
	"golang.org/x/sync/singleflight"
)

type BotProfileResponse struct {
	ID            string `json:"id"`
	LogicalKey    string `json:"logical_key"`
	Username      string `json:"username"`
	Discriminator string `json:"discriminator"`
	AvatarURL     string `json:"avatar_url"`
	Permissions   int64  `json:"permissions"`
	BotPresent    bool   `json:"bot_present"`
}

type botProfilesResponse struct {
	Status   string               `json:"status"`
	Profiles []BotProfileResponse `json:"profiles"`
}

var (
	botProfileGroup   singleflight.Group
	botProfileCacheMu sync.RWMutex
	botProfileCache   = make(map[string]cachedBotProfile)
)

type cachedBotProfile struct {
	Profile   BotProfileResponse
	ExpiresAt time.Time
}

func (s *Server) handleGuildBotProfilesGet(w http.ResponseWriter, r *http.Request, guildID string) {
	cfg := s.configManager.SnapshotConfig()
	guild, ok := findGuildSettings(cfg, guildID)
	if !ok {
		http.Error(w, "guild settings not found", http.StatusNotFound)
		return
	}

	var profiles []BotProfileResponse
	for instanceID, encToken := range guild.BotInstanceTokens {
		token := strings.TrimSpace(string(encToken))
		if token == "" {
			continue
		}

		profile, err := getBotProfileCached(r.Context(), guildID, instanceID, token)
		if err != nil {
			log.ApplicationLogger().Warn("Failed to fetch bot profile", "guildID", guildID, "instanceID", instanceID, "err", err)
			status := http.StatusBadGateway
			if strings.Contains(err.Error(), "429") {
				status = http.StatusTooManyRequests
			}
			http.Error(w, "Failed to fetch bot profile", status)
			return
		}
		profiles = append(profiles, profile)
	}

	writeJSON(w, http.StatusOK, botProfilesResponse{
		Status:   "ok",
		Profiles: profiles,
	})
}

func getBotProfileCached(ctx context.Context, guildID, logicalKey, token string) (BotProfileResponse, error) {
	cacheKey := guildID + ":" + token
	botProfileCacheMu.RLock()
	cached, ok := botProfileCache[cacheKey]
	botProfileCacheMu.RUnlock()

	if ok && time.Now().Before(cached.ExpiresAt) {
		return cached.Profile, nil
	}

	v, err, _ := botProfileGroup.Do(cacheKey, func() (interface{}, error) {
		botProfileCacheMu.RLock()
		cached, ok := botProfileCache[cacheKey]
		botProfileCacheMu.RUnlock()

		if ok && time.Now().Before(cached.ExpiresAt) {
			return cached.Profile, nil
		}

		user, err := fetchDiscordUser(ctx, token)
		if err != nil {
			return BotProfileResponse{}, err
		}

		var perms int64
		member, memberErr := fetchDiscordGuildMember(ctx, token, guildID, user.ID)
		if memberErr == nil {
			if guildRoles, rolesErr := fetchDiscordGuildRoles(ctx, token, guildID); rolesErr == nil {
				for _, r := range guildRoles {
					if r.ID == guildID {
						perms |= r.Permissions
					}
				}
				for _, r := range guildRoles {
					for _, mr := range member.Roles {
						if r.ID == mr {
							perms |= r.Permissions
						}
					}
				}
				if (perms & PermissionAdministrator) == PermissionAdministrator {
					// We just use a large bitmask if admin
					perms |= (1 << 3) | (1 << 4) | (1 << 5) // Admin, ManageChannels, ManageGuild
				}
			}
		}

		profile := BotProfileResponse{
			ID:            user.ID,
			LogicalKey:    logicalKey,
			Username:      user.Username,
			Discriminator: "", // Not used anymore by Discord mostly, but we can leave empty
			AvatarURL:     user.AvatarURL(),
			Permissions:   perms,
			BotPresent:    memberErr == nil,
		}

		botProfileCacheMu.Lock()
		if len(botProfileCache) >= 100 {
			botProfileCache = make(map[string]cachedBotProfile)
		}
		botProfileCache[cacheKey] = cachedBotProfile{
			Profile:   profile,
			ExpiresAt: time.Now().Add(10 * time.Minute),
		}
		botProfileCacheMu.Unlock()

		return profile, nil
	})

	if err != nil {
		return BotProfileResponse{}, err
	}

	prof := v.(BotProfileResponse)
	prof.LogicalKey = logicalKey
	return prof, nil
}

func fetchDiscordUser(ctx context.Context, token string) (*User, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://discord.com/api/v10/users/@me", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bot "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	var u User
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return nil, err
	}
	return &u, nil
}

func fetchDiscordGuildMember(ctx context.Context, token, guildID, userID string) (*Member, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://discord.com/api/v10/guilds/"+guildID+"/members/"+userID, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bot "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	var m Member
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		return nil, err
	}
	return &m, nil
}

func fetchDiscordGuildRoles(ctx context.Context, token, guildID string) ([]*Role, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://discord.com/api/v10/guilds/"+guildID+"/roles", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bot "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	var roles []*Role
	if err := json.NewDecoder(resp.Body).Decode(&roles); err != nil {
		return nil, err
	}
	return roles, nil
}
