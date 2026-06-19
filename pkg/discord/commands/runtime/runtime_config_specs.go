package runtime

import (
	"sort"
	"strings"
)

func allSpecs() []spec {
	// Keep groups stable and short (helps readability in embed fields). Order is
	// load-bearing: allGroups derives the group display order from this sequence.
	var specs []spec
	specs = append(specs, themeSpecs()...)
	specs = append(specs, loggingServiceSpecs()...)
	specs = append(specs, moderationSpecs()...)
	specs = append(specs, presenceWatchSpecs()...)
	specs = append(specs, messageCacheSpecs()...)
	specs = append(specs, backfillSpecs()...)
	specs = append(specs, safetySpecs()...)
	return specs
}

func themeSpecs() []spec {
	return []spec{
		{
			Key:         runtimeKeyBotTheme,
			Group:       "THEME",
			Type:        vtString,
			DefaultHint: "(default)",
			ShortHelp:   "Theme name (empty = default)",
			RestartHint: restartRecommended,
			MaxInputLen: 60,
		},
	}
}

func loggingServiceSpecs() []spec {
	return []spec{
		{
			Key:         runtimeKeyDisableDBCleanup,
			Group:       "SERVICES (LOGGING)",
			Type:        vtBool,
			DefaultHint: "false",
			ShortHelp:   "Disable periodic DB cleanup",
			RestartHint: restartRequired, // still a goroutine in runner; hot-apply intentionally not handled
		},
		{
			Key:         runtimeKeyDisableMessageLogs,
			Group:       "SERVICES (LOGGING)",
			Type:        vtBool,
			DefaultHint: "false",
			ShortHelp:   "Disable message logging service startup",
			RestartHint: restartRecommended,
		},
		{
			Key:         runtimeKeyDisableEntryExitLogs,
			Group:       "SERVICES (LOGGING)",
			Type:        vtBool,
			DefaultHint: "false",
			ShortHelp:   "Disable entry/exit logging service startup",
			RestartHint: restartRecommended,
		},
		{
			Key:         runtimeKeyDisableReactionLogs,
			Group:       "SERVICES (LOGGING)",
			Type:        vtBool,
			DefaultHint: "false",
			ShortHelp:   "Disable reaction logging service startup",
			RestartHint: restartRecommended,
		},
		{
			Key:         runtimeKeyDisableUserLogs,
			Group:       "SERVICES (LOGGING)",
			Type:        vtBool,
			DefaultHint: "false",
			ShortHelp:   "Disable user log handlers (avatars/roles)",
			RestartHint: restartRecommended,
		},
	}
}

func moderationSpecs() []spec {
	return []spec{
		{
			Key:         runtimeKeyModerationLogging,
			Group:       "MODERATION",
			Type:        vtBool,
			DefaultHint: "true",
			ShortHelp:   "Enable/disable moderation case embeds",
			RestartHint: restartRecommended,
		},
	}
}

func presenceWatchSpecs() []spec {
	return []spec{
		{
			Key:         runtimeKeyPresenceWatchUserID,
			Group:       "PRESENCE WATCH",
			Type:        vtString,
			DefaultHint: "(empty)",
			ShortHelp:   "Log presence updates for a specific user ID",
			RestartHint: restartRecommended,
			MaxInputLen: 32,
		},
		{
			Key:         runtimeKeyPresenceWatchBot,
			Group:       "PRESENCE WATCH",
			Type:        vtBool,
			DefaultHint: "false",
			ShortHelp:   "Log presence updates for the bot user",
			RestartHint: restartRecommended,
		},
	}
}

func messageCacheSpecs() []spec {
	return []spec{
		{
			Key:         runtimeKeyMessageCacheTTLHours,
			Group:       "MESSAGE CACHE",
			Type:        vtInt,
			DefaultHint: "72",
			ShortHelp:   "Cache TTL in hours for message edit/delete logging (0 = default)",
			RestartHint: restartRequired,
			MaxInputLen: 8,
		},
		{
			Key:         runtimeKeyMessageDeleteOnLog,
			Group:       "MESSAGE CACHE",
			Type:        vtBool,
			DefaultHint: "false",
			ShortHelp:   "Delete cached message record after it is logged",
			RestartHint: restartRecommended,
		},
		{
			Key:         runtimeKeyMessageCacheCleanup,
			Group:       "MESSAGE CACHE",
			Type:        vtBool,
			DefaultHint: "false",
			ShortHelp:   "Cleanup expired cached messages on startup",
			RestartHint: restartRecommended,
		},
	}
}

func backfillSpecs() []spec {
	return []spec{
		{
			Key:         runtimeKeyBackfillChannelID,
			Group:       "BACKFILL",
			Type:        vtString,
			DefaultHint: "(empty)",
			ShortHelp:   "Channel ID to backfill from (required to run)",
			RestartHint: restartRequired,
			MaxInputLen: 32,
		},
		{
			Key:         runtimeKeyBackfillStartDay,
			Group:       "BACKFILL",
			Type:        vtDate,
			DefaultHint: "today (UTC)",
			ShortHelp:   "Start day (YYYY-MM-DD) for backfill",
			RestartHint: restartRequired,
			MaxInputLen: 16,
		},
		{
			Key:         runtimeKeyBackfillInitialDate,
			Group:       "BACKFILL",
			Type:        vtDate,
			DefaultHint: "(empty)",
			ShortHelp:   "Initial scan start date (fixed) when never processed",
			RestartHint: restartRequired,
			MaxInputLen: 16,
			GuildOnly:   true,
		},
	}
}

func safetySpecs() []spec {
	return []spec{
		{
			Key:         runtimeKeyDisableBotRolePermMirror,
			Group:       "SAFETY",
			Type:        vtBool,
			DefaultHint: "false",
			ShortHelp:   "Disable bot role permission mirroring safety feature",
			RestartHint: restartRecommended, // effective at event time; no restart needed for behavior
		},
		{
			Key:         runtimeKeyBotRolePermMirrorActorRoleID,
			Group:       "SAFETY",
			Type:        vtString,
			DefaultHint: "(default)",
			ShortHelp:   "Role ID used as the actor when mirroring permissions",
			RestartHint: restartRecommended,
			MaxInputLen: 32,
		},
	}
}

func specByKey(k runtimeKey) (spec, bool) {
	for _, sp := range allSpecs() {
		if sp.Key == k {
			return sp, true
		}
	}
	return spec{}, false
}

func allGroups() []string {
	set := map[string]struct{}{"ALL": {}}
	for _, sp := range allSpecs() {
		set[sp.Group] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for g := range set {
		out = append(out, g)
	}
	sort.Strings(out)
	// Keep ALL first if present
	if len(out) > 0 && out[0] != "ALL" {
		for i := range out {
			if out[i] == "ALL" {
				out[0], out[i] = out[i], out[0]
				break
			}
		}
	}
	return out
}

func specsForGroup(group string) []spec {
	if strings.TrimSpace(group) == "" || group == "ALL" {
		// return deterministic order by group then key
		sps := append([]spec(nil), allSpecs()...)
		sort.Slice(sps, func(i, j int) bool {
			if sps[i].Group == sps[j].Group {
				return string(sps[i].Key) < string(sps[j].Key)
			}
			return sps[i].Group < sps[j].Group
		})
		return sps
	}

	var out []spec
	for _, sp := range allSpecs() {
		if sp.Group == group {
			out = append(out, sp)
		}
	}
	sort.Slice(out, func(i, j int) bool { return string(out[i].Key) < string(out[j].Key) })
	return out
}
