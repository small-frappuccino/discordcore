package qotd

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/small-frappuccino/discordcore/pkg/files"
)

func (s *Service) Settings(guildID string) (files.QOTDConfig, error) {
	if err := s.validate(); err != nil {
		return files.QOTDConfig{}, fmt.Errorf("Service.Settings: %w", err)
	}
	settings, err := s.configManager.QOTDConfig(guildID)
	if err != nil {
		return files.QOTDConfig{}, fmt.Errorf("Service.Settings: %w", err)
	}
	return files.DashboardQOTDConfig(settings), nil
}

func (s *Service) GetSettings(guildID string) (files.QOTDConfig, error) {
	return s.Settings(guildID)
}

func (s *Service) UpdateSettings(guildID string, cfg files.QOTDConfig) (files.QOTDConfig, error) {
	if err := s.validate(); err != nil {
		return files.QOTDConfig{}, fmt.Errorf("Service.UpdateSettings: %w", err)
	}
	guildID = strings.TrimSpace(guildID)
	lifecycleLock := s.guildLifecycleLock(guildID)
	lifecycleLock.Lock()
	defer lifecycleLock.Unlock()
	return s.updateSettingsLocked(guildID, cfg)
}

func (s *Service) resolveDashboardDeck(guildID, deckID string) (files.QOTDDeckConfig, error) {
	settings, err := s.Settings(guildID)
	if err != nil {
		return files.QOTDDeckConfig{}, fmt.Errorf("Service.resolveDashboardDeck: %w", err)
	}
	deckID = strings.TrimSpace(deckID)
	if deckID != "" {
		deck, ok := settings.DeckByID(deckID)
		if !ok {
			return files.QOTDDeckConfig{}, ErrDeckNotFound
		}
		return deck, nil
	}
	deck, ok := settings.ActiveDeck()
	if !ok {
		return files.QOTDDeckConfig{}, ErrDeckNotFound
	}
	return deck, nil
}

func (s *Service) deleteRemovedDeckQuestions(ctx context.Context, guildID string, current, next files.QOTDConfig) error {
	removedDeckIDs := missingDeckIDs(current.Decks, next.Decks)
	if len(removedDeckIDs) == 0 {
		return nil
	}
	if err := s.store.DeleteQOTDQuestionsByDecks(ctx, guildID, removedDeckIDs); err != nil {
		return fmt.Errorf("delete removed qotd deck questions: %w", err)
	}
	return nil
}

func missingDeckIDs(current, next []files.QOTDDeckConfig) []string {
	nextIDs := make(map[string]struct{}, len(next))
	for _, deck := range next {
		nextIDs[strings.TrimSpace(deck.ID)] = struct{}{}
	}
	removed := make([]string, 0)
	for _, deck := range current {
		deckID := strings.TrimSpace(deck.ID)
		if deckID == "" {
			continue
		}
		if _, ok := nextIDs[deckID]; ok {
			continue
		}
		removed = append(removed, deckID)
	}
	return removed
}

func (s *Service) guildLifecycleLock(guildID string) *sync.Mutex {
	key := strings.TrimSpace(guildID)
	lock, _ := s.guildLifecycleLocks.LoadOrStore(key, &sync.Mutex{})
	return lock.(*sync.Mutex)
}
