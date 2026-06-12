package runtime

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordgo"
)

func handleComponent(s *discordgo.Session, i *discordgo.InteractionCreate, configManager *files.ConfigManager, applier runtimeConfigApplier) {
	cc := i.MessageComponentData()
	routeID, _, _ := strings.Cut(cc.CustomID, stateSep)
	ackPolicy := runtimeComponentAckPolicy(routeID)

	action, st := parseActionAndState(cc.CustomID)
	if action == "" {
		handleInvalidComponentState(s, i, ackPolicy, routeID)
		return
	}

	c := runtimeComponentContext{s: s, i: i, configManager: configManager, applier: applier, action: action}

	rc, err := loadRuntimeConfig(configManager, st.Scope)
	if err != nil {
		c.respondConfigLoadError(ackPolicy, err)
		return
	}

	// Guard: enforce restrictions
	if sp, ok := specByKey(st.Key); ok {
		if sp.GuildOnly && st.Scope == "global" {
			// Skip editing if global
			if action == cidButtonEdit || action == cidButtonToggle || action == cidButtonReset {
				c.edit(errorEmbed("This setting can only be configured per-guild."), renderMainComponents(rc, st), "guild_only_restriction")
				return
			}
		}
	}

	switch action {
	case cidSelectGroup, cidSelectKey:
		c.handleSelect(rc, st, cc.Values)
	case cidButtonMain, cidButtonBack:
		c.handleNavMain(rc, st)
	case cidButtonHelp:
		c.handleNavHelp(st)
	case cidButtonDetail:
		c.handleNavDetail(rc, st)
	case cidButtonReload:
		c.handleReload(rc, st)
	case cidButtonReset:
		c.handleReset(rc, st)
	case cidButtonToggle:
		c.handleToggle(rc, st)
	case cidButtonEdit:
		c.handleEdit(rc, st)
	default:
		c.edit(errorEmbed("Unknown action"), nil, "unknown_action")
	}
}

// runtimeComponentContext threads the interaction and config dependencies through the
// per-action handlers of handleComponent and provides the shared respond/edit logging
// wrappers, which stamp each interaction stage with the active action.
type runtimeComponentContext struct {
	s             *discordgo.Session
	i             *discordgo.InteractionCreate
	configManager *files.ConfigManager
	applier       runtimeConfigApplier
	action        string
}

func (c runtimeComponentContext) respond(resp *discordgo.InteractionResponse, stage string) {
	respondInteractionWithLog(c.s, c.i, resp, c.action+"."+stage)
}

func (c runtimeComponentContext) edit(embed *discordgo.MessageEmbed, components []discordgo.MessageComponent, stage string) {
	editInteractionMessageWithLog(c.s, c.i, embed, components, c.action+"."+stage)
}

// handleInvalidComponentState replies when a component custom ID cannot be parsed into
// an action. The action is unknown here, so the route ID stamps the log stage.
func handleInvalidComponentState(s *discordgo.Session, i *discordgo.InteractionCreate, ackPolicy core.InteractionAckPolicy, routeID string) {
	if ackPolicy.Mode == core.InteractionAckModeNone {
		respondInteractionWithLog(s, i, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags:  discordgo.MessageFlagsEphemeral,
				Embeds: []*discordgo.MessageEmbed{errorEmbed("Invalid interaction state")},
			},
		}, routeID+".invalid_state.respond_error")
		return
	}
	editInteractionMessageWithLog(s, i, errorEmbed("Invalid interaction state"), nil, routeID+".invalid_state.render_error")
}

func (c runtimeComponentContext) respondConfigLoadError(ackPolicy core.InteractionAckPolicy, err error) {
	msg := fmt.Sprintf("The runtime configuration couldn't be loaded, so this reply stays private: %v", err)
	if ackPolicy.Mode == core.InteractionAckModeNone {
		c.respond(&discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags:  discordgo.MessageFlagsEphemeral,
				Embeds: []*discordgo.MessageEmbed{errorEmbed(msg)},
			},
		}, "load_runtime_config_error")
		return
	}
	c.edit(errorEmbed(msg), nil, "load_runtime_config_error")
}

