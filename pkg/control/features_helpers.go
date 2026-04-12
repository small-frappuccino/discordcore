package control

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	discordlogging "github.com/small-frappuccino/discordcore/pkg/discord/logging"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

func normalizeFeatureRoutePath(path string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(path), "/")
	if trimmed == "" {
		return "/"
	}
	return trimmed
}

func splitGlobalFeatureRoute(path string) (string, bool) {
	const prefix = "/v1/features/"
	if !strings.HasPrefix(path, prefix) {
		return "", false
	}
	trimmed := strings.Trim(strings.TrimPrefix(path, prefix), "/")
	if trimmed == "" || strings.Contains(trimmed, "/") {
		return "", false
	}
	return trimmed, true
}

func cloneRawPayload(in map[string]json.RawMessage) map[string]json.RawMessage {
	out := make(map[string]json.RawMessage, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func consumeNullableBool(payload map[string]json.RawMessage, key string) (bool, *bool, error) {
	raw, ok := payload[key]
	if !ok {
		return false, nil, nil
	}
	delete(payload, key)
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return true, nil, nil
	}
	var parsed bool
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return false, nil, featurePatchBadRequestError{message: fmt.Sprintf("%s must be a boolean or null: %v", key, err)}
	}
	return true, &parsed, nil
}

func consumeBool(payload map[string]json.RawMessage, key string) (bool, bool, error) {
	raw, ok := payload[key]
	if !ok {
		return false, false, nil
	}
	delete(payload, key)
	var parsed bool
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return false, false, featurePatchBadRequestError{message: fmt.Sprintf("%s must be a boolean: %v", key, err)}
	}
	return true, parsed, nil
}

func consumeString(payload map[string]json.RawMessage, key string) (bool, string, error) {
	raw, ok := payload[key]
	if !ok {
		return false, "", nil
	}
	delete(payload, key)
	var parsed string
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return false, "", featurePatchBadRequestError{message: fmt.Sprintf("%s must be a string: %v", key, err)}
	}
	return true, strings.TrimSpace(parsed), nil
}

func consumeStringSlice(payload map[string]json.RawMessage, key string) (bool, []string, error) {
	raw, ok := payload[key]
	if !ok {
		return false, nil, nil
	}
	delete(payload, key)
	var parsed []string
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return false, nil, featurePatchBadRequestError{message: fmt.Sprintf("%s must be a string array: %v", key, err)}
	}
	return true, parsed, nil
}

func consumeInt(payload map[string]json.RawMessage, key string) (bool, int, error) {
	raw, ok := payload[key]
	if !ok {
		return false, 0, nil
	}
	delete(payload, key)
	var parsed int
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return false, 0, featurePatchBadRequestError{message: fmt.Sprintf("%s must be an integer: %v", key, err)}
	}
	return true, parsed, nil
}

func unknownPatchFieldsError(payload map[string]json.RawMessage) error {
	fields := make([]string, 0, len(payload))
	for key := range payload {
		fields = append(fields, key)
	}
	slices.Sort(fields)
	return featurePatchBadRequestError{message: fmt.Sprintf("unsupported patch field(s): %s", strings.Join(fields, ", "))}
}

func normalizeStringList(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func cloneBool(in *bool) *bool {
	if in == nil {
		return nil
	}
	value := *in
	return &value
}

func logFeatureChannelID(guild *files.GuildConfig, eventType discordlogging.LogEventType) string {
	if guild == nil {
		return ""
	}
	switch eventType {
	case discordlogging.LogEventAvatarChange:
		return strings.TrimSpace(guild.Channels.AvatarLogging)
	case discordlogging.LogEventRoleChange:
		return strings.TrimSpace(guild.Channels.RoleUpdate)
	case discordlogging.LogEventMemberJoin:
		return strings.TrimSpace(guild.Channels.MemberJoin)
	case discordlogging.LogEventMemberLeave:
		return strings.TrimSpace(guild.Channels.MemberLeave)
	case discordlogging.LogEventMessageEdit:
		return strings.TrimSpace(guild.Channels.MessageEdit)
	case discordlogging.LogEventMessageDelete:
		return strings.TrimSpace(guild.Channels.MessageDelete)
	case discordlogging.LogEventAutomodAction:
		return strings.TrimSpace(guild.Channels.AutomodAction)
	case discordlogging.LogEventModerationCase:
		return strings.TrimSpace(guild.Channels.ModerationCase)
	case discordlogging.LogEventCleanAction:
		return strings.TrimSpace(guild.Channels.CleanAction)
	default:
		return ""
	}
}

func setLogFeatureChannelID(guild *files.GuildConfig, eventType discordlogging.LogEventType, channelID string) {
	if guild == nil {
		return
	}
	switch eventType {
	case discordlogging.LogEventAvatarChange:
		guild.Channels.AvatarLogging = channelID
	case discordlogging.LogEventRoleChange:
		guild.Channels.RoleUpdate = channelID
	case discordlogging.LogEventMemberJoin:
		guild.Channels.MemberJoin = channelID
	case discordlogging.LogEventMemberLeave:
		guild.Channels.MemberLeave = channelID
	case discordlogging.LogEventMessageEdit:
		guild.Channels.MessageEdit = channelID
	case discordlogging.LogEventMessageDelete:
		guild.Channels.MessageDelete = channelID
	case discordlogging.LogEventAutomodAction:
		guild.Channels.AutomodAction = channelID
	case discordlogging.LogEventModerationCase:
		guild.Channels.ModerationCase = channelID
	case discordlogging.LogEventCleanAction:
		guild.Channels.CleanAction = channelID
	}
}
