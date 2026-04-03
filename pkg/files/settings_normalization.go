package files

import (
	"fmt"
	"strings"

	"github.com/small-frappuccino/discordcore/pkg/persistence"
)

// NormalizeRuntimeConfig canonicalizes runtime config sections used by the
// control dashboard before they are persisted as part of broader config writes.
func NormalizeRuntimeConfig(in RuntimeConfig) (RuntimeConfig, error) {
	out := cloneRuntimeConfig(in)

	if normalizedDB, ok, err := normalizeRuntimeDatabaseConfig(out.Database); err != nil {
		return RuntimeConfig{}, fmt.Errorf("database: %w", err)
	} else if ok {
		out.Database = normalizedDB
	}
	if out.GlobalMaxWorkers < 0 {
		return RuntimeConfig{}, fmt.Errorf("global_max_workers must be >= 0")
	}

	updates := out.NormalizedWebhookEmbedUpdates()
	if len(updates) > 0 {
		normalized := make([]WebhookEmbedUpdateConfig, 0, len(updates))
		seen := make(map[string]struct{}, len(updates))
		for idx, item := range updates {
			next, err := normalizeWebhookEmbedUpdateConfig(item)
			if err != nil {
				return RuntimeConfig{}, fmt.Errorf("webhook_embed_updates[%d]: %w", idx, err)
			}
			if _, exists := seen[next.MessageID]; exists {
				return RuntimeConfig{}, fmt.Errorf("%w: message_id=%s", ErrWebhookEmbedUpdateAlreadyExists, next.MessageID)
			}
			seen[next.MessageID] = struct{}{}
			normalized = append(normalized, next)
		}
		out.WebhookEmbedUpdates = normalized
		out.WebhookEmbedUpdate = WebhookEmbedUpdateConfig{}
	} else if !out.WebhookEmbedUpdate.IsZero() {
		normalized, err := normalizeWebhookEmbedUpdateConfig(out.WebhookEmbedUpdate)
		if err != nil {
			return RuntimeConfig{}, fmt.Errorf("webhook_embed_update: %w", err)
		}
		out.WebhookEmbedUpdate = normalized
	}

	out.WebhookEmbedValidation = out.WebhookEmbedValidation.Normalized()

	return out, nil
}

// NormalizePartnerBoardConfig canonicalizes the partner board config so broad
// config writes share the same validation rules as the dedicated board service.
func NormalizePartnerBoardConfig(in PartnerBoardConfig) (PartnerBoardConfig, error) {
	target, err := normalizeEmbedUpdateTargetConfig(in.Target)
	if err != nil {
		return PartnerBoardConfig{}, fmt.Errorf("target: %w", err)
	}

	partners, err := canonicalizePartnerEntries(in.Partners)
	if err != nil {
		return PartnerBoardConfig{}, fmt.Errorf("partners: %w", err)
	}

	return PartnerBoardConfig{
		Target:   target,
		Template: normalizePartnerBoardTemplate(in.Template),
		Partners: clonePartnerEntries(partners),
	}, nil
}

// NormalizeQOTDConfig canonicalizes guild QOTD settings so dedicated routes and
// broad config writes can share the same validation behavior.
func NormalizeQOTDConfig(in QOTDConfig) (QOTDConfig, error) {
	out := QOTDConfig{
		Enabled:        in.Enabled,
		ForumChannelID: strings.TrimSpace(in.ForumChannelID),
		QuestionTagID:  strings.TrimSpace(in.QuestionTagID),
		ReplyTagID:     strings.TrimSpace(in.ReplyTagID),
	}

	var err error
	if out.StaffRoleIDs, err = normalizeQOTDStaffRoleIDs(in.StaffRoleIDs); err != nil {
		return QOTDConfig{}, err
	}

	if out.IsZero() {
		return QOTDConfig{}, nil
	}
	if out.ForumChannelID != "" && !isAllDigits(out.ForumChannelID) {
		return QOTDConfig{}, invalidQOTDInput("forum_channel_id must be numeric")
	}
	if out.QuestionTagID != "" && !isAllDigits(out.QuestionTagID) {
		return QOTDConfig{}, invalidQOTDInput("question_tag_id must be numeric")
	}
	if out.ReplyTagID != "" && !isAllDigits(out.ReplyTagID) {
		return QOTDConfig{}, invalidQOTDInput("reply_tag_id must be numeric")
	}
	if out.QuestionTagID != "" && out.QuestionTagID == out.ReplyTagID {
		return QOTDConfig{}, invalidQOTDInput("question_tag_id and reply_tag_id must differ")
	}
	if out.Enabled {
		if out.ForumChannelID == "" {
			return QOTDConfig{}, invalidQOTDInput("forum_channel_id is required when enabled")
		}
		if out.QuestionTagID == "" {
			return QOTDConfig{}, invalidQOTDInput("question_tag_id is required when enabled")
		}
		if out.ReplyTagID == "" {
			return QOTDConfig{}, invalidQOTDInput("reply_tag_id is required when enabled")
		}
	}

	return out, nil
}

func normalizeRuntimeDatabaseConfig(in DatabaseRuntimeConfig) (DatabaseRuntimeConfig, bool, error) {
	cfg := persistence.Config{
		Driver:              in.Driver,
		DatabaseURL:         in.DatabaseURL,
		MaxOpenConns:        in.MaxOpenConns,
		MaxIdleConns:        in.MaxIdleConns,
		ConnMaxLifetimeSecs: in.ConnMaxLifetimeSecs,
		ConnMaxIdleTimeSecs: in.ConnMaxIdleTimeSecs,
		PingTimeoutMS:       in.PingTimeoutMS,
	}

	if cfg == (persistence.Config{}) {
		return DatabaseRuntimeConfig{}, false, nil
	}

	normalized := cfg.Normalized()
	if err := normalized.Validate(); err != nil {
		return DatabaseRuntimeConfig{}, false, err
	}

	return DatabaseRuntimeConfig{
		Driver:              normalized.Driver,
		DatabaseURL:         normalized.DatabaseURL,
		MaxOpenConns:        normalized.MaxOpenConns,
		MaxIdleConns:        normalized.MaxIdleConns,
		ConnMaxLifetimeSecs: normalized.ConnMaxLifetimeSecs,
		ConnMaxIdleTimeSecs: normalized.ConnMaxIdleTimeSecs,
		PingTimeoutMS:       normalized.PingTimeoutMS,
	}, true, nil
}
