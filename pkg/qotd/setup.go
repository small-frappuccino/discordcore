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
	Settings   files.QOTDConfig
	Summary    Summary
	DeckID     string
	ChannelID  string
	ChannelURL string
}

func (s *Service) SetupChannel(ctx context.Context, guildID, deckID string, session *discordgo.Session) (*SetupResult, error) {
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

	setupResult, err := s.publisher.SetupChannel(ctx, session, discordqotd.SetupChannelParams{
		GuildID:            guildID,
		PreferredChannelID: strings.TrimSpace(deck.ChannelID),
		VerifiedRoleID:     strings.TrimSpace(currentDashboard.VerifiedRoleID),
	})
	if err != nil {
		return nil, err
	}

	updatedSettings, err := s.updateSettingsLocked(
		guildID,
		applySetupChannelSettings(currentDashboard, deck.ID, setupResult.ChannelID),
	)
	if err != nil {
		return nil, err
	}

	if _, err := s.store.UpsertQOTDSurface(ctx, storage.QOTDSurfaceRecord{
		GuildID:   guildID,
		DeckID:    deck.ID,
		ChannelID: setupResult.ChannelID,
	}); err != nil {
		return nil, fmt.Errorf("upsert qotd surface: %w", err)
	}

	summary, err := s.GetSummary(ctx, guildID)
	if err != nil {
		return nil, err
	}

	return &SetupResult{
		Settings:   updatedSettings,
		Summary:    summary,
		DeckID:     deck.ID,
		ChannelID:  setupResult.ChannelID,
		ChannelURL: setupResult.ChannelURL,
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

func applySetupChannelSettings(cfg files.QOTDConfig, deckID, channelID string) files.QOTDConfig {
	next := files.DashboardQOTDConfig(cfg)
	next.ActiveDeckID = deckID
	next.Decks = append([]files.QOTDDeckConfig(nil), next.Decks...)
	for idx := range next.Decks {
		if strings.TrimSpace(next.Decks[idx].ID) != deckID {
			continue
		}
		next.Decks[idx].Enabled = true
		next.Decks[idx].ChannelID = strings.TrimSpace(channelID)
		return next
	}
	next.Decks = append(next.Decks, files.QOTDDeckConfig{
		ID:        deckID,
		Name:      deckID,
		Enabled:   true,
		ChannelID: strings.TrimSpace(channelID),
	})
	return next
}
