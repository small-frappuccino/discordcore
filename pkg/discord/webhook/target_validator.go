package webhook

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

const defaultWebhookTargetValidationTimeout = 3 * time.Second

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

// MessageTargetValidation defines the target data used for remote validation.
type MessageTargetValidation struct {
	MessageID  string
	WebhookURL string
	Timeout    time.Duration
}

// ValidateMessageTarget verifies that:
// 1) webhook_id+token are valid and accessible
// 2) target message_id is accessible through that webhook
func ValidateMessageTarget(session *discordgo.Session, validation MessageTargetValidation) error {
	if session == nil {
		return errors.New("validate webhook target: nil discord session")
	}

	messageID := strings.TrimSpace(validation.MessageID)
	if messageID == "" {
		return errors.New("validate webhook target: missing message_id")
	}

	webhookID, webhookToken, err := parseWebhookURL(strings.TrimSpace(validation.WebhookURL))
	if err != nil {
		return fmt.Errorf("validate webhook target: %w", err)
	}

	timeout := validation.Timeout
	if timeout <= 0 {
		timeout = defaultWebhookTargetValidationTimeout
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	reqOpts := []discordgo.RequestOption{
		discordgo.WithContext(ctx),
		discordgo.WithRestRetries(0),
		discordgo.WithRetryOnRatelimit(false),
	}

	if _, err := session.WebhookWithToken(webhookID, webhookToken, reqOpts...); err != nil {
		return wrapTargetValidationError("webhook lookup", err)
	}

	if _, err := session.WebhookMessage(webhookID, webhookToken, messageID, reqOpts...); err != nil {
		return wrapTargetValidationError("message lookup", err)
	}

	return nil
}

func wrapTargetValidationError(operation string, err error) error {
	var restErr *discordgo.RESTError
	if errors.As(err, &restErr) && restErr != nil && restErr.Response != nil {
		status := restErr.Response.StatusCode
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
	if strings.Contains(strings.ToLower(err.Error()), "rate limit") {
		return &TargetValidationError{
			Operation:  operation,
			StatusCode: http.StatusTooManyRequests,
			Class:      TargetValidationClassRateLimited,
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
