package runtime

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/utils/json/option"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

type InteractionReplier interface {
	RespondInteraction(ctx context.Context, interactionID discord.InteractionID, token string, resp api.InteractionResponse) error
	EditInteractionResponse(ctx context.Context, appID discord.AppID, token string, data api.EditInteractionResponseData) (*discord.Message, error)
}

type Handler struct {
	replier InteractionReplier
	cm      *files.ConfigManager
	applier runtimeConfigApplier
	logger  *slog.Logger
}

func NewHandler(replier InteractionReplier, cm *files.ConfigManager, applier runtimeConfigApplier, logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.Default() // Fallback
	}
	return &Handler{
		replier: replier,
		cm:      cm,
		applier: applier,
		logger:  logger,
	}
}

func (h *Handler) respond(ctx context.Context, i *discord.InteractionEvent, resp api.InteractionResponse) error {
	return h.replier.RespondInteraction(ctx, i.ID, i.Token, resp)
}

func (h *Handler) edit(ctx context.Context, i *discord.InteractionEvent, data api.EditInteractionResponseData) error {
	_, err := h.replier.EditInteractionResponse(ctx, i.AppID, i.Token, data)
	return err
}

func (h *Handler) denyEphemeral(ctx context.Context, i *discord.InteractionEvent, message string) error {
	embeds := []discord.Embed{errorEmbed(message)}
	return h.respond(ctx, i, api.InteractionResponse{
		Type: api.MessageInteractionWithSource,
		Data: &api.InteractionResponseData{
			Embeds: &embeds,
			Flags:  discord.EphemeralMessage,
		},
	})
}

func (h *Handler) authorizeInteraction(ctx context.Context, i *discord.InteractionEvent, expectedToken string) bool {
	var userID discord.UserID
	if i.Member != nil {
		userID = i.Member.User.ID
	} else if i.User != nil {
		userID = i.User.ID
	}

	actualToken := runtimeInteractionAuthToken(userID.String())

	if actualToken == "" || expectedToken != actualToken {
		_ = h.denyEphemeral(ctx, i, "Only the person who opened this runtime config panel can use it.")
		return false
	}
	return true
}

func (h *Handler) HandleSlash(ctx context.Context, i *discord.InteractionEvent) error {
	scope := "global"
	if i.GuildID.IsValid() {
		scope = i.GuildID.String()
	}

	rc, err := loadRuntimeConfig(h.cm, scope)
	if err != nil {
		return h.denyEphemeral(ctx, i, fmt.Sprintf("Failed to load runtime configuration: %v", err))
	}

	h.logger.Info("Interaction routed to runtime configuration slash command",
		slog.String("guild_id", i.GuildID.String()),
		slog.String("request_id", i.ID.String()))

	st := panelState{
		Mode:  pageMain,
		Group: "ALL",
		Scope: scope,
	}

	embeds := []discord.Embed{renderMainEmbed(rc, st)}
	comps := renderMainComponents(rc, st)

	return h.respond(ctx, i, api.InteractionResponse{
		Type: api.MessageInteractionWithSource,
		Data: &api.InteractionResponseData{
			Embeds:     &embeds,
			Components: &comps,
			Flags:      discord.EphemeralMessage,
		},
	})
}

