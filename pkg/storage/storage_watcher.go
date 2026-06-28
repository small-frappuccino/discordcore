package storage

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/lib/pq"
	"github.com/small-frappuccino/discordcore/pkg/core"
)

// ConfigChangeEvent representa a mensagem JSON enviada pelo Postgres.
type ConfigChangeEvent struct {
	Action        string `json:"action"` // "ENABLE" ou "DISABLE"
	GuildID       string `json:"guild_id"`
	FeatureName   string `json:"feature_name"`
	ApplicationID string `json:"application_id"`
	BotToken      string `json:"bot_token"`
}

// StartConfigWatcher arranca uma goroutine via errgroup que escuta o canal do Postgres.
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

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		defer listener.Close()
		for {
			select {
			case <-egCtx.Done():
				slog.Info("A encerrar o Watcher de configurações...")
				return egCtx.Err()

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
	})

	return eg.Wait()
}

// applyMutation decide como alterar o mapa Zero-Allocation.
func applyMutation(event ConfigChangeEvent, registry *core.InMemoryFeatureRegistry) {
	featEnum, ok := stringToFeature(event.FeatureName)
	if !ok {
		slog.Warn("Feature desconhecida ignorada na mutação dinâmica", "feature", event.FeatureName)
		return
	}

	if event.Action == "ENABLE" {
		slog.Info("Recebida mutação dinâmica: Ativar Feature", "guilda", event.GuildID, "feature", event.FeatureName)

		botInstance := core.BotInstance{
			ApplicationID: event.ApplicationID,
			GuildID:       event.GuildID,
			Token:         core.Token(event.BotToken),
		}

		err := registry.UpdateRoute(event.GuildID, featEnum, botInstance)
		if err != nil {
			slog.Error("Rejeitada mutação de Ativação", "guilda", event.GuildID, "feature", event.FeatureName, "erro", err)
		}
	} else if event.Action == "DISABLE" {
		slog.Info("Recebida mutação dinâmica: Desativar Feature", "guilda", event.GuildID, "feature", event.FeatureName)
		err := registry.RemoveRoute(event.GuildID, featEnum)
		if err != nil {
			slog.Error("Rejeitada mutação de Desativação", "guilda", event.GuildID, "feature", event.FeatureName, "erro", err)
		}
	}
}
