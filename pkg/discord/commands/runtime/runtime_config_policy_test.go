package runtime

import (
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

func TestRuntimeVisibilityPolicy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		class         runtimeVisibilityClass
		wantEphemeral bool
	}{
		{name: "admin panel stays private", class: runtimeVisibilityAdministrativePanel, wantEphemeral: true},
		{name: "read stays private", class: runtimeVisibilityRead, wantEphemeral: true},
		{name: "list stays private", class: runtimeVisibilityList, wantEphemeral: true},
		{name: "preview stays private", class: runtimeVisibilityPreview, wantEphemeral: true},
		{name: "rendered payload stays private", class: runtimeVisibilityRenderedPayload, wantEphemeral: true},
		{name: "detailed error stays private", class: runtimeVisibilityDetailedError, wantEphemeral: true},
		{name: "short confirmation may be public", class: runtimeVisibilityShortConfirmation, wantEphemeral: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := runtimeVisibilityIsEphemeral(tt.class); got != tt.wantEphemeral {
				t.Fatalf("runtimeVisibilityIsEphemeral(%q) = %v, want %v", tt.class, got, tt.wantEphemeral)
			}
		})
	}
}

func TestRegisterCommands_RuntimeComponentRejectsExpiredPanel(t *testing.T) {
	session, rec := newRuntimePanelTestSession(t, 0, 0)
	cm := files.NewMemoryConfigManager()
	if err := cm.LoadConfig(); err != nil {
		t.Fatalf("failed to load config manager: %v", err)
	}

	router := core.NewCommandRouter(session, cm)
	NewRuntimeConfigCommands(cm).RegisterCommands(router)

	interaction := newRuntimeComponentInteraction(cidButtonMain + stateSep + panelState{
		Mode:  pageMain,
		Group: "ALL",
		Key:   runtimeKeyBotTheme,
		Scope: "global",
	}.encode())
	interaction.Message.InteractionMetadata = nil
	interaction.Message.Interaction = nil

	router.HandleInteraction(session, interaction)

	if rec.callbackCount() != 1 {
		t.Fatalf("expected one deferred component ack, got %d", rec.callbackCount())
	}
	if rec.webhookPatchCount() != 0 {
		t.Fatalf("expected expired component to avoid editing the panel, got %d patches", rec.webhookPatchCount())
	}
	if rec.followupCount() != 1 {
		t.Fatalf("expected one ephemeral follow-up denial, got %d", rec.followupCount())
	}
	if !strings.Contains(rec.followupBody(), runtimeConfigInteractionExpiredText) {
		t.Fatalf("expected expired follow-up body to mention expiration, got %q", rec.followupBody())
	}
	if !strings.Contains(rec.followupBody(), `"flags":64`) {
		t.Fatalf("expected expired follow-up to be ephemeral, got %q", rec.followupBody())
	}
}

func TestRegisterCommands_RuntimeModalRejectsExpiredState(t *testing.T) {
	session, rec := newRuntimePanelTestSession(t, 0, 0)
	cm := files.NewMemoryConfigManager()
	if err := cm.LoadConfig(); err != nil {
		t.Fatalf("failed to load config manager: %v", err)
	}

	router := core.NewCommandRouter(session, cm)
	NewRuntimeConfigCommands(cm).RegisterCommands(router)

	st := panelState{
		Mode:  pageMain,
		Group: "ALL",
		Key:   runtimeKeyBotTheme,
		Scope: "global",
	}
	interaction := newRuntimeModalInteraction(st, "nebula")
	components := interaction.ModalSubmitData().Components
	interaction.Data = discordgo.ModalSubmitInteractionData{
		CustomID:   modalEditValueID + stateSep + "invalid",
		Components: components,
	}

	router.HandleInteraction(session, interaction)

	if rec.callbackCount() != 1 {
		t.Fatalf("expected one deferred modal ack, got %d", rec.callbackCount())
	}
	if rec.webhookPatchCount() != 0 {
		t.Fatalf("expected expired modal to avoid editing the panel, got %d patches", rec.webhookPatchCount())
	}
	if rec.followupCount() != 1 {
		t.Fatalf("expected one ephemeral modal denial, got %d", rec.followupCount())
	}
	if !strings.Contains(rec.followupBody(), runtimeConfigInteractionExpiredText) {
		t.Fatalf("expected expired modal follow-up body to mention expiration, got %q", rec.followupBody())
	}
	if !strings.Contains(rec.followupBody(), `"flags":64`) {
		t.Fatalf("expected expired modal follow-up to be ephemeral, got %q", rec.followupBody())
	}
}