package partner

import (
	"errors"
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
)

func TestPartnerVisibilityPolicy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		class         partnerVisibilityClass
		wantEphemeral bool
	}{
		{name: "entry mutation stays private", class: partnerVisibilityEntryMutation, wantEphemeral: true},
		{name: "entry read stays private", class: partnerVisibilityEntryRead, wantEphemeral: true},
		{name: "board state stays private", class: partnerVisibilityBoardState, wantEphemeral: true},
		{name: "administrative action stays private", class: partnerVisibilityAdministrativeAction, wantEphemeral: true},
		{name: "detailed error stays private", class: partnerVisibilityDetailedError, wantEphemeral: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := partnerVisibilityIsEphemeral(tt.class); got != tt.wantEphemeral {
				t.Fatalf("partnerVisibilityIsEphemeral(%q) = %v, want %v", tt.class, got, tt.wantEphemeral)
			}
		})
	}
}

func TestPartnerDetailedCommandErrorUsesPrivatePolicy(t *testing.T) {
	t.Parallel()

	err := partnerDetailedCommandError("boom")
	var cmdErr *core.CommandError
	if !errors.As(err, &cmdErr) {
		t.Fatalf("expected command error, got %T", err)
	}
	if !cmdErr.Ephemeral {
		t.Fatalf("expected partner detailed errors to be ephemeral")
	}
}