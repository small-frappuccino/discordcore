package config

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	"github.com/small-frappuccino/discordcore/pkg/discord/webhook"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

// ConfigWebhookEmbedCreateSubCommand - create webhook embed update entry.
type ConfigWebhookEmbedCreateSubCommand struct {
	configManager *files.ConfigManager
}

func NewConfigWebhookEmbedCreateSubCommand(configManager *files.ConfigManager) *ConfigWebhookEmbedCreateSubCommand {
	return &ConfigWebhookEmbedCreateSubCommand{configManager: configManager}
}

func (c *ConfigWebhookEmbedCreateSubCommand) Name() string { return "webhook_embed_create" }
func (c *ConfigWebhookEmbedCreateSubCommand) Description() string {
	return "Create a runtime webhook embed update entry"
}
func (c *ConfigWebhookEmbedCreateSubCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        optionMessageID,
			Description: "Existing message ID to patch",
			Required:    true,
		},
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        optionWebhookURL,
			Description: "Webhook URL that owns the message",
			Required:    true,
		},
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        optionEmbedJSON,
			Description: "Embed JSON (object or array)",
			Required:    true,
		},
		webhookScopeOption(),
		applyNowOption(),
	}
}
func (c *ConfigWebhookEmbedCreateSubCommand) RequiresGuild() bool       { return true }
func (c *ConfigWebhookEmbedCreateSubCommand) RequiresPermissions() bool { return true }
func (c *ConfigWebhookEmbedCreateSubCommand) Handle(ctx *core.Context) error {
	extractor := core.NewOptionExtractor(core.GetSubCommandOptions(ctx.Interaction))

	scopeGuildID, err := parseScope(ctx, extractor)
	if err != nil {
		return err
	}
	messageID, err := extractor.StringRequired(optionMessageID)
	if err != nil {
		return err
	}
	webhookURL, err := extractor.StringRequired(optionWebhookURL)
	if err != nil {
		return err
	}
	embedRaw, err := parseEmbedRaw(extractor)
	if err != nil {
		return err
	}
	applyNow := extractor.Bool(optionApplyNow)
	validationWarning := ""

	if !applyNow {
		validationWarning, err = validateWebhookTargetBeforePersist(
			ctx,
			c.configManager,
			scopeGuildID,
			messageID,
			webhookURL,
		)
		if err != nil {
			return err
		}
	}

	err = c.configManager.CreateWebhookEmbedUpdate(scopeGuildID, files.WebhookEmbedUpdateConfig{
		MessageID:  messageID,
		WebhookURL: webhookURL,
		Embed:      embedRaw,
	})
	if err != nil {
		if errors.Is(err, files.ErrWebhookEmbedUpdateAlreadyExists) {
			return webhookEmbedCommandError(webhookEmbedVisibilityDetailedError, "A webhook embed update with this message_id already exists in the selected scope")
		}
		return webhookEmbedCommandError(webhookEmbedVisibilityDetailedError, fmt.Sprintf("Failed to create webhook embed update: %v", err))
	}

	if applyNow {
		saved, err := c.configManager.GetWebhookEmbedUpdate(scopeGuildID, messageID)
		if err != nil {
			rollbackErr := rollbackCreatedWebhookEmbedUpdate(c.configManager, scopeGuildID, messageID)
			if rollbackErr != nil {
				return webhookEmbedCommandError(
					webhookEmbedVisibilityDetailedError,
					fmt.Sprintf("Create aborted: apply_now lookup failed and rollback failed (lookup=%v rollback=%v)", err, rollbackErr),
				)
			}
			return webhookEmbedCommandError(
				webhookEmbedVisibilityDetailedError,
				fmt.Sprintf("Create aborted because apply_now lookup failed; entry was rolled back: %v", err),
			)
		}
		if err := patchWebhookMessageNow(ctx, scopeGuildID, saved); err != nil {
			rollbackErr := rollbackCreatedWebhookEmbedUpdate(c.configManager, scopeGuildID, saved.MessageID)
			if rollbackErr != nil {
				return webhookEmbedCommandError(
					webhookEmbedVisibilityDetailedError,
					fmt.Sprintf("Create aborted: apply_now failed and rollback failed (apply=%v rollback=%v)", err, rollbackErr),
				)
			}
			return webhookEmbedCommandError(
				webhookEmbedVisibilityDetailedError,
				fmt.Sprintf("Create aborted because apply_now failed; entry was rolled back: %v", err),
			)
		}
	}

	msg := fmt.Sprintf(
		"Created webhook embed update in `%s` for message_id `%s` (webhook `%s`). apply_now=%t",
		renderScopeLabel(scopeGuildID),
		strings.TrimSpace(messageID),
		maskWebhookURL(webhookURL),
		applyNow,
	)
	if validationWarning != "" {
		msg += "\n" + validationWarning
		return webhookEmbedResponseBuilder(ctx.Session, webhookEmbedVisibilityDetailedError).Info(ctx.Interaction, msg)
	}
	return webhookEmbedResponseBuilder(ctx.Session, webhookEmbedVisibilityShortConfirmation).Success(ctx.Interaction, msg)
}

