package app

import (
	"errors"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/log"
)

var leaveRuntimeGuild = func(session *discordgo.Session, guildID string) error {
	if session == nil {
		return fmt.Errorf("discord session is unavailable")
	}
	return session.GuildLeave(guildID)
}

// Temporary hotfix: keep the bot only in explicitly approved guilds.
var runtimeGuildAllowlist = map[string]struct{}{
	"1375650791251120179": {},
	"1390069056530419823": {},
}

func isAllowedRuntimeGuild(guildID string) bool {
	_, ok := runtimeGuildAllowlist[strings.TrimSpace(guildID)]
	return ok
}

func enforceRuntimeGuildAllowlist(runtime *botRuntime) error {
	if runtime == nil {
		return fmt.Errorf("bot runtime is unavailable")
	}

	guildIDs, err := listBotGuildIDsFromSessionState(runtime.session)
	if err != nil {
		return err
	}

	leftGuilds := make(map[string]struct{})
	var leaveErrs []error
	for _, guildID := range guildIDs {
		if isAllowedRuntimeGuild(guildID) {
			continue
		}
		if err := leaveRuntimeGuild(runtime.session, guildID); err != nil {
			log.ErrorLoggerRaw().Error(
				"Failed to leave unauthorized guild during runtime initialization",
				"botInstanceID", runtime.instanceID,
				"guildID", guildID,
				"err", err,
			)
			leaveErrs = append(leaveErrs, fmt.Errorf("guild %s: %w", guildID, err))
			continue
		}
		leftGuilds[guildID] = struct{}{}
		log.ApplicationLogger().Warn(
			"Left unauthorized guild during runtime initialization",
			"botInstanceID", runtime.instanceID,
			"guildID", guildID,
		)
	}

	if len(leftGuilds) > 0 {
		pruneRuntimeSessionGuilds(runtime.session, leftGuilds)
	}
	if len(leaveErrs) > 0 {
		return errors.Join(leaveErrs...)
	}
	return nil
}

func registerRuntimeGuildAllowlistHandler(runtime *botRuntime) {
	if runtime == nil || runtime.session == nil {
		return
	}
	if runtime.cleanupStop == nil {
		runtime.cleanupStop = make(chan struct{})
	}

	cancel := runtime.session.AddHandler(func(session *discordgo.Session, event *discordgo.GuildCreate) {
		handleRuntimeGuildCreate(session, runtime.instanceID, event)
	})

	go func(stop <-chan struct{}) {
		<-stop
		cancel()
	}(runtime.cleanupStop)
}

func handleRuntimeGuildCreate(session *discordgo.Session, botInstanceID string, event *discordgo.GuildCreate) {
	if event == nil {
		return
	}

	guildID := strings.TrimSpace(event.ID)
	if guildID == "" || isAllowedRuntimeGuild(guildID) {
		return
	}

	if err := leaveRuntimeGuild(session, guildID); err != nil {
		log.ErrorLoggerRaw().Error(
			"Failed to leave unauthorized guild after guild create",
			"botInstanceID", botInstanceID,
			"guildID", guildID,
			"err", err,
		)
		return
	}

	log.ApplicationLogger().Warn(
		"Left unauthorized guild after guild create",
		"botInstanceID", botInstanceID,
		"guildID", guildID,
	)
}

func pruneRuntimeSessionGuilds(session *discordgo.Session, removed map[string]struct{}) {
	if session == nil || session.State == nil || len(removed) == 0 {
		return
	}

	filtered := session.State.Guilds[:0]
	for _, guild := range session.State.Guilds {
		if guild == nil {
			continue
		}
		if _, ok := removed[strings.TrimSpace(guild.ID)]; ok {
			continue
		}
		filtered = append(filtered, guild)
	}
	session.State.Guilds = filtered
	session.State.Ready.Guilds = filtered
}