func (h *Handler) HandleComponent(ctx context.Context, i *discord.InteractionEvent) error {
	d, ok := i.Data.(discord.ComponentInteraction)
	if !ok {
		return nil
	}

	routeID, rawState, hasState := strings.Cut(string(d.ID()), stateSep)
	if !hasState {
		h.logger.Warn("Failed to decode runtime state from component interaction",
			slog.String("custom_id", string(d.ID())),
			slog.String("request_id", i.ID.String()))
		return h.denyEphemeral(ctx, i, "Invalid interaction state format.")
	}

	st := decodeState(rawState)

	h.logger.Debug("Decoded runtime state from component",
		slog.String("request_id", i.ID.String()),
		slog.String("key", string(st.Key)),
		slog.String("mode", string(st.Mode)),
		slog.String("group", st.Group))

	if routeID != cidButtonEdit {
		_ = h.respond(ctx, i, api.InteractionResponse{
			Type: api.DeferredMessageUpdate,
		})
	}

	rc, err := loadRuntimeConfig(h.cm, st.Scope)
	if err != nil {
		embeds := []discord.Embed{errorEmbed(fmt.Sprintf("Load err: %v", err))}
		_ = h.edit(ctx, i, api.EditInteractionResponseData{
			Embeds: &embeds,
		})
		return err
	}

	switch routeID {
	case cidSelectGroup, cidSelectKey:
		var values []string
		if sel, isSel := d.(*discord.StringSelectInteraction); isSel {
			values = sel.Values
		}
		if len(values) > 0 {
			st = decodeState(values[0])
			st = sanitizeState(st.withMode(pageMain))
		}
		embeds := []discord.Embed{renderMainEmbed(rc, st)}
		comps := renderMainComponents(rc, st)
		return h.edit(ctx, i, api.EditInteractionResponseData{
			Embeds:     &embeds,
			Components: &comps,
		})

	case cidButtonMain, cidButtonBack:
		st = sanitizeState(st.withMode(pageMain))
		embeds := []discord.Embed{renderMainEmbed(rc, st)}
		comps := renderMainComponents(rc, st)
		return h.edit(ctx, i, api.EditInteractionResponseData{
			Embeds:     &embeds,
			Components: &comps,
		})

	case cidButtonHelp:
		st = st.withMode(pageHelp)
		embeds := []discord.Embed{renderHelpEmbed()}
		comps := renderHelpComponents(st)
		return h.edit(ctx, i, api.EditInteractionResponseData{
			Embeds:     &embeds,
			Components: &comps,
		})

	case cidButtonDetail:
		st = st.withMode(pageDetail)
		embeds := []discord.Embed{renderDetailsEmbed(rc, st)}
		comps := renderDetailComponents(st)
		return h.edit(ctx, i, api.EditInteractionResponseData{
			Embeds:     &embeds,
			Components: &comps,
		})

	case cidButtonReload:
		if st.Mode == pageHelp {
			embeds := []discord.Embed{renderHelpEmbed()}
			comps := renderHelpComponents(st)
			return h.edit(ctx, i, api.EditInteractionResponseData{
				Embeds:     &embeds,
				Components: &comps,
			})
		} else if st.Mode == pageDetail {
			embeds := []discord.Embed{renderDetailsEmbed(rc, st)}
			comps := renderDetailComponents(st)
			return h.edit(ctx, i, api.EditInteractionResponseData{
				Embeds:     &embeds,
				Components: &comps,
			})
		}
		embeds := []discord.Embed{renderMainEmbed(rc, st.withMode(pageMain))}
		comps := renderMainComponents(rc, st.withMode(pageMain))
		return h.edit(ctx, i, api.EditInteractionResponseData{
			Embeds:     &embeds,
			Components: &comps,
		})

	case cidButtonReset:
		st = st.withMode(pageMain)
		rc2, ok := resetValue(rc, st.Key)
		if !ok {
			embeds := []discord.Embed{errorEmbed("Unknown key.")}
			return h.edit(ctx, i, api.EditInteractionResponseData{
				Embeds: &embeds,
			})
		}
		_ = saveRuntimeConfig(h.cm, rc2, st.Scope)
		var applyErr error
		if h.applier != nil {
			applyErr = h.applier.Apply(ctx, rc2)
		}
		embeds := []discord.Embed{withHotApplyWarning(renderMainEmbed(rc2, st), applyErr)}
		comps := renderMainComponents(rc2, st)
		return h.edit(ctx, i, api.EditInteractionResponseData{
			Embeds:     &embeds,
			Components: &comps,
		})

	case cidButtonToggle:
		st = st.withMode(pageMain)
		rc2, err := toggleBool(rc, st.Key)
		if err != nil {
			embeds := []discord.Embed{errorEmbed(fmt.Sprintf("Toggle failed: %v", err))}
			return h.edit(ctx, i, api.EditInteractionResponseData{
				Embeds: &embeds,
			})
		}
		_ = saveRuntimeConfig(h.cm, rc2, st.Scope)
		var applyErr error
		if h.applier != nil {
			applyErr = h.applier.Apply(ctx, rc2)
		}
		embeds := []discord.Embed{withHotApplyWarning(renderMainEmbed(rc2, st), applyErr)}
		comps := renderMainComponents(rc2, st)
		return h.edit(ctx, i, api.EditInteractionResponseData{
			Embeds:     &embeds,
			Components: &comps,
		})

	case cidButtonEdit:
		sp, ok := specByKey(st.Key)
		if !ok || sp.Type == vtBool {
			return h.denyEphemeral(ctx, i, "Invalid key or type for editing.")
		}

		cur, _ := getValue(rc, st.Key)
		maxLen := sp.MaxInputLen
		if maxLen <= 0 {
			maxLen = 200
		}

		var userID discord.UserID
		if i.Member != nil {
			userID = i.Member.User.ID
		} else if i.User != nil {
			userID = i.User.ID
		}

		comps := discord.ContainerComponents{
			&discord.ActionRowComponent{
				&discord.TextInputComponent{
					CustomID:     discord.ComponentID(modalEditValueID),
					Label:        fmt.Sprintf("%s (%s)", sp.Key, sp.Type),
					Style:        discord.TextInputShortStyle,
					Placeholder:  sp.DefaultHint,
					Value:        cur,
					Required:     false,
					LengthLimits: [2]int{0, maxLen},
				},
			},
		}

		return h.respond(ctx, i, api.InteractionResponse{
			Type: api.ModalResponse,
			Data: &api.InteractionResponseData{
				CustomID:   option.NewNullableString(encodeRuntimeModalState(st, userID.String())),
				Title:      option.NewNullableString(string(sp.Key)),
				Components: &comps,
			},
		})
	}

	return nil
}