// ConfigWebhookEmbedReadSubCommand - read a single webhook embed update entry.
type ConfigWebhookEmbedReadSubCommand struct {
	configManager *files.ConfigManager
}

func NewConfigWebhookEmbedReadSubCommand(configManager *files.ConfigManager) *ConfigWebhookEmbedReadSubCommand {
	return &ConfigWebhookEmbedReadSubCommand{configManager: configManager}
}

func (c *ConfigWebhookEmbedReadSubCommand) Name() string { return "webhook_embed_read" }
func (c *ConfigWebhookEmbedReadSubCommand) Description() string {
	return "Read one runtime webhook embed update entry by message_id"
}
func (c *ConfigWebhookEmbedReadSubCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        optionMessageID,
			Description: "Message ID entry key",
			Autocomplete: true,
			Required:    true,
		},
		webhookScopeOption(),
	}
}
func (c *ConfigWebhookEmbedReadSubCommand) AutocompleteRouteHandler() core.AutocompleteHandler {
	return webhookEmbedMessageIDAutocompleteHandler(c.configManager)
}
func (c *ConfigWebhookEmbedReadSubCommand) RequiresGuild() bool       { return true }
func (c *ConfigWebhookEmbedReadSubCommand) RequiresPermissions() bool { return true }
func (c *ConfigWebhookEmbedReadSubCommand) Handle(ctx *core.Context) error {
	extractor := core.NewOptionExtractor(core.GetSubCommandOptions(ctx.Interaction))

	scopeGuildID, err := parseScope(ctx, extractor)
	if err != nil {
		return err
	}
	messageID, err := extractor.StringRequired(optionMessageID)
	if err != nil {
		return err
	}

	entry, err := c.configManager.GetWebhookEmbedUpdate(scopeGuildID, messageID)
	if err != nil {
		if errors.Is(err, files.ErrWebhookEmbedUpdateNotFound) {
			return webhookEmbedCommandError(webhookEmbedVisibilityDetailedError, "No webhook embed update found with that message_id in the selected scope")
		}
		return webhookEmbedCommandError(webhookEmbedVisibilityDetailedError, fmt.Sprintf("Failed to read webhook embed update: %v", err))
	}

	content := strings.Join([]string{
		fmt.Sprintf("Scope: `%s`", renderScopeLabel(scopeGuildID)),
		fmt.Sprintf("Message ID: `%s`", strings.TrimSpace(entry.MessageID)),
		fmt.Sprintf("Webhook: `%s`", maskWebhookURL(entry.WebhookURL)),
		"Embed JSON:",
		renderEmbedPreview(entry.Embed),
	}, "\n")

	return webhookEmbedResponseBuilder(ctx.Session, webhookEmbedVisibilityRenderedPayload).Info(ctx.Interaction, content)
}

// ConfigWebhookEmbedUpdateSubCommand - update an existing webhook embed update entry.
type ConfigWebhookEmbedUpdateSubCommand struct {
	configManager *files.ConfigManager
}

func NewConfigWebhookEmbedUpdateSubCommand(configManager *files.ConfigManager) *ConfigWebhookEmbedUpdateSubCommand {
	return &ConfigWebhookEmbedUpdateSubCommand{configManager: configManager}
}

