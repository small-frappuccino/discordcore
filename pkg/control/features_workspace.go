package control

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/small-frappuccino/discordcore/pkg/files"
)

func (s *Server) handleGlobalFeaturesList(w http.ResponseWriter) {
	svc := s.featureControl()
	if svc == nil {
		http.Error(w, "control server unavailable", http.StatusInternalServerError)
		return
	}

	workspace, err := svc.workspace("")
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to build global feature workspace: %v", err), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":    "ok",
		"workspace": workspace,
	})
}

func (s *Server) handleGlobalFeatureGet(w http.ResponseWriter, featureID string) {
	svc := s.featureControl()
	if svc == nil {
		http.Error(w, "control server unavailable", http.StatusInternalServerError)
		return
	}

	record, err := svc.feature("", featureID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, errUnknownFeatureID) {
			status = http.StatusNotFound
		}
		http.Error(w, fmt.Sprintf("failed to read feature: %v", err), status)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"feature": record,
	})
}

func (s *Server) handleGuildFeaturesList(w http.ResponseWriter, guildID string) {
	svc := s.featureControl()
	if svc == nil {
		http.Error(w, "control server unavailable", http.StatusInternalServerError)
		return
	}

	workspace, err := svc.workspace(guildID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, files.ErrGuildConfigNotFound) {
			status = http.StatusNotFound
		}
		http.Error(w, fmt.Sprintf("failed to build guild feature workspace: %v", err), status)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":    "ok",
		"workspace": workspace,
	})
}

func (s *Server) handleGuildFeatureGet(w http.ResponseWriter, guildID, featureID string) {
	svc := s.featureControl()
	if svc == nil {
		http.Error(w, "control server unavailable", http.StatusInternalServerError)
		return
	}

	record, err := svc.feature(guildID, featureID)
	if err != nil {
		status := http.StatusInternalServerError
		switch {
		case errors.Is(err, errUnknownFeatureID):
			status = http.StatusNotFound
		case errors.Is(err, files.ErrGuildConfigNotFound):
			status = http.StatusNotFound
		}
		http.Error(w, fmt.Sprintf("failed to read guild feature: %v", err), status)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"guild_id": guildID,
		"feature":  record,
	})
}
