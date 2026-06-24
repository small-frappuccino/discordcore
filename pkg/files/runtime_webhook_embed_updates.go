package files

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
)

var (
	// ErrWebhookEmbedUpdateNotFound indicates no entry matched the provided message_id.
	ErrWebhookEmbedUpdateNotFound = errors.New("webhook embed update not found")
	// ErrWebhookEmbedUpdateAlreadyExists indicates message_id already exists in the target scope.
	ErrWebhookEmbedUpdateAlreadyExists = errors.New("webhook embed update already exists")
)

// ListWebhookEmbedUpdates returns webhook embed update entries for the given scope.
// Scope behavior:
// - guildID empty or "global": bot-level runtime_config
// - guildID set: guild-level runtime_config for that guild ID
func (mgr *ConfigManager) ListWebhookEmbedUpdates(guildID string) ([]WebhookEmbedUpdateConfig, error) {
	scope := normalizeWebhookScope(guildID)

	cfg := mgr.Config()
	if cfg == nil {
		return nil, nil
	}
	rc, err := runtimeConfigForScope(cfg, scope)
	if err != nil {
		return nil, fmt.Errorf("ConfigManager.ListWebhookEmbedUpdates: %w", err)
	}
	if rc == nil {
		return nil, nil
	}
	return cloneWebhookEmbedUpdateList(rc.NormalizedWebhookEmbedUpdates()), nil
}

// GetWebhookEmbedUpdate fetches one entry by message_id from the target scope.
func (mgr *ConfigManager) GetWebhookEmbedUpdate(guildID, messageID string) (_ WebhookEmbedUpdateConfig, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("get webhook embed update: %w", err)
		}
	}()
	targetID := strings.TrimSpace(messageID)
	if targetID == "" {
		return WebhookEmbedUpdateConfig{}, fmt.Errorf("message_id is required")
	}

	scope := normalizeWebhookScope(guildID)

	cfg := mgr.Config()
	if cfg == nil {
		return WebhookEmbedUpdateConfig{}, fmt.Errorf("%w: message_id=%s", ErrWebhookEmbedUpdateNotFound, targetID)
	}
	rc, err := runtimeConfigForScope(cfg, scope)
	if err != nil {
		return WebhookEmbedUpdateConfig{}, fmt.Errorf("ConfigManager.GetWebhookEmbedUpdate: %w", err)
	}
	if rc == nil {
		return WebhookEmbedUpdateConfig{}, fmt.Errorf("%w: message_id=%s", ErrWebhookEmbedUpdateNotFound, targetID)
	}

	updates := rc.NormalizedWebhookEmbedUpdates()
	idx := findWebhookEmbedUpdateIndexByMessageID(updates, targetID)
	if idx < 0 {
		return WebhookEmbedUpdateConfig{}, fmt.Errorf("%w: message_id=%s", ErrWebhookEmbedUpdateNotFound, targetID)
	}
	return cloneWebhookEmbedUpdateConfig(updates[idx]), nil
}

// CreateWebhookEmbedUpdate appends a new entry to the target scope.
func (mgr *ConfigManager) CreateWebhookEmbedUpdate(guildID string, update WebhookEmbedUpdateConfig) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("create webhook embed update: %w", err)
		}
	}()
	scope := normalizeWebhookScope(guildID)

	normalized, err := normalizeWebhookEmbedUpdateConfig(update)
	if err != nil {
		return fmt.Errorf("ConfigManager.CreateWebhookEmbedUpdate: %w", err)
	}
	return mgr.updateRuntimeConfigScope(scope, func(rc *RuntimeConfig) error {
		updates := rc.NormalizedWebhookEmbedUpdates()
		if findWebhookEmbedUpdateIndexByMessageID(updates, normalized.MessageID) >= 0 {
			return fmt.Errorf("%w: message_id=%s", ErrWebhookEmbedUpdateAlreadyExists, normalized.MessageID)
		}

		updates = append(updates, normalized)
		setWebhookEmbedUpdatesCanonical(rc, updates)
		return nil
	})
}

// UpdateWebhookEmbedUpdate replaces an existing entry selected by message_id.
func (mgr *ConfigManager) UpdateWebhookEmbedUpdate(guildID, messageID string, update WebhookEmbedUpdateConfig) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("update webhook embed update: %w", err)
		}
	}()
	scope := normalizeWebhookScope(guildID)
	targetID := strings.TrimSpace(messageID)
	if targetID == "" {
		return fmt.Errorf("message_id is required")
	}

	normalized, err := normalizeWebhookEmbedUpdateConfig(update)
	if err != nil {
		return fmt.Errorf("ConfigManager.UpdateWebhookEmbedUpdate: %w", err)
	}
	return mgr.updateRuntimeConfigScope(scope, func(rc *RuntimeConfig) error {
		updates := rc.NormalizedWebhookEmbedUpdates()
		idx := findWebhookEmbedUpdateIndexByMessageID(updates, targetID)
		if idx < 0 {
			return fmt.Errorf("%w: message_id=%s", ErrWebhookEmbedUpdateNotFound, targetID)
		}

		if normalized.MessageID != targetID {
			dupIdx := findWebhookEmbedUpdateIndexByMessageID(updates, normalized.MessageID)
			if dupIdx >= 0 && dupIdx != idx {
				return fmt.Errorf("%w: message_id=%s", ErrWebhookEmbedUpdateAlreadyExists, normalized.MessageID)
			}
		}

		updates[idx] = normalized
		setWebhookEmbedUpdatesCanonical(rc, updates)
		return nil
	})
}