func (c *ConfigWebhookEmbedUpdateSubCommand) Name() string { return "webhook_embed_update" }
func (c *ConfigWebhookEmbedUpdateSubCommand) Description() string {
	return "Update an existing runtime webhook embed update entry"
}
func (c *ConfigWebhookEmbedUpdateSubCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        optionMessageID,
			Description: "Existing message ID entry key",
			Autocomplete: true,
			Required:    true,
		},
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        optionWebhookURL,
			Description: "Webhook URL that owns the target message",
			Required:    true,
		},
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        optionEmbedJSON,
			Description: "Embed JSON (object or array)",
			Required:    true,
		},
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        optionNewMessage,
			Description: "New message ID (optional; defaults to existing message_id)",
			Required:    false,
		},
		webhookScopeOption(),
		applyNowOption(),
	}
}
func (c *ConfigWebhookEmbedUpdateSubCommand) AutocompleteRouteHandler() core.AutocompleteHandler {
	return webhookEmbedMessageIDAutocompleteHandler(c.configManager)
}
func (c *ConfigWebhookEmbedUpdateSubCommand) RequiresGuild() bool       { return true }
func (c *ConfigWebhookEmbedUpdateSubCommand) RequiresPermissions() bool { return true }
func (c *ConfigWebhookEmbedUpdateSubCommand) Handle(ctx *core.Context) error {
	extractor := core.NewOptionExtractor(core.GetSubCommandOptions(ctx.Interaction))

	scopeGuildID, err := parseScope(ctx, extractor)
	if err != nil {
		return err
	}
	targetMessageID, err := extractor.StringRequired(optionMessageID)
	if err != nil {
		return err
	}
	newMessageID := strings.TrimSpace(extractor.String(optionNewMessage))
	if newMessageID == "" {
		newMessageID = targetMessageID
	}
	webhookURL, err := extractor.StringRequired(optionWebhookURL)
	if err != nil {
		return err
	}
	embedRaw, err := parseEmbedRaw(extractor)
	if err != nil {
		return err
	}
	applyNow := extractor.Bool(optionApplyNow)
	validationWarning := ""

	previous, err := c.configManager.GetWebhookEmbedUpdate(scopeGuildID, targetMessageID)
	if err != nil {
		if errors.Is(err, files.ErrWebhookEmbedUpdateNotFound) {
			return webhookEmbedCommandError(webhookEmbedVisibilityDetailedError, "No webhook embed update found with that message_id in the selected scope")
		}
		return webhookEmbedCommandError(webhookEmbedVisibilityDetailedError, fmt.Sprintf("Failed to load webhook embed update before update: %v", err))
	}

	if !applyNow {
		validationWarning, err = validateWebhookTargetBeforePersist(
			ctx,
			c.configManager,
			scopeGuildID,
			newMessageID,
			webhookURL,
		)
		if err != nil {
			return err
		}
	}

	err = c.configManager.UpdateWebhookEmbedUpdate(scopeGuildID, targetMessageID, files.WebhookEmbedUpdateConfig{
		MessageID:  newMessageID,
		WebhookURL: webhookURL,
		Embed:      embedRaw,
	})
	if err != nil {
		if errors.Is(err, files.ErrWebhookEmbedUpdateNotFound) {
			return webhookEmbedCommandError(webhookEmbedVisibilityDetailedError, "No webhook embed update found with that message_id in the selected scope")
		}
		if errors.Is(err, files.ErrWebhookEmbedUpdateAlreadyExists) {
			return webhookEmbedCommandError(webhookEmbedVisibilityDetailedError, "The new message_id is already used by another entry in the selected scope")
		}
		return webhookEmbedCommandError(webhookEmbedVisibilityDetailedError, fmt.Sprintf("Failed to update webhook embed update: %v", err))
	}

	if applyNow {
		saved, err := c.configManager.GetWebhookEmbedUpdate(scopeGuildID, newMessageID)
		if err != nil {
			rollbackErr := rollbackUpdatedWebhookEmbedUpdate(c.configManager, scopeGuildID, newMessageID, previous)
			if rollbackErr != nil {
				return webhookEmbedCommandError(
					webhookEmbedVisibilityDetailedError,
					fmt.Sprintf("Update aborted: apply_now lookup failed and rollback failed (lookup=%v rollback=%v)", err, rollbackErr),
				)
			}
			return webhookEmbedCommandError(
				webhookEmbedVisibilityDetailedError,
				fmt.Sprintf("Update aborted because apply_now lookup failed; previous entry was restored: %v", err),
			)
		}
		if err := patchWebhookMessageNow(ctx, scopeGuildID, saved); err != nil {
			rollbackErr := rollbackUpdatedWebhookEmbedUpdate(c.configManager, scopeGuildID, saved.MessageID, previous)
			if rollbackErr != nil {
				return webhookEmbedCommandError(
					webhookEmbedVisibilityDetailedError,
					fmt.Sprintf("Update aborted: apply_now failed and rollback failed (apply=%v rollback=%v)", err, rollbackErr),
				)
			}
			return webhookEmbedCommandError(
				webhookEmbedVisibilityDetailedError,
				fmt.Sprintf("Update aborted because apply_now failed; previous entry was restored: %v", err),
			)
		}
	}

	msg := fmt.Sprintf(
		"Updated webhook embed entry in `%s`: `%s` -> `%s` (webhook `%s`). apply_now=%t",
		renderScopeLabel(scopeGuildID),
		strings.TrimSpace(targetMessageID),
		strings.TrimSpace(newMessageID),
		maskWebhookURL(webhookURL),
		applyNow,
	)
	if validationWarning != "" {
		msg += "\n" + validationWarning
		return webhookEmbedResponseBuilder(ctx.Session, webhookEmbedVisibilityDetailedError).Info(ctx.Interaction, msg)
	}
	return webhookEmbedResponseBuilder(ctx.Session, webhookEmbedVisibilityShortConfirmation).Success(ctx.Interaction, msg)
}

