# Domain Architecture: app

## Layout Topology
```text
app/
├── bot_runtime.go
├── command_handler.go
├── contracts.go
├── gateway.go
├── registry.go
└── startup.go
```

## Source Stream Aggregation

// === FILE: pkg/app/bot_runtime.go ===
```go
package app

import "context"

// HexagonalRuntime represents the clean architecture container for the bot.
// It relies entirely on injected ports rather than concrete dependencies.
type HexagonalRuntime struct {
	instanceID string
	gateway    DiscordGateway
	store      ConfigStore
	queue      TaskQueue
	logger     Logger
	handler    InteractionHandler
}

// NewHexagonalRuntime injects the adapters via ports.
func NewHexagonalRuntime(
	instanceID string,
	gateway DiscordGateway,
	store ConfigStore,
	queue TaskQueue,
	logger Logger,
	handler InteractionHandler,
) *HexagonalRuntime {
	return &HexagonalRuntime{
		instanceID: instanceID,
		gateway:    gateway,
		store:      store,
		queue:      queue,
		logger:     logger,
		handler:    handler,
	}
}

// Run attaches the generic handler to the gateway and connects.
func (h *HexagonalRuntime) Run(ctx context.Context) error {
	h.gateway.OnInteraction(h.handler)
	return h.gateway.Connect(ctx)
}

```

// === FILE: pkg/app/command_handler.go ===
```go
package app

import (
	"context"
)

// DomainService abstracts the business logic execution for a given route.
type DomainService interface {
	ExecuteCommand(ctx context.Context, payload InteractionPayload) error
}

// HexagonalCommandHandler acts as a pure dispatcher.
// It receives generic payloads, routes them in O(1) time, and injects execution into a TaskQueue.
type HexagonalCommandHandler struct {
	queue  TaskQueue
	router map[string]DomainService
	logger Logger
}

// NewHexagonalCommandHandler initializes the dispatcher with its routing table and async queue.
func NewHexagonalCommandHandler(queue TaskQueue, logger Logger, router map[string]DomainService) *HexagonalCommandHandler {
	return &HexagonalCommandHandler{
		queue:  queue,
		router: router,
		logger: logger,
	}
}

// HandleInteraction implements the InteractionHandler port.
func (ch *HexagonalCommandHandler) HandleInteraction(ctx context.Context, payload InteractionPayload) error {
	service, exists := ch.router[payload.RoutePath]
	if !exists {
		ch.logger.Info("No handler found for route", "routePath", payload.RoutePath)
		return nil
	}

	// Dispatch responsibility to an isolated async context via TaskQueue.
	// This prevents the main Goroutine from blocking.
	return ch.queue.Enqueue(ctx, func(taskCtx context.Context) error {
		// Domain logic parsing and execution.
		return service.ExecuteCommand(taskCtx, payload)
	})
}

```

// === FILE: pkg/app/contracts.go ===
```go
package app

import "context"

// BotInstance representa o isolamento perfeito definido nas suas regras:
// 1 Token -> 1 Bot -> 1 Guilda.
type BotInstance struct {
	ApplicationID string
	GuildID       string
	Token         string // Token único gerado via OAuth
}

// FeatureRegistry é o contrato que o nosso Event Bus ou Config Store vai implementar.
// Ele responde a uma única pergunta: "Qual bot tem o direito de rodar essa feature nesta guilda?"
type FeatureRegistry interface {
	// ResolveOwner retorna a instância do bot autorizada a executar a feature.
	// Retorna erro (ex: ErrFeatureNotAssigned) se nenhum bot ou um bot diferente for o dono.
	ResolveOwner(ctx context.Context, guildID string, featureName string) (*BotInstance, error)
}

// DiscordGateway abstrai a comunicação com o provedor (Arikawa/DiscordGo).
type DiscordGateway interface {
	Connect(ctx context.Context) error
	Disconnect(ctx context.Context) error
	OnInteraction(handler InteractionHandler)
	UpdatePresence(ctx context.Context, status string) error
}

// InteractionPayload representa um evento de interação agnóstico de protocolo.
type InteractionPayload struct {
	CommandID string
	RoutePath string
	GuildID   string
	Data      []byte
}

// InteractionHandler recebe eventos agnósticos mapeados pelos adapters.
type InteractionHandler interface {
	HandleInteraction(ctx context.Context, payload InteractionPayload) error
}

// ConfigStore abstrai o acesso a configurações.
type ConfigStore interface {
	GetGuildConfig(guildID string) interface{}
}

// TaskQueue abstrai o roteamento de tarefas assíncronas.
type TaskQueue interface {
	Enqueue(ctx context.Context, task func(context.Context) error) error
}

// Logger abstrai a instrumentação.
type Logger interface {
	Info(msg string, args ...any)
	Error(msg string, args ...any)
}

```

