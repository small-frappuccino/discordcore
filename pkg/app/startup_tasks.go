package app

import (
	"context"
	stdErrors "errors"
	"fmt"
	"strings"
	"sync"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/control"
	"github.com/small-frappuccino/discordcore/pkg/discord/webhook"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/partners"
	"github.com/small-frappuccino/discordcore/pkg/runtimeapply"
)

type controlServerHolder struct {
	mu     sync.Mutex
	server *control.Server
}

func (h *controlServerHolder) Set(server *control.Server) {
	if h == nil || server == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.server = server
}

func (h *controlServerHolder) Stop(ctx context.Context) error {
	if h == nil {
		return nil
	}

	h.mu.Lock()
	server := h.server
	h.server = nil
	h.mu.Unlock()

	if server == nil {
		return nil
	}
	return server.Stop(ctx)
}

type controlStartupTaskOptions struct {
	runOptions            RunOptions
	configManager         *files.ConfigManager
	runtimeApplier        *runtimeapply.Manager
	controlBearerToken    string
	defaultBotInstanceID  string
	runtimeResolver       *botRuntimeResolver
	partnerBoardService   partners.BoardService
	partnerSyncExecutor   partners.GuildSyncExecutor
	controlServerRegistry *controlServerHolder
}

func scheduleRuntimeConfiguredGuildLogging(
	runtime *botRuntime,
	configManager *files.ConfigManager,
	defaultBotInstanceID string,
	startupTasks *startupTaskOrchestrator,
) {
	if runtime == nil || runtime.session == nil || configManager == nil {
		return
	}

	run := func(context.Context) error {
		if err := files.LogConfiguredGuildsForBot(configManager, runtime.session, runtime.instanceID, defaultBotInstanceID); err != nil {
			log.ErrorLoggerRaw().Error(
				"Some configured guilds could not be accessed",
				"botInstanceID", runtime.instanceID,
				"err", err,
			)
		}
		return nil
	}

	if startupTasks == nil {
		_ = run(context.Background())
		return
	}

	startupTasks.GoLight("log_configured_guilds:"+runtime.instanceID, run)
}

func scheduleStartupWebhookEmbedUpdates(
	startupTasks *startupTaskOrchestrator,
	cfg *files.BotConfig,
	defaultSession *discordgo.Session,
) {
	if cfg == nil || defaultSession == nil {
		return
	}

	run := func(ctx context.Context) error {
		for _, item := range collectStartupWebhookEmbedUpdates(cfg) {
			if err := ctx.Err(); err != nil {
				return err
			}

			operation := fmt.Sprintf(
				"runtime_config.webhook_embed_updates[%s:%d]",
				item.scope,
				item.index,
			)
			if err := webhook.PatchMessageEmbed(defaultSession, webhook.MessageEmbedPatch{
				MessageID:  item.update.MessageID,
				WebhookURL: item.update.WebhookURL,
				Embed:      item.update.Embed,
			}); err != nil {
				log.ApplicationLogger().Warn(
					"Webhook embed patch failed",
					"operation", operation,
					"scope", item.scope,
					"messageID", strings.TrimSpace(item.update.MessageID),
					"error", err,
				)
			} else {
				log.ApplicationLogger().Info(
					"Webhook embed patch applied",
					"operation", operation,
					"scope", item.scope,
					"messageID", strings.TrimSpace(item.update.MessageID),
				)
			}
		}
		return nil
	}

	if startupTasks == nil {
		_ = run(context.Background())
		return
	}

	startupTasks.GoLight("startup_webhook_embed_updates", run)
}

func scheduleControlServerStartup(startupTasks *startupTaskOrchestrator, opts controlStartupTaskOptions) {
	run := func(ctx context.Context) error {
		return startControlServerStartupTask(ctx, opts)
	}

	if startupTasks == nil {
		if err := run(context.Background()); err != nil {
			log.ApplicationLogger().Warn("Control server startup failed", "err", err)
		}
		return
	}

	startupTasks.GoLight("control_server", run)
}

