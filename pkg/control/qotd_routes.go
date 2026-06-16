package control

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/qotd"
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

	writeJSON(w, s.log(), http.StatusOK, QOTDSummaryResponse{
		Status:  "ok",
		GuildID: guildID,
		Summary: buildQOTDSummaryResponse(guildID, summary),
	})
}

func (s *Server) handleQOTDSettingsGet(w http.ResponseWriter, _ *http.Request, guildID string) {
	settings, err := s.qotdService.Settings(guildID)
	if err != nil {
		status := qotdErrorStatus(err)
		http.Error(w, fmt.Sprintf("failed to read qotd settings: %v", err), status)
		return
	}

	writeJSON(w, s.log(), http.StatusOK, QOTDSettingsResponse{
		Status:   "ok",
		GuildID:  guildID,
		Settings: settings,
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

	writeJSON(w, s.log(), http.StatusOK, QOTDSettingsResponse{
		Status:   "ok",
		GuildID:  guildID,
		Settings: settings,
	})
}

func (s *Server) handleQOTDQuestionsGet(w http.ResponseWriter, r *http.Request, guildID string) {
	questions, err := s.qotdService.ListQuestions(r.Context(), guildID, r.URL.Query().Get("deck_id"))
	if err != nil {
		status := qotdErrorStatus(err)
		http.Error(w, fmt.Sprintf("failed to list qotd questions: %v", err), status)
		return
	}

	writeJSON(w, s.log(), http.StatusOK, QOTDQuestionsResponse{
		Status:    "ok",
		GuildID:   guildID,
		Questions: buildQOTDQuestionsResponse(questions),
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

	writeJSON(w, s.log(), http.StatusCreated, QOTDQuestionResponse{
		Status:   "ok",
		GuildID:  guildID,
		Question: buildQOTDQuestionsResponse([]qotd.QuestionRecord{*question})[0],
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

	response := QOTDQuestionsBatchResponse{
		Status:    "ok",
		GuildID:   guildID,
		Questions: buildQOTDQuestionsResponse(createdRecords),
	}

	if err != nil {
		response.Status = "error"
		response.Error = errMessage
		// If at least one was created, we can return 207 Multi-Status, but let's just stick to what the UI expects for now or return a 400 with the questions.
		// Sending 207 is safer. Wait, qotdErrorStatus will return 400 or 500. We can just send that status, but still include the "questions".
	}

	writeJSON(w, s.log(), statusCode, response)
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

	writeJSON(w, s.log(), http.StatusOK, QOTDQuestionResponse{
		Status:   "ok",
		GuildID:  guildID,
		Question: buildQOTDQuestionsResponse([]qotd.QuestionRecord{*question})[0],
	})
}

func (s *Server) handleQOTDQuestionsDelete(w http.ResponseWriter, r *http.Request, guildID string, questionID int64) {
	if err := s.qotdService.DeleteQuestion(r.Context(), guildID, questionID); err != nil {
		status := qotdErrorStatus(err)
		http.Error(w, fmt.Sprintf("failed to delete qotd question: %v", err), status)
		return
	}

	writeJSON(w, s.log(), http.StatusOK, QOTDDeleteQuestionResponse{
		Status:    "ok",
		GuildID:   guildID,
		DeletedID: questionID,
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

	writeJSON(w, s.log(), http.StatusOK, QOTDQuestionsResponse{
		Status:    "ok",
		GuildID:   guildID,
		Questions: buildQOTDQuestionsResponse(questions),
	})
}

func (s *Server) handleQOTDPublishNowPost(w http.ResponseWriter, r *http.Request, guildID string, auth requestAuthorization) {
	type publishNowRequest struct {
		ConsumeAutomaticSlot *bool `json:"consume_automatic_slot,omitempty"`
	}
	payload := publishNowRequest{}
	if r.Body != nil && r.ContentLength != 0 {
		r.Body = http.MaxBytesReader(w, r.Body, defaultMaxBodyBytes)
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && !errors.Is(err, io.EOF) {
			http.Error(w, fmt.Sprintf("invalid payload: %v", err), http.StatusBadRequest)
			return
		}
	}

	result, err := s.qotdService.PublishNowWithParams(r.Context(), guildID, qotd.PublishNowParams{
		ConsumeAutomaticSlot: payload.ConsumeAutomaticSlot,
	})
	if err != nil {
		status := qotdErrorStatus(err)
		s.log().Warn(
			"QOTD manual publish failed",
			"operation", "control.qotd.publish_now",
			"guildID", guildID,
			"consumeAutomaticSlot", payload.ConsumeAutomaticSlot == nil || *payload.ConsumeAutomaticSlot,
			"userID", settingsRequestUserID(auth),
			"err", err,
		)
		http.Error(w, fmt.Sprintf("failed to publish qotd: %v", err), status)
		return
	}

	writeJSON(w, s.log(), http.StatusOK, QOTDPublishNowResponse{
		Status:  "ok",
		GuildID: guildID,
		Result: QOTDPublishResult{
			PostURL:      result.PostURL,
			Question:     buildQOTDQuestionsResponse([]qotd.QuestionRecord{result.Question})[0],
			OfficialPost: buildQOTDOfficialPostResponse(guildID, &result.OfficialPost),
		},
	})
}

func (s *Server) handleQOTDReconcilePost(w http.ResponseWriter, r *http.Request, guildID string, auth requestAuthorization) {

	if err := s.qotdService.ReconcileGuild(r.Context(), guildID); err != nil {
		status := qotdErrorStatus(err)
		s.log().Warn(
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

	writeJSON(w, s.log(), http.StatusOK, QOTDSummaryResponse{
		Status:  "ok",
		GuildID: guildID,
		Summary: buildQOTDSummaryResponse(guildID, summary),
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
