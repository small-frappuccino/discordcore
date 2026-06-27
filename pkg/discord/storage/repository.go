package storage

import (
	"context"
)

// GuildFeatureConfig é o modelo de dados puro que vem da Base de Dados.
// Ele é DTO (Data Transfer Object) sem qualquer lógica de negócio.
type GuildFeatureConfig struct {
	GuildID       string
	FeatureName   string // ex: "moderation", "roles", "logging"
	ApplicationID string // O ID do bot que tem os direitos desta feature
	BotToken      string // O Token de rede para chamadas HTTP
	IsActive      bool
}

// FeatureRepository é a interface que garante a inversão de controlo.
// A camada de negócio só conhece este contrato, nunca a tecnologia de base de dados.
type FeatureRepository interface {
	// FetchAllActive resolve todas as configurações que estão ligadas.
	FetchAllActive(ctx context.Context) ([]GuildFeatureConfig, error)
}
