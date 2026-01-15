package maintenance

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/logging"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/storage"
	"github.com/small-frappuccino/discordcore/pkg/theme"
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

type affectedMember struct {
	userID string
	roles  []string
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
		previewOnly := !gcfg.UnverifiedPurge.Enabled
		if !gcfg.UnverifiedPurge.Enabled && strings.TrimSpace(gcfg.Roles.VerificationRole) == "" {
			continue
		}
		verifiedRoleID := strings.TrimSpace(gcfg.Roles.VerificationRole)
		if verifiedRoleID == "" {
			log.ApplicationLogger().Warn("Non-Verified Members Purge enabled but verified role id is empty", "guildID", gcfg.GuildID)
			continue
		}

		botID := ""
		if s.session.State != nil && s.session.State.User != nil {
			botID = s.session.State.User.ID
		}

		graceDays := resolveInt(gcfg.UnverifiedPurge.GraceDays, 7)
		cutoff := now.Add(-time.Duration(graceDays) * 24 * time.Hour)

		joins, err := s.store.GetAllMemberJoins(gcfg.GuildID)
		if err != nil {
			log.ApplicationLogger().Warn("Non-Verified Members Purge: failed to load member joins", "guildID", gcfg.GuildID, "err", err)
			continue
		}
		memberRoles, err := s.store.GetAllGuildMemberRoles(gcfg.GuildID)
		if err != nil {
			log.ApplicationLogger().Warn("Non-Verified Members Purge: failed to load member roles", "guildID", gcfg.GuildID, "err", err)
			memberRoles = map[string][]string{}
		}

		exempt := make(map[string]struct{}, len(gcfg.UnverifiedPurge.ExemptRoleIDs))
		for _, rid := range gcfg.UnverifiedPurge.ExemptRoleIDs {
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

		sort.Slice(candidates, func(i, j int) bool {
			return candidates[i].joinedAt.Before(candidates[j].joinedAt)
		})

		maxKicks := resolveInt(gcfg.UnverifiedPurge.MaxKicksPerRun, 200)
		if maxKicks < 1 {
			maxKicks = len(candidates)
		}
		if len(candidates) > maxKicks {
			candidates = candidates[:maxKicks]
		}

		var throttle <-chan time.Time
		var throttleTicker *time.Ticker
		kps := gcfg.UnverifiedPurge.KicksPerSecond
		if kps <= 0 {
			kps = 4
		}
		if kps > 0 {
			interval := time.Duration(float64(time.Second) / float64(kps))
			if interval < 50*time.Millisecond {
				interval = 50 * time.Millisecond
			}
			throttleTicker = time.NewTicker(interval)
			throttle = throttleTicker.C
		}

		kicked := 0
		checked := 0
		var affectedMembers []affectedMember
		var previewEntries []string
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
			if member.PremiumSince != nil {
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
			if gcfg.UnverifiedPurge.DryRun {
				log.ApplicationLogger().Info("Non-Verified Members Purge (dry-run): would kick member", "guildID", gcfg.GuildID, "userID", c.userID, "joinedAt", joinedAt.Format(time.RFC3339))
				affectedMembers = append(affectedMembers, affectedMember{userID: c.userID, roles: member.Roles})
				continue
			}
			if previewOnly {
				display := member.User.Username
				if display == "" {
					display = "unknown"
				}
				previewEntries = append(previewEntries, fmt.Sprintf("%s (`%s`)", display, c.userID))
				continue
			}

			reason := fmt.Sprintf("nonverified-members-purge: missing verified role after %d days", graceDays)
			reason = truncateAuditReason(reason)
			if err := s.session.GuildMemberDeleteWithReason(gcfg.GuildID, c.userID, reason); err != nil {
				log.ApplicationLogger().Warn("Non-Verified Members Purge: failed to kick member", "guildID", gcfg.GuildID, "userID", c.userID, "err", err)
				continue
			}
			kicked++
			affectedMembers = append(affectedMembers, affectedMember{userID: c.userID, roles: member.Roles})
		}

		if throttleTicker != nil {
			throttleTicker.Stop()
		}

		log.ApplicationLogger().Info("Non-Verified Members Purge completed", "guildID", gcfg.GuildID, "candidates", len(candidates), "checked", checked, "kicked", kicked, "dryRun", gcfg.UnverifiedPurge.DryRun)
		if previewOnly {
			s.logPreviewEntries(gcfg.GuildID, previewEntries, len(candidates))
			continue
		}
		s.sendRunEmbed(gcfg.GuildID, botID, verifiedRoleID, graceDays, cutoff, len(candidates), checked, kicked, maxKicks, kps, gcfg.UnverifiedPurge.DryRun, affectedMembers)
	}
}

func (s *UnverifiedPurgeService) resolveScanInterval() time.Duration {
	cfg := s.configManager.Config()
	if cfg == nil {
		return 2 * time.Hour
	}
	mins := 120
	for _, g := range cfg.Guilds {
		if !g.UnverifiedPurge.Enabled {
			continue
		}
		if g.UnverifiedPurge.ScanIntervalMins > 0 && g.UnverifiedPurge.ScanIntervalMins < mins {
			mins = g.UnverifiedPurge.ScanIntervalMins
		}
	}
	if mins < 5 {
		mins = 5
	}
	return time.Duration(mins) * time.Minute
}

