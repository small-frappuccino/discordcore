package config

import (
	"errors"
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
)

func TestServiceConfigVisibilityPolicy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		class         serviceConfigVisibilityClass
		wantEphemeral bool
	}{
		{name: "setup state stays private", class: serviceConfigVisibilitySetupState, wantEphemeral: true},
		{name: "detailed errors stay private", class: serviceConfigVisibilityDetailedError, wantEphemeral: true},
		{name: "short confirmation may be public", class: serviceConfigVisibilityShortConfirmation, wantEphemeral: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := serviceConfigVisibilityIsEphemeral(tt.class); got != tt.wantEphemeral {
				t.Fatalf("serviceConfigVisibilityIsEphemeral(%q) = %v, want %v", tt.class, got, tt.wantEphemeral)
			}
		})
	}
}

func TestServiceConfigDetailedCommandErrorUsesPrivatePolicy(t *testing.T) {
	t.Parallel()

	err := serviceConfigDetailedCommandError("boom")
	var cmdErr *core.CommandError
	if !errors.As(err, &cmdErr) {
		t.Fatalf("expected command error, got %T", err)
	}
	if !cmdErr.Ephemeral {
		t.Fatalf("expected service config detailed errors to be ephemeral")
	}
}