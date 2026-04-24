package control

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/small-frappuccino/discordcore/pkg/discord/messageupdate"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/partners"
)

func (s *Server) handlePartnerBoardGet(w http.ResponseWriter, _ *http.Request, guildID string) {
	board, err := s.partnerBoardService.PartnerBoard(guildID)
	if err != nil {
		status := partnerBoardErrorStatus(err)
		http.Error(w, fmt.Sprintf("failed to read partner board: %v", err), status)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":        "ok",
		"guild_id":      guildID,
		"partner_board": board,
	})
}

func (s *Server) handlePartnerBoardTargetGet(w http.ResponseWriter, _ *http.Request, guildID string) {
	target, err := s.partnerBoardService.PartnerBoardTarget(guildID)
	if err != nil {
		status := partnerBoardErrorStatus(err)
		http.Error(w, fmt.Sprintf("failed to read partner board target: %v", err), status)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"guild_id": guildID,
		"target":   target,
	})
}

func (s *Server) handlePartnerBoardTargetPut(w http.ResponseWriter, r *http.Request, guildID string) {
	var payload files.EmbedUpdateTargetConfig
	if err := decodeJSONBody(w, r, &payload); err != nil {
		return
	}

	if err := s.partnerBoardService.SetPartnerBoardTarget(guildID, payload); err != nil {
		status := partnerBoardErrorStatus(err)
		http.Error(w, fmt.Sprintf("failed to set partner board target: %v", err), status)
		return
	}

	target, err := s.partnerBoardService.PartnerBoardTarget(guildID)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to read updated target: %v", err), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"guild_id": guildID,
		"target":   target,
	})
}

func (s *Server) handlePartnerBoardTemplateGet(w http.ResponseWriter, _ *http.Request, guildID string) {
	template, err := s.partnerBoardService.PartnerBoardTemplate(guildID)
	if err != nil {
		status := partnerBoardErrorStatus(err)
		http.Error(w, fmt.Sprintf("failed to read partner board template: %v", err), status)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"guild_id": guildID,
		"template": template,
	})
}

func (s *Server) handlePartnerBoardTemplatePut(w http.ResponseWriter, r *http.Request, guildID string) {
	var payload files.PartnerBoardTemplateConfig
	if err := decodeJSONBody(w, r, &payload); err != nil {
		return
	}

	if err := s.partnerBoardService.SetPartnerBoardTemplate(guildID, payload); err != nil {
		status := partnerBoardErrorStatus(err)
		http.Error(w, fmt.Sprintf("failed to set partner board template: %v", err), status)
		return
	}

	template, err := s.partnerBoardService.PartnerBoardTemplate(guildID)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to read updated template: %v", err), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"guild_id": guildID,
		"template": template,
	})
}

func (s *Server) handlePartnerBoardPartnersList(w http.ResponseWriter, _ *http.Request, guildID string) {
	partners, err := s.partnerBoardService.ListPartners(guildID)
	if err != nil {
		status := partnerBoardErrorStatus(err)
		http.Error(w, fmt.Sprintf("failed to list partners: %v", err), status)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"guild_id": guildID,
		"partners": partners,
	})
}

func (s *Server) handlePartnerBoardPartnersCreate(w http.ResponseWriter, r *http.Request, guildID string) {
	var payload files.PartnerEntryConfig
	if err := decodeJSONBody(w, r, &payload); err != nil {
		return
	}

	if err := s.partnerBoardService.CreatePartner(guildID, payload); err != nil {
		status := partnerBoardErrorStatus(err)
		if errors.Is(err, files.ErrPartnerAlreadyExists) {
			status = http.StatusConflict
		}
		http.Error(w, fmt.Sprintf("failed to create partner: %v", err), status)
		return
	}

	created, err := s.partnerBoardService.Partner(guildID, payload.Name)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to read created partner: %v", err), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"status":   "ok",
		"guild_id": guildID,
		"partner":  created,
	})
}

func (s *Server) handlePartnerBoardPartnersUpdate(w http.ResponseWriter, r *http.Request, guildID string) {
	var payload struct {
		CurrentName string                   `json:"current_name"`
		Partner     files.PartnerEntryConfig `json:"partner"`
	}
	if err := decodeJSONBody(w, r, &payload); err != nil {
		return
	}

	if strings.TrimSpace(payload.CurrentName) == "" {
		http.Error(w, "failed to update partner: current_name is required", http.StatusBadRequest)
		return
	}

	if err := s.partnerBoardService.UpdatePartner(guildID, payload.CurrentName, payload.Partner); err != nil {
		status := partnerBoardErrorStatus(err)
		if errors.Is(err, files.ErrPartnerNotFound) {
			status = http.StatusNotFound
		} else if errors.Is(err, files.ErrPartnerAlreadyExists) {
			status = http.StatusConflict
		}
		http.Error(w, fmt.Sprintf("failed to update partner: %v", err), status)
		return
	}

	updated, err := s.partnerBoardService.Partner(guildID, payload.Partner.Name)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to read updated partner: %v", err), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"guild_id": guildID,
		"partner":  updated,
	})
}

func (s *Server) handlePartnerBoardPartnersDelete(w http.ResponseWriter, r *http.Request, guildID string) {
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	if name == "" {
		http.Error(w, "failed to delete partner: name query parameter is required", http.StatusBadRequest)
		return
	}

	if err := s.partnerBoardService.DeletePartner(guildID, name); err != nil {
		status := partnerBoardErrorStatus(err)
		if errors.Is(err, files.ErrPartnerNotFound) {
			status = http.StatusNotFound
		}
		http.Error(w, fmt.Sprintf("failed to delete partner: %v", err), status)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"guild_id": guildID,
		"deleted":  strings.TrimSpace(name),
	})
}

func (s *Server) handlePartnerBoardSyncPost(w http.ResponseWriter, r *http.Request, guildID string) {
	if s.partnerBoardSyncer == nil {
		http.Error(w, "partner board sync unavailable", http.StatusServiceUnavailable)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), defaultSyncTimeout)
	defer cancel()

	if err := s.partnerBoardSyncer.SyncGuild(ctx, guildID); err != nil {
		status := partnerBoardErrorStatus(err)
		http.Error(w, fmt.Sprintf("failed to sync partner board: %v", err), status)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"guild_id": guildID,
		"synced":   true,
	})
}

func partnerBoardErrorStatus(err error) int {
	if err == nil {
		return http.StatusInternalServerError
	}

	switch {
	case errors.Is(err, files.ErrGuildConfigNotFound):
		return http.StatusNotFound
	case errors.Is(err, files.ErrInvalidPartnerBoardInput),
		errors.Is(err, messageupdate.ErrInvalidTarget),
		errors.Is(err, partners.ErrInvalidPartnerBoardEntry),
		errors.Is(err, partners.ErrInvalidPartnerBoardTemplate),
		errors.Is(err, partners.ErrPartnerBoardExceedsEmbedLimit):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}