func (c runtimeComponentContext) handleSelect(rc files.RuntimeConfig, st panelState, values []string) {
	if len(values) == 0 {
		embed := renderMainEmbed(rc, st.withMode(pageMain))
		c.edit(embed, renderMainComponents(rc, st.withMode(pageMain)), "select.empty_values")
		return
	}
	// The value in the select menu options is the full encoded state.
	st = decodeState(values[0])
	if refreshed, loadErr := loadRuntimeConfig(c.configManager, st.Scope); loadErr == nil {
		rc = refreshed
	} else {
		slog.Warn("Runtime config panel failed to refresh state after selection",
			"action", c.action,
			"scope", st.Scope,
			"key", string(st.Key),
			"err", loadErr,
		)
	}
	st = ensureKeyInGroup(st.withMode(pageMain))
	embed := renderMainEmbed(rc, st)
	c.edit(embed, renderMainComponents(rc, st), "select.apply_state")
}

func (c runtimeComponentContext) handleNavMain(rc files.RuntimeConfig, st panelState) {
	st = st.withMode(pageMain)
	st = ensureKeyInGroup(st)
	embed := renderMainEmbed(rc, st)
	c.edit(embed, renderMainComponents(rc, st), "nav.main")
}

func (c runtimeComponentContext) handleNavHelp(st panelState) {
	st = st.withMode(pageHelp)
	embed := renderHelpEmbed()
	c.edit(embed, renderHelpComponents(st), "nav.help")
}

func (c runtimeComponentContext) handleNavDetail(rc files.RuntimeConfig, st panelState) {
	st = st.withMode(pageDetail)
	embed := renderDetailsEmbed(rc, st)
	c.edit(embed, renderDetailComponents(st), "nav.detail")
}

func (c runtimeComponentContext) handleReload(rc files.RuntimeConfig, st panelState) {
	if refreshed, loadErr := loadRuntimeConfig(c.configManager, st.Scope); loadErr == nil {
		rc = refreshed
	} else {
		slog.Warn("Runtime config panel failed to reload from storage",
			"action", c.action,
			"scope", st.Scope,
			"key", string(st.Key),
			"err", loadErr,
		)
	}
	st = ensureKeyInGroup(st)
	switch st.Mode {
	case pageHelp:
		embed := renderHelpEmbed()
		c.edit(embed, renderHelpComponents(st), "reload.help")
	case pageDetail:
		embed := renderDetailsEmbed(rc, st)
		c.edit(embed, renderDetailComponents(st), "reload.detail")
	default:
		embed := renderMainEmbed(rc, st.withMode(pageMain))
		c.edit(embed, renderMainComponents(rc, st.withMode(pageMain)), "reload.main")
	}
}

func (c runtimeComponentContext) handleReset(rc files.RuntimeConfig, st panelState) {
	st = st.withMode(pageMain)
	rc2, ok := resetValue(rc, st.Key)
	if !ok {
		c.edit(errorEmbed("Unknown key"), nil, "reset.unknown_key")
		return
	}
	if err := saveRuntimeConfig(c.configManager, rc2, st.Scope); err != nil {
		c.edit(errorEmbed(fmt.Sprintf("Failed to save: %v", err)), nil, "reset.save_error")
		return
	}
	applyErr := applyRuntimeConfigWithLog(c.applier, rc2, c.i, c.action+".reset.hot_apply", st)
	embed := renderMainEmbed(rc2, st)
	embed = withHotApplyWarning(embed, applyErr)
	c.edit(embed, renderMainComponents(rc2, st), "reset.render")
}

func (c runtimeComponentContext) handleToggle(rc files.RuntimeConfig, st panelState) {
	st = st.withMode(pageMain)
	sp, ok := specByKey(st.Key)
	if !ok {
		c.edit(errorEmbed("Unknown key"), nil, "toggle.unknown_key")
		return
	}
	if sp.Type != vtBool {
		c.edit(errorEmbed("TOGGLE is only valid for boolean keys"), renderMainComponents(rc, st), "toggle.invalid_type")
		return
	}
	rc2, err := toggleBool(rc, st.Key)
	if err != nil {
		c.edit(errorEmbed(fmt.Sprintf("Toggle failed: %v", err)), renderMainComponents(rc, st), "toggle.failed")
		return
	}
	if err := saveRuntimeConfig(c.configManager, rc2, st.Scope); err != nil {
		c.edit(errorEmbed(fmt.Sprintf("Failed to save: %v", err)), nil, "toggle.save_error")
		return
	}
	applyErr := applyRuntimeConfigWithLog(c.applier, rc2, c.i, c.action+".toggle.hot_apply", st)
	embed := renderMainEmbed(rc2, st)
	embed = withHotApplyWarning(embed, applyErr)
	c.edit(embed, renderMainComponents(rc2, st), "toggle.render")
}

