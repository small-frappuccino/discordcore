package core

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
)

var (
	ErrFeatureNotAssigned = errors.New("feature não está atribuída a nenhum bot nesta guilda")
	ErrGuildCapReached    = errors.New("limite de 5 bots distintos por guilda atingido")
	ErrRouteTheft         = errors.New("feature já está atribuída a outro bot. Remova-a primeiro")
	ErrInvalidFeature     = errors.New("feature inválida")
)

type GuildFeatures [NumFeatures]BotInstance

type GuildNode struct {
	features atomic.Pointer[GuildFeatures]
	buffer1  GuildFeatures
	buffer2  GuildFeatures
	mu       sync.Mutex // Single-writer enforcement for this specific guild
}

func newGuildNode() *GuildNode {
	n := &GuildNode{}
	n.features.Store(&n.buffer1)
	return n
}

type registryState struct {
	routes map[string]*GuildNode
}

type InMemoryFeatureRegistry struct {
	state atomic.Pointer[registryState]
	mu    sync.Mutex // Lock for adding new guilds to the map
}

func NewInMemoryFeatureRegistry() *InMemoryFeatureRegistry {
	r := &InMemoryFeatureRegistry{}
	r.state.Store(&registryState{
		routes: make(map[string]*GuildNode),
	})
	return r
}

func (r *InMemoryFeatureRegistry) ResolveOwner(ctx context.Context, guildID string, feature Feature) (BotInstance, error) {
	if feature >= NumFeatures {
		return BotInstance{}, ErrInvalidFeature
	}
	state := r.state.Load()
	if state == nil {
		return BotInstance{}, ErrFeatureNotAssigned
	}
	node, exists := state.routes[guildID]
	if !exists {
		return BotInstance{}, ErrFeatureNotAssigned
	}

	feats := node.features.Load()
	if bot := feats[feature]; bot.ApplicationID != "" {
		return bot, nil
	}

	return BotInstance{}, ErrFeatureNotAssigned
}

func (r *InMemoryFeatureRegistry) getOrAddNode(guildID string) *GuildNode {
	// Fast path
	state := r.state.Load()
	if node, exists := state.routes[guildID]; exists {
		return node
	}

	// Slow path
	r.mu.Lock()
	defer r.mu.Unlock()
	state = r.state.Load()
	if node, exists := state.routes[guildID]; exists {
		return node
	}

	newRoutes := make(map[string]*GuildNode, len(state.routes)+1)
	for k, v := range state.routes {
		newRoutes[k] = v
	}
	node := newGuildNode()
	newRoutes[guildID] = node
	r.state.Store(&registryState{routes: newRoutes})
	return node
}

// UpdateRoute binds a feature to a bot. It enforces the 5-bot cap and prevents route theft.
func (r *InMemoryFeatureRegistry) UpdateRoute(guildID string, feature Feature, bot BotInstance) error {
	if feature >= NumFeatures {
		return ErrInvalidFeature
	}

	node := r.getOrAddNode(guildID)

	node.mu.Lock()
	defer node.mu.Unlock()

	oldFeats := node.features.Load()

	var distinctBots [5]string
	var numDistinct int
	for i := 0; i < int(NumFeatures); i++ {
		if b := oldFeats[i]; b.ApplicationID != "" {
			found := false
			for j := 0; j < numDistinct; j++ {
				if distinctBots[j] == b.ApplicationID {
					found = true
					break
				}
			}
			if !found && numDistinct < 5 {
				distinctBots[numDistinct] = b.ApplicationID
				numDistinct++
			}
		}
	}

	if existing := oldFeats[feature]; existing.ApplicationID != "" && existing.ApplicationID != bot.ApplicationID {
		return ErrRouteTheft
	}

	found := false
	for j := 0; j < numDistinct; j++ {
		if distinctBots[j] == bot.ApplicationID {
			found = true
			break
		}
	}
	if !found && numDistinct >= 5 {
		return ErrGuildCapReached
	}

	var newFeats *GuildFeatures
	if oldFeats == &node.buffer1 {
		newFeats = &node.buffer2
	} else {
		newFeats = &node.buffer1
	}

	*newFeats = *oldFeats // copy stack array
	newFeats[feature] = bot
	node.features.Store(newFeats)

	return nil
}

func (r *InMemoryFeatureRegistry) RemoveRoute(guildID string, feature Feature) error {
	if feature >= NumFeatures {
		return ErrInvalidFeature
	}

	state := r.state.Load()
	node, exists := state.routes[guildID]
	if !exists {
		return nil
	}

	node.mu.Lock()
	defer node.mu.Unlock()

	oldFeats := node.features.Load()
	if oldFeats[feature].ApplicationID == "" {
		return nil
	}

	var newFeats *GuildFeatures
	if oldFeats == &node.buffer1 {
		newFeats = &node.buffer2
	} else {
		newFeats = &node.buffer1
	}

	*newFeats = *oldFeats
	newFeats[feature] = BotInstance{}
	node.features.Store(newFeats)

	return nil
}
