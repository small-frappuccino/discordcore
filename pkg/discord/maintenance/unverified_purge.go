package maintenance

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/storage"
)

type UnverifiedPurgeService struct {
	session       *discordgo.Session
	configManager *files.ConfigManager
	store         *storage.Store

	stopOnce sync.Once
	stopCh   chan struct{}
	wg       sync.WaitGroup

	mu      sync.RWMutex
	running bool
}

func NewUnverifiedPurgeService(session *discordgo.Session, configManager *files.ConfigManager, store *storage.Store) *UnverifiedPurgeService {
	return &UnverifiedPurgeService{
		session:       session,
		configManager: configManager,
		store:         store,
		stopCh:        make(chan struct{}),
	}
}

func (s *UnverifiedPurgeService) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()

	s.wg.Add(1)
	go s.loop()
}

func (s *UnverifiedPurgeService) Stop() {
	s.stopOnce.Do(func() { close(s.stopCh) })
	s.wg.Wait()

	s.mu.Lock()
	s.running = false
	s.mu.Unlock()
}

func (s *UnverifiedPurgeService) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

func (s *UnverifiedPurgeService) loop() {
	defer s.wg.Done()

	initialDelay := s.resolveInitialDelay()
	if initialDelay > 0 {
		timer := time.NewTimer(initialDelay)
		select {
		case <-timer.C:
		case <-s.stopCh:
			timer.Stop()
			return
		}
	}

	ticker := time.NewTicker(s.resolveScanInterval())
	defer ticker.Stop()

	s.runOnce()

	for {
		select {
		case <-ticker.C:
			s.runOnce()
		case <-s.stopCh:
			return
		}
	}
}

type purgeCandidate struct {
	userID   string
	joinedAt time.Time
}

func (s *UnverifiedPurgeService) runOnce() {
	cfg := s.configManager.Config()
	if cfg == nil || len(cfg.Guilds) == 0 {
		return
	}
	if s.session == nil || s.store == nil {
		return
	}

	now := time.Now().UTC()

	for _, gcfg := range cfg.Guilds {
		if !gcfg.UnverifiedPurgeEnabled {
			continue
		}
		verifiedRoleID := strings.TrimSpace(gcfg.UnverifiedPurgeVerifiedRoleID)
		if verifiedRoleID == "" {
			log.ApplicationLogger().Warn("unverified purge enabled but verified role id is empty", "guildID", gcfg.GuildID)
			continue
		}

		graceDays := resolveInt(gcfg.UnverifiedPurgeGraceDays, 7)
		cutoff := now.Add(-time.Duration(graceDays) * 24 * time.Hour)

		joins, err := s.store.GetAllMemberJoins(gcfg.GuildID)
		if err != nil {
			log.ApplicationLogger().Warn("unverified purge: failed to load member joins", "guildID", gcfg.GuildID, "err", err)
			continue
		}
		memberRoles, err := s.store.GetAllGuildMemberRoles(gcfg.GuildID)
		if err != nil {
			log.ApplicationLogger().Warn("unverified purge: failed to load member roles", "guildID", gcfg.GuildID, "err", err)
			memberRoles = map[string][]string{}
		}

		exempt := make(map[string]struct{}, len(gcfg.UnverifiedPurgeExemptRoleIDs))
		for _, rid := range gcfg.UnverifiedPurgeExemptRoleIDs {
			rid = strings.TrimSpace(rid)
			if rid != "" {
				exempt[rid] = struct{}{}
			}
		}

		var candidates []purgeCandidate
		for userID, joinedAt := range joins {
			if joinedAt.IsZero() || joinedAt.After(cutoff) {
				continue
			}
			if hasRole(memberRoles[userID], verifiedRoleID) || hasAnyRole(memberRoles[userID], exempt) {
				continue
			}
			candidates = append(candidates, purgeCandidate{userID: userID, joinedAt: joinedAt})
		}

		if len(candidates) == 0 {
			continue
		}

		sort.Slice(candidates, func(i, j int) bool {
			return candidates[i].joinedAt.Before(candidates[j].joinedAt)
		})

		maxKicks := resolveInt(gcfg.UnverifiedPurgeMaxKicksPerRun, 200)
		if maxKicks < 1 {
			maxKicks = len(candidates)
		}
		if len(candidates) > maxKicks {
			candidates = candidates[:maxKicks]
		}

		var throttle <-chan time.Time
		kps := gcfg.UnverifiedPurgeKicksPerSecond
		if kps <= 0 {
			kps = 4
		}
		if kps > 0 {
			interval := time.Duration(float64(time.Second) / float64(kps))
			if interval < 50*time.Millisecond {
				interval = 50 * time.Millisecond
			}
			t := time.NewTicker(interval)
			defer t.Stop()
			throttle = t.C
		}

		kicked := 0
		checked := 0
		for _, c := range candidates {
			if throttle != nil {
				select {
				case <-s.stopCh:
					return
				case <-throttle:
				}
			}

			member, err := s.session.GuildMember(gcfg.GuildID, c.userID)
			if err != nil || member == nil || member.User == nil {
				continue
			}
			if member.User.Bot {
				continue
			}

			joinedAt := member.JoinedAt
			if joinedAt.IsZero() {
				joinedAt = c.joinedAt
			}
			if joinedAt.After(cutoff) {
				continue
			}
			if hasRole(member.Roles, verifiedRoleID) || hasAnyRole(member.Roles, exempt) {
				continue
			}

			checked++
			if gcfg.UnverifiedPurgeDryRun {
				log.ApplicationLogger().Info("unverified purge (dry-run): would kick member", "guildID", gcfg.GuildID, "userID", c.userID, "joinedAt", joinedAt.Format(time.RFC3339))
				continue
			}

			reason := fmt.Sprintf("unverified-purge: missing verified role after %d days", graceDays)
			reason = truncateAuditReason(reason)
			if err := s.session.GuildMemberDeleteWithReason(gcfg.GuildID, c.userID, reason); err != nil {
				log.ApplicationLogger().Warn("unverified purge: failed to kick member", "guildID", gcfg.GuildID, "userID", c.userID, "err", err)
				continue
			}
			kicked++
		}

		if kicked > 0 || gcfg.UnverifiedPurgeDryRun {
			log.ApplicationLogger().Info("unverified purge completed", "guildID", gcfg.GuildID, "candidates", len(candidates), "checked", checked, "kicked", kicked, "dryRun", gcfg.UnverifiedPurgeDryRun)
		}
	}
}

