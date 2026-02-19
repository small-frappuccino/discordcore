package config

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	"github.com/small-frappuccino/discordcore/pkg/discord/webhook"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

const (
	optionScope      = "scope"
	optionMessageID  = "message_id"
	optionWebhookURL = "webhook_url"
	optionEmbedJSON  = "embed_json"
	optionNewMessage = "new_message_id"
	optionApplyNow   = "apply_now"
	scopeGlobal      = "global"
	scopeGuild       = "guild"
)

func webhookScopeOption() *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionString,
		Name:        optionScope,
		Description: "Target runtime scope",
		Required:    false,
		Choices: []*discordgo.ApplicationCommandOptionChoice{
			{Name: "Global", Value: scopeGlobal},
			{Name: "Guild (this server)", Value: scopeGuild},
		},
	}
}

func parseScope(ctx *core.Context, extractor *core.OptionExtractor) (string, error) {
	scope := strings.ToLower(strings.TrimSpace(extractor.String(optionScope)))
	switch scope {
	case "":
		// Safe default: when command runs in guild context, default to guild scope.
		// Global writes require explicit scope=global.
		if strings.TrimSpace(ctx.GuildID) != "" {
			return ctx.GuildID, nil
		}
		return "", core.NewValidationError(optionScope, "Scope is required outside guild context (use global)")
	case scopeGlobal:
		return "", nil
	case scopeGuild:
		if strings.TrimSpace(ctx.GuildID) == "" {
			return "", core.NewCommandError("Guild scope requires a guild context", true)
		}
		return ctx.GuildID, nil
	default:
		return "", core.NewValidationError(optionScope, "Invalid scope (use global or guild)")
	}
}

func parseEmbedRaw(extractor *core.OptionExtractor) (json.RawMessage, error) {
	raw, err := extractor.StringRequired(optionEmbedJSON)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(strings.TrimSpace(raw)), nil
}

func applyNowOption() *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionBoolean,
		Name:        optionApplyNow,
		Description: "Apply patch immediately after saving config",
		Required:    false,
	}
}

func patchWebhookMessageNow(ctx *core.Context, scopeGuildID string, update files.WebhookEmbedUpdateConfig) error {
	if ctx == nil || ctx.Session == nil {
		return fmt.Errorf("discord session is unavailable")
	}

	err := webhook.PatchMessageEmbed(ctx.Session, webhook.MessageEmbedPatch{
		MessageID:  update.MessageID,
		WebhookURL: update.WebhookURL,
		Embed:      update.Embed,
	})
	if err != nil {
		return fmt.Errorf(
			"apply_now failed for scope=%s message_id=%s: %w",
			renderScopeLabel(scopeGuildID),
			strings.TrimSpace(update.MessageID),
			err,
		)
	}
	return nil
}
