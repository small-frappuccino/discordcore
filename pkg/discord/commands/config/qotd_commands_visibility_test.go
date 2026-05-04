package config

import (
	"errors"
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
)

func TestQOTDConfigVisibilityPolicy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		class         qotdConfigVisibilityClass
		wantEphemeral bool
	}{
		{name: "detailed errors stay private", class: qotdConfigVisibilityDetailedError, wantEphemeral: true},
		{name: "short confirmation may be public", class: qotdConfigVisibilityShortConfirmation, wantEphemeral: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := qotdConfigVisibilityIsEphemeral(tt.class); got != tt.wantEphemeral {
				t.Fatalf("qotdConfigVisibilityIsEphemeral(%q) = %v, want %v", tt.class, got, tt.wantEphemeral)
			}
		})
	}
}

func TestQOTDConfigDetailedCommandErrorUsesPrivatePolicy(t *testing.T) {
	t.Parallel()

	err := qotdConfigDetailedCommandError("boom")
	var cmdErr *core.CommandError
	if !errors.As(err, &cmdErr) {
		t.Fatalf("expected command error, got %T", err)
	}
	if !cmdErr.Ephemeral {
		t.Fatalf("expected qotd config detailed errors to be ephemeral")
	}
}