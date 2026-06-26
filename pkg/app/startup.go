package app

import (
	"context"
	"log/slog"
	"sync"

	"github.com/small-frappuccino/discordcore/pkg/discord/moderation"
)

// Application é o contentor global de estado e dependências.
type Application struct {
	registry *InMemoryFeatureRegistry
	modSvc   *moderation.Service
	router   *moderation.Router
	// No futuro: base de dados PostgreSQL, Redis, EventBus...
}

// NewApplication liga (wires) os componentes sem iniciar nenhuma goroutine.
func NewApplication() (*Application, error) {
	// 1. Inicializar a Fonte da Verdade em Memória (O nosso mapa Zero-Allocation)
	registry := NewInMemoryFeatureRegistry()

	// 2. Inicializar o Gateway HTTP Otimizado
	restGateway := moderation.NewRESTGateway()

	// 3. Instanciar o Serviço de Moderação com Load Shedding
	// Vamos alocar um canal com buffer para 5.000 requisições simultâneas de moderação.
	modService := moderation.NewService(restGateway, 5000)

	// 4. Ligar o Roteador de Interações à Moderação
	router := moderation.NewRouter(registry, modService)

	return &Application{
		registry: registry,
		modSvc:   modService,
		router:   router,
	}, nil
}

// Run arranca o motor CSP (Goroutines) e bloqueia a execução até à paragem.
func (a *Application) Run(ctx context.Context) error {
	var wg sync.WaitGroup

	// 1. Levantar o Worker Pool de Moderação
	// Definimos 10 goroutines simultâneas a drenar o canal de Jobs.
	// Isso impede o estrangulamento (429 Rate Limits) na API do Discord.
	slog.Info("A inicializar o Worker Pool de moderação...")
	a.modSvc.Start(ctx, 10)

	// 2. Levantar o Listener de Eventos (WebSocket ou Webhook)
	wg.Add(1)
	go func() {
		defer wg.Done()
		slog.Info("Gateway do Discord a escutar eventos...")

		// Aqui entraria o código real de leitura do socket, chamando:
		// a.router.HandleInteraction(...)

		// Bloqueia esta goroutine até o contexto receber o sinal de SIGTERM
		<-ctx.Done()
		slog.Info("Sinal de encerramento recebido na Gateway.")
	}()

	// 3. Sincronização de paragem
	// O código fica parado aqui no `<-ctx.Done()` até fazermos Ctrl+C
	<-ctx.Done()
	slog.Info("A iniciar sequência de drenagem (Draining). À espera dos workers...")

	// Aguardamos que o ListenLoop pare de receber novos eventos
	wg.Wait()

	// Como o ctx.Done() foi propagado para dentro de `a.modSvc.Start`,
	// todos os 10 workers vão finalizar graciosamente após processarem a fila atual.

	return nil
}
