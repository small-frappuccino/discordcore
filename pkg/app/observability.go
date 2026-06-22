package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/diamondburned/arikawa/v3/state"
	"github.com/small-frappuccino/discordcore/pkg/discord/qotd"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
	domain "github.com/small-frappuccino/discordcore/pkg/qotd"
)

// dualSDKPublisher wraps the Arikawa publisher and dynamically injects the appropriate
// Discord client token based on the target guild ID, enabling an active dual-SDK rollout.
type dualSDKPublisher struct {
	resolver *botRuntimeResolver
	mu       sync.RWMutex
	clients  map[string]*state.State // botInstanceID -> Arikawa State
}

// newDualSDKPublisher creates a new dualSDKPublisher for the QOTD service.
func newDualSDKPublisher(resolver *botRuntimeResolver) *dualSDKPublisher {
	slog.Info("Architectural state transition: Allocating dual-SDK publisher orchestrator")
	return &dualSDKPublisher{
		resolver: resolver,
		clients:  make(map[string]*state.State),
	}
}

// getArikawaPublisher resolves the guild's bot instance and returns a cached Arikawa publisher.
func (p *dualSDKPublisher) getArikawaPublisher(guildID string) (domain.Publisher, error) {
	_, botInstanceID, err := p.resolver.runtimeForGuild(guildID, "qotd")
	if err != nil {
		errWrap := fmt.Errorf("resolve bot instance for guild %s: %w", guildID, err)
		log.EmitBlockingError("Blocking structural failure: Orchestrator failed to resolve QOTD runtime capability for target guild", errWrap, log.GenerateRequestID())
		return nil, errWrap
	}

	p.mu.RLock()
	st, ok := p.clients[botInstanceID]
	p.mu.RUnlock()

	if ok {
		slog.Debug("Tracking complex conditional branch: Arikawa state publisher resolved from memory cache",
			slog.String("bot_instance_id", botInstanceID),
			slog.String("guild_id", guildID),
		)
		return qotd.NewArikawaPublisher(st.Session.Client), nil
	}

	// Fallback to legacy session token extraction to instantiate Arikawa capability wrapper.
	session, err := p.resolver.sessionForGuild(guildID, "qotd")
	if err != nil {
		errWrap := fmt.Errorf("resolve discord session for guild %s: %w", guildID, err)
		log.EmitBlockingError("Blocking structural failure: Orchestrator failed to resolve Discord session pointer for target guild", errWrap, log.GenerateRequestID())
		return nil, errWrap
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if st, ok := p.clients[botInstanceID]; ok {
		return qotd.NewArikawaPublisher(st.Session.Client), nil
	}

	token := session.Token
	if !strings.HasPrefix(token, "Bot ") {
		token = "Bot " + token
	}

	slog.Info("Architectural state transition: Materializing new Arikawa state publisher instance",
		slog.String("bot_instance_id", botInstanceID),
	)

	st = state.New("Bot " + strings.TrimPrefix(token, "Bot "))
	p.clients[botInstanceID] = st

	return qotd.NewArikawaPublisher(st.Session.Client), nil
}

// PublishOfficialPost implements domain.Publisher by routing the call to Arikawa.
func (p *dualSDKPublisher) PublishOfficialPost(ctx context.Context, params domain.PublishOfficialPostParams) (*domain.PublishedOfficialPost, error) {
	slog.Debug("Granular inspection: Routing PublishOfficialPost payload through dual-SDK gateway",
		slog.String("guild_id", params.GuildID),
	)

	pub, err := p.getArikawaPublisher(params.GuildID)
	if err != nil {
		slog.Error("Blocking structural failure: QOTD publisher resolution aborted",
			slog.String("guild_id", params.GuildID),
			slog.String("error", err.Error()),
		)
		return nil, err
	}

	res, err := pub.PublishOfficialPost(ctx, params)
	if err != nil {
		slog.Error("Blocking structural failure: QOTD payload rejected by upstream",
			slog.String("guild_id", params.GuildID),
			slog.String("error", err.Error()),
		)
	}
	return res, err
}

// DeleteOfficialPost implements domain.Publisher by routing the call to Arikawa.
func (p *dualSDKPublisher) DeleteOfficialPost(ctx context.Context, params domain.DeleteOfficialPostParams) error {
	slog.Debug("Granular inspection: Routing DeleteOfficialPost payload through dual-SDK gateway",
		slog.String("guild_id", params.GuildID),
	)

	pub, err := p.getArikawaPublisher(params.GuildID)
	if err != nil {
		slog.Error("Blocking structural failure: QOTD publisher resolution aborted",
			slog.String("guild_id", params.GuildID),
			slog.String("error", err.Error()),
		)
		return err
	}
	err = pub.DeleteOfficialPost(ctx, params)
	if err != nil {
		slog.Error("Blocking structural failure: QOTD deletion rejected by upstream",
			slog.String("guild_id", params.GuildID),
			slog.String("error", err.Error()),
		)
	}
	return err
}

const (
	// lifecycleWebhookEnv is the env var operators set to receive
	// shutdown notifications on a Discord webhook URL. Empty / unset
	// disables the notification path; production deployments set this
	// alongside the OS-level supervisor (NSSM/Task Scheduler) so a
	// graceful stop emits a chat message before the supervisor relaunches.
	lifecycleWebhookEnv = "DISCORDCORE_LIFECYCLE_WEBHOOK_URL"

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
		slog.Debug("Tracking complex conditional branch: Lifecycle webhook notification suppressed due to empty environment binding")
		return
	}

	slog.Info("Architectural state transition: Initiating out-of-band lifecycle notification sequence",
		slog.String("reason", reason),
	)

	// Serialize payload symmetrically with Discord's webhook interface expectations.
	content := buildLifecycleContent(reason, detail)
	payload, err := json.Marshal(map[string]string{"content": content})
	if err != nil {
		slog.Warn("Mitigated service degradation: Discarding lifecycle webhook transmission due to JSON marshal failure",
			slog.String("operation", "lifecycle.webhook"),
			slog.String("reason", reason),
			slog.String("error", err.Error()),
		)
		return
	}

	// Bound HTTP transport lifecycle to prevent blocking the primary teardown sequence.
	ctx, cancel := context.WithTimeout(context.Background(), lifecycleWebhookTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(payload))
	if err != nil {
		slog.Warn("Mitigated service degradation: HTTP request construction aborted during lifecycle webhook transmission",
			slog.String("operation", "lifecycle.webhook"),
			slog.String("reason", reason),
			slog.String("error", err.Error()),
		)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	slog.Debug("Granular inspection: Executing HTTP POST to external lifecycle webhook endpoint",
		slog.String("content", content),
		slog.Int("payload_bytes", len(payload)),
	)

	client := &http.Client{Timeout: lifecycleWebhookTimeout}
	resp, err := client.Do(req)
	if err != nil {
		slog.Warn("Mitigated service degradation: External webhook endpoint unreachable; timeout or DNS failure",
			slog.String("operation", "lifecycle.webhook"),
			slog.String("reason", reason),
			slog.String("error", err.Error()),
		)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		retryAfter := resp.Header.Get("Retry-After")
		if retryAfter == "" {
			retryAfter = "0"
		}
		slog.Warn("Mitigated service degradation: Discord upstream rejected lifecycle webhook payload",
			slog.String("operation", "lifecycle.webhook"),
			slog.String("reason", reason),
			slog.Int("status_code", resp.StatusCode),
			slog.String("retry_after", retryAfter),
		)
		return
	}

	slog.Info("Architectural state transition: Lifecycle webhook notification transmitted successfully",
		slog.String("reason", reason),
	)
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

	rendered := strings.Join(parts, " ")
	slog.Debug("Tracking complex conditional branch: Lifecycle message content string compiled",
		slog.String("rendered_string", rendered),
	)

	return rendered
}

type startupWebhookEmbedUpdate struct {
	scope  string
	index  int
	update files.WebhookEmbedUpdateConfig
}

func collectStartupWebhookEmbedUpdates(cfg *files.BotConfig) []startupWebhookEmbedUpdate {
	if cfg == nil {
		return nil
	}

	var out []startupWebhookEmbedUpdate

	// Extract globally scoped embed configurations prior to iterating over guild-specific overrides.
	for idx, update := range cfg.RuntimeConfig.NormalizedWebhookEmbedUpdates() {
		out = append(out, startupWebhookEmbedUpdate{
			scope:  "global",
			index:  idx,
			update: update,
		})
	}

	for _, guild := range cfg.Guilds {
		guildID := strings.TrimSpace(guild.GuildID)
		if guildID == "" {
			continue
		}
		for idx, update := range guild.RuntimeConfig.NormalizedWebhookEmbedUpdates() {
			out = append(out, startupWebhookEmbedUpdate{
				scope:  "guild:" + guildID,
				index:  idx,
				update: update,
			})
		}
	}

	return out
}
