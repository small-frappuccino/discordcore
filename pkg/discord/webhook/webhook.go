package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/api/webhook"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/utils/httputil"
	"github.com/small-frappuccino/discordcore/pkg/log"
)

// TargetValidationClass classifies webhook target validation failures.
type TargetValidationClass string

const (
	TargetValidationClassAuthDenied         TargetValidationClass = "auth_denied"
	TargetValidationClassNotFound           TargetValidationClass = "not_found"
	TargetValidationClassRateLimited        TargetValidationClass = "rate_limited"
	TargetValidationClassDiscordUnavailable TargetValidationClass = "discord_unavailable"
	TargetValidationClassUnknown            TargetValidationClass = "unknown"
)

// TargetValidationError provides structured classification for remote validation failures.
type TargetValidationError struct {
	Operation  string
	StatusCode int
	Class      TargetValidationClass
	Temporary  bool
	Cause      error
}

func (e *TargetValidationError) Error() string {
	if e == nil {
		return "target validation error"
	}

	statusLabel := "status unknown"
	if e.StatusCode > 0 {
		statusLabel = fmt.Sprintf("status %d", e.StatusCode)
	}

	var base string
	switch e.Class {
	case TargetValidationClassAuthDenied:
		base = fmt.Sprintf("%s denied (%s: invalid token or missing permission)", e.Operation, statusLabel)
	case TargetValidationClassNotFound:
		base = fmt.Sprintf("%s failed (%s: webhook or message not found)", e.Operation, statusLabel)
	case TargetValidationClassRateLimited:
		base = fmt.Sprintf("%s failed (%s: rate limited; temporary)", e.Operation, statusLabel)
	case TargetValidationClassDiscordUnavailable:
		base = fmt.Sprintf("%s failed (%s: Discord API unavailable; temporary)", e.Operation, statusLabel)
	default:
		base = fmt.Sprintf("%s failed (%s)", e.Operation, statusLabel)
	}

	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", base, e.Cause)
	}
	return base
}

func (e *TargetValidationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// API interface matching exactly the required methods
type API interface {
	WebhookMessageEdit(ctx context.Context, webhookID discord.WebhookID, webhookToken string, messageID discord.MessageID, data webhook.EditMessageData) (*discord.Message, error)
	WebhookWithToken(ctx context.Context, webhookID discord.WebhookID, webhookToken string) (*discord.Webhook, error)
	WebhookMessage(ctx context.Context, webhookID discord.WebhookID, webhookToken string, messageID discord.MessageID) (*discord.Message, error)
}

// ArikawaAPI implements API via arikawa/v3
type ArikawaAPI struct {
	Client *api.Client // Optional base client
}

func (a *ArikawaAPI) WebhookMessageEdit(ctx context.Context, webhookID discord.WebhookID, webhookToken string, messageID discord.MessageID, data webhook.EditMessageData) (*discord.Message, error) {
	c := webhook.New(webhookID, webhookToken).WithContext(ctx)
	c.Client.Retries = 0
	slog.Debug("Granular transient state inspection: Dispatching webhook message edit payload",
		slog.String("webhook_id", webhookID.String()),
		slog.String("message_id", messageID.String()),
	)
	return c.EditMessage(messageID, data)
}

func (a *ArikawaAPI) WebhookWithToken(ctx context.Context, webhookID discord.WebhookID, webhookToken string) (*discord.Webhook, error) {
	c := webhook.New(webhookID, webhookToken).WithContext(ctx)
	c.Client.Retries = 0
	slog.Debug("Granular transient state inspection: Dispatching webhook target lookup",
		slog.String("webhook_id", webhookID.String()),
	)
	return c.Get()
}

func (a *ArikawaAPI) WebhookMessage(ctx context.Context, webhookID discord.WebhookID, webhookToken string, messageID discord.MessageID) (*discord.Message, error) {
	c := webhook.New(webhookID, webhookToken).WithContext(ctx)
	c.Client.Retries = 0
	slog.Debug("Granular transient state inspection: Dispatching webhook message lookup",
		slog.String("webhook_id", webhookID.String()),
		slog.String("message_id", messageID.String()),
	)
	return c.Message(messageID)
}

// Ensure the implementation is correct
var _ API = (*ArikawaAPI)(nil)

// ParseWebhookURL parses standard Discord webhook URLs
func ParseWebhookURL(rawURL string) (discord.WebhookID, string, error) {
	if rawURL == "" {
		return 0, "", errors.New("missing webhook_url")
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return 0, "", errors.New("invalid webhook_url format")
	}

	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	for i := 0; i < len(parts); i++ {
		if parts[i] != "webhooks" {
			continue
		}
		if i+2 >= len(parts) {
			return 0, "", errors.New("invalid webhook_url path")
		}
		webhookIDStr := strings.TrimSpace(parts[i+1])
		webhookToken := strings.TrimSpace(parts[i+2])
		if webhookIDStr == "" || webhookToken == "" {
			return 0, "", errors.New("invalid webhook_url credentials")
		}

		sf, err := discord.ParseSnowflake(webhookIDStr)
		if err != nil {
			return 0, "", fmt.Errorf("invalid webhook_id: %w", err)
		}

		return discord.WebhookID(sf), webhookToken, nil
	}

	return 0, "", errors.New("invalid webhook_url path")
}

// MessageEmbedPatch ...
type MessageEmbedPatch struct {
	MessageID  string
	WebhookURL string
	Embed      json.RawMessage
}

// PatchMessageEmbed ...
func PatchMessageEmbed(ctx context.Context, client API, patch MessageEmbedPatch) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("patch webhook message embed: %w", err)
		}
	}()
	if client == nil {
		err = errors.New("nil client API")
		log.EmitBlockingError("Blocking structural failure: nil client API provided for webhook patch", err, log.GenerateRequestID())
		return err
	}

	messageIDStr := strings.TrimSpace(patch.MessageID)
	if messageIDStr == "" {
		return errors.New("missing message_id")
	}
	messageSF, err := discord.ParseSnowflake(messageIDStr)
	if err != nil {
		return fmt.Errorf("invalid message_id: %w", err)
	}
	messageID := discord.MessageID(messageSF)

	webhookID, webhookToken, err := ParseWebhookURL(strings.TrimSpace(patch.WebhookURL))
	if err != nil {
		return fmt.Errorf("PatchMessageEmbed: %w", err)
	}

	embeds, err := decodeEmbeds(patch.Embed)
	if err != nil {
		return fmt.Errorf("PatchMessageEmbed: %w", err)
	}

	data := webhook.EditMessageData{
		Embeds: &embeds,
	}

	_, err = client.WebhookMessageEdit(ctx, webhookID, webhookToken, messageID, data)
	if err != nil {
		slog.Warn("Intercepted and mitigated service degradation: Webhook edit operation failed",
			slog.String("message_id", messageID.String()),
			slog.String("error", err.Error()),
		)
		return fmt.Errorf("edit message_id=%s: %w", messageID, err)
	}

	slog.Info("Baseline operational telemetry: Webhook message embed successfully patched",
		slog.String("message_id", messageID.String()),
		slog.String("webhook_id", webhookID.String()),
	)
	return nil
}

