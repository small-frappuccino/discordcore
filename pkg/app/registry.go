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
