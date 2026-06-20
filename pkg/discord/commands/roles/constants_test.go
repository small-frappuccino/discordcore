package roles

import "testing"

func TestConstants(t *testing.T) {
	if rolePanelCommandName != "roles" {
		t.Errorf("expected roles")
	}
}
