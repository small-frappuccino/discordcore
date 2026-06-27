package core

import (
	"context"
	"errors"
	"sync/atomic"
)

var ErrFeatureNotAssigned = errors.New("feature não está atribuída a nenhum bot nesta guilda")

type registryKey struct {
	guildID     string
	featureName string
}

type registryState struct {
	routes map[registryKey]*BotInstance
}

type InMemoryFeatureRegistry struct {
	state atomic.Pointer[registryState]
}

func NewInMemoryFeatureRegistry() *InMemoryFeatureRegistry {
	r := &InMemoryFeatureRegistry{}
	r.state.Store(&registryState{
		routes: make(map[registryKey]*BotInstance),
	})
	return r
}

func (r *InMemoryFeatureRegistry) ResolveOwner(ctx context.Context, guildID string, featureName string) (*BotInstance, error) {
	state := r.state.Load()
	if state == nil {
		return nil, ErrFeatureNotAssigned
	}
	key := registryKey{guildID: guildID, featureName: featureName}
	if bot, exists := state.routes[key]; exists {
		return bot, nil
	}
	return nil, ErrFeatureNotAssigned
}

func (r *InMemoryFeatureRegistry) UpdateRoute(guildID string, featureName string, bot *BotInstance) {
	key := registryKey{guildID: guildID, featureName: featureName}
	for {
		oldState := r.state.Load()
		var oldRoutes map[registryKey]*BotInstance
		if oldState != nil {
			oldRoutes = oldState.routes
		}

		newRoutes := make(map[registryKey]*BotInstance, len(oldRoutes)+1)
		for k, v := range oldRoutes {
			newRoutes[k] = v
		}
		newRoutes[key] = bot

		newState := &registryState{routes: newRoutes}
		if r.state.CompareAndSwap(oldState, newState) {
			return
		}
	}
}

func (r *InMemoryFeatureRegistry) RemoveRoute(guildID string, featureName string) {
	key := registryKey{guildID: guildID, featureName: featureName}
	for {
		oldState := r.state.Load()
		if oldState == nil || oldState.routes == nil {
			return
		}
		if _, exists := oldState.routes[key]; !exists {
			return
		}

		newRoutes := make(map[registryKey]*BotInstance, len(oldState.routes)-1)
		for k, v := range oldState.routes {
			if k != key {
				newRoutes[k] = v
			}
		}

		newState := &registryState{routes: newRoutes}
		if r.state.CompareAndSwap(oldState, newState) {
			return
		}
	}
}