// ConfigWebhookEmbedDeleteSubCommand - delete a webhook embed update entry.
type ConfigWebhookEmbedDeleteSubCommand struct {
	configManager *files.ConfigManager
}

func NewConfigWebhookEmbedDeleteSubCommand(configManager *files.ConfigManager) *ConfigWebhookEmbedDeleteSubCommand {
	return &ConfigWebhookEmbedDeleteSubCommand{configManager: configManager}
}

func (c *ConfigWebhookEmbedDeleteSubCommand) Name() string { return "webhook_embed_delete" }
func (c *ConfigWebhookEmbedDeleteSubCommand) Description() string {
	return "Delete a runtime webhook embed update entry"
}
func (c *ConfigWebhookEmbedDeleteSubCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        optionMessageID,
			Description: "Message ID entry key",
			Autocomplete: true,
			Required:    true,
		},
		webhookScopeOption(),
		applyNowOption(),
	}
}
func (c *ConfigWebhookEmbedDeleteSubCommand) AutocompleteRouteHandler() core.AutocompleteHandler {
	return webhookEmbedMessageIDAutocompleteHandler(c.configManager)
}
func (c *ConfigWebhookEmbedDeleteSubCommand) RequiresGuild() bool       { return true }
func (c *ConfigWebhookEmbedDeleteSubCommand) RequiresPermissions() bool { return true }
func (c *ConfigWebhookEmbedDeleteSubCommand) Handle(ctx *core.Context) error {
	extractor := core.NewOptionExtractor(core.GetSubCommandOptions(ctx.Interaction))

	scopeGuildID, err := parseScope(ctx, extractor)
	if err != nil {
		return err
	}
	messageID, err := extractor.StringRequired(optionMessageID)
	if err != nil {
		return err
	}
	applyNow := extractor.Bool(optionApplyNow)

	if applyNow {
		current, err := c.configManager.GetWebhookEmbedUpdate(scopeGuildID, messageID)
		if err != nil {
			if errors.Is(err, files.ErrWebhookEmbedUpdateNotFound) {
				return webhookEmbedCommandError(webhookEmbedVisibilityDetailedError, "No webhook embed update found with that message_id in the selected scope")
			}
			return webhookEmbedCommandError(webhookEmbedVisibilityDetailedError, fmt.Sprintf("Failed to load webhook embed update before delete: %v", err))
		}

		if err := patchWebhookMessageNow(ctx, scopeGuildID, current); err != nil {
			return webhookEmbedCommandError(webhookEmbedVisibilityDetailedError, fmt.Sprintf("Delete aborted because apply_now failed: %v", err))
		}
	}

	if err := c.configManager.DeleteWebhookEmbedUpdate(scopeGuildID, messageID); err != nil {
		if errors.Is(err, files.ErrWebhookEmbedUpdateNotFound) {
			return webhookEmbedCommandError(webhookEmbedVisibilityDetailedError, "No webhook embed update found with that message_id in the selected scope")
		}
		return webhookEmbedCommandError(webhookEmbedVisibilityDetailedError, fmt.Sprintf("Failed to delete webhook embed update: %v", err))
	}

	msg := fmt.Sprintf(
		"Deleted webhook embed update from `%s` for message_id `%s`. apply_now=%t",
		renderScopeLabel(scopeGuildID),
		strings.TrimSpace(messageID),
		applyNow,
	)
	return webhookEmbedResponseBuilder(ctx.Session, webhookEmbedVisibilityShortConfirmation).Success(ctx.Interaction, msg)
}