// MessageTargetValidation ...
type MessageTargetValidation struct {
	MessageID  string
	WebhookURL string
	Timeout    time.Duration
}

const defaultWebhookTargetValidationTimeout = 3 * time.Second

// ValidateMessageTarget ...
func ValidateMessageTarget(ctx context.Context, client API, validation MessageTargetValidation) error {
	if client == nil {
		err := errors.New("validate webhook target: nil client API")
		log.EmitBlockingError("Blocking structural failure: nil client API provided for validation", err, log.GenerateRequestID())
		return err
	}

	messageIDStr := strings.TrimSpace(validation.MessageID)
	if messageIDStr == "" {
		return errors.New("validate webhook target: missing message_id")
	}
	messageSF, err := discord.ParseSnowflake(messageIDStr)
	if err != nil {
		return fmt.Errorf("validate webhook target: invalid message_id: %w", err)
	}
	messageID := discord.MessageID(messageSF)

	webhookID, webhookToken, err := ParseWebhookURL(strings.TrimSpace(validation.WebhookURL))
	if err != nil {
		return fmt.Errorf("validate webhook target: %w", err)
	}

	timeout := validation.Timeout
	if timeout <= 0 {
		timeout = defaultWebhookTargetValidationTimeout
	}

	tCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if _, err := client.WebhookWithToken(tCtx, webhookID, webhookToken); err != nil {
		return wrapTargetValidationError("webhook lookup", err)
	}

	if _, err := client.WebhookMessage(tCtx, webhookID, webhookToken, messageID); err != nil {
		return wrapTargetValidationError("message lookup", err)
	}

	slog.Info("Baseline operational telemetry: Webhook message target successfully validated",
		slog.String("message_id", messageID.String()),
		slog.String("webhook_id", webhookID.String()),
	)
	return nil
}

func decodeEmbeds(raw json.RawMessage) ([]discord.Embed, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, errors.New("missing embed payload")
	}

	if trimmed[0] == '[' {
		var embeds []discord.Embed
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
		var embeds []discord.Embed
		if err := json.Unmarshal(embedsPayload, &embeds); err != nil {
			return nil, fmt.Errorf("invalid embeds field: %w", err)
		}
		if len(embeds) == 0 {
			return nil, errors.New("embeds field is empty")
		}
		return embeds, nil
	}

	var embed discord.Embed
	if err := json.Unmarshal(trimmed, &embed); err != nil {
		return nil, fmt.Errorf("invalid embed object: %w", err)
	}
	return []discord.Embed{embed}, nil
}

func wrapTargetValidationError(operation string, err error) error {
	var httpErr *httputil.HTTPError
	if errors.As(err, &httpErr) {
		status := httpErr.Status
		switch status {
		case http.StatusUnauthorized, http.StatusForbidden:
			return &TargetValidationError{
				Operation:  operation,
				StatusCode: status,
				Class:      TargetValidationClassAuthDenied,
				Temporary:  false,
				Cause:      err,
			}
		case http.StatusNotFound:
			return &TargetValidationError{
				Operation:  operation,
				StatusCode: status,
				Class:      TargetValidationClassNotFound,
				Temporary:  false,
				Cause:      err,
			}
		case http.StatusTooManyRequests:
			return &TargetValidationError{
				Operation:  operation,
				StatusCode: status,
				Class:      TargetValidationClassRateLimited,
				Temporary:  true,
				Cause:      err,
			}
		default:
			if status >= 500 && status < 600 {
				return &TargetValidationError{
					Operation:  operation,
					StatusCode: status,
					Class:      TargetValidationClassDiscordUnavailable,
					Temporary:  true,
					Cause:      err,
				}
			}
			return &TargetValidationError{
				Operation:  operation,
				StatusCode: status,
				Class:      TargetValidationClassUnknown,
				Temporary:  false,
				Cause:      err,
			}
		}
	}

	if strings.Contains(err.Error(), "HTTP 5") {
		return &TargetValidationError{
			Operation:  operation,
			StatusCode: http.StatusInternalServerError,
			Class:      TargetValidationClassDiscordUnavailable,
			Temporary:  true,
			Cause:      err,
		}
	}

	return &TargetValidationError{
		Operation:  operation,
		StatusCode: 0,
		Class:      TargetValidationClassUnknown,
		Temporary:  false,
		Cause:      err,
	}
}
