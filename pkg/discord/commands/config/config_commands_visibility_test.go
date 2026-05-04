package config

import (
	"errors"
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
)

func TestConfigCommandVisibilityPolicy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		class         configCommandVisibilityClass
		wantEphemeral bool
	}{
		{name: "read stays private", class: configCommandVisibilityRead, wantEphemeral: true},
		{name: "list stays private", class: configCommandVisibilityList, wantEphemeral: true},
		{name: "detailed errors stay private", class: configCommandVisibilityDetailedError, wantEphemeral: true},
		{name: "short confirmation may be public", class: configCommandVisibilityShortConfirmation, wantEphemeral: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := configCommandVisibilityIsEphemeral(tt.class); got != tt.wantEphemeral {
				t.Fatalf("configCommandVisibilityIsEphemeral(%q) = %v, want %v", tt.class, got, tt.wantEphemeral)
			}
		})
	}
}

func TestConfigCommandDetailedCommandErrorUsesPrivatePolicy(t *testing.T) {
	t.Parallel()

	err := configCommandDetailedCommandError("boom")
	var cmdErr *core.CommandError
	if !errors.As(err, &cmdErr) {
		t.Fatalf("expected command error, got %T", err)
	}
	if !cmdErr.Ephemeral {
		t.Fatalf("expected config command detailed errors to be ephemeral")
	}
}