// ConfigWebhookEmbedListSubCommand - list webhook embed update entries.
type ConfigWebhookEmbedListSubCommand struct {
	configManager *files.ConfigManager
}

func NewConfigWebhookEmbedListSubCommand(configManager *files.ConfigManager) *ConfigWebhookEmbedListSubCommand {
	return &ConfigWebhookEmbedListSubCommand{configManager: configManager}
}

func (c *ConfigWebhookEmbedListSubCommand) Name() string { return "webhook_embed_list" }
func (c *ConfigWebhookEmbedListSubCommand) Description() string {
	return "List runtime webhook embed update entries"
}
func (c *ConfigWebhookEmbedListSubCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		webhookScopeOption(),
	}
}
func (c *ConfigWebhookEmbedListSubCommand) RequiresGuild() bool       { return true }
func (c *ConfigWebhookEmbedListSubCommand) RequiresPermissions() bool { return true }
func (c *ConfigWebhookEmbedListSubCommand) Handle(ctx *core.Context) error {
	extractor := core.NewOptionExtractor(core.GetSubCommandOptions(ctx.Interaction))

	scopeGuildID, err := parseScope(ctx, extractor)
	if err != nil {
		return err
	}
	updates, err := c.configManager.ListWebhookEmbedUpdates(scopeGuildID)
	if err != nil {
		return webhookEmbedCommandError(webhookEmbedVisibilityDetailedError, fmt.Sprintf("Failed to list webhook embed updates: %v", err))
	}
	if len(updates) == 0 {
		return webhookEmbedResponseBuilder(ctx.Session, webhookEmbedVisibilityList).Info(
			ctx.Interaction,
			fmt.Sprintf("No webhook embed updates configured in `%s`.", renderScopeLabel(scopeGuildID)),
		)
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Webhook embed updates in `%s`:\n", renderScopeLabel(scopeGuildID)))

	limit := len(updates)
	if limit > maxListEntries {
		limit = maxListEntries
	}

	for i := 0; i < limit; i++ {
		item := updates[i]
		b.WriteString(fmt.Sprintf(
			"%d. message_id=`%s` webhook=`%s` embed_bytes=%d\n",
			i+1,
			strings.TrimSpace(item.MessageID),
			maskWebhookURL(item.WebhookURL),
			len(bytes.TrimSpace(item.Embed)),
		))
	}
	if len(updates) > limit {
		b.WriteString(fmt.Sprintf("...and %d more entries.\n", len(updates)-limit))
	}
	b.WriteString("Use `/config webhook_embed_read` with `message_id` for full details.")

	return webhookEmbedResponseBuilder(ctx.Session, webhookEmbedVisibilityList).Info(ctx.Interaction, b.String())
}

func rollbackCreatedWebhookEmbedUpdate(configManager *files.ConfigManager, scopeGuildID, messageID string) error {
	err := configManager.DeleteWebhookEmbedUpdate(scopeGuildID, messageID)
	if err == nil || errors.Is(err, files.ErrWebhookEmbedUpdateNotFound) {
		return nil
	}
	return err
}

func rollbackUpdatedWebhookEmbedUpdate(
	configManager *files.ConfigManager,
	scopeGuildID, currentMessageID string,
	previous files.WebhookEmbedUpdateConfig,
) error {
	return configManager.UpdateWebhookEmbedUpdate(scopeGuildID, currentMessageID, previous)
}

func webhookEmbedMessageIDAutocompleteHandler(configManager *files.ConfigManager) core.AutocompleteHandler {
	return core.AutocompleteHandlerFunc(func(ctx *core.Context, focusedOption string) ([]*discordgo.ApplicationCommandOptionChoice, error) {
		if focusedOption != optionMessageID || ctx == nil || ctx.Interaction == nil || configManager == nil {
			return []*discordgo.ApplicationCommandOptionChoice{}, nil
		}

		extractor := core.NewOptionExtractor(core.GetSubCommandOptions(ctx.Interaction))
		scopeGuildID, err := parseScope(ctx, extractor)
		if err != nil {
			return []*discordgo.ApplicationCommandOptionChoice{}, nil
		}

		updates, err := configManager.ListWebhookEmbedUpdates(scopeGuildID)
		if err != nil {
			return []*discordgo.ApplicationCommandOptionChoice{}, nil
		}

		query := focusedAutocompleteValue(ctx)
		return webhookEmbedAutocompleteChoices(updates, query), nil
	})
}

func focusedAutocompleteValue(ctx *core.Context) string {
	if ctx == nil || ctx.Interaction == nil {
		return ""
	}
	focused, ok := core.HasFocusedOption(core.GetSubCommandOptions(ctx.Interaction))
	if !ok || focused == nil || focused.Value == nil {
		return ""
	}
	if value, ok := focused.Value.(string); ok {
		return strings.TrimSpace(value)
	}
	return strings.TrimSpace(fmt.Sprint(focused.Value))
}

func webhookEmbedAutocompleteChoices(updates []files.WebhookEmbedUpdateConfig, query string) []*discordgo.ApplicationCommandOptionChoice {
	query = strings.ToLower(strings.TrimSpace(query))
	ids := make([]string, 0, len(updates))
	for _, update := range updates {
		messageID := strings.TrimSpace(update.MessageID)
		if messageID == "" {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(messageID), query) {
			continue
		}
		ids = append(ids, messageID)
	}

	sort.Strings(ids)
	if len(ids) > 25 {
		ids = ids[:25]
	}

	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0, len(ids))
	for _, messageID := range ids {
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
			Name:  messageID,
			Value: messageID,
		})
	}
	return choices
}