func (h *Handler) HandleModal(ctx context.Context, i *discord.InteractionEvent) error {
	d, ok := i.Data.(*discord.ModalInteraction)
	if !ok {
		return nil
	}

	st, token, valid := decodeRuntimeModalState(string(d.CustomID))
	if !valid {
		h.logger.Warn("Failed to decode runtime state from modal interaction",
			slog.String("custom_id", string(d.CustomID)),
			slog.String("request_id", i.ID.String()))
		return h.denyEphemeral(ctx, i, "Invalid modal interaction.")
	}

	h.logger.Debug("Decoded runtime modal state",
		slog.String("request_id", i.ID.String()),
		slog.String("key", string(st.Key)))

	if !h.authorizeInteraction(ctx, i, token) {
		h.logger.Warn("Interaction authorization failed for runtime modal",
			slog.String("guild_id", i.GuildID.String()),
			slog.String("request_id", i.ID.String()))
		return h.denyEphemeral(ctx, i, "You do not have permission to submit this modal.")
	}

	_ = h.respond(ctx, i, api.InteractionResponse{
		Type: api.DeferredMessageUpdate,
	})

	val := ""
	for _, row := range d.Components {
		if actionRow, ok := row.(*discord.ActionRowComponent); ok {
			for _, comp := range *actionRow {
				if textInput, ok := comp.(*discord.TextInputComponent); ok {
					if string(textInput.CustomID) == modalEditValueID {
						val = textInput.Value
					}
				}
			}
		}
	}

	sp, ok := specByKey(st.Key)
	if !ok {
		embeds := []discord.Embed{errorEmbed("Unknown config key.")}
		return h.edit(ctx, i, api.EditInteractionResponseData{
			Embeds: &embeds,
		})
	}

	rc, err := loadRuntimeConfig(h.cm, st.Scope)
	if err != nil {
		embeds := []discord.Embed{errorEmbed(fmt.Sprintf("Failed to load: %v", err))}
		return h.edit(ctx, i, api.EditInteractionResponseData{
			Embeds: &embeds,
		})
	}

	next, err := setValue(rc, sp, val)
	if err != nil {
		embeds := []discord.Embed{errorEmbed(fmt.Sprintf("Invalid value: %v", err))}
		comps := renderMainComponents(rc, st.withMode(pageMain))
		return h.edit(ctx, i, api.EditInteractionResponseData{
			Embeds:     &embeds,
			Components: &comps,
		})
	}

	_ = saveRuntimeConfig(h.cm, next, st.Scope)
	var applyErr error
	if h.applier != nil {
		applyErr = h.applier.Apply(ctx, next)
	}

	st = st.withMode(pageMain)
	embeds := []discord.Embed{withHotApplyWarning(renderMainEmbed(next, st), applyErr)}
	comps := renderMainComponents(next, st)
	return h.edit(ctx, i, api.EditInteractionResponseData{
		Embeds:     &embeds,
		Components: &comps,
	})
}
