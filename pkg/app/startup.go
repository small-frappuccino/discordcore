package app

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"golang.org/x/sync/errgroup"

	"github.com/small-frappuccino/discordcore/pkg/core"
	"github.com/small-frappuccino/discordcore/pkg/discord"
	"github.com/small-frappuccino/discordcore/pkg/moderation"
	"github.com/small-frappuccino/discordcore/pkg/storage"
)

// Application é o contentor global de estado e dependências.
type Application struct {
	registry *core.InMemoryFeatureRegistry
	modSvc   *moderation.Service
	router   *moderation.Router
	// No futuro: base de dados PostgreSQL, Redis, EventBus...
}

func NewApplication(dbConn *sql.DB) (*Application, error) {
	registry := core.NewInMemoryFeatureRegistry()
	restGateway := discord.NewRESTGateway()
	modService := moderation.NewService(restGateway, 5000)
	router := moderation.NewRouter(registry, modService)

	// INSTANCIAMOS A INFRAESTRUTURA
	featureRepo := storage.NewPostgresFeatureRepo(dbConn)

	// HIDRATAMOS A MEMÓRIA COM DADOS DO POSTGRESQL (I/O intensivo feito APENAS UMA VEZ)
	// Em caso de erro aqui, a aplicação "pânica" e cai (Fail-Fast), pois um bot
	// sem configurações não consegue rotear comandos.
	if err := storage.HydrateRegistry(context.Background(), featureRepo, registry); err != nil {
		return nil, fmt.Errorf("falha crítica de boot: %w", err)
	}

	return &Application{
		registry: registry,
		modSvc:   modService,
		router:   router,
	}, nil
}

// Run arranca o motor CSP (Goroutines) e bloqueia a execução até à paragem.
func (a *Application) Run(ctx context.Context) error {
	eg, egCtx := errgroup.WithContext(ctx)

	slog.Info("A inicializar o Worker Pool de moderação...")
	a.modSvc.Start(egCtx, 10)

	eg.Go(func() error {
		slog.Info("Gateway do Discord a escutar eventos...")
		<-egCtx.Done()
		slog.Info("Sinal de encerramento recebido na Gateway.")
		return nil
	})

	slog.Info("A iniciar sequência de drenagem (Draining). À espera dos workers...")

	return eg.Wait()
}
