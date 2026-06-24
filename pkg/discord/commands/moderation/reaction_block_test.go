package moderation

import (
	"testing"
)

func TestReactionBlockCommand_Parity(t *testing.T) {
	t.Parallel()
	cmd := NewReactionBlockCommand(nil, nil, nil)

	if cmd.Name() != "reaction_block" {
		t.Errorf("expected reaction_block name")
	}
}
