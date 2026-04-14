package qotd

import (
	"context"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	discordqotd "github.com/small-frappuccino/discordcore/pkg/discord/qotd"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/storage"
)

type SetupResult struct {
	Settings             files.QOTDConfig
	Summary              Summary
	DeckID               string
	ForumChannelID       string
	ForumChannelURL      string
	QuestionListThreadID string
	QuestionListPostURL  string
}

func (s *Service) SetupForum(ctx context.Context, guildID, deckID string, session *discordgo.Session) (*SetupResult, error) {
	if err := s.validate(); err != nil {
		return nil, err
	}
	if session == nil {
		return nil, ErrDiscordUnavailable
	}

	guildID = strings.TrimSpace(guildID)
	deckID = strings.TrimSpace(deckID)
	lifecycleLock := s.guildLifecycleLock(guildID)
	lifecycleLock.Lock()
	defer lifecycleLock.Unlock()

	current, err := s.configManager.GetQOTDConfig(guildID)
	if err != nil {
		return nil, err
	}
	currentDashboard := files.DashboardQOTDConfig(current)
	deck, err := resolveSetupDeck(currentDashboard, deckID)
	if err != nil {
		return nil, err
	}

	surface, err := s.store.GetQOTDForumSurfaceByDeck(ctx, guildID, deck.ID)
	if err != nil {
		return nil, err
	}
	setupResult, err := s.publisher.SetupForum(ctx, session, discordqotd.SetupForumParams{
		GuildID:                       guildID,
		PreferredForumChannelID:       strings.TrimSpace(deck.ForumChannelID),
		PreferredQuestionListThreadID: qotdForumSurfaceQuestionListThreadID(surface),
	})
	if err != nil {
		return nil, err
	}

	updatedSettings, err := s.updateSettingsLocked(
		guildID,
		applySetupForumSettings(currentDashboard, deck.ID, setupResult.ForumChannelID),
	)
	if err != nil {
		return nil, err
	}

	if _, err := s.store.UpsertQOTDForumSurface(ctx, storage.QOTDForumSurfaceRecord{
		GuildID:              guildID,
		DeckID:               deck.ID,
		ForumChannelID:       setupResult.ForumChannelID,
		QuestionListThreadID: setupResult.QuestionListThreadID,
	}); err != nil {
		return nil, fmt.Errorf("upsert qotd forum surface: %w", err)
	}

	summary, err := s.GetSummary(ctx, guildID)
	if err != nil {
		return nil, err
	}

	return &SetupResult{
		Settings:             updatedSettings,
		Summary:              summary,
		DeckID:               deck.ID,
		ForumChannelID:       setupResult.ForumChannelID,
		ForumChannelURL:      setupResult.ForumChannelURL,
		QuestionListThreadID: setupResult.QuestionListThreadID,
		QuestionListPostURL:  setupResult.QuestionListPostURL,
	}, nil
}

func (s *Service) updateSettingsLocked(guildID string, cfg files.QOTDConfig) (files.QOTDConfig, error) {
	normalized, err := files.NormalizeQOTDConfig(cfg)
	if err != nil {
		return files.QOTDConfig{}, err
	}
	current, err := s.configManager.GetQOTDConfig(guildID)
	if err != nil {
		return files.QOTDConfig{}, err
	}
	currentDashboard := files.DashboardQOTDConfig(current)
	nextDashboard := files.DashboardQOTDConfig(normalized)
	if err := s.configManager.SetQOTDConfig(guildID, normalized); err != nil {
		return files.QOTDConfig{}, err
	}
	if err := s.deleteRemovedDeckQuestions(context.Background(), guildID, currentDashboard, nextDashboard); err != nil {
		if rollbackErr := s.configManager.SetQOTDConfig(guildID, current); rollbackErr != nil {
			return files.QOTDConfig{}, fmt.Errorf("delete removed qotd deck questions: %w (rollback qotd config: %v)", err, rollbackErr)
		}
		return files.QOTDConfig{}, err
	}
	updated, err := s.configManager.GetQOTDConfig(guildID)
	if err != nil {
		return files.QOTDConfig{}, err
	}
	return files.DashboardQOTDConfig(updated), nil
}

func resolveSetupDeck(cfg files.QOTDConfig, deckID string) (files.QOTDDeckConfig, error) {
	if deckID != "" {
		deck, ok := cfg.DeckByID(deckID)
		if !ok {
			return files.QOTDDeckConfig{}, ErrDeckNotFound
		}
		return deck, nil
	}
	deck, ok := cfg.ActiveDeck()
	if !ok {
		return files.QOTDDeckConfig{}, ErrDeckNotFound
	}
	return deck, nil
}

func applySetupForumSettings(cfg files.QOTDConfig, deckID, forumChannelID string) files.QOTDConfig {
	next := files.DashboardQOTDConfig(cfg)
	next.ActiveDeckID = deckID
	next.Decks = append([]files.QOTDDeckConfig(nil), next.Decks...)
	for idx := range next.Decks {
		if strings.TrimSpace(next.Decks[idx].ID) != deckID {
			continue
		}
		next.Decks[idx].Enabled = true
		next.Decks[idx].ForumChannelID = strings.TrimSpace(forumChannelID)
		return next
	}
	next.Decks = append(next.Decks, files.QOTDDeckConfig{
		ID:             deckID,
		Name:           deckID,
		Enabled:        true,
		ForumChannelID: strings.TrimSpace(forumChannelID),
	})
	return next
}