// DeleteWebhookEmbedUpdate removes an entry from the target scope.
func (mgr *ConfigManager) DeleteWebhookEmbedUpdate(guildID, messageID string) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("delete webhook embed update: %w", err)
		}
	}()
	scope := normalizeWebhookScope(guildID)
	targetID := strings.TrimSpace(messageID)
	if targetID == "" {
		return fmt.Errorf("message_id is required")
	}

	return mgr.updateRuntimeConfigScope(scope, func(rc *RuntimeConfig) error {
		updates := rc.NormalizedWebhookEmbedUpdates()
		idx := findWebhookEmbedUpdateIndexByMessageID(updates, targetID)
		if idx < 0 {
			return fmt.Errorf("%w: message_id=%s", ErrWebhookEmbedUpdateNotFound, targetID)
		}

		updates = slices.Delete(updates, idx, idx+1)
		setWebhookEmbedUpdatesCanonical(rc, updates)
		return nil
	})
}

func normalizeWebhookScope(guildID string) string {
	scope := strings.TrimSpace(guildID)
	if strings.EqualFold(scope, "global") {
		return ""
	}
	return scope
}

func normalizeWebhookEmbedUpdateConfig(in WebhookEmbedUpdateConfig) (WebhookEmbedUpdateConfig, error) {
	out := WebhookEmbedUpdateConfig{
		MessageID:  strings.TrimSpace(in.MessageID),
		WebhookURL: strings.TrimSpace(in.WebhookURL),
	}

	if out.MessageID == "" {
		return WebhookEmbedUpdateConfig{}, fmt.Errorf("message_id is required")
	}
	if out.WebhookURL == "" {
		return WebhookEmbedUpdateConfig{}, fmt.Errorf("webhook_url is required")
	}
	if err := validateDiscordWebhookURL(out.WebhookURL); err != nil {
		return WebhookEmbedUpdateConfig{}, fmt.Errorf("webhook_url is invalid: %w", err)
	}

	raw := bytes.TrimSpace(in.Embed)
	if len(raw) == 0 {
		return WebhookEmbedUpdateConfig{}, fmt.Errorf("embed payload is required")
	}
	if !json.Valid(raw) {
		return WebhookEmbedUpdateConfig{}, fmt.Errorf("embed payload must be valid JSON")
	}
	if raw[0] != '{' && raw[0] != '[' {
		return WebhookEmbedUpdateConfig{}, fmt.Errorf("embed payload must be a JSON object or array")
	}

	out.Embed = append(json.RawMessage(nil), raw...)
	return out, nil
}

func setWebhookEmbedUpdatesCanonical(rc *RuntimeConfig, updates []WebhookEmbedUpdateConfig) {
	if rc == nil {
		return
	}
	rc.WebhookEmbedUpdates = cloneWebhookEmbedUpdateList(updates)
}

func cloneWebhookEmbedUpdateConfig(in WebhookEmbedUpdateConfig) WebhookEmbedUpdateConfig {
	out := WebhookEmbedUpdateConfig{
		MessageID:  strings.TrimSpace(in.MessageID),
		WebhookURL: strings.TrimSpace(in.WebhookURL),
	}
	if len(in.Embed) > 0 {
		out.Embed = append(json.RawMessage(nil), in.Embed...)
	}
	return out
}

func cloneWebhookEmbedUpdateList(in []WebhookEmbedUpdateConfig) []WebhookEmbedUpdateConfig {
	if len(in) == 0 {
		return nil
	}
	out := make([]WebhookEmbedUpdateConfig, 0, len(in))
	for _, item := range in {
		out = append(out, cloneWebhookEmbedUpdateConfig(item))
	}
	return out
}

func findWebhookEmbedUpdateIndexByMessageID(updates []WebhookEmbedUpdateConfig, messageID string) int {
	targetID := strings.TrimSpace(messageID)
	if targetID == "" {
		return -1
	}
	for i, item := range updates {
		if strings.TrimSpace(item.MessageID) == targetID {
			return i
		}
	}
	return -1
}

func validateDiscordWebhookURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("must be a valid URL")
	}

	if strings.TrimSpace(u.Scheme) == "" || strings.TrimSpace(u.Host) == "" {
		return fmt.Errorf("must include scheme and host")
	}

	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	for i := 0; i < len(parts); i++ {
		if parts[i] != "webhooks" {
			continue
		}
		if i+2 >= len(parts) {
			return fmt.Errorf("must match /webhooks/{id}/{token}")
		}

		webhookID := strings.TrimSpace(parts[i+1])
		webhookToken := strings.TrimSpace(parts[i+2])
		if webhookID == "" || webhookToken == "" {
			return fmt.Errorf("must include non-empty webhook id and token")
		}
		if !isAllDigits(webhookID) {
			return fmt.Errorf("webhook id must be numeric")
		}
		return nil
	}

	return fmt.Errorf("must match /webhooks/{id}/{token}")
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
