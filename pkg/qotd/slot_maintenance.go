package qotd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/storage"
)

func (s *Service) ReanimateSlot(ctx context.Context, guildID string, session *discordgo.Session, params SlotMaintenanceParams) (SlotMaintenanceResult, error) {
	if err := s.validate(); err != nil {
		return SlotMaintenanceResult{}, err
	}

	guildID = strings.TrimSpace(guildID)
	lifecycleLock := s.guildLifecycleLock(guildID)
	lifecycleLock.Lock()
	defer lifecycleLock.Unlock()

	now := s.clock()
	cfg, err := s.configManager.QOTDConfig(guildID)
	if err != nil {
		return SlotMaintenanceResult{}, err
	}
	publishDateUTC, err := s.resolveSlotMaintenanceDateUTC(ctx, guildID, cfg, now, params)
	if err != nil {
		return SlotMaintenanceResult{}, err
	}

	post, err := s.loadSlotOfficialPost(ctx, guildID, publishDateUTC)
	if err != nil {
		return SlotMaintenanceResult{}, err
	}
	if post == nil {
		return SlotMaintenanceResult{}, ErrOfficialPostNotFound
	}

	state := OfficialPostState(strings.TrimSpace(post.State))
	if state != OfficialPostStateAbandoned && state != OfficialPostStateFailed {
		return SlotMaintenanceResult{}, fmt.Errorf("%w: expected abandoned or failed, got %q", ErrOfficialPostState, strings.TrimSpace(post.State))
	}

	if err := deleteOfficialPostDiscordArtifacts(session, *post); err != nil {
		return SlotMaintenanceResult{}, err
	}
	if err := s.store.DeleteQOTDOfficialPostByID(ctx, post.ID); err != nil {
		return SlotMaintenanceResult{}, err
	}

	result := SlotMaintenanceResult{
		PublishDateUTC:       publishDateUTC,
		OfficialPostsCleared: 1,
	}
	if released, err := s.releaseOfficialPostQuestion(ctx, *post); err != nil {
		return result, err
	} else if released {
		result.QuestionsReleased = 1
	}

	result.ClearedSuppression = cfg.SuppressesScheduledPublishDate(publishDateUTC)
	s.clearScheduledPublishSuppressionForDate(guildID, publishDateUTC)
	return result, nil
}

func (s *Service) ClearPublishedDayState(ctx context.Context, guildID string, session *discordgo.Session, params SlotMaintenanceParams) (SlotMaintenanceResult, error) {
	if err := s.validate(); err != nil {
		return SlotMaintenanceResult{}, err
	}

	guildID = strings.TrimSpace(guildID)
	lifecycleLock := s.guildLifecycleLock(guildID)
	lifecycleLock.Lock()
	defer lifecycleLock.Unlock()

	now := s.clock()
	cfg, err := s.configManager.QOTDConfig(guildID)
	if err != nil {
		return SlotMaintenanceResult{}, err
	}
	publishDateUTC, err := s.resolveSlotMaintenanceDateUTC(ctx, guildID, cfg, now, params)
	if err != nil {
		return SlotMaintenanceResult{}, err
	}

	posts, err := s.store.ListQOTDOfficialPostsByDate(ctx, guildID, publishDateUTC)
	if err != nil {
		return SlotMaintenanceResult{}, err
	}

	result := SlotMaintenanceResult{PublishDateUTC: publishDateUTC}
	if len(posts) == 0 {
		result.ClearedSuppression = cfg.SuppressesScheduledPublishDate(publishDateUTC)
		s.clearScheduledPublishSuppressionForDate(guildID, publishDateUTC)
		return result, nil
	}

	releasedQuestionIDs := make(map[int64]struct{}, len(posts))
	for _, post := range posts {
		if err := deleteOfficialPostDiscordArtifacts(session, post); err != nil {
			return result, err
		}
		if err := s.store.DeleteQOTDOfficialPostByID(ctx, post.ID); err != nil {
			return result, err
		}
		result.OfficialPostsCleared++
		if _, seen := releasedQuestionIDs[post.QuestionID]; seen {
			continue
		}
		released, err := s.releaseOfficialPostQuestion(ctx, post)
		if err != nil {
			return result, err
		}
		if released {
			releasedQuestionIDs[post.QuestionID] = struct{}{}
			result.QuestionsReleased++
		}
	}

	result.ClearedSuppression = cfg.SuppressesScheduledPublishDate(publishDateUTC)
	s.clearScheduledPublishSuppressionForDate(guildID, publishDateUTC)
	return result, nil
}

func (s *Service) resolveSlotMaintenanceDateUTC(ctx context.Context, guildID string, cfg files.QOTDConfig, now time.Time, params SlotMaintenanceParams) (time.Time, error) {
	if params.DateUTC != nil {
		date := NormalizePublishDateUTC(*params.DateUTC)
		if date.IsZero() {
			return time.Time{}, fmt.Errorf("slot maintenance date is required")
		}
		return date, nil
	}

	slotState, err := s.loadDueSlotState(ctx, guildID, cfg, now)
	if err != nil {
		return time.Time{}, err
	}
	if !slotState.ScheduleConfigured || slotState.PublishDateUTC.IsZero() {
		return time.Time{}, ErrQOTDDisabled
	}
	return slotState.PublishDateUTC, nil
}

func (s *Service) releaseOfficialPostQuestion(ctx context.Context, post storage.QOTDOfficialPostRecord) (bool, error) {
	if post.QuestionID <= 0 {
		return false, nil
	}
	question, err := s.store.GetQOTDQuestion(ctx, post.GuildID, post.QuestionID)
	if err != nil {
		return false, err
	}
	if question == nil {
		return false, nil
	}
	if err := s.releaseReservedQuestion(ctx, *question); err != nil {
		return false, err
	}
	return true, nil
}

func deleteOfficialPostDiscordArtifacts(session *discordgo.Session, post storage.QOTDOfficialPostRecord) error {
	if session == nil {
		return nil
	}

	threadID := strings.TrimSpace(post.DiscordThreadID)
	starterMessageID := strings.TrimSpace(post.DiscordStarterMessageID)
	if starterMessageID != "" {
		channelID := threadID
		if channelID == "" {
			channelID = strings.TrimSpace(post.ChannelID)
		}
		if channelID != "" {
			if err := session.ChannelMessageDelete(channelID, starterMessageID); err != nil && !isIgnorableDiscordDeleteError(err) {
				return fmt.Errorf("delete qotd starter message: %w", err)
			}
		}
	}
	if threadID != "" {
		if _, err := session.ChannelDelete(threadID); err != nil && !isIgnorableDiscordDeleteError(err) {
			return fmt.Errorf("delete qotd thread: %w", err)
		}
	}
	return nil
}

func isIgnorableDiscordDeleteError(err error) bool {
	var restErr *discordgo.RESTError
	if !errors.As(err, &restErr) || restErr == nil {
		return false
	}
	if restErr.Response != nil {
		switch restErr.Response.StatusCode {
		case http.StatusNotFound, http.StatusForbidden, http.StatusUnauthorized:
			return true
		}
	}
	if restErr.Message != nil {
		switch restErr.Message.Code {
		case discordgo.ErrCodeUnknownChannel,
			discordgo.ErrCodeUnknownGuild,
			discordgo.ErrCodeUnknownMessage,
			discordgo.ErrCodeMissingAccess,
			discordgo.ErrCodeMissingPermissions,
			discordgo.ErrCodeUnauthorized:
			return true
		}
	}
	return false
}