func validateWebhookTargetBeforePersist(
	ctx *core.Context,
	configManager *files.ConfigManager,
	scopeGuildID, messageID, webhookURL string,
) (string, error) {
	validation := webhookEmbedValidationPolicy(configManager, scopeGuildID)
	if validation.Mode == files.WebhookEmbedValidationModeOff {
		return "", nil
	}

	err := webhook.ValidateMessageTarget(ctx.Session, webhook.MessageTargetValidation{
		MessageID:  strings.TrimSpace(messageID),
		WebhookURL: strings.TrimSpace(webhookURL),
		Timeout:    time.Duration(validation.TimeoutMS) * time.Millisecond,
	})
	if err == nil {
		return "", nil
	}

	if validation.Mode == files.WebhookEmbedValidationModeStrict {
		return "", webhookEmbedCommandError(
			webhookEmbedVisibilityDetailedError,
			fmt.Sprintf("Webhook target validation failed in strict mode; config was not saved: %v", err),
		)
	}

	return fmt.Sprintf(
		"Warning: webhook target validation failed in soft mode; config was saved anyway: %v",
		err,
	), nil
}

func webhookEmbedValidationPolicy(configManager *files.ConfigManager, scopeGuildID string) files.WebhookEmbedValidationConfig {
	if configManager == nil {
		return (files.WebhookEmbedValidationConfig{}).Normalized()
	}

	cfg := configManager.Config()
	if cfg == nil {
		return (files.WebhookEmbedValidationConfig{}).Normalized()
	}

	rc := cfg.ResolveRuntimeConfig(scopeGuildID)
	return rc.EffectiveWebhookEmbedValidation()
}
