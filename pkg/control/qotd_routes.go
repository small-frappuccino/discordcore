package control

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/qotd"
	"github.com/small-frappuccino/discordcore/pkg/storage"
)

func (s *Server) handleGuildQOTDRoutes(w http.ResponseWriter, r *http.Request, guildID string, tail []string, auth requestAuthorization) {
	if !s.requireQOTDService(w) {
		return
	}

	switch {
	case len(tail) == 1:
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleQOTDSummaryGet(w, r, guildID)
		return
	case len(tail) == 2 && tail[1] == "settings":
		switch r.Method {
		case http.MethodGet:
			s.handleQOTDSettingsGet(w, r, guildID)
		case http.MethodPut:
			s.handleQOTDSettingsPut(w, r, guildID)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
		return
	case len(tail) == 2 && tail[1] == "collector":
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleQOTDCollectorGet(w, r, guildID)
		return
	case len(tail) == 2 && tail[1] == "questions":
		switch r.Method {
		case http.MethodGet:
			s.handleQOTDQuestionsGet(w, r, guildID)
		case http.MethodPost:
			s.handleQOTDQuestionsCreate(w, r, guildID, auth)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
		return
	case len(tail) == 3 && tail[1] == "questions" && tail[2] == "batch":
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleQOTDQuestionsCreateBatch(w, r, guildID, auth)
		return
	case len(tail) == 3 && tail[1] == "questions" && tail[2] == "reorder":
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleQOTDQuestionsReorder(w, r, guildID)
		return
	case len(tail) == 3 && tail[1] == "collector" && tail[2] == "collect":
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleQOTDCollectorCollectPost(w, r, guildID, auth)
		return
	case len(tail) == 3 && tail[1] == "collector" && tail[2] == "export":
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleQOTDCollectorExportGet(w, r, guildID)
		return
	case len(tail) == 3 && tail[1] == "collector" && tail[2] == "remove-duplicates":
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleQOTDCollectorRemoveDuplicatesPost(w, r, guildID, auth)
		return
	case len(tail) == 3 && tail[1] == "questions":
		questionID, err := strconv.ParseInt(strings.TrimSpace(tail[2]), 10, 64)
		if err != nil || questionID <= 0 {
			http.Error(w, "invalid question id", http.StatusBadRequest)
			return
		}
		switch r.Method {
		case http.MethodPut:
			s.handleQOTDQuestionsUpdate(w, r, guildID, questionID)
		case http.MethodDelete:
			s.handleQOTDQuestionsDelete(w, r, guildID, questionID)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
		return
	case len(tail) == 3 && tail[1] == "actions" && tail[2] == "publish-now":
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleQOTDPublishNowPost(w, r, guildID, auth)
		return
	case len(tail) == 3 && tail[1] == "actions" && tail[2] == "reconcile":
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleQOTDReconcilePost(w, r, guildID, auth)
		return
	default:
		http.NotFound(w, r)
		return
	}
}

func (s *Server) handleQOTDSummaryGet(w http.ResponseWriter, r *http.Request, guildID string) {
	summary, err := s.qotdService.GetSummary(r.Context(), guildID)
	if err != nil {
		status := qotdErrorStatus(err)
		http.Error(w, fmt.Sprintf("failed to read qotd summary: %v", err), status)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"guild_id": guildID,
		"summary":  buildQOTDSummaryResponse(guildID, summary),
	})
}

func (s *Server) handleQOTDSettingsGet(w http.ResponseWriter, _ *http.Request, guildID string) {
	settings, err := s.qotdService.Settings(guildID)
	if err != nil {
		status := qotdErrorStatus(err)
		http.Error(w, fmt.Sprintf("failed to read qotd settings: %v", err), status)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"guild_id": guildID,
		"settings": settings,
	})
}

func (s *Server) handleQOTDSettingsPut(w http.ResponseWriter, r *http.Request, guildID string) {
	var payload files.QOTDConfig
	if err := decodeJSONBody(w, r, &payload); err != nil {
		return
	}

	settings, err := s.qotdService.UpdateSettings(guildID, payload)
	if err != nil {
		status := qotdErrorStatus(err)
		http.Error(w, fmt.Sprintf("failed to update qotd settings: %v", err), status)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"guild_id": guildID,
		"settings": settings,
	})
}

