package control

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
)

var errBotGuildIDsProviderUnavailable = errors.New("bot guild ids provider unavailable")

type botGuildBindingSource struct {
	mu               sync.RWMutex
	idsProvider      botGuildIDsProvider
	bindingsProvider botGuildBindingsProvider
}

func newBotGuildBindingSource() *botGuildBindingSource {
	return &botGuildBindingSource{}
}

func (src *botGuildBindingSource) SetIDsProvider(provider botGuildIDsProvider) {
	if src == nil || provider == nil {
		return
	}

	src.mu.Lock()
	defer src.mu.Unlock()

	src.idsProvider = provider
	src.bindingsProvider = func(ctx context.Context) ([]BotGuildBinding, error) {
		ids, err := provider(ctx)
		if err != nil {
			return nil, err
		}
		out := make([]BotGuildBinding, 0, len(ids))
		for _, guildID := range ids {
			out = append(out, BotGuildBinding{GuildID: guildID})
		}
		return out, nil
	}
}

func (src *botGuildBindingSource) SetBindingsProvider(provider botGuildBindingsProvider) {
	if src == nil || provider == nil {
		return
	}

	src.mu.Lock()
	defer src.mu.Unlock()

	src.bindingsProvider = provider
	src.idsProvider = func(ctx context.Context) ([]string, error) {
		bindings, err := provider(ctx)
		if err != nil {
			return nil, err
		}
		ids := make([]string, 0, len(bindings))
		seen := make(map[string]struct{}, len(bindings))
		for _, binding := range bindings {
			guildID := strings.TrimSpace(binding.GuildID)
			if guildID == "" {
				continue
			}
			if _, ok := seen[guildID]; ok {
				continue
			}
			seen[guildID] = struct{}{}
			ids = append(ids, guildID)
		}
		return ids, nil
	}
}

func (src *botGuildBindingSource) Bindings(ctx context.Context) ([]BotGuildBinding, error) {
	if src == nil {
		return nil, errBotGuildIDsProviderUnavailable
	}

	src.mu.RLock()
	provider := src.bindingsProvider
	src.mu.RUnlock()
	if provider == nil {
		return nil, errBotGuildIDsProviderUnavailable
	}

	bindings, err := provider(ctx)
	if err != nil {
		return nil, fmt.Errorf("resolve bot guild bindings: %w", err)
	}
	return bindings, nil
}

func (src *botGuildBindingSource) GuildIDSet(ctx context.Context) (map[string]struct{}, error) {
	bindings, err := src.Bindings(ctx)
	if err != nil {
		return nil, err
	}

	set := make(map[string]struct{}, len(bindings))
	for _, binding := range bindings {
		guildID := strings.TrimSpace(binding.GuildID)
		if guildID == "" {
			continue
		}
		set[guildID] = struct{}{}
	}
	return set, nil
}
