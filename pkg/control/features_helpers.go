package control

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/logpolicy"
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

func logFeatureChannelID(guild *files.GuildConfig, eventType logpolicy.LogEventType) string {
	if guild == nil {
		return ""
	}
	switch eventType {
	case logpolicy.LogEventAvatarChange:
		return strings.TrimSpace(guild.Channels.AvatarLogging)
	case logpolicy.LogEventRoleChange:
		return strings.TrimSpace(guild.Channels.RoleUpdate)
	case logpolicy.LogEventMemberJoin:
		return strings.TrimSpace(guild.Channels.MemberJoin)
	case logpolicy.LogEventMemberLeave:
		return strings.TrimSpace(guild.Channels.MemberLeave)
	case logpolicy.LogEventMessageEdit:
		return strings.TrimSpace(guild.Channels.MessageEdit)
	case logpolicy.LogEventMessageDelete:
		return strings.TrimSpace(guild.Channels.MessageDelete)
	case logpolicy.LogEventAutomodAction:
		return strings.TrimSpace(guild.Channels.AutomodAction)
	case logpolicy.LogEventModerationCase:
		return strings.TrimSpace(guild.Channels.ModerationCase)
	case logpolicy.LogEventCleanAction:
		return strings.TrimSpace(guild.Channels.CleanAction)
	default:
		return ""
	}
}

func setLogFeatureChannelID(guild *files.GuildConfig, eventType logpolicy.LogEventType, channelID string) {
	if guild == nil {
		return
	}
	switch eventType {
	case logpolicy.LogEventAvatarChange:
		guild.Channels.AvatarLogging = channelID
	case logpolicy.LogEventRoleChange:
		guild.Channels.RoleUpdate = channelID
	case logpolicy.LogEventMemberJoin:
		guild.Channels.MemberJoin = channelID
	case logpolicy.LogEventMemberLeave:
		guild.Channels.MemberLeave = channelID
	case logpolicy.LogEventMessageEdit:
		guild.Channels.MessageEdit = channelID
	case logpolicy.LogEventMessageDelete:
		guild.Channels.MessageDelete = channelID
	case logpolicy.LogEventAutomodAction:
		guild.Channels.AutomodAction = channelID
	case logpolicy.LogEventModerationCase:
		guild.Channels.ModerationCase = channelID
	case logpolicy.LogEventCleanAction:
		guild.Channels.CleanAction = channelID
	}
}
