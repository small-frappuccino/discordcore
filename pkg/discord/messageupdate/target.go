package messageupdate

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/webhook"
)

const (
	TargetTypeWebhookMessage = "webhook_message"
	TargetTypeChannelMessage = "channel_message"
)

var (
	// ErrInvalidTarget indicates a target definition is invalid.
	ErrInvalidTarget = errors.New("invalid embed update target")
)

// EmbedUpdateTarget identifies the destination message that should receive embed updates.
// Supported target types:
// - webhook_message: requires message_id + webhook_url
// - channel_message: requires message_id + channel_id
type EmbedUpdateTarget struct {
	Type       string
	MessageID  string
	ChannelID  string
	WebhookURL string
}

// EmbedUpdater abstracts target-based embed updates.
type EmbedUpdater interface {
	UpdateEmbeds(session *discordgo.Session, target EmbedUpdateTarget, embeds []*discordgo.MessageEmbed) error
}

// DefaultEmbedUpdater implements EmbedUpdater using Discord REST calls.
type DefaultEmbedUpdater struct{}

func NewDefaultEmbedUpdater() *DefaultEmbedUpdater {
	return &DefaultEmbedUpdater{}
}

func (u *DefaultEmbedUpdater) UpdateEmbeds(
	session *discordgo.Session,
	target EmbedUpdateTarget,
	embeds []*discordgo.MessageEmbed,
) error {
	return UpdateEmbeds(session, target, embeds)
}

// Normalize validates and canonicalizes a target.
func (t EmbedUpdateTarget) Normalize() (EmbedUpdateTarget, error) {
	out := EmbedUpdateTarget{
		Type:       strings.ToLower(strings.TrimSpace(t.Type)),
		MessageID:  strings.TrimSpace(t.MessageID),
		ChannelID:  strings.TrimSpace(t.ChannelID),
		WebhookURL: strings.TrimSpace(t.WebhookURL),
	}

	if out.Type == "" {
		switch {
		case out.WebhookURL != "":
			out.Type = TargetTypeWebhookMessage
		case out.ChannelID != "":
			out.Type = TargetTypeChannelMessage
		}
	}

	if out.Type == "" {
		return EmbedUpdateTarget{}, fmt.Errorf("%w: type is required", ErrInvalidTarget)
	}
	if out.MessageID == "" {
		return EmbedUpdateTarget{}, fmt.Errorf("%w: message_id is required", ErrInvalidTarget)
	}
	if !isAllDigits(out.MessageID) {
		return EmbedUpdateTarget{}, fmt.Errorf("%w: message_id must be numeric", ErrInvalidTarget)
	}

	switch out.Type {
	case TargetTypeWebhookMessage:
		if out.WebhookURL == "" {
			return EmbedUpdateTarget{}, fmt.Errorf("%w: webhook_url is required for type=%s", ErrInvalidTarget, out.Type)
		}
		if _, _, err := parseWebhookURL(out.WebhookURL); err != nil {
			return EmbedUpdateTarget{}, fmt.Errorf("%w: webhook_url: %v", ErrInvalidTarget, err)
		}
		out.ChannelID = ""
	case TargetTypeChannelMessage:
		if out.ChannelID == "" {
			return EmbedUpdateTarget{}, fmt.Errorf("%w: channel_id is required for type=%s", ErrInvalidTarget, out.Type)
		}
		if !isAllDigits(out.ChannelID) {
			return EmbedUpdateTarget{}, fmt.Errorf("%w: channel_id must be numeric", ErrInvalidTarget)
		}
		out.WebhookURL = ""
	default:
		return EmbedUpdateTarget{}, fmt.Errorf(
			"%w: type must be %s or %s",
			ErrInvalidTarget,
			TargetTypeWebhookMessage,
			TargetTypeChannelMessage,
		)
	}

	return out, nil
}

// UpdateEmbeds applies embed updates to either webhook or channel message targets.
func UpdateEmbeds(session *discordgo.Session, target EmbedUpdateTarget, embeds []*discordgo.MessageEmbed) error {
	if session == nil {
		return fmt.Errorf("update embeds: nil discord session")
	}
	if len(embeds) == 0 {
		return fmt.Errorf("update embeds: embeds are required")
	}
	if len(embeds) > 10 {
		return fmt.Errorf("update embeds: embeds exceed Discord limit (10)")
	}
	for i, embed := range embeds {
		if embed == nil {
			return fmt.Errorf("update embeds: embeds[%d] is nil", i)
		}
	}

	normalized, err := target.Normalize()
	if err != nil {
		return fmt.Errorf("update embeds: %w", err)
	}

	switch normalized.Type {
	case TargetTypeWebhookMessage:
		payload, err := json.Marshal(embeds)
		if err != nil {
			return fmt.Errorf("update embeds: marshal embeds for webhook target: %w", err)
		}
		if err := webhook.PatchMessageEmbed(session, webhook.MessageEmbedPatch{
			MessageID:  normalized.MessageID,
			WebhookURL: normalized.WebhookURL,
			Embed:      payload,
		}); err != nil {
			return fmt.Errorf("update embeds: webhook target patch failed: %w", err)
		}
		return nil
	case TargetTypeChannelMessage:
		edit := &discordgo.MessageEdit{
			ID:      normalized.MessageID,
			Channel: normalized.ChannelID,
			Embeds:  &embeds,
		}
		if _, err := session.ChannelMessageEditComplex(edit); err != nil {
			return fmt.Errorf(
				"update embeds: channel target patch failed (channel_id=%s message_id=%s): %w",
				normalized.ChannelID,
				normalized.MessageID,
				err,
			)
		}
		return nil
	default:
		return fmt.Errorf("update embeds: unsupported target type %q", normalized.Type)
	}
}

func parseWebhookURL(rawURL string) (string, string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", "", fmt.Errorf("missing webhook URL")
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", "", fmt.Errorf("invalid webhook URL")
	}

	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	for i := 0; i < len(parts); i++ {
		if parts[i] != "webhooks" {
			continue
		}
		if i+2 >= len(parts) {
			return "", "", fmt.Errorf("webhook URL must match /webhooks/{id}/{token}")
		}
		webhookID := strings.TrimSpace(parts[i+1])
		webhookToken := strings.TrimSpace(parts[i+2])
		if webhookID == "" || webhookToken == "" {
			return "", "", fmt.Errorf("webhook URL must include id and token")
		}
		if !isAllDigits(webhookID) {
			return "", "", fmt.Errorf("webhook id must be numeric")
		}
		return webhookID, webhookToken, nil
	}

	return "", "", fmt.Errorf("webhook URL must match /webhooks/{id}/{token}")
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
