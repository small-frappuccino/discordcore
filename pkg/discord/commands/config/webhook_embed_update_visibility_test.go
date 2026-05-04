package config

import (
	"errors"
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
)

func TestWebhookEmbedVisibilityPolicy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		class         webhookEmbedVisibilityClass
		wantEphemeral bool
	}{
		{name: "read stays private", class: webhookEmbedVisibilityRead, wantEphemeral: true},
		{name: "list stays private", class: webhookEmbedVisibilityList, wantEphemeral: true},
		{name: "preview stays private", class: webhookEmbedVisibilityPreview, wantEphemeral: true},
		{name: "payload stays private", class: webhookEmbedVisibilityRenderedPayload, wantEphemeral: true},
		{name: "detailed error stays private", class: webhookEmbedVisibilityDetailedError, wantEphemeral: true},
		{name: "short confirmation may be public", class: webhookEmbedVisibilityShortConfirmation, wantEphemeral: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := webhookEmbedVisibilityIsEphemeral(tt.class); got != tt.wantEphemeral {
				t.Fatalf("webhookEmbedVisibilityIsEphemeral(%q) = %v, want %v", tt.class, got, tt.wantEphemeral)
			}
		})
	}
}

func TestWebhookEmbedDetailedCommandErrorUsesPrivatePolicy(t *testing.T) {
	t.Parallel()

	err := webhookEmbedDetailedCommandError("boom")
	var cmdErr *core.CommandError
	if !errors.As(err, &cmdErr) {
		t.Fatalf("expected command error, got %T", err)
	}
	if !cmdErr.Ephemeral {
		t.Fatalf("expected detailed webhook error to be ephemeral")
	}
}