func (s *Server) handleQOTDCollectorGet(w http.ResponseWriter, r *http.Request, guildID string) {
	summary, err := s.qotdService.GetCollectorSummary(r.Context(), guildID)
	if err != nil {
		status := qotdErrorStatus(err)
		http.Error(w, fmt.Sprintf("failed to read qotd collector summary: %v", err), status)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"guild_id": guildID,
		"summary":  buildQOTDCollectorSummaryResponse(summary),
	})
}

func (s *Server) handleQOTDQuestionsGet(w http.ResponseWriter, r *http.Request, guildID string) {
	questions, err := s.qotdService.ListQuestions(r.Context(), guildID, r.URL.Query().Get("deck_id"))
	if err != nil {
		status := qotdErrorStatus(err)
		http.Error(w, fmt.Sprintf("failed to list qotd questions: %v", err), status)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":    "ok",
		"guild_id":  guildID,
		"questions": buildQOTDQuestionsResponse(questions),
	})
}

func (s *Server) handleQOTDQuestionsCreate(w http.ResponseWriter, r *http.Request, guildID string, auth requestAuthorization) {
	var payload struct {
		DeckID string `json:"deck_id"`
		Body   string `json:"body"`
		Status string `json:"status"`
	}
	if err := decodeJSONBody(w, r, &payload); err != nil {
		return
	}

	question, err := s.qotdService.CreateQuestion(r.Context(), guildID, settingsRequestUserID(auth), qotd.QuestionMutation{
		DeckID: strings.TrimSpace(payload.DeckID),
		Body:   payload.Body,
		Status: qotd.QuestionStatus(strings.TrimSpace(payload.Status)),
	})
	if err != nil {
		status := qotdErrorStatus(err)
		http.Error(w, fmt.Sprintf("failed to create qotd question: %v", err), status)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"status":   "ok",
		"guild_id": guildID,
		"question": buildQOTDQuestionsResponse([]storage.QOTDQuestionRecord{*question})[0],
	})
}

func (s *Server) handleQOTDQuestionsCreateBatch(w http.ResponseWriter, r *http.Request, guildID string, auth requestAuthorization) {
	var payload struct {
		Questions []struct {
			DeckID string `json:"deck_id"`
			Body   string `json:"body"`
			Status string `json:"status"`
		} `json:"questions"`
	}
	if err := decodeJSONBody(w, r, &payload); err != nil {
		return
	}

	mutations := make([]qotd.QuestionMutation, 0, len(payload.Questions))
	for _, q := range payload.Questions {
		mutations = append(mutations, qotd.QuestionMutation{
			DeckID: strings.TrimSpace(q.DeckID),
			Body:   q.Body,
			Status: qotd.QuestionStatus(strings.TrimSpace(q.Status)),
		})
	}

	createdRecords, err := s.qotdService.CreateQuestionsBatch(r.Context(), guildID, settingsRequestUserID(auth), mutations)
	statusCode := http.StatusCreated

	var errMessage string
	if err != nil {
		statusCode = qotdErrorStatus(err)
		errMessage = err.Error()
	}

	response := map[string]any{
		"status":    "ok",
		"guild_id":  guildID,
		"questions": buildQOTDQuestionsResponse(createdRecords),
	}

	if err != nil {
		response["status"] = "error"
		response["error"] = errMessage
		// If at least one was created, we can return 207 Multi-Status, but let's just stick to what the UI expects for now or return a 400 with the questions.
		// Sending 207 is safer. Wait, qotdErrorStatus will return 400 or 500. We can just send that status, but still include the "questions".
	}

	writeJSON(w, statusCode, response)
}

