package control

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
)

type featureControlService struct {
	builder *featureWorkspaceBuilder
	applier *featureMutationApplier
}

func newFeatureControlService(
	builder *featureWorkspaceBuilder,
	applier *featureMutationApplier,
) *featureControlService {
	return &featureControlService{
		builder: builder,
		applier: applier,
	}
}

func (s *Server) featureControl() *featureControlService {
	if s == nil {
		return nil
	}
	return s.featureControlSvc
}

func (svc *featureControlService) catalog() []featureCatalogEntry {
	items := make([]featureCatalogEntry, 0, len(featureDefinitions))
	for _, def := range featureDefinitions {
		items = append(items, featureCatalogEntry{
			ID:                    def.ID,
			Category:              def.Category,
			Label:                 def.Label,
			Description:           def.Description,
			Area:                  def.Area,
			Tags:                  slices.Clone(def.Tags),
			SupportsGuildOverride: def.SupportsGuildOverride,
			GlobalEditableFields:  slices.Clone(def.GlobalEditableFields),
			GuildEditableFields:   slices.Clone(def.GuildEditableFields),
		})
	}
	return items
}

func (svc *featureControlService) workspace(guildID string) (featureWorkspace, error) {
	if svc == nil || svc.builder == nil {
		return featureWorkspace{}, fmt.Errorf("config manager unavailable")
	}
	return svc.builder.Workspace(guildID)
}

func (svc *featureControlService) feature(guildID, featureID string) (featureRecord, error) {
	if svc == nil || svc.builder == nil {
		return featureRecord{}, fmt.Errorf("config manager unavailable")
	}
	return svc.builder.Feature(guildID, featureID)
}

func (svc *featureControlService) patch(r *http.Request, guildID, featureID string) (featureRecord, error) {
	if svc == nil || svc.builder == nil || svc.applier == nil {
		return featureRecord{}, fmt.Errorf("config manager unavailable")
	}

	updated, err := svc.applier.ApplyPatch(r, guildID, featureID)
	if err != nil {
		return featureRecord{}, err
	}
	return svc.builder.FeatureFromConfig(updated, guildID, featureID)
}

func (s *Server) handleFeatureCatalogGet(w http.ResponseWriter) {
	svc := s.featureControl()
	if svc == nil {
		http.Error(w, "control server unavailable", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"catalog": svc.catalog(),
	})
}

func (s *Server) handleGlobalFeaturePatch(w http.ResponseWriter, r *http.Request, featureID string) {
	svc := s.featureControl()
	if svc == nil {
		http.Error(w, "control server unavailable", http.StatusInternalServerError)
		return
	}

	record, err := svc.patch(r, "", featureID)
	if err != nil {
		status := statusForFeatureMutationError(err)
		http.Error(w, fmt.Sprintf("failed to update feature: %v", err), status)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"feature": record,
	})
}

func (s *Server) handleGuildFeaturePatch(w http.ResponseWriter, r *http.Request, guildID, featureID string) {
	svc := s.featureControl()
	if svc == nil {
		http.Error(w, "control server unavailable", http.StatusInternalServerError)
		return
	}

	record, err := svc.patch(r, guildID, featureID)
	if err != nil {
		status := statusForFeatureMutationError(err)
		http.Error(w, fmt.Sprintf("failed to update guild feature: %v", err), status)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"guild_id": guildID,
		"feature":  record,
	})
}

func statusForFeatureMutationError(err error) int {
	if err == nil {
		return http.StatusInternalServerError
	}
	if errors.Is(err, errUnknownFeatureID) {
		return http.StatusNotFound
	}

	var badRequest featurePatchBadRequestError
	if errors.As(err, &badRequest) {
		return http.StatusBadRequest
	}

	return statusForSettingsMutationError(err)
}