func (s *UnverifiedPurgeService) sendRunEmbed(guildID, botID, verifiedRoleID string, graceDays int, cutoff time.Time, candidates, checked, kicked, maxKicks, kps int, dryRun bool, affectedMembers []affectedMember) {
	if s.session == nil || s.configManager == nil || guildID == "" {
		return
	}
	if candidates == 0 || len(affectedMembers) == 0 {
		return
	}

	if botID == "" && s.session.State != nil && s.session.State.User != nil {
		botID = s.session.State.User.ID
	}
	if !logging.ShouldLogModerationEvent(s.configManager, guildID, botID, botID, logging.ModerationSourceUnknown) {
		return
	}
	channelID, ok := logging.ResolveModerationLogChannel(s.session, s.configManager, guildID)
	if !ok || strings.TrimSpace(channelID) == "" {
		return
	}

	mode := "Live"
	if dryRun {
		mode = "Dry run"
	}

	title := "Non-Verified Members Purge"
	desc := fmt.Sprintf("Summary for members without <@&%s> after **%d days**. Members listed below did not have <@&%s> at the time of the purge.", verifiedRoleID, graceDays, verifiedRoleID)

	fields := []*discordgo.MessageEmbedField{
		{Name: "Actor", Value: "<@" + botID + "> (`" + botID + "`)", Inline: true},
		{Name: "Mode", Value: mode, Inline: true},
		{Name: "Verified Role", Value: "<@&" + verifiedRoleID + "> (`" + verifiedRoleID + "`)", Inline: false},
		{Name: "Joined Before", Value: fmt.Sprintf("<t:%d:R>", cutoff.Unix()), Inline: true},
		{Name: "Removed", Value: fmt.Sprintf("%d", kicked), Inline: true},
	}

	if len(affectedMembers) > 0 {
		entries, omitted := buildAffectedMembersList(affectedMembers, 1024)
		if entries != "" {
			if omitted > 0 {
				entries = fmt.Sprintf("%s\n... and %d more", entries, omitted)
			}
			fields = append(fields, &discordgo.MessageEmbedField{
				Name:   "Affected Members (roles)",
				Value:  truncateFieldValue(entries),
				Inline: false,
			})
		}
	}

	embed := &discordgo.MessageEmbed{
		Title:       title,
		Color:       theme.AutomodAction(),
		Description: desc,
		Fields:      fields,
		Timestamp:   time.Now().Format(time.RFC3339),
		Footer: &discordgo.MessageEmbedFooter{
			Text: "nonverified-members-purge",
		},
	}

	if _, err := s.session.ChannelMessageSendEmbed(channelID, embed); err != nil {
		log.ErrorLoggerRaw().Error("Failed to send Non-Verified Members Purge moderation log", "guildID", guildID, "channelID", channelID, "err", err)
	}
}

func (s *UnverifiedPurgeService) logPreviewEntries(guildID string, entries []string, candidates int) {
	if len(entries) == 0 {
		if candidates > 0 {
			log.ApplicationLogger().Info("Non-Verified Members Purge preview: no eligible members after verification", "guildID", guildID, "candidates", candidates)
		}
		return
	}

	maxList := 50
	if len(entries) < maxList {
		maxList = len(entries)
	}
	lines := strings.Join(entries[:maxList], ", ")
	if len(entries) > maxList {
		lines += fmt.Sprintf(", and %d more", len(entries)-maxList)
	}
	log.ApplicationLogger().Info("Non-Verified Members Purge preview (disabled): eligible members without verified role", "guildID", guildID, "count", len(entries), "members", lines)
}

func (s *UnverifiedPurgeService) resolveInitialDelay() time.Duration {
	cfg := s.configManager.Config()
	if cfg == nil {
		return 2 * time.Minute
	}
	secs := 120
	for _, g := range cfg.Guilds {
		if !g.UnverifiedPurge.Enabled {
			continue
		}
		if g.UnverifiedPurge.InitialDelaySecs > 0 && g.UnverifiedPurge.InitialDelaySecs < secs {
			secs = g.UnverifiedPurge.InitialDelaySecs
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

func truncateFieldValue(value string) string {
	const maxLen = 1024
	if len(value) <= maxLen {
		return value
	}
	if maxLen <= 3 {
		return value[:maxLen]
	}
	return value[:maxLen-3] + "..."
}

func buildAffectedMembersList(members []affectedMember, maxLen int) (string, int) {
	if len(members) == 0 || maxLen <= 0 {
		return "", 0
	}

	var b strings.Builder
	omitted := 0

	for i, m := range members {
		roles := "roles: none"
		if len(m.roles) > 0 {
			roleMentions := make([]string, 0, len(m.roles))
			for _, roleID := range m.roles {
				if strings.TrimSpace(roleID) == "" {
					continue
				}
				roleMentions = append(roleMentions, "<@&"+roleID+">")
			}
			if len(roleMentions) > 0 {
				roles = "roles: " + strings.Join(roleMentions, ", ")
			}
		}

		line := fmt.Sprintf("- <@%s> (`%s`): %s", m.userID, m.userID, roles)
		lineLen := len(line)
		if b.Len() > 0 {
			lineLen++
		}
		if b.Len()+lineLen > maxLen {
			omitted = len(members) - i
			break
		}
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(line)
	}

	return b.String(), omitted
}