// === FILE: pkg/app/gateway.go ===
```go
package app

import (
	"context"
	"log/slog"
)

// Suponha que esta função seja o loop contínuo de leitura do WebSocket do Discord
// ou o Handler do servidor HTTP Webhook.
func (g *DiscordGateway) ListenLoop(ctx context.Context) {
	for {
		// Lê os bytes puros da conexão (Zero-allocation até aqui)
		payload, err := g.connection.ReadMessage()
		if err != nil {
			break
		}

		// DISPARO MASSIVO: Criamos uma goroutine IMEDIATAMENTE para processar o payload.
		// Go consegue levantar 100.000 goroutines em milissegundos.
		// Isso libera o loop instantaneamente para ler o próximo evento da rede.
		go func(p []byte) {
			// Usamos um timeout no contexto para garantir que nenhuma goroutine viva para sempre
			routeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()

			// Chama o roteador central
			if err := g.router.HandleInteraction(routeCtx, p); err != nil {
				// Ignoramos erros normais como ErrModerationQueueFull no log crítico,
				// pois o Load Shedding é um comportamento esperado sob ataque.
			}
		}(payload)
	}
}

```

// === FILE: pkg/app/registry.go ===
```go
package app

import (
	"context"
	"errors"
	"sync"
)

var ErrFeatureNotAssigned = errors.New("feature não está atribuída a nenhum bot nesta guilda")

// registryKey atua como uma chave composta.
// DICA DE PERFORMANCE: Usar uma struct como chave de mapa no Go evita
// alocações na Heap (Zero-Allocation), pois não precisamos de concatenar
// strings como "guildID:featureName".
type registryKey struct {
	guildID     string
	featureName string
}

// InMemoryFeatureRegistry implementa a nossa interface FeatureRegistry.
type InMemoryFeatureRegistry struct {
	// Usamos RWMutex (Read-Write Mutex) em vez do Mutex padrão.
	// Isso permite que infinitas goroutines leiam o mapa simultaneamente,
	// bloqueando apenas quando uma configuração for atualizada (escrita).
	mu     sync.RWMutex
	routes map[registryKey]*BotInstance
}

// NewInMemoryFeatureRegistry constrói a nossa fonte de verdade na memória.
func NewInMemoryFeatureRegistry() *InMemoryFeatureRegistry {
	return &InMemoryFeatureRegistry{
		routes: make(map[registryKey]*BotInstance),
	}
}

// ResolveOwner é o "Hot-Path". Esta função será chamada milhares de vezes por segundo.
func (r *InMemoryFeatureRegistry) ResolveOwner(ctx context.Context, guildID string, featureName string) (*BotInstance, error) {
	// RLock (Read Lock): Múltiplas rotinas podem passar por aqui ao mesmo tempo
	// sem esperarem umas pelas outras.
	r.mu.RLock()
	defer r.mu.RUnlock()

	key := registryKey{guildID: guildID, featureName: featureName}

	if bot, exists := r.routes[key]; exists {
		return bot, nil
	}

	return nil, ErrFeatureNotAssigned
}

// UpdateRoute é a porta de mutação de estado.
// Só será chamada quando o "WatchConfig" (o nosso Event Bus) detetar
// que um Administrador ativou ou trocou o bot responsável pela moderação.
func (r *InMemoryFeatureRegistry) UpdateRoute(guildID string, featureName string, bot *BotInstance) {
	// Lock exclusivo (Write Lock). Pausa as leituras por nanossegundos
	// apenas o tempo estritamente necessário para atualizar o ponteiro no mapa.
	r.mu.Lock()
	defer r.mu.Unlock()

	key := registryKey{guildID: guildID, featureName: featureName}
	r.routes[key] = bot
}

// RemoveRoute limpa a atribuição (ex: quando um bot é expulso da guilda ou a feature desativada).
func (r *InMemoryFeatureRegistry) RemoveRoute(guildID string, featureName string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := registryKey{guildID: guildID, featureName: featureName}
	delete(r.routes, key)
}

```

// === FILE: pkg/app/startup.go ===
```go
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

```