func (s *UnverifiedPurgeService) resolveScanInterval() time.Duration {
	cfg := s.configManager.Config()
	if cfg == nil {
		return 2 * time.Hour
	}
	mins := 120
	for _, g := range cfg.Guilds {
		if !g.UnverifiedPurgeEnabled {
			continue
		}
		if g.UnverifiedPurgeScanIntervalMins > 0 && g.UnverifiedPurgeScanIntervalMins < mins {
			mins = g.UnverifiedPurgeScanIntervalMins
		}
	}
	if mins < 5 {
		mins = 5
	}
	return time.Duration(mins) * time.Minute
}

func (s *UnverifiedPurgeService) resolveInitialDelay() time.Duration {
	cfg := s.configManager.Config()
	if cfg == nil {
		return 2 * time.Minute
	}
	secs := 120
	for _, g := range cfg.Guilds {
		if !g.UnverifiedPurgeEnabled {
			continue
		}
		if g.UnverifiedPurgeInitialDelaySecs > 0 && g.UnverifiedPurgeInitialDelaySecs < secs {
			secs = g.UnverifiedPurgeInitialDelaySecs
		}
	}
	if secs < 0 {
		secs = 0
	}
	return time.Duration(secs) * time.Second
}

func resolveInt(v, def int) int {
	if v <= 0 {
		return def
	}
	return v
}

func hasRole(roles []string, roleID string) bool {
	if roleID == "" || len(roles) == 0 {
		return false
	}
	for _, r := range roles {
		if r == roleID {
			return true
		}
	}
	return false
}

func hasAnyRole(roles []string, set map[string]struct{}) bool {
	if len(roles) == 0 || len(set) == 0 {
		return false
	}
	for _, r := range roles {
		if _, ok := set[r]; ok {
			return true
		}
	}
	return false
}

func truncateAuditReason(reason string) string {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return reason
	}
	const maxLen = 512
	if len(reason) <= maxLen {
		return reason
	}
	return reason[:maxLen]
}