func (c runtimeComponentContext) handleEdit(rc files.RuntimeConfig, st panelState) {
	sp, ok := specByKey(st.Key)
	if !ok {
		// This interaction path normally opens a modal, so we intentionally do NOT
		// ack with a message update earlier. If we hit an error, we must still
		// respond once to avoid an "interaction failed" on the client.
		c.respond(&discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags: discordgo.MessageFlagsEphemeral,
				Embeds: []*discordgo.MessageEmbed{
					errorEmbed("Unknown key"),
				},
			},
		}, "edit.unknown_key")
		return
	}
	if sp.Type == vtBool {
		c.respond(&discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags: discordgo.MessageFlagsEphemeral,
				Embeds: []*discordgo.MessageEmbed{
					errorEmbed("EDIT is not valid for boolean keys (use TOGGLE)"),
				},
			},
		}, "edit.invalid_type")
		return
	}

	cur, _ := getValue(rc, st.Key)
	if strings.TrimSpace(cur) == "" {
		cur = ""
	}
	if sp.Type == vtInt && strings.TrimSpace(cur) == "0" {
		cur = ""
	}

	maxLen := sp.MaxInputLen
	if maxLen <= 0 {
		maxLen = 200
	}
	label := fmt.Sprintf("%s (%s)", sp.Key, sp.Type)

	c.respond(&discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			CustomID: encodeRuntimeModalState(st, runtimeInteractionUserID(c.i)),
			Title:    string(sp.Key),
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID:    modalFieldValue,
							Label:       label,
							Style:       discordgo.TextInputShort,
							Placeholder: sp.DefaultHint,
							Value:       cur,
							Required:    new(bool),
							MinLength:   0,
							MaxLength:   maxLen,
						},
					},
				},
			},
		},
	}, "edit.open_modal")
}

func handleModalSubmit(s *discordgo.Session, i *discordgo.InteractionCreate, configManager *files.ConfigManager, applier runtimeConfigApplier) {
	m := i.ModalSubmitData()
	st, _, ok := decodeRuntimeModalState(m.CustomID)
	if !ok {
		return
	}

	edit := func(embed *discordgo.MessageEmbed, components []discordgo.MessageComponent, stage string) {
		editInteractionMessageWithLog(s, i, embed, components, "modal_submit."+stage)
	}

	sp, ok := specByKey(st.Key)
	if !ok {
		embed := errorEmbed("Unknown key")
		edit(embed, renderMainComponents(files.RuntimeConfig{}, st.withMode(pageMain)), "unknown_key")
		return
	}
	if sp.Type == vtBool {
		embed := errorEmbed("Invalid modal for bool key")
		edit(embed, renderMainComponents(files.RuntimeConfig{}, st.withMode(pageMain)), "invalid_bool_key")
		return
	}

	val := extractModalValue(m, modalFieldValue)

	rc, err := loadRuntimeConfig(configManager, st.Scope)
	if err != nil {
		edit(errorEmbed(fmt.Sprintf("The runtime configuration couldn't be loaded, so this reply stays private: %v", err)), nil, "load_runtime_config_error")
		return
	}

	next, err := setValue(rc, sp, val)
	if err != nil {
		embed := errorEmbed(fmt.Sprintf("Invalid value: %v", err))
		st = ensureKeyInGroup(st.withMode(pageMain))
		edit(embed, renderMainComponents(rc, st), "invalid_value")
		return
	}
	if err := saveRuntimeConfig(configManager, next, st.Scope); err != nil {
		edit(errorEmbed(fmt.Sprintf("Failed to save: %v", err)), nil, "save_error")
		return
	}

	applyErr := applyRuntimeConfigWithLog(applier, next, i, "modal_submit.hot_apply", st)

	// After saving, return to MAIN with refreshed values so the user can keep navigating.
	st = ensureKeyInGroup(st.withMode(pageMain))
	embed := renderMainEmbed(next, st)
	embed = withHotApplyWarning(embed, applyErr)
	edit(embed, renderMainComponents(next, st), "render")
}

func interactionLogFields(i *discordgo.InteractionCreate) []any {
	fields := []any{}
	if i == nil {
		return fields
	}
	fields = append(fields,
		"interactionType", int(i.Type),
		"interactionID", i.ID,
		"guildID", i.GuildID,
		"channelID", i.ChannelID,
	)
	if userID := interactionUserID(i); userID != "" {
		fields = append(fields, "userID", userID)
	}
	return fields
}

