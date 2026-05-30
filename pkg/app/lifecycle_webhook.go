package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
)

const (
	// lifecycleWebhookEnv is the env var operators set to receive
	// shutdown notifications on a Discord webhook URL. Empty / unset
	// disables the notification path; production deployments set this
	// alongside the OS-level supervisor (NSSM/Task Scheduler) so a
	// graceful stop emits a chat message before the supervisor relaunches.
	lifecycleWebhookEnv = "ALICE_LIFECYCLE_WEBHOOK_URL"

	// lifecycleWebhookTimeout caps how long the shutdown notification
	// blocks the actual process exit. Three seconds is enough for one
	// HTTP POST round-trip to discord.com on a slow link; longer would
	// delay restarts under a supervisor.
	lifecycleWebhookTimeout = 3 * time.Second
)

// notifyLifecycleEvent best-effort POSTs a one-line content message to
// the configured Discord webhook. Caller passes the high-level reason
// ("starting", "stopping", "fatal") and an optional detail string. Any
// failure (no URL configured, network error, Discord 5xx) is swallowed
// after a warn log — the shutdown path must not block on this.
//
// This is intentionally not the discordgo session API: during shutdown
// the bot's gateway connection is already being torn down, and we want
// the notification to work even if the bot died in a way that prevents
// it from making API calls (e.g. token revoked). A plain HTTP POST
// against the webhook URL needs no session state.
func notifyLifecycleEvent(reason, detail string) {
	webhookURL := strings.TrimSpace(files.EnvString(lifecycleWebhookEnv, ""))
	if webhookURL == "" {
		return
	}

	content := buildLifecycleContent(reason, detail)
	payload, err := json.Marshal(map[string]string{"content": content})
	if err != nil {
		log.ApplicationLogger().Warn(
			"Lifecycle webhook payload encode failed",
			"operation", "lifecycle.webhook",
			"reason", reason,
			"err", err,
		)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), lifecycleWebhookTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(payload))
	if err != nil {
		log.ApplicationLogger().Warn(
			"Lifecycle webhook request construction failed",
			"operation", "lifecycle.webhook",
			"reason", reason,
			"err", err,
		)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: lifecycleWebhookTimeout}
	resp, err := client.Do(req)
	if err != nil {
		log.ApplicationLogger().Warn(
			"Lifecycle webhook POST failed",
			"operation", "lifecycle.webhook",
			"reason", reason,
			"err", err,
		)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		log.ApplicationLogger().Warn(
			"Lifecycle webhook returned error status",
			"operation", "lifecycle.webhook",
			"reason", reason,
			"status", resp.StatusCode,
		)
	}
}

// buildLifecycleContent renders the chat message body. Keep it short and
// human-readable — operators see this in the alert channel and need
// "what happened, which bot, which version" at a glance, not JSON.
func buildLifecycleContent(reason, detail string) string {
	app := strings.TrimSpace(files.ConfiguredAppName)
	if app == "" {
		app = "discordcore"
	}
	version := strings.TrimSpace(files.AppVersion)
	if version == "" {
		version = files.DiscordCoreVersion
	}
	host := strings.TrimSpace(files.DiscordBotName)

	parts := []string{fmt.Sprintf("**%s** (%s)", app, version)}
	if host != "" {
		parts = append(parts, "as `"+host+"`")
	}
	parts = append(parts, "→", reason)
	if detail = strings.TrimSpace(detail); detail != "" {
		parts = append(parts, "—", detail)
	}
	return strings.Join(parts, " ")
}
