package core

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/task"
)

// TaskCommandOrphanCleanup is the task type for background guild command sweeping.
const TaskCommandOrphanCleanup = "commands:orphan_cleanup"

// RegisterOrphanCleanupTask registers the task handler for orphaned guild commands sweep.
func RegisterOrphanCleanupTask(router *task.TaskRouter, session *discordgo.Session, configManager *files.ConfigManager) {
	if router == nil {
		return
	}
	router.RegisterHandler(TaskCommandOrphanCleanup, func(ctx context.Context, payload any) error {
		return handleOrphanCleanupTask(ctx, session, configManager)
	})
}

// scheduleOrphanCleanupTask dispatches the cleanup task cleanly into the background queue.
func scheduleOrphanCleanupTask(router *task.TaskRouter, session *discordgo.Session) {
	if router == nil || session == nil || session.State == nil {
		return
	}
	sessionID := session.State.SessionID
	if sessionID == "" {
		sessionID = "unknown"
	}
	t := task.Task{
		Type:    TaskCommandOrphanCleanup,
		Payload: task.EmptyPayload{},
		Options: task.TaskOptions{
			IdempotencyKey: "orphan_cleanup_" + sessionID,
			IdempotencyTTL: 5 * time.Minute,
		},
	}
	_ = router.Dispatch(context.Background(), t)
}

func guildScopedSyncTargets(cfg *files.BotConfig, session *discordgo.Session) map[string]struct{} {
	if cfg == nil || len(cfg.Guilds) == 0 {
		return nil
	}

	sessionGuildIDs := make(map[string]struct{})
	session.State.RLock()
	for _, g := range session.State.Guilds {
		if g != nil && strings.TrimSpace(g.ID) != "" {
			sessionGuildIDs[strings.TrimSpace(g.ID)] = struct{}{}
		}
	}
	session.State.RUnlock()

	filterBySession := len(sessionGuildIDs) > 0
	targets := make(map[string]struct{})

	for _, guild := range cfg.Guilds {
		guildID := strings.TrimSpace(guild.GuildID)
		if guildID == "" {
			continue
		}
		if filterBySession {
			if _, ok := sessionGuildIDs[guildID]; !ok {
				continue
			}
		}
		targets[guildID] = struct{}{}
	}
	return targets
}

func usesGuildScopedSync(cfg *files.BotConfig) bool {
	if cfg == nil {
		return false
	}
	for _, guild := range cfg.Guilds {
		if len(guild.BotInstanceTokens) > 0 {
			return true
		}
	}
	return false
}

func handleOrphanCleanupTask(ctx context.Context, session *discordgo.Session, configManager *files.ConfigManager) error {
	if session == nil || session.State == nil || session.State.User == nil || configManager == nil {
		return nil
	}
	cfg := configManager.Config()
	if cfg == nil {
		return nil
	}

	appID := session.State.User.ID
	usesProfiles := usesGuildScopedSync(cfg)
	syncTargets := guildScopedSyncTargets(cfg, session)

	// Build a stable snapshot of all currently known session guilds
	session.State.RLock()
	var allGuilds []string
	for _, g := range session.State.Guilds {
		if g != nil && strings.TrimSpace(g.ID) != "" {
			allGuilds = append(allGuilds, strings.TrimSpace(g.ID))
		}
	}
	session.State.RUnlock()

	var toClean []string
	if !usesProfiles {
		// If globally synced, clean out ALL stray guild commands
		toClean = allGuilds
	} else {
		// If profile-driven, clean only the unmanaged guild targets
		for _, guildID := range allGuilds {
			if _, ok := syncTargets[guildID]; !ok {
				toClean = append(toClean, guildID)
			}
		}
	}

	for _, guildID := range toClean {
		if err := cleanGuildCommands(ctx, session, appID, guildID); err != nil {
			slog.Warn("Failed to clean orphan commands", "guildID", guildID, "error", err)
		}
		// Respect contextual lifecycle shutdowns
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}

	return nil
}

// cleanGuildCommands performs a single synchronous GET per guild to find orphans,
// then executes direct raw HTTP DELETEs that strictly observe RateLimit headers.
func cleanGuildCommands(ctx context.Context, session *discordgo.Session, appID, guildID string) error {
	commands, err := session.ApplicationCommands(appID, guildID)
	if err != nil {
		return fmt.Errorf("fetch commands for %s: %w", guildID, err)
	}
	if len(commands) == 0 {
		return nil
	}

	for _, cmd := range commands {
	retryDelete:
		req, err := http.NewRequestWithContext(ctx, "DELETE", discordgo.EndpointApplicationGuildCommand(appID, guildID, cmd.ID), nil)
		if err != nil {
			continue
		}
		req.Header.Set("Authorization", session.Token)
		if session.UserAgent != "" {
			req.Header.Set("User-Agent", session.UserAgent)
		}

		resp, err := session.Client.Do(req)
		if err != nil {
			slog.Warn("Failed to execute direct DELETE", "error", err)
			continue
		}

		statusCode := resp.StatusCode
		remaining := resp.Header.Get("X-RateLimit-Remaining")
		resetAfter := resp.Header.Get("X-RateLimit-Reset-After")
		retryAfter := resp.Header.Get("Retry-After")
		resp.Body.Close()

		if statusCode == 429 {
			delay := 5 * time.Second // Fallback delay to prevent zero-wait busy loops
			if ra, parseErr := strconv.ParseFloat(retryAfter, 64); parseErr == nil {
				delay = time.Duration(ra * float64(time.Second))
			}
			timer := time.NewTimer(delay)
			select {
			case <-timer.C:
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			}
			timer.Stop()
			goto retryDelete
		} else if remaining == "0" && resetAfter != "" {
			if ra, parseErr := strconv.ParseFloat(resetAfter, 64); parseErr == nil {
				delay := time.Duration(ra * float64(time.Second))
				timer := time.NewTimer(delay)
				select {
				case <-timer.C:
				case <-ctx.Done():
					timer.Stop()
					return ctx.Err()
				}
				timer.Stop()
			}
		}

		if statusCode == 204 {
			slog.Info(fmt.Sprintf("Orphan command removed (sweep) (guild %s): /%s", guildID, cmd.Name))
		}
	}
	return nil
}
