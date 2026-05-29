package webhook

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/errutil"
)

// MessageEmbedPatch defines the payload for editing an existing webhook message.
type MessageEmbedPatch struct {
	MessageID  string
	WebhookURL string
	Embed      json.RawMessage
}

// PatchMessageEmbed edits an existing webhook message by replacing its embeds.
// The embed payload accepts:
// - a single embed object
// - an embeds array
// - an object containing { "embeds": [...] }
func PatchMessageEmbed(session *discordgo.Session, patch MessageEmbedPatch) (err error) {
	defer func() { err = errutil.Wrap(err, "patch webhook message embed") }()
	if session == nil {
		return errors.New("nil discord session")
	}

	messageID := strings.TrimSpace(patch.MessageID)
	if messageID == "" {
		return errors.New("missing message_id")
	}

	webhookID, webhookToken, err := ParseWebhookURL(strings.TrimSpace(patch.WebhookURL))
	if err != nil {
		return err
	}

	embeds, err := decodeEmbeds(patch.Embed)
	if err != nil {
		return err
	}

	_, err = session.WebhookMessageEdit(webhookID, webhookToken, messageID, &discordgo.WebhookEdit{
		Embeds: &embeds,
	})
	if err != nil {
		return fmt.Errorf("edit message_id=%s: %w", messageID, err)
	}
	return nil
}

// ParseWebhookURL extracts the webhook ID and token from a standard Discord webhook URL.
func ParseWebhookURL(rawURL string) (string, string, error) {
	if rawURL == "" {
		return "", "", errors.New("missing webhook_url")
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return "", "", errors.New("invalid webhook_url format")
	}

	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	for i := 0; i < len(parts); i++ {
		if parts[i] != "webhooks" {
			continue
		}
		if i+2 >= len(parts) {
			return "", "", errors.New("invalid webhook_url path")
		}
		webhookID := strings.TrimSpace(parts[i+1])
		webhookToken := strings.TrimSpace(parts[i+2])
		if webhookID == "" || webhookToken == "" {
			return "", "", errors.New("invalid webhook_url credentials")
		}
		return webhookID, webhookToken, nil
	}

	return "", "", errors.New("invalid webhook_url path")
}

func decodeEmbeds(raw json.RawMessage) ([]*discordgo.MessageEmbed, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, errors.New("missing embed payload")
	}

	if trimmed[0] == '[' {
		var embeds []*discordgo.MessageEmbed
		if err := json.Unmarshal(trimmed, &embeds); err != nil {
			return nil, fmt.Errorf("invalid embeds array: %w", err)
		}
		if len(embeds) == 0 {
			return nil, errors.New("empty embeds array")
		}
		return embeds, nil
	}

	if trimmed[0] != '{' {
		return nil, errors.New("embed payload must be an object or array")
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &obj); err != nil {
		return nil, fmt.Errorf("invalid embed object: %w", err)
	}

	if embedsPayload, ok := obj["embeds"]; ok {
		var embeds []*discordgo.MessageEmbed
		if err := json.Unmarshal(embedsPayload, &embeds); err != nil {
			return nil, fmt.Errorf("invalid embeds field: %w", err)
		}
		if len(embeds) == 0 {
			return nil, errors.New("embeds field is empty")
		}
		return embeds, nil
	}

	var embed discordgo.MessageEmbed
	if err := json.Unmarshal(trimmed, &embed); err != nil {
		return nil, fmt.Errorf("invalid embed object: %w", err)
	}
	return []*discordgo.MessageEmbed{&embed}, nil
}
