package core

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