func (s *Server) handleQOTDQuestionsUpdate(w http.ResponseWriter, r *http.Request, guildID string, questionID int64) {
	var payload struct {
		DeckID string `json:"deck_id"`
		Body   string `json:"body"`
		Status string `json:"status"`
	}
	if err := decodeJSONBody(w, r, &payload); err != nil {
		return
	}

	question, err := s.qotdService.UpdateQuestion(r.Context(), guildID, questionID, qotd.QuestionMutation{
		DeckID: strings.TrimSpace(payload.DeckID),
		Body:   payload.Body,
		Status: qotd.QuestionStatus(strings.TrimSpace(payload.Status)),
	})
	if err != nil {
		status := qotdErrorStatus(err)
		http.Error(w, fmt.Sprintf("failed to update qotd question: %v", err), status)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"guild_id": guildID,
		"question": buildQOTDQuestionsResponse([]storage.QOTDQuestionRecord{*question})[0],
	})
}

func (s *Server) handleQOTDQuestionsDelete(w http.ResponseWriter, r *http.Request, guildID string, questionID int64) {
	if err := s.qotdService.DeleteQuestion(r.Context(), guildID, questionID); err != nil {
		status := qotdErrorStatus(err)
		http.Error(w, fmt.Sprintf("failed to delete qotd question: %v", err), status)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":     "ok",
		"guild_id":   guildID,
		"deleted_id": questionID,
	})
}

func (s *Server) handleQOTDQuestionsReorder(w http.ResponseWriter, r *http.Request, guildID string) {
	var payload struct {
		DeckID     string  `json:"deck_id"`
		OrderedIDs []int64 `json:"ordered_ids"`
	}
	if err := decodeJSONBody(w, r, &payload); err != nil {
		return
	}

	questions, err := s.qotdService.ReorderQuestions(r.Context(), guildID, strings.TrimSpace(payload.DeckID), payload.OrderedIDs)
	if err != nil {
		status := qotdErrorStatus(err)
		http.Error(w, fmt.Sprintf("failed to reorder qotd questions: %v", err), status)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":    "ok",
		"guild_id":  guildID,
		"questions": buildQOTDQuestionsResponse(questions),
	})
}

func (s *Server) handleQOTDCollectorCollectPost(w http.ResponseWriter, r *http.Request, guildID string, auth requestAuthorization) {
	session, err := s.discordSessionForGuild(guildID)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to resolve discord session: %v", err), http.StatusServiceUnavailable)
		return
	}

	result, err := s.qotdService.CollectArchivedQuestions(r.Context(), guildID, session)
	if err != nil {
		status := qotdErrorStatus(err)
		log.ApplicationLogger().Warn(
			"QOTD collector scan failed",
			"operation", "control.qotd.collector.collect",
			"guildID", guildID,
			"userID", settingsRequestUserID(auth),
			"err", err,
		)
		http.Error(w, fmt.Sprintf("failed to collect qotd history: %v", err), status)
		return
	}

	summary, err := s.qotdService.GetCollectorSummary(r.Context(), guildID)
	if err != nil {
		status := qotdErrorStatus(err)
		http.Error(w, fmt.Sprintf("failed to read qotd collector summary: %v", err), status)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"guild_id": guildID,
		"result":   buildQOTDCollectorRunResultResponse(result),
		"summary":  buildQOTDCollectorSummaryResponse(summary),
	})
}

func (s *Server) handleQOTDCollectorExportGet(w http.ResponseWriter, r *http.Request, guildID string) {
	exported, err := s.qotdService.ExportCollectedQuestionsTXT(r.Context(), guildID)
	if err != nil {
		status := qotdErrorStatus(err)
		http.Error(w, fmt.Sprintf("failed to export qotd history: %v", err), status)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="qotd-collected-questions.txt"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(exported))
}