func startControlServerStartupTask(ctx context.Context, opts controlStartupTaskOptions) error {
	controlRuntime, err := resolveControlRuntime(ctx, opts.runOptions)
	if err != nil && stdErrors.Is(err, errControlLocalTLSUnavailable) {
		log.ApplicationLogger().Warn(
			"Embedded local control HTTPS is unavailable; continuing without control server",
			"err", err,
		)
		return nil
	} else if err != nil {
		return fmt.Errorf("resolve control runtime: %w", err)
	}

	controlServer := control.NewServer(controlRuntime.bindAddr, opts.configManager, opts.runtimeApplier)
	if controlServer == nil {
		log.ApplicationLogger().Warn("Control server disabled (invalid parameters)")
		return nil
	}

	if opts.controlBearerToken == "" && controlRuntime.oauthConfig == nil {
		log.ApplicationLogger().Info(
			"Control server authentication is not configured",
			"addr", controlRuntime.bindAddr,
			"dashboard_only", true,
		)
	}
	if opts.controlBearerToken != "" {
		controlServer.SetBearerToken(opts.controlBearerToken)
	}
	controlServer.SetDefaultBotInstanceID(opts.defaultBotInstanceID)
	controlServer.SetPartnerBoardService(opts.partnerBoardService)
	controlServer.SetPartnerBoardSyncExecutor(opts.partnerSyncExecutor)
	controlServer.SetDiscordSessionResolver(func(guildID string) (*discordgo.Session, error) {
		return opts.runtimeResolver.sessionForGuild(guildID)
	})
	controlServer.SetBotGuildBindingsProvider(func(ctx context.Context) ([]control.BotGuildBinding, error) {
		return opts.runtimeResolver.guildBindings(ctx)
	})
	controlServer.SetGuildRegistrationResolver(func(ctx context.Context, guildID, botInstanceID string) error {
		return opts.runtimeResolver.registerGuild(ctx, guildID, botInstanceID)
	})
	if err := controlServer.SetPublicOrigin(controlRuntime.publicOrigin); err != nil {
		return fmt.Errorf("configure control public origin: %w", err)
	}
	if controlRuntime.tlsCertFile != "" || controlRuntime.tlsKeyFile != "" {
		if err := controlServer.SetTLSCertificates(controlRuntime.tlsCertFile, controlRuntime.tlsKeyFile); err != nil {
			return fmt.Errorf("configure control tls certificates: %w", err)
		}
	}
	if controlRuntime.oauthConfig != nil {
		if err := controlServer.SetDiscordOAuthConfig(*controlRuntime.oauthConfig); err != nil {
			return fmt.Errorf("configure control discord oauth: %w", err)
		}
		log.ApplicationLogger().Info(
			"Control server Discord OAuth enabled",
			"scopes", strings.Join(control.DiscordOAuthScopes(controlRuntime.oauthConfig.IncludeGuildsMembersRead), " "),
		)
		if controlRuntime.tlsCertFile == "" || controlRuntime.tlsKeyFile == "" {
			log.ApplicationLogger().Warn("Discord OAuth is enabled but control TLS certificate/key are not configured; ensure HTTPS termination in front of control server so Secure cookies persist")
		}
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	if err := controlServer.Start(); err != nil {
		if stdErrors.Is(err, control.ErrControlServerBind) {
			log.ApplicationLogger().Warn(
				"Control server unavailable; continuing without dashboard listener",
				"addr", controlRuntime.bindAddr,
				"err", err,
			)
			return nil
		}
		return fmt.Errorf("start control server: %w", err)
	}

	if err := ctx.Err(); err != nil {
		stopCtx, cancel := context.WithCancel(context.Background())
		defer cancel()
		_ = controlServer.Stop(stopCtx)
		return err
	}

	if opts.controlServerRegistry != nil {
		opts.controlServerRegistry.Set(controlServer)
	}
	return nil
}
