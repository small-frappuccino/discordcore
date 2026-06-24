package roles

import "testing"

func TestConstants(t *testing.T) {
	t.Parallel()
	if rolePanelCommandName != "roles" {
		t.Errorf("expected roles")
	}
}
