package config

import (
	"bytes"
	"encoding/json"
	"net/url"
	"strings"
)

const (
	maxEmbedPreviewCh = 900
	maxListEntries    = 20
)

func maskWebhookURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "—"
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "(invalid url)"
	}

	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	for i := 0; i < len(parts); i++ {
		if parts[i] != "webhooks" || i+2 >= len(parts) {
			continue
		}
		webhookID := strings.TrimSpace(parts[i+1])
		if webhookID == "" {
			break
		}
		prefix := "https://discord.com/api/webhooks/"
		return prefix + webhookID + "/***"
	}

	return "(unrecognized webhook url)"
}

func renderScopeLabel(scopeGuildID string) string {
	if strings.TrimSpace(scopeGuildID) == "" {
		return "global"
	}
	return "guild:" + scopeGuildID
}

func renderEmbedPreview(raw json.RawMessage) string {
	compacted := &bytes.Buffer{}
	if err := json.Compact(compacted, raw); err != nil {
		return "(invalid json)"
	}
	s := compacted.String()
	if len(s) > maxEmbedPreviewCh {
		s = s[:maxEmbedPreviewCh] + "..."
	}
	return "```json\n" + s + "\n```"
}
