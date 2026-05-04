package runtime

import (
	"hash/fnv"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
)

const (
	runtimeConfigInteractionDeniedText  = "Only the person who opened this runtime config panel can use it. This reply stays private because it belongs to that admin session."
	runtimeConfigInteractionExpiredText = "This runtime config panel is no longer valid. Reopen /config runtime to continue."
)

type runtimeVisibilityClass string

const (
	runtimeVisibilityAdministrativePanel runtimeVisibilityClass = "administrative_panel"
	runtimeVisibilityRead                runtimeVisibilityClass = "read"
	runtimeVisibilityList                runtimeVisibilityClass = "list"
	runtimeVisibilityPreview             runtimeVisibilityClass = "preview"
	runtimeVisibilityRenderedPayload     runtimeVisibilityClass = "rendered_payload"
	runtimeVisibilityDetailedError       runtimeVisibilityClass = "detailed_error"
	runtimeVisibilityShortConfirmation   runtimeVisibilityClass = "short_confirmation"
)

func runtimeVisibilityIsEphemeral(class runtimeVisibilityClass) bool {
	switch class {
	case runtimeVisibilityShortConfirmation:
		return false
	case runtimeVisibilityAdministrativePanel,
		runtimeVisibilityRead,
		runtimeVisibilityList,
		runtimeVisibilityPreview,
		runtimeVisibilityRenderedPayload,
		runtimeVisibilityDetailedError:
		return true
	default:
		return true
	}
}

func authorizeRuntimeComponentInteraction(ctx *core.Context, ackPolicy core.InteractionAckPolicy) (bool, error) {
	ownerUserID, ok := runtimeOriginalPanelUserID(ctx.Interaction)
	if !ok {
		return true, denyRuntimeInteraction(ctx, ackPolicy, runtimeConfigInteractionExpiredText)
	}
	if strings.TrimSpace(ctx.UserID) != ownerUserID {
		return true, denyRuntimeInteraction(ctx, ackPolicy, runtimeConfigInteractionDeniedText)
	}
	return false, nil
}

func authorizeRuntimeModalInteraction(ctx *core.Context, ackPolicy core.InteractionAckPolicy) (bool, error) {
	_, authToken, ok := decodeRuntimeModalState(ctx.RouteKey.CustomID)
	if !ok {
		return true, denyRuntimeInteraction(ctx, ackPolicy, runtimeConfigInteractionExpiredText)
	}
	if authToken == "" || authToken != runtimeInteractionAuthToken(runtimeInteractionUserID(ctx.Interaction)) {
		return true, denyRuntimeInteraction(ctx, ackPolicy, runtimeConfigInteractionDeniedText)
	}
	return false, nil
}

func runtimeOriginalPanelUserID(i *discordgo.InteractionCreate) (string, bool) {
	if i == nil || i.Message == nil {
		return "", false
	}
	if meta := i.Message.InteractionMetadata; meta != nil {
		if userID := runtimeUserID(meta.User); userID != "" {
			return userID, true
		}
		if trigger := meta.TriggeringInteractionMetadata; trigger != nil {
			if userID := runtimeUserID(trigger.User); userID != "" {
				return userID, true
			}
		}
	}
	if interaction := i.Message.Interaction; interaction != nil {
		if interaction.Member != nil && interaction.Member.User != nil {
			if userID := strings.TrimSpace(interaction.Member.User.ID); userID != "" {
				return userID, true
			}
		}
		if userID := runtimeUserID(interaction.User); userID != "" {
			return userID, true
		}
	}
	return "", false
}

func runtimeInteractionUserID(i *discordgo.InteractionCreate) string {
	if i == nil || i.Interaction == nil {
		return ""
	}
	if i.Member != nil && i.Member.User != nil {
		return strings.TrimSpace(i.Member.User.ID)
	}
	if i.User != nil {
		return strings.TrimSpace(i.User.ID)
	}
	return ""
}

func runtimeUserID(user *discordgo.User) string {
	if user == nil {
		return ""
	}
	return strings.TrimSpace(user.ID)
}

func runtimeInteractionAuthToken(userID string) string {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return ""
	}
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(userID))
	return strconv.FormatUint(uint64(hash.Sum32()), 36)
}

func encodeRuntimeModalState(st panelState, actorUserID string) string {
	scope := strings.TrimSpace(st.Scope)
	if scope == "" {
		scope = "global"
	}
	return modalEditValueID + stateSep + string(st.Key) + stateSep + scope + stateSep + runtimeInteractionAuthToken(actorUserID)
}

func decodeRuntimeModalState(customID string) (panelState, string, bool) {
	routeID, rawState, hasState := strings.Cut(customID, stateSep)
	if routeID != modalEditValueID || !hasState {
		return panelState{}, "", false
	}
	parts := strings.SplitN(rawState, stateSep, 3)
	if len(parts) != 3 {
		return panelState{}, "", false
	}
	key := runtimeKey(strings.TrimSpace(parts[0]))
	scope := strings.TrimSpace(parts[1])
	if scope == "" {
		scope = "global"
	}
	st := panelState{
		Mode:  pageMain,
		Group: runtimeGroupForKey(key),
		Key:   key,
		Scope: scope,
	}
	return sanitizeState(st), strings.TrimSpace(parts[2]), true
}

func runtimeGroupForKey(key runtimeKey) string {
	if sp, ok := specByKey(key); ok {
		if group := strings.TrimSpace(sp.Group); group != "" {
			return group
		}
	}
	return "ALL"
}

func denyRuntimeInteraction(ctx *core.Context, ackPolicy core.InteractionAckPolicy, message string) error {
	if ctx == nil || ctx.Session == nil || ctx.Interaction == nil {
		return nil
	}
	if ackPolicy.Mode != core.InteractionAckModeNone {
		return core.NewResponseManager(ctx.Session).FollowUp(ctx.Interaction, message, true)
	}
	return core.NewResponseBuilder(ctx.Session).Ephemeral().Error(ctx.Interaction, message)
}