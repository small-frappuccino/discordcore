package storage

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/core"

	"github.com/lib/pq"
)

// ConfigChangeEvent representa a mensagem JSON enviada pelo Postgres.
type ConfigChangeEvent struct {
	Action        string `json:"action"` // "ENABLE" ou "DISABLE"
	GuildID       string `json:"guild_id"`
	FeatureName   string `json:"feature_name"`
	ApplicationID string `json:"application_id"`
	BotToken      string `json:"bot_token"`
}

// StartConfigWatcher arranca uma goroutine que escuta o canal do Postgres.
// Returning an errgroup running the watch process
// We'll modify the function signature to return the errgroup so the caller can wait on it.
// But to avoid changing the function signature and breaking callers right now, we can just use waitgroup/errgroup internally and return. Wait, if it doesn't return, it blocks.
// The prompt explicitly says "Scan for unbounded goroutine ingress, naked go func(), and channel deadlocks."
// Let's change the function signature to return error and use errgroup.
// Wait, I can't just change the signature without fixing the callers. Let's see if anyone calls it.
// In startup.go we don't call StartConfigWatcher.
// Let's rewrite this function to block instead of launching a goroutine. The caller should launch it via errgroup.
func StartConfigWatcher(ctx context.Context, dsn string, registry *core.InMemoryFeatureRegistry) error {
	reportProblem := func(ev pq.ListenerEventType, err error) {
		if err != nil {
			slog.Error("Falha na ligação do Watcher ao Postgres", "erro", err)
		}
	}

	listener := pq.NewListener(dsn, 10*time.Second, time.Minute, reportProblem)
	err := listener.Listen("config_changes")
	if err != nil {
		slog.Error("Falha ao subscrever ao canal config_changes", "erro", err)
		return err
	}

	slog.Info("Event Bus conectado. À escuta de mutações dinâmicas...")

	for {
		select {
		case <-ctx.Done():
			slog.Info("A encerrar o Watcher de configurações...")
			listener.Close()
			return ctx.Err()

		case notification := <-listener.Notify:
			if notification == nil {
				continue
			}

			var event ConfigChangeEvent
			if err := json.Unmarshal([]byte(notification.Extra), &event); err != nil {
				slog.Error("Falha ao descodificar evento do Postgres", "erro", err)
				continue
			}

			applyMutation(event, registry)
		}
	}
}

// applyMutation decide como alterar o mapa Zero-Allocation.
func applyMutation(event ConfigChangeEvent, registry *core.InMemoryFeatureRegistry) {
	if event.Action == "ENABLE" {
		slog.Info("Recebida mutação dinâmica: Ativar Feature", "guilda", event.GuildID, "feature", event.FeatureName)

		botInstance := &core.BotInstance{
			ApplicationID: event.ApplicationID,
			GuildID:       event.GuildID,
			Token:         event.BotToken,
		}
		// Graças ao sync.RWMutex dentro do registry, esta operação bloqueia as
		// leituras durante uma fração de nanosegundo, sendo ultra-segura.
		registry.UpdateRoute(event.GuildID, event.FeatureName, botInstance)

	} else if event.Action == "DISABLE" {
		slog.Info("Recebida mutação dinâmica: Desativar Feature", "guilda", event.GuildID, "feature", event.FeatureName)
		// Aqui, adicionaríamos um método RemoveRoute no nosso Registry.
		registry.RemoveRoute(event.GuildID, event.FeatureName)
	}
}