func (s *Server) handleQOTDCollectorRemoveDuplicatesPost(w http.ResponseWriter, r *http.Request, guildID string, auth requestAuthorization) {
	var payload struct {
		DeckID string `json:"deck_id"`
	}
	if err := decodeJSONBody(w, r, &payload); err != nil {
		return
	}

	result, err := s.qotdService.RemoveDeckDuplicatesFromCollector(r.Context(), guildID, strings.TrimSpace(payload.DeckID))
	if err != nil {
		status := qotdErrorStatus(err)
		log.ApplicationLogger().Warn(
			"QOTD collector duplicate removal failed",
			"operation", "control.qotd.collector.remove_duplicates",
			"guildID", guildID,
			"deckID", strings.TrimSpace(payload.DeckID),
			"userID", settingsRequestUserID(auth),
			"err", err,
		)
		http.Error(w, fmt.Sprintf("failed to remove qotd deck duplicates: %v", err), status)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"guild_id": guildID,
		"result":   buildQOTDCollectorRemoveDuplicatesResultResponse(result),
	})
}

func (s *Server) handleQOTDPublishNowPost(w http.ResponseWriter, r *http.Request, guildID string, auth requestAuthorization) {
	session, err := s.discordSessionForGuild(guildID)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to resolve discord session: %v", err), http.StatusServiceUnavailable)
		return
	}

	result, err := s.qotdService.PublishNow(r.Context(), guildID, session)
	if err != nil {
		status := qotdErrorStatus(err)
		log.ApplicationLogger().Warn(
			"QOTD manual publish failed",
			"operation", "control.qotd.publish_now",
			"guildID", guildID,
			"userID", settingsRequestUserID(auth),
			"err", err,
		)
		http.Error(w, fmt.Sprintf("failed to publish qotd: %v", err), status)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"guild_id": guildID,
		"result": map[string]any{
			"post_url":      result.PostURL,
			"question":      buildQOTDQuestionsResponse([]storage.QOTDQuestionRecord{result.Question})[0],
			"official_post": buildQOTDOfficialPostResponse(guildID, &result.OfficialPost),
		},
	})
}

func (s *Server) handleQOTDReconcilePost(w http.ResponseWriter, r *http.Request, guildID string, auth requestAuthorization) {
	session, err := s.discordSessionForGuild(guildID)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to resolve discord session: %v", err), http.StatusServiceUnavailable)
		return
	}

	if err := s.qotdService.ReconcileGuild(r.Context(), guildID, session); err != nil {
		status := qotdErrorStatus(err)
		log.ApplicationLogger().Warn(
			"QOTD reconcile failed",
			"operation", "control.qotd.reconcile",
			"guildID", guildID,
			"userID", settingsRequestUserID(auth),
			"err", err,
		)
		http.Error(w, fmt.Sprintf("failed to reconcile qotd: %v", err), status)
		return
	}

	summary, err := s.qotdService.GetSummary(r.Context(), guildID)
	if err != nil {
		status := qotdErrorStatus(err)
		http.Error(w, fmt.Sprintf("failed to read qotd summary: %v", err), status)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"guild_id": guildID,
		"summary":  buildQOTDSummaryResponse(guildID, summary),
	})
}

func qotdErrorStatus(err error) int {
	switch {
	case err == nil:
		return http.StatusInternalServerError
	case errors.Is(err, files.ErrGuildConfigNotFound),
		errors.Is(err, qotd.ErrQuestionNotFound),
		errors.Is(err, qotd.ErrDeckNotFound):
		return http.StatusNotFound
	case errors.Is(err, files.ErrInvalidQOTDInput),
		errors.Is(err, qotd.ErrImmutableQuestion):
		return http.StatusBadRequest
	case errors.Is(err, qotd.ErrQOTDDisabled),
		errors.Is(err, qotd.ErrAlreadyPublished),
		errors.Is(err, qotd.ErrNoQuestionsAvailable):
		return http.StatusConflict
	case errors.Is(err, qotd.ErrDiscordUnavailable):
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}