func respondInteractionWithLog(s *discordgo.Session, i *discordgo.InteractionCreate, resp *discordgo.InteractionResponse, reason string) {
	if s == nil || i == nil || i.Interaction == nil {
		slog.Error("Runtime config interaction respond skipped due to missing context", "reason", reason)
		return
	}
	if err := s.InteractionRespond(i.Interaction, resp); err != nil {
		fields := []any{"reason", reason, "err", err}
		fields = append(fields, interactionLogFields(i)...)
		slog.Error("Runtime config interaction respond failed", fields...)
	}
}

func editInteractionMessageWithLog(
	s *discordgo.Session,
	i *discordgo.InteractionCreate,
	embed *discordgo.MessageEmbed,
	components []discordgo.MessageComponent,
	reason string,
) {
	if s == nil || i == nil || i.Interaction == nil {
		slog.Error("Runtime config interaction edit skipped due to missing context", "reason", reason)
		return
	}
	if err := editInteractionMessage(s, i, embed, components); err != nil {
		fields := []any{"reason", reason, "err", err}
		fields = append(fields, interactionLogFields(i)...)
		slog.Error("Runtime config interaction edit failed", fields...)
	}
}

func applyRuntimeConfigWithLog(
	applier runtimeConfigApplier,
	next files.RuntimeConfig,
	i *discordgo.InteractionCreate,
	reason string,
	st panelState,
) error {
	if applier == nil {
		return nil
	}

	if err := applier.Apply(context.Background(), next); err != nil {
		fields := []any{
			"reason", reason,
			"scope", st.Scope,
			"key", string(st.Key),
			"err", err,
		}
		fields = append(fields, interactionLogFields(i)...)
		slog.Error("Runtime config hot-apply failed", fields...)
		return fmt.Errorf("applyRuntimeConfigWithLog: %w", err)
	}
	return nil
}

func withHotApplyWarning(embed *discordgo.MessageEmbed, applyErr error) *discordgo.MessageEmbed {
	if embed == nil || applyErr == nil {
		return embed
	}

	clone := *embed
	msg := fmt.Sprintf(
		"The runtime configuration was saved, but the change couldn't be applied immediately. A restart may be required.\nError: %v",
		applyErr,
	)
	if strings.TrimSpace(clone.Description) == "" {
		clone.Description = msg
	} else {
		clone.Description = strings.TrimSpace(clone.Description) + "\n\n" + msg
	}
	return &clone
}

func extractModalValue(m discordgo.ModalSubmitInteractionData, fieldID string) string {
	for _, comp := range m.Components {
		row, ok := comp.(*discordgo.ActionsRow)
		if !ok || row == nil {
			continue
		}
		for _, c := range row.Components {
			ti, ok := c.(*discordgo.TextInput)
			if ok && ti.CustomID == fieldID {
				return ti.Value
			}
		}
	}
	return ""
}

// parseActionAndState decodes "action|mode|group|key"
func parseActionAndState(customID string) (action string, st panelState) {
	routeID, rawState, hasState := strings.Cut(customID, stateSep)
	if !isKnownRuntimeComponentRoute(routeID) {
		return "", panelState{}
	}
	switch routeID {
	case cidSelectGroup, cidSelectKey:
		if hasState {
			return routeID, decodeState(rawState)
		}
		return routeID, panelState{Mode: pageMain, Group: "ALL", Key: runtimeKeyBotTheme}
	case cidButtonMain, cidButtonHelp, cidButtonBack,
		cidButtonDetail, cidButtonToggle, cidButtonEdit,
		cidButtonReset, cidButtonReload:
		if !hasState {
			return "", panelState{}
		}
		return routeID, decodeState(rawState)
	default:
		return "", panelState{}
	}
}

func isKnownRuntimeComponentRoute(routeID string) bool {
	switch routeID {
	case cidSelectGroup, cidSelectKey,
		cidButtonMain, cidButtonHelp, cidButtonBack,
		cidButtonDetail, cidButtonToggle, cidButtonEdit,
		cidButtonReset, cidButtonReload:
		return true
	default:
		return false
	}
}

func editInteractionMessage(s *discordgo.Session, i *discordgo.InteractionCreate, embed *discordgo.MessageEmbed, components []discordgo.MessageComponent) error {
	embeds := []*discordgo.MessageEmbed{}
	if embed != nil {
		embeds = []*discordgo.MessageEmbed{embed}
	}
	_, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Embeds:     &embeds,
		Components: &components,
	})
	return err
}
